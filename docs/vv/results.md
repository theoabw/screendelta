# Test execution results

Running record of every measurement the report cites. Newest first. Raw output referenced
here is stored under `docs/vv/evidence/`.

The reference machine and the measuring rules are fixed in
`docs/vv/evidence/benchmark-host.txt` and in `plan.md`, so a number here means the same
thing whenever it is regenerated.

## Summary

| Date | Requirement | Level | Expected | Actual | Result | Evidence |
|---|---|---|---|---|---|---|
| 2026-09-26 | FR-006, FR-007 | Functional | User story 2 acceptance scenarios | Partial. Stability and non-reuse are demonstrated. An independent review falsified six claims and five were reproduced, so the requirements stay in progress: the engine cannot yet tell a cover from a content change, an unchanged element cannot be reported as gone, and a partial change replaces a tracked footprint. What was fixed and what remains open is in the evidence file | partial | `docs/vv/evidence/identity-2026-09-26.txt` |
| 2026-09-26 | NFR-009 | Maintainability | At least 80 percent line coverage on the geometry and identity modules | `internal/diff` 80.2 percent, `internal/identity` 80.6 percent, reported by `go test -cover` | pass | `docs/vv/evidence/coverage-2026-09-26.txt` |
| 2026-09-26 | NFR-001, SC-002 | Performance | p95 at or below 12 ms and p99 at or below 25 ms per 1080p frame pair on one CPU core | p50 8.29 ms, p95 9.00 ms, p99 9.69 ms on a change of two elements per frame; a change of about half the screen costs p95 39.31 ms and is recorded separately | pass | `docs/vv/evidence/perf-2026-09-26.txt` |
| 2026-09-26 | NFR-002 | Performance | At least 30 frame pairs per second sustained at 1080p on one core | 77.1 pairs per second over 300 pairs | pass | `docs/vv/evidence/perf-2026-09-26.txt` |
| 2026-09-26 | NFR-006, SC-001 | Accuracy | At least 5,000 generated pairs, F1 at or above 0.98, classes correct where the case states them | 27,203 pairs scored, F1 1.0000, 0 false positives, 0 false negatives, every asserted class correct | pass | `docs/vv/evidence/accuracy-2026-09-26.txt` |
| 2026-09-26 | NFR-005 | Accuracy | Zero false removals on frames that differ only by capture noise | 0 false removals, 0 regions on 119 noise-only pairs | pass | `docs/vv/evidence/accuracy-2026-09-26.txt` |
| 2026-09-26 | NFR-003, SC-003 | Memory | At most 128 MB resident over 10,000 frames, no growth with stream length | peak resident 22.5 MiB and peak heap 24.0 MiB at 1080p, heap identical at frame 1,000 and frame 10,000; real pipeline 10.0 MiB resident | pass | `docs/vv/evidence/memory-2026-09-26.txt` |
| 2026-09-26 | NFR-003 | Allocation | Allocation per frame reaches a steady state | 8,220 and 8,226 bytes per frame in two consecutive windows of 1,000, exactly 2 pixel buffers | pass | `docs/vv/evidence/memory-2026-09-26.txt` |
| 2026-09-26 | FR-002, FR-003, FR-004, FR-011, FR-013, FR-015 | Functional | Each behaviour holds for the case that exercises it | Every row of `docs/traceability.md` names the test that covers it | pass | test sources |

Not yet measured, and deliberately still `planned` in the traceability matrix: determinism
across thread counts (NFR-004, partly covered), sustained decision rate (SC-004), usability
(SC-005, NFR-007) and the independent consumer (SC-006).

The first revision of the two rows above was withdrawn. An independent review showed that
the measurement could pass with broken classification, that it scored 27 of the pairs it
claimed, and that a deliberate 512 byte per frame leak survived both memory tests. The
rows now describe the revised measurement, and AUD-007 records what was wrong with the
first one, because a measurement that reports success without measuring is the failure mode
this chapter exists to catch.

## Metrics

| Metric | Value | Measured on |
|---|---|---|
| Corpus cases scored | 5, changed-label, moving-button, occluded-button, noise, sweep | 2026-09-26 |
| Frame pairs scored | 27,203 | 2026-09-26 |
| Region detection F1 | 1.0000 | 2026-09-26 |
| False removals on noise | 0 | 2026-09-26 |
| Frames streamed for the memory measurement | 10,000 at 1920x1080, and 10,000 at 320x240 through the real pipeline | 2026-09-26 |
| Peak resident over that stream | 22.5 MiB against a 128 MiB ceiling | 2026-09-26 |
| Allocation per frame at steady state | 8,220 bytes | 2026-09-26 |
| Line coverage, `internal/diff` | 91.8 percent | 2026-09-26 |
| p95 latency per 1080p frame pair, one core | 9.59 ms against a 12 ms target, after identity tracking added about a millisecond | 2026-09-26 |
| p99 latency per 1080p frame pair, one core | 11.82 ms against a 25 ms target | 2026-09-26 |
| Sustained throughput, one core | 73.7 frame pairs per second against a 30 per second target | 2026-09-26 |
| Line coverage, `internal/diff` and `internal/identity` | 80.7 and 82.9 percent against an 80 percent target | 2026-09-26 |

## Defects

Both functional defects below were found by the measurement rather than by reading the code,
which is the argument for measuring before writing the report.

| ID | Requirement | Symptom | Root cause | Resolution | Status |
|---|---|---|---|---|---|
| AUD-001 | FR-002, NFR-006 | A translated element scored F1 0.3333 | The engine reported one region spanning the whole movement, which covers pixels that did not change and misses the two areas that did | A translation now reports the area left and the area arrived in as two moved regions, each carrying the element's earlier position | fixed |
| AUD-002 | FR-005, NFR-005 | A frame pair differing only by noise reported one region as removed, and the occluded case scored F1 0.8000 | Every previous element that no changed area accounted for was classified as gone, so an element that simply did not change was reported as removed | That pass was deleted. An element whose pixels did not change is still on the screen and is absent from the delta by definition. Removal is now only reported when a changed area covers a previous element of a different size, which is observable | fixed |
| AUD-003 | FR-015, NFR-004 | `make memcheck` and `make accuracy` reported success while measuring nothing | `go test` treats a missing test name as "no tests to run" and exits zero | Both targets now ask `go test -list` whether the test exists and fail with the task number that will add it | fixed |
| AUD-004 | FR-002, NFR-006 | Sweep precision 0.9995 with 13 duplicate regions | A sixteen pixel tile grid decided what a change was by the tile mean, so a change reaching into a tile under the floor was cut at the boundary and reported twice | The tile grid was removed. Changed pixels are found per pixel and grouped by connectivity, which removed all thirteen duplicates and is a smaller implementation | fixed |
| AUD-005 | FR-003, NFR-006 | Sweep recall 0.9947 and one wrong class in the occluded case | Remembered geometry was rewritten from the current frame's changed areas, so a pair that changed nothing erased the baseline and the next change looked new | Elements whose pixels did not change carry over; the returning overlay is now classified changed rather than added | fixed |
| AUD-006 | FR-002 | Two elements a tile apart were reported as one region | Areas were merged by tile adjacency rather than by pixel connectivity | Grouping is by pixel connectivity, so each element is its own region | fixed |
| AUD-012 | FR-006 | A large element could take a small element's identity, and the answer depended on allocation order | Containment scored a perfect overlap and tied with an exact match, so the older identity won | Matches are ranked: an overlap match beats a containment match, ties go to the smallest element and then the lowest identifier | fixed |
| AUD-013 | FR-006, FR-007 | An element that sat still for longer than the occlusion window was retired and the next change to it looked like a reacquisition of something new | Aging counted frames rather than evidence, so an unchanged screen looked like an occlusion | An element ages only when the area it occupies changed and it was not matched | fixed |
| AUD-014 | NFR-003 | Retired identities were kept forever, so the table grew with the session | Retention was unbounded | Retired entries are dropped once too old to reacquire, which can only turn a reacquisition into an addition, and the counter keeps rising | fixed |
| AUD-015 | FR-006, FR-007 | A removal could name a newly allocated live identity | The fallback for a removal with nothing to retire allocated and left it live | The fallback identity is retired at once | fixed |
| AUD-016 | FR-006 | A viewport change could leave identities live, including a scale change and a size round trip | The engine skips the comparison for a viewport change, so the differ never saw the discontinuity | The differ interface has Reset, the engine calls it on a viewport change, and both cases are covered | fixed |
| AUD-017 | FR-006 | A contained rectangle of three tenths the area lost its identity while one of 0.29 kept it | The size gate applied to the containment path only above the overlap threshold | The containment path no longer applies a size ratio, and the ranking makes the two paths agree | fixed |
| AUD-018 | FR-007 | The engine cannot report a reacquisition after a cover, so the uncertainty marker is unreachable through the classifier | A covering area is attributed to the element it covers, and a return is attributed to the cover, because both are the same changed area | Not fixed. The test records what the engine does say, and the limitation is written into the evidence rather than hidden | open |
| AUD-019 | FR-007 | An element whose pixels did not change cannot be reported as gone, so a disappearance and a content change look alike | Distinguishing them needs appearance reasoning the stage does not have | Not fixed, and deliberately: inventing a disappearance from an unchanged screen would be worse than saying nothing. Recorded as a limitation for the report's threats to validity | open |
| AUD-020 | FR-006 | A partial change replaces an element's tracked footprint, so successive changes in different parts of one element can get different identities | The footprint is the union of the regions seen in a frame, which is the changed area rather than the element | Not fixed. It needs translation estimation, which is the next design decision rather than a bug fix | open |
| AUD-021 | FR-015 | delta.Decode accepts a document missing the identity fields | A missing JSON field becomes a valid zero value in Go | Not fixed. The engine always emits them; a library consumer reading a foreign document is not protected. The decoder strictness question is recorded for the report | open |
| AUD-010 | FR-006 | Rebuilding the identity table at a viewport change reissued identifier numbers mid-session | The table was recreated to invalidate normalised bounds after a geometry change, which reset the counter |  A viewport change now retires every element and keeps the counter rising, so an identity that stops being reported never reappears on something else. Found by a test that asserted the counter, not by reading the code | fixed |
| AUD-011 | FR-006, FR-007 | A translated element lost its identity at the second movement | The identity layer looked a moved region up by the area that changed rather than by where the element was, and the classification memory holds changed areas rather than element footprints, so each half of a movement became its own element | A moved region is looked up by the element's earlier position, the element's footprint for a frame is the union of the regions that named it, and an area inside a tracked element is attributed to it. Found by a debug trace showing identity 1 becoming identities 2 and 3 at step three | fixed |
| AUD-008 | NFR-001, SC-002 | p95 latency 25.26 ms against a 12 ms target | The luma conversion ran six million times per frame pair, the fingerprint summed every pixel of every cell in a second full walk, and the comparison walked the whole mask to find component starts | The differ keeps the two most recent luma planes so both passes read one conversion per frame, cell values are sampled at a stride of a quarter of a cell, and the comparison scans only changed rows and skips eight bytes at a time within them. p95 25.26 ms to 9.00 ms. Fusing the grid into the conversion pass was tried and measured slower, 8.2 ms against 6.7 ms, and was reverted; unrolling the conversion four pixels per iteration gained 2 percent and was also reverted | fixed |
| AUD-009 | NFR-001 | A latency run inside `go test ./...` reported p95 18.35 ms with a p50 of 8.73 ms | Test packages run in parallel, so the measured tail belonged to the machine rather than the engine | The measurement compares the median against the fastest pair and skips with a message when the run was contended; `make perf` runs with -p 1 for the recorded numbers | fixed |
| AUD-007 | NFR-003, SC-001, NFR-005, NFR-006 | The first measurement was passable with broken classification, unscored transitions, and a leak | The suite scored 27 selected pairs, asserted no classes, and allowed 8 MB of heap growth | Ground truth comes from the pixels, every adjacent pair is scored, 5,000 pairs are required, classes are asserted, the memory bound is 1 MB and resident memory is read. Verified by mutation: a 512 byte per frame leak now fails both memory tests, and an engine calling every change "changed" now fails classification | fixed |

AUD-003 is recorded here rather than only in a commit message because it is the failure mode
the verification chapter is about: a green result that measured nothing is worse than a red
one, because it is believed.

## Reproducing the results

```bash
make accuracy   # corpus scoring: precision, recall, F1, false removals
make memcheck   # 10,000 frames at 1080p, peak heap and steady state allocation
make check      # requirement traceability
```

Each command was verified to fail when the thing it measures is absent or broken, so a green
result means the measurement ran.

## Open items

- Latency and throughput are unmeasured, so no performance claim appears in the README yet.
- The corpus is synthesised. Real screenshots are a sanity check, not a scored set, and the
  report's threats-to-validity section says so.
- Identifier stability across frames is measured by a unit test at present; the occlusion and
  uncertainty rules arrive with user story two.
