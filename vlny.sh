#!/bin/bash

STATE_FILE=".counter_state"

if [ -f "$STATE_FILE" ]; then
    count=$(cat "$STATE_FILE")
else
    count=1
fi

for file in *.txt; do
    new_name=$(printf "%03d.txt" $count)
    
    echo "Processing $file -> $new_name"
    #mv "$file" "$new_name" # Uncomment this to enable renaming
    
    ((count++))
done

echo "$count" > "$STATE_FILE"

cat ~/2026/07-cerven/motorizovanaKresba01/motorizovanaKresba01.pde | ./gen.sh
#./gomotor --speed 75 --feed 1.0 --file /home/kof/2026/07-cerven/motorizovanaKresba01/01.svg
./gomotor --center --scale=0.5 --file /home/kof/2026/07-cerven/motorizovanaKresba01/01.svg
echo "$count/100" | ./gen.sh

