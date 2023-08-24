package main

import (
	"flag"
	"fmt"
	"github.com/fatih/color"
	"github.com/tarm/serial"
	"log"
	"math"
	"os"
	"time"
)

func calculateDistance(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	distance := math.Sqrt(dx*dx + dy*dy)
	return distance
}

func polarToCartesian(angleDegrees, magnitude float64) (float64, float64) {
	angleRadians := angleDegrees * (math.Pi) / 180.0
	x := magnitude * math.Cos(angleRadians)
	y := magnitude * math.Sin(angleRadians)
	return x, y
}

func directions(da float64, db float64, s *serial.Port) {
	if da > 0 {
		_, err := s.Write([]byte(fmt.Sprint("x")))
		if err != nil {
			log.Fatal(err)
		}
	}

	if da < 0 {
		_, err := s.Write([]byte(fmt.Sprint("c")))
		if err != nil {
			log.Fatal(err)
		}
	}

	if db > 0 {

		_, err2 := s.Write([]byte(fmt.Sprint("y")))
		if err2 != nil {
			log.Fatal(err2)
		}

	}

	if db < 0 {

		_, err2 := s.Write([]byte(fmt.Sprint("u")))
		if err2 != nil {
			log.Fatal(err2)
		}
	}

}

func quadrant(angleDegrees float64, s *serial.Port) {
	if angleDegrees >= 0 && angleDegrees < 90 {
		_, err := s.Write([]byte(fmt.Sprint("c")))
		if err != nil {
			log.Fatal(err)
		}
		_, err2 := s.Write([]byte(fmt.Sprint("y")))
		if err2 != nil {
			log.Fatal(err2)
		}
	} else if angleDegrees >= 90 && angleDegrees < 180 {
		_, err := s.Write([]byte(fmt.Sprint("x")))
		if err != nil {
			log.Fatal(err)
		}
		_, err2 := s.Write([]byte(fmt.Sprint("y")))
		if err2 != nil {
			log.Fatal(err2)
		}
	} else if angleDegrees >= 180 && angleDegrees < 270 {
		_, err := s.Write([]byte(fmt.Sprint("x")))
		if err != nil {
			log.Fatal(err)
		}
		_, err2 := s.Write([]byte(fmt.Sprint("u")))
		if err2 != nil {
			log.Fatal(err2)
		}
	} else if angleDegrees >= 270 && angleDegrees < 360 {
		_, err := s.Write([]byte(fmt.Sprint("c")))
		if err != nil {
			log.Fatal(err)
		}
		_, err2 := s.Write([]byte(fmt.Sprint("u")))
		if err2 != nil {
			log.Fatal(err2)
		}
	}
}

func main() {
	c := &serial.Config{Name: "/dev/ttyUSB0", Baud: 115200}
	s, err := serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
	}

	deg := flag.Float64("a", 0.0, "angle")
	mag := flag.Float64("m", 50.0, "magnitude")
	count := flag.Int("c", 1, "count")
	bpm := flag.Float64("b", 960.0, "pulses per minute")
	speed := flag.Int("s", 2, "speed")

	flag.Parse()

	beatNo, barNo, totalNo := 0, 0, 0

	start := time.Now()
	dur := time.Duration(60000 / *bpm) * time.Millisecond
	var drift time.Duration

	xlen, ylen := polarToCartesian(float64(*deg), float64(*mag))
	// get x,y coordiantes from polar

	// 19800 = 21cm
	dista := calculateDistance(xlen, ylen, -34650, -44314) - calculateDistance(0, 0, -34650, -44314)
	distb := calculateDistance(xlen, ylen, 34650, -44314) - calculateDistance(0, 0, 34650, -44314)

	// set stepper directions
	// quadrant(*deg, s)
	directions(distb, dista, s)

	X, Y, A, B := *speed, *speed, dista, distb

	// main timed loop
	for {
		t := time.Now()
		elapsed := t.Sub(start)
		drift = time.Duration(elapsed.Milliseconds()%dur.Milliseconds()) * time.Millisecond

		n, err := s.Write([]byte(fmt.Sprintf("X%05dY%05dA%05dB%05d", X, Y, int(math.Abs(A)), int(math.Abs(B)))))
		if err != nil {
			log.Fatal(err)
		}

		if beatNo == 0 {
			color.Green("%04d %04d %08d T %v\n", barNo, beatNo, totalNo, elapsed.Round(time.Duration(1*time.Millisecond)))
		} else {
			fmt.Printf("%04d %04d %08d T %v\n", barNo, beatNo, totalNo, elapsed.Round(time.Duration(1*time.Millisecond)))
		}

		totalNo = totalNo + 1
		beatNo = beatNo + 1

		// calculate drift correction
		ms := time.Duration(dur.Milliseconds()-drift.Milliseconds()) * time.Millisecond
		time.Sleep(ms)

		buf := make([]byte, 128)
		n, err = s.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%v", string(buf[:n]))

		if beatNo >= *count {
			beatNo = 0
			barNo = barNo + 1
			os.Exit(0)
		}
	}

}
