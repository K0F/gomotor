# GoMotor

Code for V-plotter drawing from SVG files. A PC host converts SVG (or text) into
stepper-motor commands and streams them over serial (115200 baud) to an Arduino
driver running two NEMA steppers and a pen servo.

## How it works

Two motors at anchors `(±370, 500)` mm wind strings that hold a 60 mm gondola
with the pen. The PC computes the string lengths for each point and sends the
delta as steps per motor:

```go
stepsA := (lenA - centerLenA) * 200 // steps per mm
stepsB := (lenB - centerLenB) * 200
```

## Files

| Path | Purpose |
|------|---------|
| `main.go` | SVG -> plotter host (paths, transforms, curves, scaling, IK, serial) |
| `gen.go` | Text-to-SVG stroke-font generator |
| `gen_font.py` | Builds `gomotor.ttf` from the hardcoded `alphabet` in `gen.go` |
| `50hz.go` | Live Czech grid-frequency recorder |
| `flatenner.go` | Nearest-neighbor SVG path optimizer |
| `main.c` | Older C version of the SVG host (M/L only) |
| `gomotor_driver/*.ino` | Arduino firmware (Bresenham + acceleration ramps, pen) |
| `MultipleSteppers/*.pde` | AccelStepper-based driver variant |
| `gen.sh`, `genLive.sh` | Text -> SVG -> plot pipelines |

Each `*.go` file has its own `main()`; build them individually.

## Serial protocol

| Command | Meaning |
|---------|---------|
| `X<steps>Y<steps>` | Move both motors (relative) |
| `P0` / `P1` | Pen up / down |
| `S<value>` | Set speed (lower = faster) |
| `F<mult>` | Feedrate multiplier |

Every command is acknowledged with `ok`.

## Usage

```sh
# plot an SVG
go build -o gomotor main.go
./gomotor --file art.svg --center --fit

# text -> SVG -> plotter
echo "Hello" | ./gen.sh

# hardcoded stroke font -> TTF
python3 gen_font.py   # writes gomotor.ttf (pen strokes as filled outlines)

# C host
gcc -o plotter main.c -lm && ./plotter -f art.svg
```

The Gomotor font and `gen_font.py` are Copyright (c) 2026 KOF. All rights reserved.

## `main.go` flags

`--file` (required), `--port` (`/dev/ttyUSB0`), `--center`, `--scale`, `--yscale`,
`--fit` (fit to A4), `--inverty` (default true), `--offx`, `--offy`, `--speed`
(300), `--feed` (1.0), `--persp`, `--minlift` (2.0).

## Geometry

Anchors at `(±370, 500)` mm, gondola width 60 mm, 200 steps/mm, A4 paper with a
10 mm safety margin. Pin assignments and direction inversion are set in
`gomotor_driver.ino`.

SVG support: paths (`M/L/H/V/C/S/Q/T/Z`, curves flattened), `rect` (hatched),
`line`, `circle`, `ellipse`, `polyline`, `polygon`, nested `transform`s.
