#!/bin/bash

Y_POS=10
SCALE=2.5
Y_STEP=160

process_line() {
    local line="$1"
    [ -z "$line" ] && return

    rm -f vystup.svg
    go run gen.go -text="$line" -y="$Y_POS" -scale="$SCALE" -out=vystup.svg

    if [ -f "vystup.svg" ]; then
        go run ./main.go --file="vystup.svg" --speed=300 --feed=1.0
        Y_POS=$(( Y_POS + Y_STEP ))
    else
        echo "Chyba: Soubor nebyl vygenerován."
        exit 1
    fi
}

if [ ! -t 0 ]; then
    while IFS= read -r line; do
        process_line "$line"
    done
else
    process_line "$1"
fi
