package main

import (
	"flag"
	"fmt"
	//	"github.com/fatih/color"
	"github.com/tarm/serial"
	"log"
	"math"
	"os"
	//
	// "time"
)

func calculateDistance(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	distance := math.Sqrt(dx*dx + dy*dy)
	return distance
}

func main() {
	c := &serial.Config{Name: "/dev/ttyUSB0", Baud: 115200}

	s, err := serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
	}

	x := flag.Int("x", 0, "goto X position")
	y := flag.Int("y", 0, "goto X position")
	Q := flag.Int("q", 12800, "maxSpeedX")
	W := flag.Int("w", 10000, "speedX")
	E := flag.Int("e", 16000, "accelX")
	A := flag.Int("a", 12800, "maxSpeedY")
	S := flag.Int("s", 10000, "speedY")
	D := flag.Int("d", 16000, "accelY")

	flag.Parse()

	if *Q != 12800 || *W != 10000 || *E != 16000 {
		fmt.Println(fmt.Sprintf("setting up X: Q%08dW%08dE%08d", int(*Q), int(*W), int(*E)))
		_, err := s.Write([]byte(fmt.Sprintf("A%08dS%08dD%08d", int(*Q), int(*W), int(*E))))
		if err != nil {
			log.Fatal(err)
		}
	}

	if *A != 12800 || *S != 10000 || *D != 16000 {

		fmt.Println(fmt.Sprintf("setting up Y: A%08dS%08dD%08d", int(*A), int(*S), int(*D)))
		_, err := s.Write([]byte(fmt.Sprintf("A%08dS%08dD%08d", int(*A), int(*S), int(*D))))
		if err != nil {
			log.Fatal(err)
		}
	}
	xlen, ylen := float64(*x), float64(*y)

	// 19800 = 21cm
	// 1.8 cm = 3600 pulses
	dista := calculateDistance(xlen, ylen, -76000, -90000) //- calculateDistance(0, 0, -76000/2, -90000/2)
	distb := calculateDistance(xlen, ylen, 76000, -90000)  //- calculateDistance(0, 0, 76000/2, -90000/2)

	// set stepper directions
	// quadrant(*deg, s)
	// directions(distb, dista, s)

	X, Y := distb, dista

	fmt.Println(fmt.Sprintf("going: X%08dY%08d", int(X), int(Y)))
	n, err := s.Write([]byte(fmt.Sprintf("X%08dY%08d", int(X), int(Y))))
	if err != nil {
		log.Fatal(err)
	}

	buf := make([]byte, 128)
	n, err = s.Read(buf)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%v", string(buf[:n]))

	if string(buf[:n]) == "ok" {
			//serial.ClosePort
			os.Exit(0)
	}

	
}
