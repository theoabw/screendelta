# Quickstart validation scenarios

Runnable scenarios that prove the engine does what the specification says. They are
the acceptance checks for the feature and the script a reviewer follows, not a test
suite; the unit and property tests live in the repository next to the code.

## Prerequisites

- Go 1.24 or newer.
- A terminal on Linux or Windows.
- Frames to work with. Either take screenshots, or generate a controlled pair with
  the corpus tool, which is the recommended path because it also writes the ground
  truth:

```sh
go build -o /tmp/screendelta ./cmd/screendelta
go run ./tools/corpusgen --case changed-label --out /tmp/frames
```

That writes `/tmp/frames/001.png`, `/tmp/frames/002.png` and
`/tmp/frames/expected.json`.

## Scenario 1: identical frames produce no regions

```sh
/tmp/screendelta diff --previous /tmp/frames/001.png --current /tmp/frames/001.png | jq '.regions, .conditions'
```

Expected: `regions` is empty and `conditions` is empty. The document contains
`frame`, `fingerprint` and `schemaVersion`, and no timing field of any kind, because
byte-identical output is a requirement. Requirements: FR-002, FR-005, NFR-004.

## Scenario 2: a changed region is reported once, with normalized bounds

```sh
/tmp/screendelta diff --previous /tmp/frames/001.png --current /tmp/frames/002.png | jq '.regions'
```

Expected: exactly one region, `class` is `changed`, `bounds` are normalized between
0 and 1, `bounds.x + bounds.w` and `bounds.y + bounds.h` are at most 1, `magnitude`
is above the configured floor, and the bounds enclose the labelled element from
`expected.json` with an intersection over union of at least 0.5. Requirements:
FR-002, FR-003, FR-004, NFR-006.

## Scenario 3: noise alone reports nothing

```sh
go run ./tools/corpusgen --case noise --out /tmp/noise
/tmp/screendelta diff --previous /tmp/noise/001.png --current /tmp/noise/002.png | jq '.regions | length'
```

Expected: `0`, and the same holds for every pair in `/tmp/noise`. This is the
zero-false-removal rule from NFR-005, and it is the first thing to check after any
change to the thresholds.

## Scenario 4: identity survives motion and is never reused

```sh
go run ./tools/corpusgen --case moving-button --frames 12 --out /tmp/stream
/tmp/screendelta stream --source /tmp/stream --out /tmp/stream.ndjson
jq -r 'select(.regions | length > 0) | .regions[] | "\(.identity) \(.class)"' /tmp/stream.ndjson | sort | uniq -c
```

Expected: one identity dominates the output across the frames in which the button is
visible, no identity is applied to two unrelated regions, and no identity that has
disappeared returns later. Requirements: FR-006, FR-010.

## Scenario 5: occlusion is reported as uncertain rather than matched

```sh
go run ./tools/corpusgen --case occluded-button --frames 12 --out /tmp/occl
/tmp/screendelta stream --source /tmp/occl --out /tmp/occl.ndjson
jq -r '.regions[] | select(.class == "added") | "identity \(.identity)"' /tmp/occl.ndjson
```

Expected: the region that reappears after the occlusion has an identity that was not
used before it disappeared, and the document does not claim a match. Requirement:
FR-007.

## Scenario 6: a fingerprint distinguishes noise from content

```sh
/tmp/screendelta fingerprint --frame /tmp/frames/001.png --out /tmp/fp.json
/tmp/screendelta diff --previous /tmp/frames/001.png --current /tmp/noise/002.png | jq '.conditions'
/tmp/screendelta diff --previous /tmp/frames/001.png --current /tmp/frames/002.png | jq '.regions | length'
```

Expected: the fingerprint of a frame compared with a noisy copy of itself reports
equal, and compared with a materially changed copy reports different with the
changed region listed. Requirements: FR-008, FR-009.

## Scenario 7: regions of interest restrict the output

```sh
cat > /tmp/roi.json <<'JSON'
{ "regionsOfInterest": [ { "label": "toolbar", "bounds": { "x": 0, "y": 0, "w": 1, "h": 0.2 } } ] }
JSON
/tmp/screendelta diff --previous /tmp/frames/001.png --current /tmp/frames/002.png --config /tmp/roi.json | jq '.regions'
```

Expected: only regions inside the toolbar are reported. An empty
`regionsOfInterest` array reports nothing at all, which is a different request from
omitting the field. Requirement: FR-011.

## Scenario 8: unusable input fails loudly and writes nothing

```sh
/tmp/screendelta diff --previous /tmp/frames/001.png --current /tmp/frames/001.png --width 100 --height 100
echo "exit code: $?"
```

Expected: one error line on standard error naming the mismatch, exit code 2, and
nothing on standard output. Requirement: FR-015.

## Scenario 9: output is deterministic across thread counts

```sh
GOMAXPROCS=1 /tmp/screendelta stream --source /tmp/stream --out /tmp/a.ndjson
GOMAXPROCS=4 /tmp/screendelta stream --source /tmp/stream --out /tmp/b.ndjson
cmp /tmp/a.ndjson /tmp/b.ndjson && echo identical
```

Expected: `identical`. Requirement: NFR-004.

## Accuracy and performance checks

```sh
make corpus    # generate the ground-truth corpus
make accuracy  # print precision, recall, F1 and the false-removal count
make bench     # print p50, p95 and p99 per frame pair
make memcheck  # stream 10,000 frames and report peak resident memory
```

Expected, as the acceptance thresholds: F1 at or above 0.98 with zero false removals
(SC-001), p95 at or below 12 ms and p99 at or below 25 ms on the reference machine
(SC-002), peak memory at or below 128 MB over 10,000 frames with no growth
attributable to stream length (SC-003).

## What a reviewer should conclude

If every scenario above behaves as described, the specification's functional
requirements hold end to end and the measurable criteria are reproducible from the
repository alone. If a scenario fails, the failing requirement identifier is the
first line of the defect report.
