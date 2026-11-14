#!/usr/bin/env python3
"""Convert the hardcoded stroke alphabet in gen.go into a TrueType font.

Copyright (c) 2026 KOF. All rights reserved.

TTF glyphs are filled outlines, so every pen stroke is rendered as a filled
outline: one thin rectangle per segment plus a round disc at every vertex,
which under the nonzero winding rule unions into the classic "pen footprint"
(round caps and joins).

The alphabet map in gen.go is the single source of truth and is parsed here,
never duplicated.
"""

import argparse
import math
import os
import re

from fontTools.fontBuilder import FontBuilder
from fontTools.pens.ttGlyphPen import TTGlyphPen
from fontTools.ttLib import TTFont

GO_MAP_RE = re.compile(r"var\s+alphabet\s*=\s*map\[rune\]\[\]\[\]Point\s*\{(.*)\}",
                       re.S)
ENTRY_RE = re.compile(r"'([^']+)'\s*:\s*(\{\{\{.*?\}\}\})", re.S)
NUM_RE = re.compile(r"-?\d+(?:\.\d+)?")


def parse_go_value(s):
    """Recursive-descent parser for Go 'map[rune][][]Point' values.

    Handles the rigid shape: {{{x,y},{x,y},...},{{x,y},...},...}
    """
    i = 0

    def skip():
        nonlocal i
        while i < len(s) and s[i].isspace():
            i += 1

    def number():
        nonlocal i
        skip()
        m = NUM_RE.match(s, i)
        if not m:
            raise ValueError(f"expected number at {i}: {s[i:i+10]!r}")
        i = m.end()
        return float(m.group())

    def item():
        nonlocal i
        skip()
        if s[i] != "{":
            raise ValueError(f"expected '{{' at {i}: {s[i:i+10]!r}")
        i += 1
        skip()
        if i < len(s) and s[i] == "{":
            items = []
            while True:
                skip()
                if s[i] == "}":
                    i += 1
                    return items
                items.append(item())
                skip()
                if s[i] == ",":
                    i += 1
        else:
            x = number()
            skip()
            if s[i] == ",":
                i += 1
            y = number()
            skip()
            if s[i] != "}":
                raise ValueError(f"expected '}}' at {i}: {s[i:i+10]!r}")
            i += 1
            return (x, y)

    return item()


def load_alphabet(gen_path):
    with open(gen_path, encoding="utf-8") as f:
        src = f.read()
    m = GO_MAP_RE.search(src)
    if not m:
        raise SystemExit(f"could not find alphabet map in {gen_path}")
    alphabet = {}
    for key, value in ENTRY_RE.findall(m.group(1)):
        strokes = parse_go_value(value)
        alphabet[key] = [[(x, y) for x, y in stroke] for stroke in strokes]
    return alphabet


def quad(p0, c, p1):
    return [("curve", p0, c, p1)]


def circle_contour(cx, cy, r):
    """Counterclockwise closed circle as four quadratic quarter arcs."""
    if r <= 0:
        return []
    m = r * (2 * math.sqrt(2) - 1) / 2  # optimal quadratic quarter-arc control
    segs = [
        (r, 0, m, m, 0, r),
        (0, r, -m, m, -r, 0),
        (-r, 0, -m, -m, 0, -r),
        (0, -r, m, -m, r, 0),
    ]
    pts = []
    for a in segs:
        x0, y0, cx0, cy0, x1, y1 = a
        pts.append(("line", (cx + x0, cy + y0)))
        pts.append(("qcurve", (cx + cx0, cy + cy0), (cx + x1, cy + y1)))
    return pts


def segment_quad(p0, p1, r):
    """Counterclockwise rectangle covering a segment of half-width r."""
    x0, y0 = p0
    x1, y1 = p1
    dx, dy = x1 - x0, y1 - y0
    length = math.hypot(dx, dy)
    if length <= 0:
        return []
    dx /= length
    dy /= length
    nx, ny = -dy, dx
    ox, oy = nx * r, ny * r
    return [
        ("line", (x0 - ox, y0 - oy)),
        ("line", (x1 - ox, y1 - oy)),
        ("line", (x1 + ox, y1 + oy)),
        ("line", (x0 + ox, y0 + oy)),
    ]


def stroke_contours(points, r):
    """Pen footprint of one polyline: per-segment quads plus vertex discs."""
    contours = []
    for i in range(len(points) - 1):
        quad = segment_quad(points[i], points[i + 1], r)
        if quad:
            contours.append(quad)
    for x, y in points:
        disc = circle_contour(x, y, r)
        if disc:
            contours.append(disc)
    return contours


def draw_pen_contours(pen, contours):
    for contour in contours:
        if not contour:
            continue
        pen.moveTo(contour[0][1])
        for op in contour[1:]:
            if op[0] == "line":
                pen.lineTo(op[1])
            elif op[0] == "qcurve":
                pen.qCurveTo(op[1], op[2])
        pen.closePath()


def compute_lsb(contours):
    xs = [p[0] for c in contours for op in c for p in op[1:]]
    return int(min(xs)) if xs else 0


def main():
    here = os.path.dirname(os.path.abspath(__file__))
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--gen", default=os.path.join(here, "gen.go"))
    ap.add_argument("--out", default=os.path.join(here, "gomotor.ttf"))
    ap.add_argument("--cap", type=float, default=700.0,
                    help="cap height in font units (0.7 em)")
    ap.add_argument("--ascent", type=float, default=840.0,
                    help="line ascent in font units (covers accents + pen)")
    ap.add_argument("--descent", type=float, default=200.0,
                    help="line descent in font units (covers comma + pen)")
    ap.add_argument("--width", type=float, default=7.0, help="pen width in %% of cap height")
    ap.add_argument("--advance", type=float, default=None,
                    help="uniform advance in font units (default: == cap height, as in gen.go)")
    args = ap.parse_args()

    alphabet = load_alphabet(args.gen)
    if not alphabet:
        raise SystemExit("alphabet map is empty")

    cap_units = 50.0   # cap height in gen.go coordinate space

    ys = [y for strokes in alphabet.values() for s in strokes for _, y in s]
    min_y = min(ys)  # highest point (negative = above cap top)
    max_y = max(ys)  # lowest point (55 for the comma descender)

    cap = args.cap
    advance = args.advance if args.advance is not None else cap
    pen_r = (args.width / 100.0) * cap / 2.0

    # Proportional vertical layout. The body (baseline..cap top) maps to the
    # full cap height; accents and the comma descender are compressed into the
    # ascent/descent margins so line height stays proportional to the cap.
    body_scale = cap / cap_units
    accent_span = max(args.ascent - cap - pen_r, 0.0)
    desc_span = max(args.descent - pen_r, 0.0)
    accent_units = 0.0 - min_y
    desc_units = max_y - cap_units

    def map_y(y):
        if y <= 0:
            t = (0.0 - y) / accent_units
            return cap + accent_span * t
        if y <= cap_units:
            return body_scale * (cap_units - y)
        t = (y - cap_units) / desc_units
        return -(desc_span * t)

    def to_font(x, y):
        return (x * body_scale, map_y(y))

    base = {cp: glyph for cp, glyph in alphabet.items()}
    base.update({cp.lower(): glyph for cp, glyph in alphabet.items() if cp.isalpha() and cp.isupper()})

    # Build glyphs.
    glyphs = {}
    metrics = {}
    glyph_order = [".notdef", "space"]

    def build_glyph(points_list):
        pen = TTGlyphPen(None)
        for stroke in points_list:
            pts = [to_font(x, y) for x, y in stroke]
            if len(pts) >= 2:
                draw_pen_contours(pen, stroke_contours(pts, pen_r))
        return pen.glyph()

    for cp in sorted(base):
        name = f"uni{ord(cp):04X}"
        contours = []
        for stroke in base[cp]:
            pts = [to_font(x, y) for x, y in stroke]
            if len(pts) >= 2:
                contours.extend(stroke_contours(pts, pen_r))
        glyph_order.append(name)
        glyphs[name] = build_glyph(base[cp])
        metrics[name] = (int(round(advance)), compute_lsb(contours))

    notdef_pen = TTGlyphPen(None)
    notdef_pen.moveTo((80, 0))
    notdef_pen.lineTo((620, 0))
    notdef_pen.lineTo((620, 700))
    notdef_pen.lineTo((80, 700))
    notdef_pen.closePath()
    glyphs[".notdef"] = notdef_pen.glyph()
    metrics[".notdef"] = (int(round(advance)), 80)
    glyphs["space"] = TTGlyphPen(None).glyph()
    metrics["space"] = (int(round(advance)), 0)

    cmap = {ord(cp): f"uni{ord(cp):04X}" for cp in base}
    cmap[32] = "space"

    # Common dash variants render as the ASCII hyphen glyph.
    for alias, target in ((0x2013, 0x002D), (0x2014, 0x002D)):
        cmap[alias] = f"uni{target:04X}"

    ascent = int(round(args.ascent))
    descent = -int(round(args.descent))

    fb = FontBuilder(1000, isTTF=True)
    fb.setupGlyphOrder(glyph_order)
    fb.setupCharacterMap(cmap)
    fb.setupGlyf(glyphs)
    fb.setupHorizontalMetrics(metrics)
    fb.setupHorizontalHeader(ascent=ascent, descent=descent)
    fb.setupNameTable({
        "copyright": "Copyleft 🄯 2026 Kof",
        "familyName": "Gomotor",
        "styleName": "Regular",
        "uniqueFontIdentifier": "Gomotor Regular 0.1",
        "fullName": "Gomotor Regular",
        "version": "Version 0.1",
        "psName": "Gomotor-Regular",
    })
    fb.setupOS2(
        sTypoAscender=ascent,
        sTypoDescender=descent,
        sTypoLineGap=0,
        usWinAscent=max(ascent, 0),
        usWinDescent=max(-descent, 0),
        sxHeight=int(round(cap)),
        sCapHeight=int(round(cap)),
    )
    fb.setupPost()
    fb.save(args.out)

    # Verify: reload and report.
    font = TTFont(args.out)
    cm = font.getBestCmap()
    missing = [cp for cp in base if ord(cp) not in cm]
    if missing:
        print("WARNING: missing cmap entries:", [hex(ord(c)) for c in missing])
    glyphs = font.getGlyphSet()
    count = sum(1 for name in glyph_order if name not in ("space",) and name in glyphs)
    print(f"wrote {args.out}")
    print(f"glyphs: {len(glyph_order)} ({len(base)} mapped codepoints + .notdef + space)")
    print(f"unitsPerEm: 1000  capHeight: {int(round(cap))}  advance: {int(round(advance))}  "
          f"stroke width: {int(round(2 * pen_r))}  ascent: {ascent}  descent: {descent}")


if __name__ == "__main__":
    main()
