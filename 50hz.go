package main

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/tarm/serial"
)

// KONFIGURACE
const (
	PAPER_WIDTH_STEPS = 1400    // Tvých 14cm
	Y_SCALE           = 150.0   // Základní měřítko pro exponenciálu
	EXP_FACTOR        = 8.0     // Jak moc to má "vystřelit"
	STEP_SIZE         = 5       // Krok X
	LOOP_DELAY        = 1000    // 1 sekunda mezi měřeními
)

func main() {
	c := &serial.Config{Name: "/dev/ttyUSB0", Baud: 115200, ReadTimeout: time.Second * 5}
	s, err := serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	time.Sleep(3 * time.Second) // Čekání na Arduino
	reader := bufio.NewReader(s)

	s.Write([]byte("P1\n")) // Pero dolů
	waitForOk(reader)

	x := 0
	dx := STEP_SIZE

	for {
		// 1. Získání reálných dat z ČEPS
		freq := getLiveFreq()
		deviation := freq - 50.0
		
		// 2. Exponenciální výpočet Y
		absDev := math.Abs(deviation)
		yVal := (math.Exp(absDev * EXP_FACTOR) - 1.0) * Y_SCALE
		if deviation < 0 {
			yVal = -yVal
		}
		y := int(yVal)

		// 3. Odeslání příkazu
		cmd := fmt.Sprintf("X%dY%d\n", x, y)
		s.Write([]byte(cmd))

		if err := waitForOk(reader); err != nil {
			log.Printf("Plotter chyba: %v", err)
			continue
		}

		// 4. Pohyb tam a zpět (Boustrophedon)
		x += dx
		if x >= PAPER_WIDTH_STEPS {
			x = PAPER_WIDTH_STEPS
			dx = -STEP_SIZE
		} else if x <= 0 {
			x = 0
			dx = STEP_SIZE
		}

		time.Sleep(LOOP_DELAY * time.Millisecond)
	}
}

// Funkce pro fetchování z webu ČEPS pomocí curl
func getLiveFreq() float64 {
	out, err := exec.Command("sh", "-c", "curl -s 'https://www.ceps.cz/cs/frekvence-soustavy' | grep -oE '[0-9]{2}\\.[0-9]{3}' | head -n 1").Output()
	if err != nil {
		return 50.0 // Pokud web nejde, vracíme stabilní střed
	}
	val, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 50.0
	}
	return val
}

// Funkce pro synchronizaci
func waitForOk(r *bufio.Reader) error {
	line, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	if line != "ok\n" && line != "ok\r\n" {
		return fmt.Errorf("nečekaná odpověď: %q", line)
	}
	return nil
}