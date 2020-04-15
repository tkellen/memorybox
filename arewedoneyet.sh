#!/usr/bin/env bash
set -euo pipefail
SCRIPT_PATH="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

MAX_LINES=500
LINE_COUNT=$(
  find ${SCRIPT_PATH} -name "*.go"         `# find all go files in the repo`\
    | grep -v test                         `# omit files w/ test in name` \
    | grep -v hack                         `# omit files w/ hack in name` \
    | xargs cat                            `# read every file left` \
    | grep -v //                           `# remove lines that are comments` \
    | grep "\S"                            `# remove empty lines` \
    | sed '/const usage/,/`/d'             `# remove usage string` \
    | wc -l                                `# count the lines that remain`
)

echo $((${MAX_LINES} - ${LINE_COUNT})) lines remain before you must be done.