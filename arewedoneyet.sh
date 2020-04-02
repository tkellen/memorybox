#!/usr/bin/env bash
set -euo pipefail

MAX_LINES=500
CURRENT_LINES=$(cat *.go | grep -v // | grep "\S" | wc -l)

echo $(( ${MAX_LINES} - ${CURRENT_LINES})) lines remain before you must be done.