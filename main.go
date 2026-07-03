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
	// 1. KROKY: Nastaveno na 160.0 (pro 1/32 mikrokrokování a 20-zubové kolečko)
	StepsPerMm = 120.0

	// 2. ROZTEČ MOTORŮ (Změř metrem od levého kolečka k pravému a vyděl dvěma)
	// Pokud je celková rozteč např. 740 mm, poloviny jsou -370 a 370
	MotorAX = -370.0
	MotorBX = 370.0

	// 3. VÝŠKA MOTORŮ (Změř svisle od motorů dolů do PŘESNÉHO STŘEDU tvé A4 plochy)
	// Pozor: Hodnota je teď KLADNÁ (+), protože motory jsou geometricky NAD papírem!
	MotorAY = 420.0
	MotorBY = 420.0

	// 4. ROZTEČ DRÁTKŮ NA VÍČKU (v milimetrech mezi zelenými úchyty)
	gondolaWidth = 55.0

	// Fixní definice formátu A4 na výšku
	A4Width      = 210.0
	A4Height     = 297.0
	SafetyMargin = 10.0 // 10 mm bezpečný vnitřní okraj
)

// Globální proměnné pro uchování výchozích délek řemenů v nulovém bodě (na středu)
var centerLenA float64
var centerLenB float64

var currentX float64 = 0.0
var currentY float64 = 0.0

// 🔄 NOVÉ: Kanál pro odchycení Ctrl+C a globální příznak přerušení
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

func moveLine(s *serial.Port, targetX, targetY float64) {
	distance := calculateDistance(currentX, currentY, targetX, targetY)
	if distance < 0.1 {
		return
	}

	// 📐 JEMNÉ SEGMENTY: 2.0 mm zaručí, že oblouky budou dokonale hladké a detailní
	const segmentSize = 2.0
	segments := math.Ceil(distance / segmentSize)
	if segments < 1 {
		segments = 1
	}

	halfGondola := gondolaWidth / 2.0

	// Fixace počátečního bodu, protože currentX/Y budeme inkrementovat průběžně
	startX := currentX
	startY := currentY

	for i := 1; i <= int(segments); i++ {
		// 🔄 NOVÉ: Nekontextová (neblokující) kontrola, zda uživatel nestiskl Ctrl+C
		select {
		case <-sigChan:
			fmt.Println("\n🛑 Detekováno Ctrl+C! Přerušuji aktuální čáru a připravuji návrat...")
			globalInterrupted = true
			return // Okamžitě vyskočíme z této dráhy
		default:
			// Žádný signál nepřišel, pokračujeme v tisku segmentu
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

		// 🛑 HARDWAROVÝ HANDSHAKE: Go čeká, až Arduino dokončí krok.
		waitForOK(s)

		// 🔄 OPRAVA: Ukládáme reálnou pozici po každém dokončeném segmentu
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
				if currentCmd == "M" {
					currentCmd = "L"
				}
				if currentCmd == "m" {
					currentCmd = "l"
				}
			}
		}
	}
	return points
}

func main() {
	svgFile := flag.String("file", "", "Cesta k SVG souboru")
	autoCenter := flag.Bool("center", true, "Automaticky vycentrovat obrázek na střed [0,0]")
	offsetX := flag.Float64("offx", 0.0, "Dodatečný posun X v mm")
	offsetY := flag.Float64("offy", 0.0, "Dodatečný posun Y v mm")
	flag.Parse()

	if *svgFile == "" {
		log.Fatal("Musíš zadat SVG soubor pomocí: --file=vystup.svg")
	}

	// 🔄 NOVÉ: Registrace kanálu pro odchycení systémového přerušení Ctrl+C
	signal.Notify(sigChan, os.Interrupt)

	// KALIBRACE STŘEDU: Výpočet základní délky řemenů, když víčko visí přesně na [0,0]
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
	numNumbers := regexp.MustCompile(`(-?\d*\.?\d+)`)

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal("Chyba čtení XML:", err)
		}

		switch se := token.(type) {
		case xml.StartElement:
			if se.Name.Local == "path" {
				for _, attr := range se.Attr {
					if attr.Name.Local == "d" {
						pathStrings = append(pathStrings, attr.Value)
					}
				}
			}
			if se.Name.Local == "polyline" || se.Name.Local == "polygon" {
				for _, attr := range se.Attr {
					if attr.Name.Local == "points" {
						pts := numNumbers.FindAllString(attr.Value, -1)
						if len(pts) >= 2 {
							dFake := "M " + pts[0] + " " + pts[1]
							for i := 2; i < len(pts)-1; i += 2 {
								dFake += " L " + pts[i] + " " + pts[i+1]
							}
							if se.Name.Local == "polygon" {
								dFake += " Z"
							}
							pathStrings = append(pathStrings, dFake)
						}
					}
				}
			}
			if se.Name.Local == "line" {
				var x1, y1, x2, y2 string
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "x1":
						x1 = attr.Value
					case "y1":
						y1 = attr.Value
					case "x2":
						x2 = attr.Value
					case "y2":
						y2 = attr.Value
					}
				}
				if x1 != "" && y1 != "" && x2 != "" && y2 != "" {
					pathStrings = append(pathStrings, fmt.Sprintf("M %s %s L %s %s", x1, y1, x2, y2))
				}
			}
		}
	}

	if len(pathStrings) == 0 {
		fmt.Println("V souboru nebyly nalezeny žádné čáry.")
		return
	}

	var shapes []Shape
	minX, maxX := math.MaxFloat64, -math.MaxFloat64
	minY, maxY := math.MaxFloat64, -math.MaxFloat64

	for _, pathD := range pathStrings {
		pts := parseSVGPath(pathD)
		if len(pts) == 0 {
			continue
		}

		shapes = append(shapes, pts)

		for _, pt := range pts {
			if pt.X < minX {
				minX = pt.X
			}
			if pt.X > maxX {
				maxX = pt.X
			}
			if pt.Y < minY {
				minY = pt.Y
			}
			if pt.Y > maxY {
				maxY = pt.Y
			}
		}
	}

	svgWidth := maxX - minX
	svgHeight := maxY - minY

	targetAreaX := A4Width - (2 * SafetyMargin)
	targetAreaY := A4Height - (2 * SafetyMargin)
	physLimitX := A4Width / 2.0
	physLimitY := A4Height / 2.0

	scaleX := targetAreaX / svgWidth
	scaleY := targetAreaY / svgHeight
	scale := math.Min(scaleX, scaleY)

	var svgCenterX, svgCenterY float64
	if *autoCenter {
		svgCenterX = minX + (svgWidth / 2.0)
		svgCenterY = minY + (svgHeight / 2.0)
	}

	c := &serial.Config{Name: "/dev/ttyUSB0", Baud: 115200}
	s, err := serial.OpenPort(c)
	if err != nil {
		log.Fatal("Nepodařilo se otevřít port:", err)
	}
	defer s.Close()

	fmt.Println("START: Gondola musí viset přesně na STŘEDU papíru. Spouštím tisk...")
	time.Sleep(3 * time.Second)

	for idx, shape := range shapes {
		// 🔄 NOVÉ: Kontrola přerušení před začátkem nového objektu
		if globalInterrupted {
			break
		}

		fmt.Printf("  -> Kreslím objekt %d/%d\n", idx+1, len(shapes))
		for _, pt := range shape {
			// 🔄 NOVÉ: Kontrola přerušení před bodem uvnitř objektu
			if globalInterrupted {
				break
			}

			// Přepočet z SVG souřadnic do reálného světa středového plotteru
			var plotterX, plotterY float64
			if *autoCenter {
				plotterX = (pt.X - svgCenterX) * scale + *offsetX
				plotterY = ((pt.Y - svgCenterY) * scale) + *offsetY
			} else {
				plotterX = pt.X * scale + *offsetX
				plotterY = (pt.Y * scale) + *offsetY
			}

			// Bezpečnostní hardwarový ořez ohraničení A4 na výšku
			if plotterX > physLimitX {
				plotterX = physLimitX
			}
			if plotterX < -physLimitX {
				plotterX = -physLimitX
			}
			if plotterY > physLimitY {
				plotterY = physLimitY
			}
			if plotterY < -physLimitY {
				plotterY = -physLimitY
			}

			moveLine(s, plotterX, plotterY)
		}
	}

	// 🔄 NOVÉ: Rozcestník po ukončení cyklu
	if globalInterrupted {
		fmt.Println("\n⚠️ Tisk byl přerušen! Provádím nouzový návrat gondoly na střed [0,0]...")
		
		// Vypneme odchytávání signálu – pokud uživatel stiskne Ctrl+C PODRUHÉ, 
		// program se natvrdo okamžitě ukončí (bezpečnostní pojistka proti zaseknutí).
		signal.Stop(sigChan) 
		
		moveLine(s, 0.0, 0.0)
		fmt.Println("Gondola je bezpečně zpět na nule. Vypínám.")
		return
	}

	// Standardní úspěšný konec tisku
	fmt.Println("Dokončeno, vracím víčko zpátky na střed...")
	moveLine(s, 0.0, 0.0)
}
