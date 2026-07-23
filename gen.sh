#!/bin/bash

Y_POS=10
SCALE=1

if [ ! -t 0 ]; then
    TEXT="$(cat)"
else
    TEXT="$1"
fi

[ -z "$TEXT" ] && exit 0

rm -f vystup.svg
go run gen.go -text="$TEXT" -y="$Y_POS" -scale="$SCALE" -out=vystup.svg

if [ -f "vystup.svg" ]; then
    go run ./main.go --file="vystup.svg" --speed=1 --feed=1
fi
