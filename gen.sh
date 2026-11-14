#!/bin/bash
set -e

Y_POS=10
SCALE=0.5

if [ $# -gt 0 ]; then
    TEXT="$*"
elif [ ! -t 0 ]; then
    TEXT="$(cat)"
else
    TEXT=""
fi

[ -z "$TEXT" ] && exit 0

rm -f vystup.svg
if [ ! -x ./gen_bin ] || [ gen.go -nt ./gen_bin ]; then
    go build -o gen_bin gen.go
fi
if [ ! -x ./gomotor ] || [ main.go -nt ./gomotor ]; then
    go build -o gomotor main.go
fi

./gen_bin -text="$TEXT" -y="$Y_POS" -scale="$SCALE" -out=vystup.svg

if [ -f "vystup.svg" ]; then
    ./gomotor --file="vystup.svg" --center --speed=300 --scale=$SCALE
fi
