# GoMotor (Drkota)

V-plotter (vertical hanging cable drawing machine) driven from SVG files, generative stroke text, or live 50 Hz electrical grid telemetry. A Linux PC host converts vector paths into dual-stepper inverse kinematics and streams commands over serial (`115200` baud) to an Arduino CNC Shield V3 driving two NEMA 17 steppers and a pen lift servo.

## How it works & Calibrated Geometry

Two stepper motors at fixed anchors `(±370, 500)` mm wind monofilament strings attached to a `45 mm` wide gondola holding the pen.
Physical calibration on the current spools/microstepping yields **`10,000 steps = 52.5 mm`** (`190.476190476 steps/mm`):

```go
const (
    StepsPerMmLeft  = 190.476190476 // 10000 steps = 52.5 mm
    StepsPerMmRight = 190.476190476 // 10000 steps = 52.5 mm
    MotorAX         = -370.0
    MotorBX         = 370.0
    MotorAY         = 500.0
    MotorBY         = 500.0
    gondolaWidth    = 45.0
)

stepsA := (lenA - centerLenA) * StepsPerMmLeft
stepsB := (lenB - centerLenB) * StepsPerMmRight
```

## Repository Structure

| Path | Purpose |
|------|---------|
| `main.go` | Primary SVG -> V-plotter host (paths, nested transforms, Bézier flattening, A4 auto-fit, perspective correction, `minlift`, serial streaming) |
| `firmware/drkota_firmware/drkota_firmware.ino` | Unified Arduino firmware: atomic `PORTD` stepping, direction guard time, selective ramping, and zero-jitter `P` servo control (`Timer1` detached during stepping) |
| `gomotor_driver/gomotor_driver.ino` | Synced copy of the active Arduino driver firmware |
| `firmware/gomotor_driver_w_pen.ino` | Original desktop servo driver variant |
| `firmware/gomotor_driver_desktop.ino` | Original desktop digital pin driver variant |
| `bin/drkota` | Unified CLI controller (`status`, `pen`, `jog`, `zero`, `enable`, `flash`, `plot`) |
| `bin/drkota-flash` | Interactive manual-reset firmware uploader (`uno`, `nano-old`, `nano-new`, `nano168`) |
| `bin/drkota-motor` | Interactive keyboard jogger, motor diagnostic test, and `StepsPerMm` calculator |
| `bin/drkota-sync` | Continuous `STK500` bootloader sync hunter |
| `gen.go` | Text-to-SVG single-line stroke-font generator (supports Czech diacritics) |
| `gen_font.py` | Builds `GoMotor.ttf` TrueType font from the stroke alphabet in `gen.go` |
| `50hz.go` | Live Czech 50 Hz power grid frequency recorder & Boustrophedon plotter |
| `flatenner.go` | Nearest-neighbor SVG path optimizer |
| `main.c` | Pure C implementation of the V-plotter SVG host |
| `sketches/motorizovanaKresba01..05/` | Processing (`.pde`) generative drawing sketches and interactive spring-physics mouse controller |
| `legacy_2023/` | Early 2023 GoMotor host (`main_2023.go`) and vector coordinate dumps (`ignac.txt`, `drawing.txt`, `test.txt`) |
| `MultipleSteppers/` | `AccelStepper`-based Arduino sketch variant |
| `gen.sh`, `genLive.sh`, `vlny.sh` | Shell pipelines for text and generative wave plotting |

## Serial Protocol (`115200` baud)

| Command | Meaning |
|---------|---------|
| `X<stepsA>Y<stepsB>` | Move motors to absolute step coordinates `(stepsA, stepsB)` from origin |
| `P1` | Lift pen UP from paper (`180°` servo angle, then detaches `Timer1`) |
| `P0` | Lower pen DOWN onto paper (`90°` servo angle, then detaches `Timer1`) |
| `P<10..180>` | Set custom pen servo angle directly (e.g. `P140`) for live calibration |
| `H1` / `H0` | Hold servo PWM continuously (`H1`) or auto-detach after move (`H0`, default) |
| `S<delay_us>` | Set cruise step delay in microseconds (default `300`, lower = faster) |
| `F<mult>` | Set feedrate multiplier (default `1.0`) |
| `E1` / `E0` | Enable (`LOW`) or disable (`HIGH`) stepper drivers on Pin 8 |
| `Z0` | Zero software step accumulators (`currentX = 0`, `currentY = 0`) at current physical position |

Every command is acknowledged with `ok\n`.

## Usage

```sh
# 1. Build native Go plotter binary
go build -o gomotor main.go

# 2. Check hardware status & test pen servo via unified CLI
./bin/drkota status
./bin/drkota pen up
./bin/drkota pen down
./bin/drkota zero

# 3. Flash firmware (with manual reset prompt)
./bin/drkota flash uno

# 4. Plot an SVG (centered and fitted to A4)
./gomotor --file ctverec.svg --center --fit

# 5. Render text to SVG and plot
echo "POZDRAV Z VINOHRAD" | ./gen.sh

# 6. Build TrueType font from stroke definitions
python3 gen_font.py
```

## `main.go` Flags

- `--file <path>`: Input SVG file (required)
- `--port <dev>`: Serial device (default `/dev/ttyUSB0`)
- `--center`: Center SVG bounding box at origin `(0, 0)`
- `--fit`: Scale SVG to fit A4 (`210 × 297 mm` minus `10 mm` safety margin)
- `--scale <float>`: Manual scale factor (default `1.0`)
- `--yscale <float>`: Vertical scale correction (default `1.0`)
- `--inverty`: Invert Y axis (default `true`)
- `--offx <mm>`, `--offy <mm>`: Translation offset in millimeters
- `--speed <us>`: Cruise delay in microseconds (default `300`)
- `--feed <mult>`: Feedrate multiplier (default `1.0`)
- `--persp <deg>`: Perspective foreshortening compensation angle in degrees
- `--minlift <mm>`: Skip pen lift on travel moves shorter than threshold (default `2.0 mm`)

## License

Copyleft 🄯 2026 Kryštof Pešek (Kof).
