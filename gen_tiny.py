#!/usr/bin/env python3
"""
gen_tiny.py — Generates ultra-small curved Bezier SVG for Drkota V-plotter:
4x "Tiny tiny" with tiny sine/spiral curves in a palindromic (boustrophedon "tam a zpět")
toolpath:
  Row 1: Left -> Right (TAM)
  Row 2: Right -> Left (ZPĚT - reversed stroke order & reversed Bezier control points)
  Row 3: Left -> Right (TAM)
  Row 4: Right -> Left (ZPĚT - reversed stroke order & reversed Bezier control points)
"""
import math

# Physical dimensions in millimeters (1 SVG unit = 1 mm when plotted with --scale=1.0)
CAP_H = 2.5       # Cap height = 2.5 mm (x-height ~ 1.65 mm)
CHAR_W = 1.45     # Character width = 1.45 mm
CHAR_GAP = 0.55   # Gap between characters = 0.55 mm
SPACE_W = 1.20    # Space width = 1.20 mm
ROW_GAP = 4.20    # Vertical pitch between rows = 4.20 mm


def glyph_strokes(ch):
    """
    Returns a list of strokes for character `ch` in normalized (0..1, 0..1) coordinates.
    Each stroke is a list of cubic Bezier segments:
      [(x0, y0), (c1x, c1y, c2x, c2y, x1, y1), ...]
    """
    if ch == 'T':
        return [
            # Curved top crossbar
            [(0.00, 0.08), (0.30, -0.04, 0.70, 0.14, 1.00, 0.02)],
            # Gently curved stem with foot curl
            [(0.50, 0.04), (0.50, 0.45, 0.46, 0.88, 0.60, 1.00)],
        ]
    elif ch == 't':
        return [
            # Curved crossbar
            [(0.12, 0.36), (0.38, 0.30, 0.66, 0.30, 0.88, 0.36)],
            # Curved stem + bottom hook
            [(0.45, 0.10), (0.45, 0.40, 0.42, 0.75, 0.46, 0.88),
             (0.50, 1.00, 0.70, 1.02, 0.86, 0.90)],
        ]
    elif ch == 'i':
        return [
            # Curved stem from x-height to baseline
            [(0.32, 0.40), (0.48, 0.32, 0.50, 0.50, 0.50, 0.72),
             (0.50, 0.90, 0.55, 1.00, 0.74, 0.94)],
            # Tiny Bezier circle dot above
            [(0.50, 0.08), (0.62, 0.08, 0.62, 0.20, 0.50, 0.20),
             (0.38, 0.20, 0.38, 0.08, 0.50, 0.08)],
        ]
    elif ch == 'n':
        return [
            # Left stem + smooth arch + right stem with foot curl (single continuous curved stroke)
            [(0.10, 0.42), (0.18, 0.36, 0.18, 0.70, 0.16, 1.00),
             (0.16, 0.70, 0.18, 0.48, 0.32, 0.38),
             (0.52, 0.26, 0.84, 0.32, 0.84, 0.58),
             (0.84, 0.80, 0.82, 0.96, 0.96, 0.98)],
        ]
    elif ch == 'y':
        return [
            # Curved u-cup + descender loop (continuous cubic Bezier path)
            [(0.12, 0.36), (0.10, 0.74, 0.28, 0.98, 0.52, 0.96),
             (0.72, 0.94, 0.84, 0.68, 0.86, 0.36),
             (0.86, 0.76, 0.86, 1.18, 0.62, 1.34),
             (0.42, 1.46, 0.14, 1.38, 0.20, 1.20),
             (0.26, 1.06, 0.58, 1.06, 0.90, 1.02)],
        ]
    return []


def wave_stroke(x_start, y_mid, width=2.8, amp=0.65):
    """Generates a tiny 2-lobe cubic Bezier sine wave flourish."""
    half = width / 2.0
    return [[
        (x_start, y_mid),
        (x_start + half * 0.35, y_mid - amp * 1.4,
         x_start + half * 0.65, y_mid - amp * 1.4,
         x_start + half, y_mid),
        (x_start + half * 1.35, y_mid + amp * 1.4,
         x_start + half * 1.65, y_mid + amp * 1.4,
         x_start + width, y_mid),
    ]]


def transform_stroke(stroke, ox, oy, sx, sy):
    """Scales and translates a stroke into millimeter coordinates."""
    x0, y0 = stroke[0]
    out = [(ox + x0 * sx, oy + y0 * sy)]
    for (c1x, c1y, c2x, c2y, x1, y1) in stroke[1:]:
        out.append((
            ox + c1x * sx, oy + c1y * sy,
            ox + c2x * sx, oy + c2y * sy,
            ox + x1 * sx, oy + y1 * sy,
        ))
    return out


def reverse_stroke(stroke):
    """Mathematically reverses a cubic Bezier chain so it plots right-to-left (zpět)."""
    pts = [stroke[0]]
    segs = stroke[1:]
    for seg in segs:
        pts.append((seg[4], seg[5]))
    # Now build reversed segments from last point to first point
    rev = [pts[-1]]
    for i in range(len(segs) - 1, -1, -1):
        c1x, c1y, c2x, c2y, _, _ = segs[i]
        prev_x, prev_y = pts[i]
        rev.append((c2x, c2y, c1x, c1y, prev_x, prev_y))
    return rev


def build_row_strokes(text, row_y):
    """Builds all strokes for one row of text flanked by tiny wave curves."""
    strokes = []
    cursor_x = 0.0

    # Left tiny wave curve
    for ws in wave_stroke(cursor_x, row_y + CAP_H * 0.65, width=2.6, amp=0.6):
        strokes.append(ws)
    cursor_x += 2.6 + 0.8

    for ch in text:
        if ch == ' ':
            cursor_x += SPACE_W
            continue
        for raw_s in glyph_strokes(ch):
            strokes.append(transform_stroke(raw_s, cursor_x, row_y, CHAR_W, CAP_H))
        cursor_x += CHAR_W + CHAR_GAP

    cursor_x += 0.25
    # Right tiny wave curve
    for ws in wave_stroke(cursor_x, row_y + CAP_H * 0.65, width=2.6, amp=0.6):
        strokes.append(ws)

    return strokes, cursor_x + 2.6


def stroke_to_svg_d(stroke):
    x0, y0 = stroke[0]
    parts = [f"M {x0:.3f},{y0:.3f}"]
    for (c1x, c1y, c2x, c2y, x1, y1) in stroke[1:]:
        parts.append(f"C {c1x:.3f},{c1y:.3f} {c2x:.3f},{c2y:.3f} {x1:.3f},{y1:.3f}")
    return " ".join(parts)


def main():
    text = "Tiny tiny"
    all_d = []
    total_w = 0.0

    for row in range(4):
        row_y = row * ROW_GAP
        row_strokes, total_w = build_row_strokes(text, row_y)

        # Palindromic / Boustrophedon ("tam a zpět"):
        # Even rows (0, 2) -> Left to Right (TAM)
        # Odd rows  (1, 3) -> Right to Left (ZPĚT)
        if row % 2 == 1:
            row_strokes = [reverse_stroke(s) for s in reversed(row_strokes)]

        for s in row_strokes:
            all_d.append(stroke_to_svg_d(s))

        # Add a tiny curved turnaround arc at the margin connecting row -> row+1
        if row < 3:
            next_y = (row + 1) * ROW_GAP + CAP_H * 0.65
            curr_y = row_y + CAP_H * 0.65
            mid_y = 0.5 * (curr_y + next_y)
            if row % 2 == 0:
                # Right margin turnaround curve (from row 0->1 or 2->3)
                rx = total_w
                turn = [
                    (rx, curr_y),
                    (rx + 1.2, curr_y, rx + 1.2, next_y, rx, next_y)
                ]
            else:
                # Left margin turnaround curve (from row 1->2)
                lx = 0.0
                turn = [
                    (lx, curr_y),
                    (lx - 1.2, curr_y, lx - 1.2, next_y, lx, next_y)
                ]
            all_d.append(stroke_to_svg_d(turn))

    d_str = " ".join(all_d)
    svg = (
        f'<svg viewBox="-5 -5 {total_w + 10:.1f} {4 * ROW_GAP + 10:.1f}" '
        f'xmlns="http://www.w3.org/2000/svg">\n'
        f'  <path d="{d_str}" stroke="black" stroke-width="0.2" fill="none" />\n'
        f'</svg>\n'
    )
    out_path = "/home/kof/src/gomotor/tiny_tiny.svg"
    with open(out_path, "w") as f:
        f.write(svg)
    print(f"Generated {out_path}: width={total_w:.2f} mm, height={4 * ROW_GAP:.2f} mm, strokes={len(all_d)}")


if __name__ == "__main__":
    main()
