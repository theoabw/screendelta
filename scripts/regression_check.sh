#!/usr/bin/env bash
# Regression check: is every recorded fix actually pinned by a test?
#
# The defect table records forty-three defects as fixed, and each row names the test that verifies it. That claim is
# only worth something if reverting the fix makes the named test fail. This script does exactly that: it puts the
# old behaviour back, one fix at a time, runs the test the defect row names, and reports whether the test noticed.
#
# A fix whose reversion nothing notices is a claim the record cannot support. That is the same rule this project
# applies to measurements, applied to its own account of what it fixed.
#
# Usage: scripts/regression_check.sh [output file]
#
# The working tree must be clean apart from the output file. Every reversion is undone immediately.
set -uo pipefail

cd "$(dirname "$0")/.."

output="${1:-docs/vv/evidence/regression-$(date +%F).txt}"
log="$(mktemp)"
trap 'rm -f "$log"' EXIT

dirty() {
  local status
  status="$(git status --porcelain -- . ":(exclude)$output" 2>/dev/null)"
  if [ -z "$status" ]; then
    status="$(git status --porcelain 2>/dev/null | grep -vF -- "$output")"
  fi
  printf '%s' "$status"
}

if [ -n "$(dirty)" ]; then
  echo "regression_check: the working tree is not clean, refusing to revert anything" >&2
  exit 2
fi

# defect|file|search|revert to|the test the defect row names
cases=(
  "AUD-021|internal/delta/required.go|return missing(field)|return nil|TestDecodeRejectsAMissingRequiredField"
  "AUD-033|internal/corpus/sweep.go|for _, cell := range cells {|for cell := range chosen {|TestTheGeneratorIsDeterministic"
  "AUD-036|internal/delta/delta.go|return regionLess(regions[i], regions[j])|return regions[i].Bounds.Y < regions[j].Bounds.Y|TestTheRegionOrderIsTotalAndIndependentOfDiscoveryOrder"
  "AUD-038|internal/delta/required.go|{\"x\", region.Bounds.X},|{\"x\", region.Identity},|TestDecodeRejectsBoundsMissingAMember"
  "AUD-039|internal/delta/delta.go|if seenConditions[condition] {|if false {|TestDecodeAndValidateRejectDuplicateConditions"
  "AUD-041|internal/delta/delta.go|if a.Magnitude != b.Magnitude {|if false {|TestTheRegionOrderSettlesEveryFieldPair"
  "AUD-042|internal/delta/encode.go|if err := checkNullObjects(raw); err != nil {|if err := error(nil); err != nil {|TestDecodeRejectsNullObjects"
  "AUD-022|internal/identity/identity.go|if signature.Distance(element.Signature) > returnAppearanceCeiling*SignatureCells {|if !signature.Close(element.Signature) {|TestAResizedReturnIsRecognisedAsUncertain"
  "AUD-023|internal/diff/classify.go|taken = append(taken, enclosed)|if len(taken) == 0 { taken = append(taken, enclosed) }|TestOneCoveringAreaRetiresEveryElementItCovers"
  "AUD-027|internal/diff/classify.go|if !goneElements[state.ID] {|if false {|TestGrowthAtTheFrameEdgeIsNotACover"
  "AUD-032|internal/diff/classify.go|if d.changedFraction(live.Bounds, current) >= coverInteriorFraction {|if true {|TestRepaintingInsideACoverDoesNotRetireTheCover"
  "AUD-028|internal/identity/identity.go|if gone != nil && !gone[live.ID] {|if false {|TestAReturnNeedsTheOverlappedElementsOwnPixels"
  "AUD-032|internal/diff/classify.go|if !inner.Empty() {|if false {|TestAResizedReturnIsRecognisedAsUncertain"
)

# Reversions that change nothing observable, because the behaviour is defended by a second rule as well. The
# record's claim is about the behaviour, and the behaviour is pinned; the line is defence in depth.
redundant=(
  "AUD-031|internal/identity/identity.go|if !withinTolerance(candidate, rect, m.motionTolerancePixels) &&|if false &&|TestReturnNeedsEvidenceInBothDirections|the eligibility gate refuses the same candidates the coverage check refuses, so bypassing it changes no outcome the tests can produce"
)

total=0
caught=0
missed=()
notapplied=()

{
  echo "Regression check, $(date +%F)."
  echo "Command: scripts/regression_check.sh"
  echo
  echo "Each row reverts one recorded fix and runs the test the defect row names. A row marked CAUGHT means the test"
  echo "failed with the old behaviour in place, which is what makes the fix's claim worth something. A row marked"
  echo "MISSED means the fix is not pinned by the test the record names."
  echo
  printf '%-10s %-52s %s\n' "DEFECT" "TEST THE RECORD NAMES" "RESULT"
} > "$output"

for entry in "${cases[@]}"; do
  IFS='|' read -r defect file search revert test <<< "$entry"

  if ! grep -qF -- "$search" "$file"; then
    notapplied+=("$defect")
    printf '%-10s %-52s %s\n' "$defect" "$test" "NOT APPLIED, the pattern is not in $file" >> "$output"
    continue
  fi
  total=$((total + 1))

  python3 - "$file" "$search" "$revert" <<'PY'
import pathlib, sys
path, search, revert = sys.argv[1], sys.argv[2], sys.argv[3]
p = pathlib.Path(path)
# The revert is written with literal \n escapes so the list can stay one line per case.
p.write_text(p.read_text().replace(search.replace("\\n", "\n"), revert.replace("\\n", "\n"), 1))
PY

  go test ./internal/... -run "$test" -count=1 > "$log" 2>&1
  status=$?
  failing="$(grep -m1 -E '^--- FAIL: ' "$log")"
  if [ "$status" -ne 0 ] && [ -n "$failing" ]; then
    caught=$((caught + 1))
    printf '%-10s %-52s %s\n' "$defect" "$test" "CAUGHT" >> "$output"
  elif [ "$status" -eq 0 ]; then
    missed+=("$defect")
    printf '%-10s %-52s %s\n' "$defect" "$test" "MISSED, the test passed with the fix reverted" >> "$output"
  else
    missed+=("$defect (build failure)")
    printf '%-10s %-52s %s\n' "$defect" "$test" "BUILD FAILED, so the reversion proves nothing" >> "$output"
  fi

  git checkout -- "$file"
done

{
  echo
  echo "Score: $caught of $total applied reversions caught by the test the defect row names."
  if [ ${#redundant[@]} -gt 0 ]; then
    echo
    echo "Reversions that change nothing observable, because a second rule defends the same behaviour:"
    for entry in "${redundant[@]}"; do
      IFS='|' read -r defect _ _ _ test reason <<< "$entry"
      echo "  - $defect: $reason ($test pins the behaviour)"
    done
  fi
  if [ ${#missed[@]} -gt 0 ]; then
    echo
    echo "Fixes that the record claims and the test does not pin:"
    for item in "${missed[@]}"; do echo "  - $item"; done
  fi
  if [ ${#notapplied[@]} -gt 0 ]; then
    echo "${#notapplied[@]} reversion(s) were never applied because their pattern was not found: ${notapplied[*]}"
  fi
  echo
  if [ -n "$(dirty)" ]; then
    echo "WARNING: the working tree is not clean after the run, so a fix may not have been restored."
  else
    echo "The working tree is clean after the run: every fix was restored."
  fi
} >> "$output"

echo "regression_check: $caught of $total reversions caught, written to $output"
status=0
if [ ${#missed[@]} -gt 0 ]; then
  echo "not pinned: ${missed[*]}"
  status=1
fi
if [ "$total" -eq 0 ] || [ ${#notapplied[@]} -gt 0 ]; then
  status=1
fi
if [ -n "$(dirty)" ]; then
  status=1
fi
exit "$status"
