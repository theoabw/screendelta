#!/usr/bin/env bash
# Run the whole verification and record each command's real exit code, not a pipeline's.
#
# The script's own exit code is the count of commands that failed, so a caller or a CI job can read it. An
# earlier version ended with an echo, so it always exited zero and a reader had to notice the failures by hand.
#
# Usage: scripts/final_verify.sh [output file]
set -uo pipefail

cd "$(dirname "$0")/.."

output="${1:-docs/vv/evidence/final-$(date +%F).txt}"
# The record is written to a temporary file and moved into place at the end. Writing it directly meant that
# `make check-strict`, which runs inside this script, read a half-written file: the report check compares the
# report's measurements against the evidence directory, and the file it was reading contained only the sections
# written so far. The check failed for a reason that had nothing to do with the report.
temporary="${output}.partial"
failed=0
ran=0

run() {
  local label="$1"; shift
  local result status
  result=$("$@" 2>&1)
  status=$?
  ran=$((ran + 1))
  if [ "$status" -ne 0 ]; then
    failed=$((failed + 1))
  fi
  echo "== $label =="
  # Package result lines are summarised by the exit code, and the measurement lines are what a reader wants.
  echo "$result" | grep -vE "^(ok|no test files)" | tail -16
  echo "exit=$status"
  echo
}

{
  echo "Final verification, $(date +%F)."
  echo "Every command below was run against this commit with a clean working tree. Each exit code is the exit"
  echo "code of the command itself, captured without a pipeline, and the script exits with the number of commands"
  echo "that failed."
  echo
} > "$temporary"

{
  run "go test ./... -count=1" go test ./... -count=1
  run "make accuracy" make accuracy
  run "make perf" make perf
  run "make memcheck" make memcheck
  run "make cross" make cross
  run "make demo" make demo
  echo "== coverage of the measured modules =="
  go test -cover ./internal/diff/... ./internal/identity/... ./internal/fingerprint/... ./internal/stream/... 2>&1 | grep -oE "coverage: [0-9.]+% of statements" | tr '\n' ' '
  echo
  echo
  echo "== formatting and vet =="
  formatting=$(gofmt -l . | wc -l)
  diagnostics=$(go vet ./... 2>&1 | wc -l)
  ran=$((ran + 2))
  echo "files needing formatting: $formatting"
  echo "vet diagnostics: $diagnostics"
  if [ "$formatting" -ne 0 ] || [ "$diagnostics" -ne 0 ]; then
    failed=$((failed + 1))
  fi
  echo
  echo "== verdict =="
  echo "$ran checks run, $failed failed."
} >> "$temporary"

# The record goes into place before the process gate runs, because the gate checks the report against this
# directory and would otherwise compare fresh figures with the previous round's evidence.
mv "$temporary" "$output"

{
  run "make check-strict" make check-strict
} >> "$output"

echo "final_verify: $ran checks run, $failed failed, written to $output"
if [ "$failed" -ne 0 ]; then
  exit 1
fi
exit 0
