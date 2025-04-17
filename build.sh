#!/bin/bash

set -euo pipefail

VERSION="0.0.1"
OS="linux"
GC_FLAGS=""
LD_FLAGS="-s -w" # strip symbol tables
# OUTPUT=./bin/headpat-counter-$VERSION-$OS
OUTPUT=./bin/server

# usage: go build [-o output] [build flags] [packages]
go build -o $OUTPUT -gcflags="$GC_FLAGS" -ldflags="$LD_FLAGS" main.go

# pack executable
upx $OUTPUT
