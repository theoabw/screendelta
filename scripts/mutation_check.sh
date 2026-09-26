#!/usr/bin/env bash
# Mutation testing: does the suite notice when the engine is wrong?
#
# Coverage says which lines the tests execute. It does not say whether a test would fail if the line were
# wrong, and this project's own history is the argument for asking: three separate measurement harnesses once
# reported success while measuring nothing, and a mutant that calls every region changed would have passed the
# first accuracy suite. This script answers the narrower, harder question for a fixed list of mutations: apply
# each one to a non-test file, run the packages that should care, and report whether anything failed.
#
# A mutation that no test catches is a gap, and the list is chosen from the rules the requirements name rather
# than at random: the identity thresholds, the cover rule, the noise floor, the luma weights, the growth margin,
# the signature tolerance and the document ordering.
#
# Usage: scripts/mutation_check.sh [output file]
#
# The working tree must be clean. Every mutation is reverted immediately after its run, and the script verifies
# that the tree is clean again at the end.
set -uo pipefail

cd "$(dirname "$0")/.."

output="${1:-docs/vv/evidence/mutation-$(date +%F).txt}"
packages="./internal/diff/... ./internal/identity/... ./internal/score/... ./internal/stream/... ./internal/delta/... ./internal/frame/... ./internal/config/..."

if [ -n "$(git status --porcelain)" ]; then
  echo "mutation_check: the working tree is not clean, refusing to mutate it" >&2
  exit 1
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
  "internal/delta/delta.go|a.Bounds.H < b.Bounds.H|a.Bounds.H <= b.Bounds.H|the last key of the region order, made non-strict"
)

total=0
killed=0
survived=()

{
  echo "Mutation testing, $(date +%F)."
  echo "Command: scripts/mutation_check.sh"
  echo
  echo "Each row applies one change to a non-test file, runs $packages, and records whether anything failed."
  echo "A mutation that survives is a rule the suite does not defend: either the rule is untested or the test"
  echo "asserts something that the mutation does not affect."
  echo
  printf '%-58s %-12s %s\n' "MUTATION" "RESULT" "NOTE"
} > "$output"

for entry in "${mutations[@]}"; do
  IFS='|' read -r file search replace note <<< "$entry"
  total=$((total + 1))

  if ! grep -qF -- "$search" "$file"; then
    printf '%-58s %-12s %s\n' "$note" "not applied" "the pattern is not in $file" >> "$output"
    continue
  fi

  python3 - "$file" "$search" "$replace" <<'PY'
import pathlib, sys
path, search, replace = sys.argv[1], sys.argv[2], sys.argv[3]
p = pathlib.Path(path)
p.write_text(p.read_text().replace(search, replace, 1))
PY

  if go test $packages -count=1 > /tmp/mutation-run.txt 2>&1; then
    outcome="survived"
    survived+=("$note")
    printf '%-58s %-12s %s\n' "$note" "SURVIVED" "no test failed; see the note at the end" >> "$output"
  else
    outcome="killed"
    killed=$((killed + 1))
    failing=$(grep -m1 -E "^--- FAIL|^FAIL" /tmp/mutation-run.txt | head -1)
    printf '%-58s %-12s %s\n' "$note" "killed" "$failing" >> "$output"
  fi

  git checkout -- "$file"
done

{
  echo
  echo "Score: $killed of $total mutations killed."
  if [ ${#survived[@]} -gt 0 ]; then
    echo
    echo "Surviving mutations, each of which is a rule no test defends:"
    for item in "${survived[@]}"; do echo "  - $item"; done
  fi
  echo
  if [ -n "$(git status --porcelain)" ]; then
    echo "WARNING: the working tree is not clean after the run."
  else
    echo "The working tree is clean and every mutation was reverted."
  fi
} >> "$output"

echo "mutation_check: $killed of $total mutations killed, written to $output"
if [ ${#survived[@]} -gt 0 ]; then
  echo "survivors: ${survived[*]}"
fi
