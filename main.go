package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tarm/serial"
)

const (
	StepsPerMm   = 200.0
	MotorAX      = -370.0
	MotorBX      = 370.0
	MotorAY      = 450.0
	MotorBY      = 450.0
	gondolaWidth = 55.0
	A4Width      = 210.0
	A4Height     = 297.0
	SafetyMargin = 10.0
)

var centerLenA float64
var centerLenB float64
var currentX float64 = 0.0
var currentY float64 = 0.0

var sigChan = make(chan os.Signal, 1)
var globalInterrupted bool

type Point struct {
	X, Y, Mode float64
}

type Shape []Point

func calculateDistance(x1, y1, x2, y2 float64) float64 {
	return math.Sqrt((x2-x1)*(x2-x1) + (y2-y1)*(y2-y1))
}

func waitForOK(s *serial.Port) {
	buf := make([]byte, 128)
	for {
		n, err := s.Read(buf)
		if err != nil {
			log.Fatal("Chyba při čtení ze sériového portu:", err)
		}
		if n > 0 {
			if strings.Contains(string(buf[:n]), "ok\n") {
				return
			}
		}
	}
}

func sendCommand(s *serial.Port, cmd string) {
	_, err := s.Write([]byte(cmd + "\n"))
	if err != nil {
		log.Fatal("Chyba při odesílání příkazu:", err)
	}
	waitForOK(s)
}

func moveLine(s *serial.Port, targetX, targetY float64) {
	distance := calculateDistance(currentX, currentY, targetX, targetY)
	if distance < 0.1 {
		return
	}

	const segmentSize = 2.0
	segments := math.Ceil(distance / segmentSize)
	if segments < 1 {
		segments = 1
	}

	halfGondola := gondolaWidth / 2.0
	startX := currentX
	startY := currentY

	for i := 1; i <= int(segments); i++ {
		select {
		case <-sigChan:
			fmt.Println("\n🛑 Detekováno Ctrl+C! Přerušuji...")
			globalInterrupted = true
			return
		default:
		}

		t := float64(i) / segments
		interX := startX + (targetX-startX)*t
		interY := startY + (targetY-startY)*t

		currentLenA := calculateDistance(interX-halfGondola, interY, MotorAX, MotorAY)
		currentLenB := calculateDistance(interX+halfGondola, interY, MotorBX, MotorBY)

		stepsA := (currentLenA - centerLenA) * StepsPerMm
		stepsB := (currentLenB - centerLenB) * StepsPerMm

		cmd := fmt.Sprintf("X%dY%d\n", int(math.Round(stepsA)), int(math.Round(stepsB)))
		_, err := s.Write([]byte(cmd))
		if err != nil {
			log.Fatal("Chyba zápisu na port:", err)
		}

		waitForOK(s)

		currentX = interX
		currentY = interY
	}
}

func parseSVGPath(dAttr string) []Point {
	var points []Point
	re := regexp.MustCompile(`([MLmlZz])|(-?\d*\.?\d+)`)
	matches := re.FindAllString(dAttr, -1)

	var currentCmd string
	var coords []float64

	for _, match := range matches {
		if match == "M" || match == "L" || match == "m" || match == "l" {
			currentCmd = match
			coords = []float64{}
			continue
		}

		if currentCmd == "M" || currentCmd == "L" || currentCmd == "m" || currentCmd == "l" {
			val, err := strconv.ParseFloat(match, 64)
			if err == nil {
				coords = append(coords, val)
				if len(coords) == 2 {
					mode := 1.0
					if currentCmd == "M" || currentCmd == "m" {
						mode = 0.0
					}
					points = append(points, Point{X: coords[0], Y: coords[1], Mode: mode})
					coords = []float64{}
					if currentCmd == "M" { currentCmd = "L" }
					if currentCmd == "m" { currentCmd = "l" }
				}
			}
		}
	}
	return points
}

func main() {
	svgFile := flag.String("file", "", "Cesta k SVG souboru")
	autoCenter := flag.Bool("center", true, "Automaticky vycentrovat")
	offsetX := flag.Float64("offx", 0.0, "Dodatečný posun X")
	offsetY := flag.Float64("offy", 0.0, "Dodatečný posun Y")
	speed := flag.Int("speed", 300, "Základní rychlost")
	feed := flag.Float64("feed", 1.0, "Násobitel rychlosti")
	flag.Parse()

	if *svgFile == "" {
		log.Fatal("Musíš zadat soubor: --file=vystup.svg")
	}

	signal.Notify(sigChan, os.Interrupt)

	halfGondola := gondolaWidth / 2.0
	centerLenA = calculateDistance(0.0-halfGondola, 0.0, MotorAX, MotorAY)
	centerLenB = calculateDistance(0.0+halfGondola, 0.0, MotorBX, MotorBY)

	file, err := os.Open(*svgFile)
	if err != nil {
		log.Fatal("Nelze otevřít soubor:", err)
	}
	defer file.Close()

	var pathStrings []string
	decoder := xml.NewDecoder(file)
	numNumbers := regexp.MustCompile(`(-?\d*\.?\d+)`) // Definice

	for {
		token, err := decoder.Token()
		if err == io.EOF { break }
		if err != nil { log.Fatal("Chyba XML:", err) }

		switch se := token.(type) {
		case xml.StartElement:
			if se.Name.Local == "path" {
				for _, attr := range se.Attr {
					if attr.Name.Local == "d" { pathStrings = append(pathStrings, attr.Value) }
				}
			}
			// ZDE SE POUŽÍVÁ numNumbers
			if se.Name.Local == "polyline" || se.Name.Local == "polygon" {
				for _, attr := range se.Attr {
					if attr.Name.Local == "points" {
						pts := numNumbers.FindAllString(attr.Value, -1) // <--- POUŽITÍ
						if len(pts) >= 2 {
							dFake := "M " + pts[0] + " " + pts[1]
							for i := 2; i < len(pts)-1; i += 2 {
								dFake += " L " + pts[i] + " " + pts[i+1]
							}
							pathStrings = append(pathStrings, dFake)
						}
					}
				}
			}
		}
	}

	var shapes []Shape
	minX, maxX := math.MaxFloat64, -math.MaxFloat64
	minY, maxY := math.MaxFloat64, -math.MaxFloat64

	for _, pathD := range pathStrings {
		pts := parseSVGPath(pathD)
		if len(pts) == 0 { continue }
		shapes = append(shapes, pts)
		for _, pt := range pts {
			if pt.X < minX { minX = pt.X }
			if pt.X > maxX { maxX = pt.X }
			if pt.Y < minY { minY = pt.Y }
			if pt.Y > maxY { maxY = pt.Y }
		}
	}

	svgWidth, svgHeight := maxX-minX, maxY-minY
	targetAreaX, targetAreaY := A4Width-(2*SafetyMargin), A4Height-(2*SafetyMargin)
	physLimitX, physLimitY := A4Width/2.0, A4Height/2.0
	scale := math.Min(targetAreaX/svgWidth, targetAreaY/svgHeight)

	var svgCenterX, svgCenterY float64
	if *autoCenter {
		svgCenterX, svgCenterY = minX+(svgWidth/2.0), minY+(svgHeight/2.0)
	}

	c := &serial.Config{Name: "/dev/ttyUSB0", Baud: 115200}
	s, err := serial.OpenPort(c)
	if err != nil {
		log.Fatal("Port nelze otevřít:", err)
	}
	defer s.Close()

	fmt.Println("START: Inicializuji rychlost...")
	time.Sleep(2 * time.Second)

	sendCommand(s, fmt.Sprintf("S%d", *speed))
	sendCommand(s, fmt.Sprintf("F%.2f", *feed))

	for _, shape := range shapes {
		if globalInterrupted { break }
		for _, pt := range shape {
			if globalInterrupted { break }

			plotterX := (pt.X-svgCenterX)*scale + *offsetX
			plotterY := ((pt.Y - svgCenterY) * scale) + *offsetY

			if plotterX > physLimitX { plotterX = physLimitX }
			if plotterX < -physLimitX { plotterX = -physLimitX }
			if plotterY > physLimitY { plotterY = physLimitY }
			if plotterY < -physLimitY { plotterY = -physLimitY }

			moveLine(s, plotterX, plotterY)
		}
	}
	moveLine(s, 0.0, 0.0)
	fmt.Println("Hotovo.")
}
