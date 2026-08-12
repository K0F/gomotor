# GoMotor

It is code for V-plotter drawing from SVG files. A PC host converts SVG (or text)
into stepper-motor commands and streams them over a serial link to an
Arduino/ATmega-based driver that runs two **NEMA stepper motors** and a pen servo.

> Status: WIP / personal art project (messy). Commands and protocols are stable
> enough to run, but the code is a collection of standalone programs rather than a
> library.

## How it works

The plotter is a classic **two-motor string plotter**: two steppers at fixed anchor
points (`A` and `B`) at the top of a board each wind/unwind a string that holds a
pen carriage (gondola). The position of the carriage is fully determined by the two
string lengths, so the PC computes the required string lengths for every target
point and converts the length *delta* into a stepper step count.

```
A (-370, 500)                 B (+370, 500)
  \                             /
   \            gondola        /
    \         ( 60 mm wide )  /
     \        (X, Y)         /
      \_____________________/   <- drawing surface
```

### Inverse kinematics

For a target carriage center `(x, y)` with gondola width `W`, the string anchor
points are offset by half the gondola width:

```go
currentLenA := distance(x - W/2, y, MotorAX, MotorAY)
currentLenB := distance(x + W/2, y, MotorBX, MotorBY)
```

Each motor receives a **relative** step count proportional to the change in string
length:

```go
stepsA := (currentLenA - centerLenA) * StepsPerMmLeft
stepsB := (currentLenB - centerLenB) * StepsPerMmRight
```

where `centerLenA`/`centerLenB` are the string lengths at the home position `(0, 0)`.

## Repository layout

| Path | Language | Purpose |
|------|----------|---------|
| `main.go` | Go | Main host application: full SVG parser (paths, transforms, primitives, curves), scaling/fitting, inverse kinematics, serial streaming to the plotter |
| `gen.go` | Go | Stroke-font text-to-SVG generator (produces `vystup.svg`) |
| `50hz.go` | Go | Live plotter recorder of Czech power-grid frequency (ČEPS), scans back and forth |
| `flatenner.go` | Go | Nearest-neighbor route optimizer for SVG paths (reorders points to minimize travel) |
| `main.c` | C | Older, simpler C implementation of the same SVG→plotter host (M/L commands only) |
| `gomotor_driver/gomotor_driver.ino` | Arduino | Main firmware: Bresenham line interpolation, trapezoidal accel/decel ramping, pen + speed control |
| `gomotor_driver2/gomotor_driver2.ino` | Arduino | Incomplete/scratch driver sketch |
| `MultipleSteppers/MultipleSteppers.pde` | Processing | Alternative driver sketch using the `AccelStepper` library |
| `gen.sh`, `genLive.sh` | Bash | Text → SVG → plot pipeline wrappers |
| `vlny.sh` | Bash | Personal "waves" art-drawing driver (renames recordings, draws via `gen.sh`/`gomotor`) |
| `*.svg`, `*.txt` | Data | Example artwork, drawings, and text recordings (mostly gitignored) |

The repository is **not a single Go module build** – each `*.go` file declares its
own `func main()`, and `main.c` is a separate C program, so they are compiled
individually (see [Building](#building)).

## Serial protocol

The PC communicates at **115200 baud** with the Arduino driver. Every command is
acknowledged with a line starting with `ok`.

| Command | Meaning |
|---------|---------|
| `X<steps>Y<steps>` | Move both motors by relative step counts (e.g. `X125Y-42`) |
| `P0` | Pen up (lift servo) |
| `P1` | Pen down (lower to paper) |
| `S<value>` | Set base cruise speed (Arduino: sets `cruiseDelay`, lower = faster) |
| `F<mult>` | Set feedrate multiplier for fine/slow work |

The Arduino firmware (`.ino`) implements:
- Bresenham line interpolation between the two motors (both motors reach their
  target simultaneously).
- Trapezoidal acceleration/deceleration with 10 ramp steps between
  `START_DELAY` (1000) and `cruiseDelay` (300 by default).
- Pen control on `PEN_PIN` and motor enable on `ENABLE_PIN`.

## Firmware (Arduino)

### `gomotor_driver/gomotor_driver.ino` — main driver

Hand-wired stepper driver (no stepper library). Motor direction/step pins and
inversion are configured at the top of the file:

```cpp
X_STEP_PIN = 2    X_DIR_PIN = 5    // X axis (motor A)
Y_STEP_PIN = 3    Y_DIR_PIN = 6    // Y axis (motor B)
ENABLE_PIN = 8    PEN_PIN = 9      // motor enable + pen servo
INVERT_X = false  INVERT_Y = false // reverse direction if needed
```

Behavior:

- **Movement (`X…Y…`)** – `stepPlotter()` moves both axes toward a *relative*
  target using Bresenham interpolation so both motors finish simultaneously, with
  trapezoidal acceleration ramping from `START_DELAY` (1000 µs/step) down to
  `cruiseDelay` (default 300 µs/step) over `RAMP_STEPS` (10) steps. Every step
  pulse is `delayMicroseconds(currentDelay / feedMult)`.
- **Speed (`S<value>`)** – sets `cruiseDelay` (lower = faster).
- **Feedrate (`F<mult>`)** – sets `feedMult`, a global multiplier on the step
  delay for fine/slow work.
- **Pen (`P0` / `P1`)** – drives `PEN_PIN` low/high.
- Every line-terminated command echoes `ok` back so the host blocks until the
  move has completed.

### `MultipleSteppers/MultipleSteppers.pde` — AccelStepper variant

Alternative Processing sketch using the `AccelStepper` library
(`AccelStepper(1, stepPin, dirPin)`), motors on pins (2,5) and (3,6), enable on
pin 8, `setMaxSpeed(8000)`, `setAcceleration(4000)`. Implements the same command
set (`P`, `S`, `F`, `X…Y…`); pen handling is stubbed out.

### `gomotor_driver2/gomotor_driver2.ino` — scratch

Incomplete/truncated draft of the main driver; not usable as-is.

### Motor configuration

The NEMA stepper step rate is baked into the host constants in `main.go`:
`StepsPerMmLeft`/`StepsPerMmRight` = 200.0 steps per mm (a NEMA17 is 200 full
steps per revolution — tune these to match your pulley/driver microstepping).
Wire size and anchoring must let both strings span the full working area from the
anchors at `(±370, 500)` mm.

## Usage

### Host (Go)

```sh
# main.go – plot an SVG file
go build -o gomotor main.go
./gomotor --file art.svg --port /dev/ttyUSB0 --center --fit

# gen.go – turn text into an SVG stroke drawing
go build -o gen gen.go
gen --text "Hello" --out vystup.svg --scale 1.0

# 50hz.go – record grid frequency (streams plotter commands in a loop)
go build -o frekvence 50hz.go
./frekvence
```

### Host (C)

```sh
gcc -o plotter main.c -lm
./plotter -f art.svg -p /dev/ttyUSB0 -s 1.0 -v 300
```

### Shell pipelines

```sh
echo "Hello world" | ./gen.sh          # text -> SVG -> plotter (one line)
printf "Line 1\nLine 2\n" | ./genLive.sh  # plot each line one by one
```

## `main.go` command-line flags

| Flag | Default | Description |
|------|---------|-------------|
| `--file` | *(required)* | Input SVG file |
| `--port` | `/dev/ttyUSB0` | Serial port |
| `--center` | `false` | Center the drawing on the plotter origin |
| `--scale` | `1.0` | Uniform scale factor |
| `--yscale` | `1.0` | Vertical scale correction |
| `--fit` | `false` | Fit drawing to A4 (minus safety margin) |
| `--inverty` | `true` | Mirror the Y axis (plotter is mounted top-down) |
| `--offx` | `0.0` | X offset in mm |
| `--offy` | `0.0` | Y offset in mm |
| `--speed` | `300` | Motor base speed |
| `--feed` | `1.0` | Feedrate multiplier |
| `--persp` | `0.0` | Perspective compensation angle (degrees); stretches Y by `1/cos(angle)` |
| `--minlift` | `2.0` | Do not lift the pen for jumps shorter than this (mm) |

## `gen.go` command-line flags

| Flag | Default | Description |
|------|---------|-------------|
| `--text` | stdin | Text to draw (uppercase, Czech diacritics supported) |
| `--out` | `vystup.svg` | Output SVG file |
| `--x` | `20.0` | Starting X position |
| `--y` | `50.0` | Starting Y position |
| `--scale` | `1.0` | Character scale |
| `--maxwidth` | `750.0` | Max line width before wrapping |

`gen.go` defines a 40×50 stroke font (`alphabet` map) covering A–Z, digits,
punctuation, and Czech diacritics (Á, Č, Ď, É, Ě, Í, Ň, Ó, Ř, Š, Ť, Ú, Ů, Ý, Ž).

## Geometry & configuration constants (`main.go`)

```go
StepsPerMmLeft  = 200.0   // steps per mm for motor A
StepsPerMmRight = 200.0   // steps per mm for motor B
MotorAX = -370.0          // anchor A position (mm)
MotorBX =  370.0          // anchor B position (mm)
MotorAY =  500.0          // anchor A/B height (mm)
MotorBY =  500.0
gondolaWidth = 60.0       // carriage width between string attachment points
A4Width  = 210.0
A4Height = 297.0
SafetyMargin = 10.0       // margin used by --fit
```

These must match your physical machine. On the Arduino side, motor direction
inversion and pin assignments are set in `gomotor_driver.ino`
(`X_STEP_PIN`, `X_DIR_PIN`, `Y_STEP_PIN`, `Y_DIR_PIN`, `ENABLE_PIN`, `PEN_PIN`,
`INVERT_X`, `INVERT_Y`).

## SVG feature support

`main.go` uses the standard `encoding/xml` parser (not an SVG library) and supports:

- `path` elements: `M/m`, `L/l`, `H/h`, `V/v`, `C/c`, `S/s`, `Q/q`, `T/t`, `Z/z`
  (curves are flattened into 10-segment polylines)
- `rect`, `line`, `circle`, `ellipse`, `polyline`, `polygon`
- Nested `transform` attributes: `translate`, `scale`, `rotate`, `matrix`
  (maintained on a transform stack)
- Rects are rendered as **hatched fill** (zigzag lines) rather than outlines

Unsupported: groups as shapes, `<defs>`, clipping, text elements, colors/fill
styles (strokes only), elliptical arcs `A/a`.

## Notes

- The `.gitignore` excludes generated binaries (`gomotor`, `gen`, `frekvence`,
  `flatenner`, `plotter`, `main`), `*.svg`, `*.txt`, `*.ttf`, and a `drawings/`
  folder – generated art is not tracked.
- Scripts and comments mix Czech and English (personal project).
- `flatenner.go` reads `input.svg` → writes `output.svg`, reordering points with a
  nearest-neighbor search to reduce pen travel.
