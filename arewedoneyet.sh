#!/usr/bin/env bash
set -euo pipefail
SCRIPT_PATH="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

MAX_LINES=500
CURRENT_LINES=$(find ${SCRIPT_PATH} -name "*.go" | grep -v _test | xargs cat | grep -v // | grep "\S" | wc -l)

echo $(( ${MAX_LINES} - ${CURRENT_LINES})) lines remain before you must be done.