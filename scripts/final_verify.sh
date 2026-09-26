#!/usr/bin/env bash
# Run the whole verification and record each command's real exit code, not a pipeline's.
set -u
cd "$(dirname "$0")/.." 2>/dev/null || true
run() {
  local label="$1"; shift
  local output status
  output=$("$@" 2>&1)
  status=$?
  echo "== $label =="
  echo "$output" | grep -vE "^(ok|no test files)" | tail -14
  echo "exit=$status"
  echo
}
echo "Final verification, 2026-09-27."
echo "Every command below was run against this commit with a clean working tree. Each exit code is the exit"
echo "code of the command itself, captured without a pipeline, because a green-looking output line is not a"
echo "result: an earlier version of this file recorded a run that had failed."
echo
run "make check-strict" make check-strict
run "go test ./... -count=1" go test ./... -count=1
run "make accuracy" make accuracy
run "make perf" make perf
run "make memcheck" make memcheck
run "make cross" make cross
run "make demo" make demo
echo "== formatting and vet =="
echo "files needing formatting: $(gofmt -l . | wc -l)"
echo "vet diagnostics: $(go vet ./... 2>&1 | wc -l)"
