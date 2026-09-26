#!/usr/bin/env bash
# Mutation testing: does the suite notice when the engine is wrong?
#
# Coverage says which lines the tests execute. It does not say whether a test would fail if the line were
# wrong, and this project's own history is the argument for asking: three separate measurement harnesses once
# reported success while measuring nothing, and a mutant that calls every region changed would have passed the
# first accuracy suite.
#
# A mutation is killed only when a test actually fails. A non-zero exit with no failing test is a build or
# package failure, which says nothing about whether the suite noticed the change, so it is counted as neither
# and reported as inconclusive. Survivors make the run fail: the finding is a rule no test defends, and an exit
# code is what a caller reads.
#
# Usage: scripts/mutation_check.sh [output file]
#
# The working tree must be clean apart from the output file. Every mutation is reverted immediately, and the
# script checks the tree again at the end to prove it.
set -uo pipefail

cd "$(dirname "$0")/.."

output="${1:-docs/vv/evidence/mutation-$(date +%F).txt}"
log="$(mktemp)"
trap 'rm -f "$log"' EXIT

packages="./internal/diff/... ./internal/identity/... ./internal/score/... ./internal/stream/... ./internal/delta/... ./internal/frame/... ./internal/config/..."

# The output file lives in the repository by default, so it is excluded from the cleanliness check: otherwise a
# second run refuses to start, and a leftover mutation cannot be told apart from the script's own artefact.
dirty() {
  local status
  status="$(git status --porcelain -- . ":(exclude)$output" 2>/dev/null)"
  if [ -z "$status" ]; then
    status="$(git status --porcelain 2>/dev/null | grep -vF -- "$output")"
  fi
  printf '%s' "$status"
}

if [ -n "$(dirty)" ]; then
  echo "mutation_check: the working tree is not clean, refusing to mutate it" >&2
  echo "note: $output is excluded from this check, so a dirty tree means a leftover mutation or an unrelated edit." >&2
  exit 2
fi

# file|search|replace|what the mutation breaks
mutations=(
  "internal/identity/geometry.go|minimumOverlap = 0.30|minimumOverlap = 0.60|the overlap needed to keep an identity"
  "internal/identity/geometry.go|minimumAreaRatio = 0.5|minimumAreaRatio = 0.05|the size gate on a match"
  "internal/identity/geometry.go|coverInteriorFraction = 0.90|coverInteriorFraction = 0.40|how much of an element must be contained before it counts as covered"
  "internal/identity/identity.go|retiredRetentionFrames = 300|retiredRetentionFrames = 2|how long a retired identity can be reacquired"
  "internal/identity/geometry.go|returnAppearanceCeiling = 60|returnAppearanceCeiling = 100000|the appearance ceiling on a return"
  "internal/identity/signature.go|SignatureTolerance = 12|SignatureTolerance = 120|the signature tolerance"
  "internal/diff/diff.go|growthMargin = 2|growthMargin = 12|the margin a reported region is grown by"
  "internal/diff/diff.go|lumaRed   = 299|lumaRed   = 199|the red luma weight"
  "internal/diff/diff.go|wordBytes = 8|wordBytes = 2|the word size used to compare pixels"
  "internal/config/config.go|DefaultNoiseFloor            = 0.02|DefaultNoiseFloor            = 0.0|the default noise floor"
  "internal/config/config.go|DefaultMinRegionAreaPixels   = 64|DefaultMinRegionAreaPixels   = 4|the default minimum region area"
  "internal/delta/delta.go|if a.Class != b.Class {|if false {|the class tie-breaker in the region order"
  "internal/delta/delta.go|return a.AreaPixels < b.AreaPixels|return false|the area tie-breaker in the region order"
  "internal/delta/required.go|if raw == nil || isNull(*raw) {|if false {|the check that a required field is present"
)

total=0
killed=0
inconclusive=0
survived=()
notapplied=()

{
  echo "Mutation testing, $(date +%F)."
  echo "Command: scripts/mutation_check.sh"
  echo
  echo "Each row applies one change to a non-test file, runs the packages that should care, and records whether a"
  echo "test failed. A mutation that survives is a rule the suite does not defend. A mutation that makes the"
  echo "package fail to build is counted as neither, because a build failure is not evidence that a test noticed."
  echo
  printf '%-58s %-12s %s\n' "MUTATION" "RESULT" "NOTE"
} > "$output"

for entry in "${mutations[@]}"; do
  IFS='|' read -r file search replace note <<< "$entry"

  if ! grep -qF -- "$search" "$file"; then
    notapplied+=("$note")
    printf '%-58s %-12s %s\n' "$note" "NOT APPLIED" "the pattern is not in $file" >> "$output"
    continue
  fi
  total=$((total + 1))

  python3 - "$file" "$search" "$replace" <<'PY'
import pathlib, sys
path, search, replace = sys.argv[1], sys.argv[2], sys.argv[3]
p = pathlib.Path(path)
p.write_text(p.read_text().replace(search, replace, 1))
PY

  go test $packages -count=1 > "$log" 2>&1
  status=$?
  failing="$(grep -m1 -E '^--- FAIL: ' "$log")"

  if [ "$status" -eq 0 ]; then
    survived+=("$note")
    printf '%-58s %-12s %s\n' "$note" "SURVIVED" "no test failed" >> "$output"
  elif [ -n "$failing" ]; then
    killed=$((killed + 1))
    printf '%-58s %-12s %s\n' "$note" "killed" "$failing" >> "$output"
  else
    inconclusive=$((inconclusive + 1))
    printf '%-58s %-12s %s\n' "$note" "INCONCLUSIVE" "$(grep -m1 -E '^(FAIL|# |cannot)' "$log")" >> "$output"
  fi

  git checkout -- "$file"
done

{
  echo
  echo "Score: $killed of $total applied mutations killed."
  if [ "$inconclusive" -gt 0 ]; then
    echo "$inconclusive mutation(s) produced a non-zero exit with no failing test, which is a build or package"
    echo "failure rather than a kill, and is counted as neither."
  fi
  if [ ${#notapplied[@]} -gt 0 ]; then
    echo "${#notapplied[@]} mutation(s) were never applied because their pattern was not found."
  fi
  if [ ${#survived[@]} -gt 0 ]; then
    echo
    echo "Surviving mutations, each of which is a rule no test defends:"
    for item in "${survived[@]}"; do echo "  - $item"; done
  fi
  echo
  if [ -n "$(dirty)" ]; then
    echo "WARNING: the working tree is not clean after the run, so a mutation may not have been reverted."
  else
    echo "The working tree is clean after the run: every mutation was reverted."
  fi
} >> "$output"

echo "mutation_check: $killed of $total applied mutations killed, written to $output"
if [ ${#survived[@]} -gt 0 ]; then
  echo "survivors: ${survived[*]}"
  exit 1
fi
exit 0
