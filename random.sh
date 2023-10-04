#!/bin/bash

min=1
max=360

range=$((max - min + 1))
random_number=$((min + RANDOM % range))
echo "$random_number"

