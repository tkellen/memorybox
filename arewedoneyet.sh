#!/usr/bin/env bash
set -euo pipefail
SCRIPT_PATH="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

function lineCount {
  local package=${1}
  go list -f '{{.Dir}}/{{join .GoFiles (printf "%s%s/" "\n" .Dir)}}' ${package} \
    | xargs cat                 `# read each source file` \
    | grep -v //                `# remove lines that are comments` \
    | grep "\S"                 `# remove empty lines` \
    | sed '/const usage/,/`/d'  `# remove usage string` \
    | wc -l                     `# count the lines that remain`
}
packageName=$(dirname $(go list -f '{{.ImportPath}}'))
maxLines=500
usedLines=0

while read package; do
    packageLineCount=$(lineCount ${package})
    printf "%s\t%s\n" ${packageLineCount} ${package#*${packageName}}
    usedLines=$((usedLines+packageLineCount))
done <<<$(go list ./...)

printf "%s\n\n%s lines remain before you must be done.\n" ${usedLines} $((${maxLines} - ${usedLines}))