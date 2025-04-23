#!/bin/bash

set -euo pipefail

export MODE=DEV
export $(cat .env | xargs) && go run main.go
