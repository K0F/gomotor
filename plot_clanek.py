#!/usr/bin/env python3
"""
plot_clanek.py — Renders a Konzdat article (.md / .txt) into a readable A4 multi-line
vector SVG with full Czech diacritics and boustrophedon ("tam a zpět") toolpath
optimization for the Drkota V-plotter.
"""
import sys
import os
import re

# Readable physical typography on A4 (180 mm x 260 mm printable area):
# Character height = 3.2 mm, character width = 2.0 mm, spacing = 0.7 mm (~64 chars/line)
CHAR_W = 2.0
CHAR_H = 3.2
CHAR_GAP = 0.7
LINE_H = 10.0
MAX_LINE_W = 175.0

# Vector stroke font (normalized 0..40 x 0..50 coordinates, matching gen.go + lowercase & punctuation)
ALPHABET = {
    'A': [[(0, 50), (20, 0), (40, 50)], [(10, 25), (30, 25)]],
    'B': [[(0, 50), (0, 0), (30, 0), (40, 12), (30, 25), (0, 25)], [(30, 25), (40, 37), (30, 50), (0, 50)]],
    'C': [[(40, 0), (0, 0), (0, 50), (40, 50)]],
    'D': [[(0, 50), (0, 0), (30, 0), (40, 25), (30, 50), (0, 50)]],
    'E': [[(40, 0), (0, 0), (0, 50), (40, 50)], [(0, 25), (30, 25)]],
    'F': [[(0, 50), (0, 0), (40, 0)], [(0, 25), (30, 25)]],
    'G': [[(40, 0), (0, 0), (0, 50), (40, 50), (40, 25), (20, 25)]],
    'H': [[(0, 0), (0, 50)], [(40, 0), (40, 50)], [(0, 25), (40, 25)]],
    'I': [[(20, 0), (20, 50)], [(8, 0), (32, 0)], [(8, 50), (32, 50)]],
    'J': [[(40, 0), (40, 50), (0, 50), (0, 25)]],
    'K': [[(0, 0), (0, 50)], [(40, 0), (0, 25), (40, 50)]],
    'L': [[(0, 0), (0, 50), (40, 50)]],
    'M': [[(0, 50), (0, 0), (20, 25), (40, 0), (40, 50)]],
    'N': [[(0, 50), (0, 0), (40, 50), (40, 0)]],
    'O': [[(0, 0), (40, 0), (40, 50), (0, 50), (0, 0)]],
    'P': [[(0, 50), (0, 0), (40, 0), (40, 25), (0, 25)]],
    'Q': [[(0, 0), (40, 0), (40, 50), (0, 50), (0, 0)], [(20, 30), (40, 50)]],
    'R': [[(0, 50), (0, 0), (40, 0), (40, 25), (0, 25)], [(20, 25), (40, 50)]],
    'S': [[(40, 0), (0, 0), (0, 25), (40, 25), (40, 50), (0, 50)]],
    'T': [[(0, 0), (40, 0)], [(20, 0), (20, 50)]],
    'U': [[(0, 0), (0, 50), (40, 50), (40, 0)]],
    'V': [[(0, 0), (20, 50), (40, 0)]],
    'W': [[(0, 0), (10, 50), (20, 25), (30, 50), (40, 0)]],
    'X': [[(0, 0), (40, 50)], [(40, 0), (0, 50)]],
    'Y': [[(0, 0), (20, 25), (40, 0)], [(20, 25), (20, 50)]],
    'Z': [[(0, 0), (40, 0), (0, 50), (40, 50)]],
    'Á': [[(0, 50), (20, 0), (40, 50)], [(10, 25), (30, 25)], [(15, -14), (28, -4)]],
    'Č': [[(40, 0), (0, 0), (0, 50), (40, 50)], [(10, -14), (20, -4), (30, -14)]],
    'Ď': [[(0, 50), (0, 0), (30, 0), (40, 25), (30, 50), (0, 50)], [(10, -14), (20, -4), (30, -14)]],
    'É': [[(40, 0), (0, 0), (0, 50), (40, 50)], [(0, 25), (30, 25)], [(15, -14), (28, -4)]],
    'Ě': [[(40, 0), (0, 0), (0, 50), (40, 50)], [(0, 25), (30, 25)], [(10, -14), (20, -4), (30, -14)]],
    'Í': [[(20, 0), (20, 50)], [(15, -14), (28, -4)]],
    'Ň': [[(0, 50), (0, 0), (40, 50), (40, 0)], [(10, -14), (20, -4), (30, -14)]],
    'Ó': [[(0, 0), (40, 0), (40, 50), (0, 50), (0, 0)], [(15, -14), (28, -4)]],
    'Ř': [[(0, 50), (0, 0), (40, 0), (40, 25), (0, 25)], [(20, 25), (40, 50)], [(10, -14), (20, -4), (30, -14)]],
    'Š': [[(40, 0), (0, 0), (0, 25), (40, 25), (40, 50), (0, 50)], [(10, -14), (20, -4), (30, -14)]],
    'Ť': [[(0, 0), (40, 0)], [(20, 0), (20, 50)], [(10, -14), (20, -4), (30, -14)]],
    'Ú': [[(0, 0), (0, 50), (40, 50), (40, 0)], [(15, -14), (28, -4)]],
    'Ů': [[(0, 0), (0, 50), (40, 50), (40, 0)], [(14, -8), (26, -8), (26, -18), (14, -18), (14, -8)]],
    'Ý': [[(0, 0), (20, 25), (40, 0)], [(20, 25), (20, 50)], [(15, -14), (28, -4)]],
    'Ž': [[(0, 0), (40, 0), (0, 50), (40, 50)], [(10, -14), (20, -4), (30, -14)]],
    '0': [[(0, 0), (40, 0), (40, 50), (0, 50), (0, 0)]],
    '1': [[(10, 15), (20, 0), (20, 50)], [(5, 50), (35, 50)]],
    '2': [[(0, 0), (40, 0), (40, 25), (0, 25), (0, 50), (40, 50)]],
    '3': [[(0, 0), (40, 0), (40, 25), (0, 25)], [(40, 25), (40, 50), (0, 50)]],
    '4': [[(0, 0), (0, 25), (40, 25)], [(30, 0), (30, 50)]],
    '5': [[(40, 0), (0, 0), (0, 25), (40, 25), (40, 50), (0, 50)]],
    '6': [[(40, 0), (0, 0), (0, 50), (40, 50), (40, 25), (0, 25)]],
    '7': [[(0, 0), (40, 0), (20, 50)]],
    '8': [[(0, 0), (40, 0), (40, 50), (0, 50), (0, 0)], [(0, 25), (40, 25)]],
    '9': [[(40, 25), (0, 25), (0, 0), (40, 0), (40, 50), (0, 50)]],
    '-': [[(5, 25), (35, 25)]],
    '+': [[(20, 10), (20, 40)], [(5, 25), (35, 25)]],
    '=': [[(5, 18), (35, 18)], [(5, 32), (35, 32)]],
    '.': [[(18, 45), (22, 45), (22, 50), (18, 50), (18, 45)]],
    ',': [[(22, 44), (16, 56)]],
    ':': [[(18, 15), (22, 15)], [(18, 45), (22, 45)]],
    '/': [[(35, 0), (5, 50)]],
    '(': [[(28, 0), (12, 25), (28, 50)]],
    ')': [[(12, 0), (28, 25), (12, 50)]],
    '?': [[(0, 12), (0, 0), (40, 0), (40, 22), (20, 32), (20, 38)], [(18, 46), (22, 46)]],
    '!': [[(20, 0), (20, 35)], [(18, 46), (22, 46)]],
}


def clean_markdown(raw):
    lines = []
    for line in raw.splitlines():
        line = line.strip()
        if not line or line.startswith("![") or line.startswith("$$"):
            continue
        line = re.sub(r'^#+\s*', '', line)
        line = re.sub(r'^>\s*', '', line)
        line = re.sub(r'[*_`~]', '', line)
        line = line.replace("\\textasciitilde", "~").replace("Δ", "d").replace("τ", "t").replace("Π", "P")
        line = re.sub(r'\\[a-zA-Z]+\{([^}]*)\}', r'\1', line)
        line = re.sub(r'\\[a-zA-Z]+', '', line)
        line = re.sub(r'\s+', ' ', line).strip()
        if line:
            lines.append(line)
    return lines


def wrap_lines(paragraphs, max_chars=62):
    wrapped = []
    for p in paragraphs:
        words = p.split()
        cur = ""
        for w in words:
            if not cur:
                cur = w
            elif len(cur) + 1 + len(w) <= max_chars:
                cur += " " + w
            else:
                wrapped.append(cur)
                cur = w
        if cur:
            wrapped.append(cur)
    return wrapped


def render_article_svg(lines, out_svg, max_rows=46):
    sx = CHAR_W / 40.0
    sy = CHAR_H / 50.0
    step_x = CHAR_W + CHAR_GAP

    all_paths = []
    selected = lines[:max_rows]

    for row_idx, line in enumerate(selected):
        row_y = row_idx * LINE_H
        row_strokes = []
        cursor_x = 0.0

        for ch in line.upper():
            if ch == ' ':
                cursor_x += step_x * 0.85
                continue
            strokes = ALPHABET.get(ch)
            if strokes:
                for st in strokes:
                    pts = [(cursor_x + x * sx, row_y + y * sy) for (x, y) in st]
                    row_strokes.append(pts)
            cursor_x += step_x

        # Palindromic / boustrophedon ("tam a zpět") row order:
        # Even rows plot Left->Right, Odd rows plot Right->Left!
        if row_idx % 2 == 1:
            row_strokes = [list(reversed(st)) for st in reversed(row_strokes)]

        for st in row_strokes:
            d = f"M {st[0][0]:.2f},{st[0][1]:.2f} " + " ".join(
                f"L {px:.2f},{py:.2f}" for (px, py) in st[1:]
            )
            all_paths.append(d)

    svg = (
        f'<svg viewBox="-5 -5 {MAX_LINE_W + 10:.1f} {len(selected) * LINE_H + 10:.1f}" '
        f'xmlns="http://www.w3.org/2000/svg">\n'
        f'  <path d="{" ".join(all_paths)}" stroke="black" stroke-width="0.25" fill="none" />\n'
        f'</svg>\n'
    )
    with open(out_svg, "w") as f:
        f.write(svg)
    return len(selected), len(all_paths)


def main():
    src = sys.argv[1] if len(sys.argv) > 1 else "/home/kof/src/konzdat/clanky/17_jak_bych_to_jeste_jen_nevzpomnel/clanek.md"
    out = sys.argv[2] if len(sys.argv) > 2 else "/home/kof/src/gomotor/clanek17.svg"
    max_rows = int(sys.argv[3]) if len(sys.argv) > 3 else 18

    with open(src, "r", encoding="utf-8") as f:
        raw = f.read()

    paras = clean_markdown(raw)
    wrapped = wrap_lines(paras, max_chars=58)
    nrows, nstrokes = render_article_svg(wrapped, out, max_rows=max_rows)
    print(f"Rendered {nrows} readable lines ({nstrokes} strokes) from {os.path.basename(src)} -> {out}")


if __name__ == "__main__":
    main()
