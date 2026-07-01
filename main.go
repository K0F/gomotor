package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tarm/serial"
)

// Geometrická konfigurace plotteru (v mm)
const (
	StepsPerMm = 200.0  // 3600 pulsů / 18 mm
	MotorAX    = -380.0 // Levý motor X
	MotorAY    = -450.0 // Levý motor Y
	MotorBX    = 380.0  // Pravý motor X
	MotorBY    = -450.0 // Pravý motor Y
)

var currentX float64 = 0.0
var currentY float64 = 0.0

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

func moveLine(s *serial.Port, targetX, targetY float64) {
	distance := calculateDistance(currentX, currentY, targetX, targetY)
	if distance < 0.1 {
		return
	}

	segments := math.Ceil(distance)

	for i := 1; i <= int(segments); i++ {
		t := float64(i) / segments
		interX := currentX + (targetX-currentX)*t
		interY := currentY + (targetY-currentY)*t

		distA := calculateDistance(interX, interY, MotorAX, MotorAY) * StepsPerMm
		distB := calculateDistance(interX, interY, MotorBX, MotorBY) * StepsPerMm

		cmd := fmt.Sprintf("X%dY%d\n", int(distB), int(distA))
		_, err := s.Write([]byte(cmd))
		if err != nil {
			log.Fatal("Chyba zápisu na port:", err)
		}

		waitForOK(s)
	}

	currentX = targetX
	currentY = targetY
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
			continue
		}
		
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
	return points
}

func main() {
	svgFile := flag.String("file", "", "Cesta k SVG souboru")
	targetWidth := flag.Float64("width", 210.0, "Požadovaná fyzická šířka obrázku v mm (A4 = 210)")
	autoCenter := flag.Bool("center", true, "Automaticky vycentrovat obrázek na střed [0,0]")
	offsetX := flag.Float64("offx", 0.0, "Dodatečný posun X v mm")
	offsetY := flag.Float64("offy", 0.0, "Dodatečný posun Y v mm")
	flag.Parse()

	if *svgFile == "" {
		log.Fatal("Musíš zadat SVG soubor pomocí: --file=vystup.svg")
	}

	file, err := os.Open(*svgFile)
	if err != nil {
		log.Fatal("Nelze otevřít soubor:", err)
	}
	defer file.Close()

	var pathStrings []string
	decoder := xml.NewDecoder(file)
	numNumbers := regexp.MustCompile(`(-?\d*\.?\d+)`)

	for {
		token, err := decoder.Token()
		if err == io.EOF { break }
		if err != nil { log.Fatal("Chyba čtení XML:", err) }

		switch se := token.(type) {
		case xml.StartElement:
			if se.Name.Local == "path" {
				for _, attr := range se.Attr {
					if attr.Name.Local == "d" { pathStrings = append(pathStrings, attr.Value) }
				}
			}
			if se.Name.Local == "polyline" || se.Name.Local == "polygon" {
				for _, attr := range se.Attr {
					if attr.Name.Local == "points" {
						pts := numNumbers.FindAllString(attr.Value, -1)
						if len(pts) >= 2 {
							dFake := "M " + pts[0] + " " + pts[1]
							for i := 2; i < len(pts)-1; i += 2 { dFake += " L " + pts[i] + " " + pts[i+1] }
							if se.Name.Local == "polygon" { dFake += " Z" }
							pathStrings = append(pathStrings, dFake)
						}
					}
				}
			}
			if se.Name.Local == "line" {
				var x1, y1, x2, y2 string
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "x1": x1 = attr.Value
					case "y1": y1 = attr.Value
					case "x2": x2 = attr.Value
					case "y2": y2 = attr.Value
					}
				}
				if x1 != "" && y1 != "" && x2 != "" && y2 != "" {
					pathStrings = append(pathStrings, fmt.Sprintf("M %s %s L %s %s", x1, y1, x2, y2))
				}
			}
		}
	}

	if len(pathStrings) == 0 {
		fmt.Println("⚠️  V souboru nebyly nalezeny žádné čáry.")
		return
	}

	// První průchod: Načtení bodů a zjištění obalového boxu (Bounding Box)
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

	svgWidth := maxX - minX
	svgHeight := maxY - minY

	// Výpočet automatického měřítka
	scale := 1.0
	if *targetWidth > 0 && svgWidth > 0 {
		scale = *targetWidth / svgWidth
	}

	// Výpočet středů pro automatické centrování
	var finalOffsetX, finalOffsetY float64
	if *autoCenter {
		svgCenterX := minX + (svgWidth / 2.0)
		svgCenterY := minY + (svgHeight / 2.0)
		finalOffsetX = -svgCenterX*scale + *offsetX
		finalOffsetY = -svgCenterY*scale + *offsetY
	} else {
		finalOffsetX = *offsetX
		finalOffsetY = *offsetY
	}

	fmt.Printf("📊 Statistiky SVG:\n")
	fmt.Printf("   - Původní velikost v souboru: %.1fx%.1f jednotek\n", svgWidth, svgHeight)
	fmt.Printf("   - Vypočtené měřítko pro A4:   %.4f\n", scale)
	fmt.Printf("   - Výsledná fyzická velikost:  %.1fx%.1f mm\n", svgWidth*scale, svgHeight*scale)

	// Připojení k Arduinu
	c := &serial.Config{Name: "/dev/ttyUSB0", Baud: 115200}
	s, err := serial.OpenPort(c)
	if err != nil { log.Fatal("Nepodařilo se otevřít port:", err) }
	defer s.Close()

	fmt.Println("Inicializace... Srovnej tužku přesně do STŘEDU kreslicí plochy [0,0]!")
	time.Sleep(2 * time.Second)
	fmt.Println("🚀 Startuji přesné kreslení...")

	for idx, shape := range shapes {
		fmt.Printf("  -> Kreslím objekt %d/%d (body: %d)\n", idx+1, len(shapes), len(shape))
		for _, pt := range shape {
			targetX := (pt.X * scale) + finalOffsetX
			targetY := (pt.Y * scale) + finalOffsetY
			moveLine(s, targetX, targetY)
		}
	}
	fmt.Println("🏁 Hotovo! Obrázek v přesném měřítku dokončen.")
}