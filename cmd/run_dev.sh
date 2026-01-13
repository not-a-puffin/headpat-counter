#!/bin/bash

set -euo pipefail

export MODE=DEV
export $(cat .env-dev | xargs) && go run .
