package main

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Point struct {
	X, Y float64
}

func Distance(p1, p2 Point) float64 {
	return math.Sqrt(math.Pow(p2.X-p1.X, 2) + math.Pow(p2.Y-p1.Y, 2))
}

func FlattenRoute(points []Point) []Point {
	if len(points) == 0 { return points }
	visited := make([]bool, len(points))
	flattened := make([]Point, 0, len(points))
	current := points[0]
	flattened = append(flattened, current)
	visited[0] = true

	for len(flattened) < len(points) {
		nearestIdx := -1
		minDist := math.MaxFloat64
		for i, p := range points {
			if !visited[i] {
				dist := Distance(current, p)
				if dist < minDist {
					minDist = dist
					nearestIdx = i
				}
			}
		}
		if nearestIdx != -1 {
			current = points[nearestIdx]
			flattened = append(flattened, current)
			visited[nearestIdx] = true
		}
	}
	return flattened
}

func main() {
	content, err := os.ReadFile("input.svg")
	if err != nil { panic(err) }

	// NEW: Regex to find the 'd' attribute in a <path>
	re := regexp.MustCompile(`d="([^"]+)"`)
	match := re.FindStringSubmatch(string(content))
	if len(match) < 2 {
		fmt.Println("No path data (d attribute) found.")
		return
	}

	dData := match[1]

	// Extract all numbers using regex
	numRegex := regexp.MustCompile(`[-+]?\d*\.?\d+`)
	matches := numRegex.FindAllString(dData, -1)

	var points []Point
	// Assuming sequence is M x y L x y...
	// We jump by 2, skipping the Command letters (M, L) if they appear
	for i := 0; i < len(matches)-1; i += 2 {
		x, _ := strconv.ParseFloat(matches[i], 64)
		y, _ := strconv.ParseFloat(matches[i+1], 64)
		points = append(points, Point{X: x, Y: y})
	}

	optimized := FlattenRoute(points)

	// Reconstruct the path string
	var newD string
	for i, p := range optimized {
		if i == 0 {
			newD += fmt.Sprintf("M %.1f %.1f ", p.X, p.Y)
		} else {
			newD += fmt.Sprintf("L %.1f %.1f ", p.X, p.Y)
		}
	}

	newSVG := strings.Replace(string(content), dData, newD, 1)
	os.WriteFile("output.svg", []byte(newSVG), 0644)
	fmt.Println("Optimized SVG saved to output.svg")
}
