#!/usr/bin/env bash

set -euo pipefail

INPUT="${INPUT_COVERAGE-coverage.out}"

COVERAGE=$(go tool cover -func="$INPUT" | tail -1 | grep -Eo '[0-9]+\.[0-9]')

echo "coverage: $COVERAGE% of statements"

if awk "BEGIN {exit !($COVERAGE >= 90)}"; then
	COLOR=brightgreen
elif awk "BEGIN {exit !($COVERAGE >= 80)}"; then
	COLOR=green
elif awk "BEGIN {exit !($COVERAGE >= 70)}"; then
	COLOR=yellowgreen
elif awk "BEGIN {exit !($COVERAGE >= 60)}"; then
	COLOR=yellow
elif awk "BEGIN {exit !($COVERAGE >= 50)}"; then
	COLOR=orange
else
	COLOR=red
fi

curl -s "https://img.shields.io/badge/coverage-$COVERAGE%25-$COLOR?style=flat" > coverage.svg
