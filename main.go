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
	StepsPerMm   = 204.0
	MotorAX      = -370.0
	MotorBX      = 370.0
	MotorAY      = 520.0
	MotorBY      = 520.0
	gondolaWidth = 60.0
	A4Width      = 200.0
	A4Height     = 250.0
	SafetyMargin = 10.0
)

var (
	transformCmdRe = regexp.MustCompile(`(translate|scale|rotate|matrix)\s*\(([^)]+)\)`)
	numRe          = regexp.MustCompile(`-?\d*\.?\d+(?:[eE][-+]?\d+)?`)
	pathTokenRe    = regexp.MustCompile(`(-?\d*\.?\d+(?:[eE][-+]?\d+)?)|([a-zA-Z])`)
)

var (
	centerLenA          float64
	centerLenB          float64
	currentX            float64 = 0.0
	currentY            float64 = 0.0
	currentPenState     float64 = -1.0
	sigChan             = make(chan os.Signal, 1)
	globalInterrupted bool
)

type Point struct {
	X, Y, Mode float64
}

type Shape []Point

type Matrix struct {
	A, B, C, D, E, F float64
}

func IdentityMatrix() Matrix {
	return Matrix{A: 1, B: 0, C: 0, D: 1, E: 0, F: 0}
}

func (m Matrix) Multiply(o Matrix) Matrix {
	return Matrix{
		A: m.A*o.A + m.C*o.B,
		B: m.B*o.A + m.D*o.B,
		C: m.A*o.C + m.C*o.D,
		D: m.B*o.C + m.D*o.D,
		E: m.A*o.E + m.C*o.F + m.E,
		F: m.B*o.E + m.D*o.F + m.F,
	}
}

func (m Matrix) Apply(x, y float64) (float64, float64) {
	return m.A*x + m.C*y + m.E, m.B*x + m.D*y + m.F
}

type TransformStack []Matrix

func (s *TransformStack) Push(m Matrix) {
	*s = append(*s, m)
}

func (s *TransformStack) Pop() Matrix {
	if len(*s) == 0 {
		return IdentityMatrix()
	}
	top := (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]
	return top
}

func (s TransformStack) Top() Matrix {
	if len(s) == 0 {
		return IdentityMatrix()
	}
	return s[len(s)-1]
}

func parseTransform(attr string) Matrix {
	m := IdentityMatrix()
	if attr == "" {
		return m
	}

	matches := transformCmdRe.FindAllStringSubmatch(attr, -1)
	for _, match := range matches {
		cmd := match[1]
		rawNums := numRe.FindAllString(match[2], -1)
		var nums []float64
		for _, n := range rawNums {
			if v, err := strconv.ParseFloat(n, 64); err == nil {
				nums = append(nums, v)
			}
		}

		switch cmd {
		case "translate":
			tx, ty := 0.0, 0.0
			if len(nums) >= 1 {
				tx = nums[0]
			}
			if len(nums) >= 2 {
				ty = nums[1]
			}
			m = m.Multiply(Matrix{A: 1, B: 0, C: 0, D: 1, E: tx, F: ty})
		case "scale":
			sx, sy := 1.0, 1.0
			if len(nums) >= 1 {
				sx = nums[0]
				sy = nums[0]
			}
			if len(nums) >= 2 {
				sy = nums[1]
			}
			m = m.Multiply(Matrix{A: sx, B: 0, C: 0, D: sy, E: 0, F: 0})
		case "rotate":
			if len(nums) >= 1 {
				rad := nums[0] * math.Pi / 180.0
				cosA := math.Cos(rad)
				sinA := math.Sin(rad)
				rotM := Matrix{A: cosA, B: sinA, C: -sinA, D: cosA, E: 0, F: 0}
				if len(nums) >= 3 {
					cx, cy := nums[1], nums[2]
					t1 := Matrix{A: 1, B: 0, C: 0, D: 1, E: cx, F: cy}
					t2 := Matrix{A: 1, B: 0, C: 0, D: 1, E: -cx, F: -cy}
					m = m.Multiply(t1.Multiply(rotM.Multiply(t2)))
				} else {
					m = m.Multiply(rotM)
				}
			}
		case "matrix":
			if len(nums) == 6 {
				m = m.Multiply(Matrix{A: nums[0], B: nums[1], C: nums[2], D: nums[3], E: nums[4], F: nums[5]})
			}
		}
	}
	return m
}

func parseSVGPath(dAttr string, m Matrix) []Point {
	var points []Point
	matches := pathTokenRe.FindAllString(dAttr, -1)

	var currX, currY, startX, startY float64
	var lastControlX, lastControlY float64
	var lastCmd, currentCmd string
	var coords []float64

	addPoint := func(x, y, mode float64) {
		tx, ty := m.Apply(x, y)
		points = append(points, Point{X: tx, Y: ty, Mode: mode})
	}

	for _, match := range matches {
		if len(match) == 1 && ((match[0] >= 'a' && match[0] <= 'z') || (match[0] >= 'A' && match[0] <= 'Z')) {
			lastCmd = currentCmd
			currentCmd = match
			coords = []float64{}
			if currentCmd == "Z" || currentCmd == "z" {
				currX, currY = startX, startY
				addPoint(currX, currY, 1.0)
			}
			continue
		}

		val, err := strconv.ParseFloat(match, 64)
		if err != nil {
			continue
		}
		coords = append(coords, val)

		switch currentCmd {
		case "M":
			if len(coords) == 2 {
				currX, currY = coords[0], coords[1]
				startX, startY = currX, currY
				addPoint(currX, currY, 0.0)
				coords = []float64{}
				currentCmd = "L"
			}
		case "m":
			if len(coords) == 2 {
				currX += coords[0]
				currY += coords[1]
				startX, startY = currX, currY
				addPoint(currX, currY, 0.0)
				coords = []float64{}
				currentCmd = "l"
			}
		case "L":
			if len(coords) == 2 {
				currX, currY = coords[0], coords[1]
				addPoint(currX, currY, 1.0)
				coords = []float64{}
			}
		case "l":
			if len(coords) == 2 {
				currX += coords[0]
				currY += coords[1]
				addPoint(currX, currY, 1.0)
				coords = []float64{}
			}
		case "H":
			if len(coords) == 1 {
				currX = coords[0]
				addPoint(currX, currY, 1.0)
				coords = []float64{}
			}
		case "h":
			if len(coords) == 1 {
				currX += coords[0]
				addPoint(currX, currY, 1.0)
				coords = []float64{}
			}
		case "V":
			if len(coords) == 1 {
				currY = coords[0]
				addPoint(currX, currY, 1.0)
				coords = []float64{}
			}
		case "v":
			if len(coords) == 1 {
				currY += coords[0]
				addPoint(currX, currY, 1.0)
				coords = []float64{}
			}
		case "C":
			if len(coords) == 6 {
				x1, y1 := coords[0], coords[1]
				x2, y2 := coords[2], coords[3]
				x3, y3 := coords[4], coords[5]
				cStartX, cStartY := currX, currY
				const steps = 10
				for step := 1; step <= steps; step++ {
					t := float64(step) / float64(steps)
					u := 1.0 - t
					px := u*u*u*cStartX + 3*u*u*t*x1 + 3*u*t*t*x2 + t*t*t*x3
					py := u*u*u*cStartY + 3*u*u*t*y1 + 3*u*t*t*y2 + t*t*t*y3
					addPoint(px, py, 1.0)
				}
				lastControlX, lastControlY = x2, y2
				currX, currY = x3, y3
				lastCmd = "C"
				coords = []float64{}
			}
		case "c":
			if len(coords) == 6 {
				x1, y1 := currX+coords[0], currY+coords[1]
				x2, y2 := currX+coords[2], currY+coords[3]
				x3, y3 := currX+coords[4], currY+coords[5]
				cStartX, cStartY := currX, currY
				const steps = 10
				for step := 1; step <= steps; step++ {
					t := float64(step) / float64(steps)
					u := 1.0 - t
					px := u*u*u*cStartX + 3*u*u*t*x1 + 3*u*t*t*x2 + t*t*t*x3
					py := u*u*u*cStartY + 3*u*u*t*y1 + 3*u*t*t*y2 + t*t*t*y3
					addPoint(px, py, 1.0)
				}
				lastControlX, lastControlY = x2, y2
				currX, currY = x3, y3
				lastCmd = "c"
				coords = []float64{}
			}
		case "S":
			if len(coords) == 4 {
				var x1, y1 float64
				if lastCmd == "C" || lastCmd == "c" || lastCmd == "S" || lastCmd == "s" {
					x1 = 2*currX - lastControlX
					y1 = 2*currY - lastControlY
				} else {
					x1, y1 = currX, currY
				}
				x2, y2 := coords[0], coords[1]
				x3, y3 := coords[2], coords[3]
				cStartX, cStartY := currX, currY
				const steps = 10
				for step := 1; step <= steps; step++ {
					t := float64(step) / float64(steps)
					u := 1.0 - t
					px := u*u*u*cStartX + 3*u*u*t*x1 + 3*u*t*t*x2 + t*t*t*x3
					py := u*u*u*cStartY + 3*u*u*t*y1 + 3*u*t*t*y2 + t*t*t*y3
					addPoint(px, py, 1.0)
				}
				lastControlX, lastControlY = x2, y2
				currX, currY = x3, y3
				lastCmd = "S"
				coords = []float64{}
			}
		case "s":
			if len(coords) == 4 {
				var x1, y1 float64
				if lastCmd == "C" || lastCmd == "c" || lastCmd == "S" || lastCmd == "s" {
					x1 = 2*currX - lastControlX
					y1 = 2*currY - lastControlY
				} else {
					x1, y1 = currX, currY
				}
				x2, y2 := currX+coords[0], currY+coords[1]
				x3, y3 := currX+coords[2], currY+coords[3]
				cStartX, cStartY := currX, currY
				const steps = 10
				for step := 1; step <= steps; step++ {
					t := float64(step) / float64(steps)
					u := 1.0 - t
					px := u*u*u*cStartX + 3*u*u*t*x1 + 3*u*t*t*x2 + t*t*t*x3
					py := u*u*u*cStartY + 3*u*u*t*y1 + 3*u*t*t*y2 + t*t*t*y3
					addPoint(px, py, 1.0)
				}
				lastControlX, lastControlY = x2, y2
				currX, currY = x3, y3
				lastCmd = "s"
				coords = []float64{}
			}
		case "Q":
			if len(coords) == 4 {
				x1, y1 := coords[0], coords[1]
				x2, y2 := coords[2], coords[3]
				qStartX, qStartY := currX, currY
				const steps = 10
				for step := 1; step <= steps; step++ {
					t := float64(step) / float64(steps)
					u := 1.0 - t
					px := u*u*qStartX + 2*u*t*x1 + t*t*x2
					py := u*u*qStartY + 2*u*t*y1 + t*t*y2
					addPoint(px, py, 1.0)
				}
				lastControlX, lastControlY = x1, y1
				currX, currY = x2, y2
				lastCmd = "Q"
				coords = []float64{}
			}
		case "q":
			if len(coords) == 4 {
				x1, y1 := currX+coords[0], currY+coords[1]
				x2, y2 := currX+coords[2], currY+coords[3]
				qStartX, qStartY := currX, currY
				const steps = 10
				for step := 1; step <= steps; step++ {
					t := float64(step) / float64(steps)
					u := 1.0 - t
					px := u*u*qStartX + 2*u*t*x1 + t*t*x2
					py := u*u*qStartY + 2*u*t*y1 + t*t*y2
					addPoint(px, py, 1.0)
				}
				lastControlX, lastControlY = x1, y1
				currX, currY = x2, y2
				lastCmd = "q"
				coords = []float64{}
			}
		case "T":
			if len(coords) == 2 {
				var x1, y1 float64
				if lastCmd == "Q" || lastCmd == "q" || lastCmd == "T" || lastCmd == "t" {
					x1 = 2*currX - lastControlX
					y1 = 2*currY - lastControlY
				} else {
					x1, y1 = currX, currY
				}
				x2, y2 := coords[0], coords[1]
				qStartX, qStartY := currX, currY
				const steps = 10
				for step := 1; step <= steps; step++ {
					t := float64(step) / float64(steps)
					u := 1.0 - t
					px := u*u*qStartX + 2*u*t*x1 + t*t*x2
					py := u*u*qStartY + 2*u*t*y1 + t*t*y2
					addPoint(px, py, 1.0)
				}
				lastControlX, lastControlY = x1, y1
				currX, currY = x2, y2
				lastCmd = "T"
				coords = []float64{}
			}
		case "t":
			if len(coords) == 2 {
				var x1, y1 float64
				if lastCmd == "Q" || lastCmd == "q" || lastCmd == "T" || lastCmd == "t" {
					x1 = 2*currX - lastControlX
					y1 = 2*currY - lastControlY
				} else {
					x1, y1 = currX, currY
				}
				x2, y2 := currX+coords[0], currY+coords[1]
				qStartX, qStartY := currX, currY
				const steps = 10
				for step := 1; step <= steps; step++ {
					t := float64(step) / float64(steps)
					u := 1.0 - t
					px := u*u*qStartX + 2*u*t*x1 + t*t*x2
					py := u*u*qStartY + 2*u*t*y1 + t*t*y2
					addPoint(px, py, 1.0)
				}
				lastControlX, lastControlY = x1, y1
				currX, currY = x2, y2
				lastCmd = "t"
				coords = []float64{}
			}
		}
	}
	return points
}

func getAttr(attrs []xml.Attr, name string) string {
	for _, a := range attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func parseAttrFloat(attrs []xml.Attr, name string) float64 {
	valStr := getAttr(attrs, name)
	if valStr == "" {
		return 0.0
	}
	v, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return 0.0
	}
	return v
}

func convertRectToPath(attrs []xml.Attr) string {
	x := parseAttrFloat(attrs, "x")
	y := parseAttrFloat(attrs, "y")
	w := parseAttrFloat(attrs, "width")
	h := parseAttrFloat(attrs, "height")
	if w <= 0 || h <= 0 {
		return ""
	}
	return fmt.Sprintf("M %f %f L %f %f L %f %f L %f %f Z", x, y, x+w, y, x+w, y+h, x, y+h)
}

func convertLineToPath(attrs []xml.Attr) string {
	x1 := parseAttrFloat(attrs, "x1")
	y1 := parseAttrFloat(attrs, "y1")
	x2 := parseAttrFloat(attrs, "x2")
	y2 := parseAttrFloat(attrs, "y2")
	return fmt.Sprintf("M %f %f L %f %f", x1, y1, x2, y2)
}

func convertCircleToPath(attrs []xml.Attr) string {
	cx := parseAttrFloat(attrs, "cx")
	cy := parseAttrFloat(attrs, "cy")
	r := parseAttrFloat(attrs, "r")
	if r <= 0 {
		return ""
	}
	return convertEllipseToPathFromValues(cx, cy, r, r)
}

func convertEllipseToPath(attrs []xml.Attr) string {
	cx := parseAttrFloat(attrs, "cx")
	cy := parseAttrFloat(attrs, "cy")
	rx := parseAttrFloat(attrs, "rx")
	ry := parseAttrFloat(attrs, "ry")
	if rx <= 0 || ry <= 0 {
		return ""
	}
	return convertEllipseToPathFromValues(cx, cy, rx, ry)
}

func convertEllipseToPathFromValues(cx, cy, rx, ry float64) string {
	k := 0.552284749831
	ox := rx * k
	oy := ry * k
	return fmt.Sprintf("M %f %f C %f %f %f %f %f %f C %f %f %f %f %f %f C %f %f %f %f %f %f C %f %f %f %f %f %f Z",
		cx, cy-ry,
		cx+ox, cy-ry, cx+rx, cy-oy, cx+rx, cy,
		cx+rx, cy+oy, cx+ox, cy+ry, cx, cy+ry,
		cx-ox, cy+ry, cx-rx, cy+oy, cx-rx, cy,
		cx-rx, cy-oy, cx-ox, cy-ry, cx, cy-ry)
}

func calculateDistance(x1, y1, x2, y2 float64) float64 {
	return math.Sqrt((x2-x1)*(x2-x1) + (y2-y1)*(y2-y1))
}

func waitForOK(s *serial.Port) {
	buf := make([]byte, 1)
	var line strings.Builder
	for {
		n, err := s.Read(buf)
		if err != nil {
			log.Fatal("Chyba při čtení ze sériového portu:", err)
		}
		if n > 0 {
			line.WriteByte(buf[0])
			if buf[0] == '\n' {
				if strings.Contains(line.String(), "ok") {
					return
				}
				line.Reset()
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

func setPenState(s *serial.Port, mode float64) {
	if mode == currentPenState {
		return
	}
	if mode == 0.0 {
		sendCommand(s, "P0")
	} else if mode == 1.0 {
		sendCommand(s, "P1")
	}
	currentPenState = mode
}

func moveLine(s *serial.Port, targetX, targetY float64) {
	distance := calculateDistance(currentX, currentY, targetX, targetY)
	if distance < 0.05 {
		return
	}

	const segmentSize = 0.5
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
			fmt.Println("\nDetekováno Ctrl+C! Přerušuji...")
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

		cmd := fmt.Sprintf("X%dY%d", int(math.Round(stepsA)), int(math.Round(stepsB)))
		sendCommand(s, cmd)

		currentX = interX
		currentY = interY
	}
}

func main() {
	svgFile := flag.String("file", "", "Cesta k SVG souboru")
	portName := flag.String("port", "/dev/ttyUSB0", "Sériový port")
	autoCenter := flag.Bool("center", false, "Automaticky vycentrovat do středu papíru")
	scaleFlag := flag.Float64("scale", 1.0, "Měřítko (1.0 = 1 px jako 1 mm)")
	fitToA4 := flag.Bool("fit", false, "Automaticky roztáhnout/zmenšit na velikost A4")
	invertY := flag.Bool("inverty", true, "Otočit/invertovat osu Y")
	offsetX := flag.Float64("offx", 0.0, "Dodatečný posun X (mm)")
	offsetY := flag.Float64("offy", 0.0, "Dodatečný posun Y (mm)")
	speed := flag.Int("speed", 300, "Základní rychlost")
	feed := flag.Float64("feed", 1.0, "Násobitel rychlosti")
	perspAngle := flag.Float64("persp", 30.0, "Úhel náklonu desky ve stupních")
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

	var shapes []Shape
	decoder := xml.NewDecoder(file)
	stack := TransformStack{}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal("Chyba XML:", err)
		}

		switch se := token.(type) {
		case xml.StartElement:
			parent := stack.Top()
			local := IdentityMatrix()
			for _, attr := range se.Attr {
				if attr.Name.Local == "transform" {
					local = parseTransform(attr.Value)
					break
				}
			}
			currentTransform := parent.Multiply(local)
			stack.Push(currentTransform)

			var dPath string
			switch se.Name.Local {
			case "path":
				dPath = getAttr(se.Attr, "d")
			case "rect":
				dPath = convertRectToPath(se.Attr)
			case "line":
				dPath = convertLineToPath(se.Attr)
			case "circle":
				dPath = convertCircleToPath(se.Attr)
			case "ellipse":
				dPath = convertEllipseToPath(se.Attr)
			case "polyline", "polygon":
				ptsRaw := numRe.FindAllString(getAttr(se.Attr, "points"), -1)
				if len(ptsRaw) >= 2 {
					dPath = "M " + ptsRaw[0] + " " + ptsRaw[1]
					for i := 2; i < len(ptsRaw)-1; i += 2 {
						dPath += " L " + ptsRaw[i] + " " + ptsRaw[i+1]
					}
					if se.Name.Local == "polygon" {
						dPath += " Z"
					}
				}
			}

			if dPath != "" {
				pts := parseSVGPath(dPath, currentTransform)
				if len(pts) > 0 {
					shapes = append(shapes, pts)
				}
			}

		case xml.EndElement:
			stack.Pop()
		}
	}

	if len(shapes) == 0 {
		log.Fatal("V SVG souboru nebyly nalezeny žádné vykreslitelné tvary.")
	}

	minX, maxX := math.MaxFloat64, -math.MaxFloat64
	minY, maxY := math.MaxFloat64, -math.MaxFloat64

	for _, shape := range shapes {
		for _, pt := range shape {
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

	svgWidth, svgHeight := maxX-minX, maxY-minY
	if svgWidth <= 0 {
		svgWidth = 1.0
	}
	if svgHeight <= 0 {
		svgHeight = 1.0
	}

	targetAreaX, targetAreaY := A4Width-(2*SafetyMargin), A4Height-(2*SafetyMargin)

	scale := *scaleFlag
	if *fitToA4 {
		scale = math.Min(targetAreaX/svgWidth, targetAreaY/svgHeight)
	}

	fmt.Printf("SVG Bounding Box: %.1f x %.1f px/mm\n", svgWidth, svgHeight)
	fmt.Printf("Použité měřítko (scale): %.4f\n", scale)

	var svgCenterX, svgCenterY float64
	if *autoCenter {
		svgCenterX, svgCenterY = minX+(svgWidth/2.0), minY+(svgHeight/2.0)
	}

	c := &serial.Config{Name: *portName, Baud: 115200}
	s, err := serial.OpenPort(c)
	if err != nil {
		log.Fatal("Port nelze otevřít:", err)
	}
	defer s.Close()

	fmt.Println("START: Inicializuji rychlost...")
	time.Sleep(2 * time.Second)

	sendCommand(s, fmt.Sprintf("S%d", *speed))
	sendCommand(s, fmt.Sprintf("F%.2f", *feed))

	yDir := -1.0
	if *invertY {
		yDir = 1.0
	}

	perspFactor := 1.0 / math.Cos(*perspAngle*math.Pi/180.0)

	for _, shape := range shapes {
		if globalInterrupted {
			break
		}
		for _, pt := range shape {
			if globalInterrupted {
				break
			}

			plotterX := (pt.X - svgCenterX) * scale + *offsetX
			plotterY := yDir * ((pt.Y - svgCenterY) * scale * perspFactor) + *offsetY

			setPenState(s, pt.Mode)
			moveLine(s, plotterX, plotterY)
		}
	}

	setPenString := 0.0
	setPenState(s, setPenString)
	moveLine(s, 0.0, 0.0)
	fmt.Println("Hotovo.")
}
