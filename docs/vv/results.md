# Test execution results

Running record of every measurement the report cites. Newest first. Raw output referenced
here is stored under `docs/vv/evidence/`.

The reference machine and the measuring rules are fixed in
`docs/vv/evidence/benchmark-host.txt` and in `plan.md`, so a number here means the same
thing whenever it is regenerated.

## Summary

| Date | Requirement | Level | Expected | Actual | Result | Evidence |
|---|---|---|---|---|---|---|
| 2026-09-26 | NFR-006, SC-001 | Accuracy | F1 at or above 0.98 on the generated corpus | F1 1.0000 across four cases, 9 true positives, 0 false positives, 0 false negatives | pass | `docs/vv/evidence/accuracy-2026-09-26.txt` |
| 2026-09-26 | NFR-005 | Accuracy | Zero false removals on frames that differ only by noise | 0 false removals, 0 regions reported across 20 noise-only pairs | pass | `docs/vv/evidence/accuracy-2026-09-26.txt` |
| 2026-09-26 | NFR-003, SC-003 | Memory | At most 128 MB over 10,000 frames, no growth with stream length | peak heap 24.0 MiB, peak system 37.2 MiB, heap identical at frame 1,000 and frame 10,000 | pass | `docs/vv/evidence/memory-2026-09-26.txt` |
| 2026-09-26 | NFR-003 | Allocation | Allocation per frame reaches a steady state | 8,220 bytes per frame in two consecutive windows of 1,000, exactly 2 pixel buffers | pass | `docs/vv/evidence/memory-2026-09-26.txt` |
| 2026-09-26 | FR-002, FR-003, FR-004, FR-011, FR-013, FR-015 | Functional | Each behaviour holds for the case that exercises it | Every row of `docs/traceability.md` names the test that covers it | pass | test sources |

Not yet measured, and deliberately still `planned` in the traceability matrix: latency and
throughput (NFR-001, NFR-002, SC-002), which need the diff package's benchmarks;
determinism across thread counts (NFR-004, partly covered); sustained decision rate
(SC-004); usability (SC-005, NFR-007); the independent consumer (SC-006); and coverage of
the geometry and identity packages (NFR-009).

## Metrics

| Metric | Value | Measured on |
|---|---|---|
| Corpus cases scored | 4, changed-label, moving-button, occluded-button, noise | 2026-09-26 |
| Region detection F1 | 1.0000 | 2026-09-26 |
| False removals on noise | 0 | 2026-09-26 |
| Frames streamed for the memory measurement | 10,000 at 1920x1080 | 2026-09-26 |
| Peak heap over that stream | 24.0 MiB against a 128 MiB ceiling | 2026-09-26 |
| Allocation per frame at steady state | 8,220 bytes | 2026-09-26 |
| Line coverage, `internal/diff` | 91.8 percent | 2026-09-26 |

## Defects

Both functional defects below were found by the measurement rather than by reading the code,
which is the argument for measuring before writing the report.

| ID | Requirement | Symptom | Root cause | Resolution | Status |
|---|---|---|---|---|---|
| AUD-001 | FR-002, NFR-006 | A translated element scored F1 0.3333 | The engine reported one region spanning the whole movement, which covers pixels that did not change and misses the two areas that did | A translation now reports the area left and the area arrived in as two moved regions, each carrying the element's earlier position | fixed |
| AUD-002 | FR-005, NFR-005 | A frame pair differing only by noise reported one region as removed, and the occluded case scored F1 0.8000 | Every previous element that no changed area accounted for was classified as gone, so an element that simply did not change was reported as removed | That pass was deleted. An element whose pixels did not change is still on the screen and is absent from the delta by definition. Removal is now only reported when a changed area covers a previous element of a different size, which is observable | fixed |
| AUD-003 | FR-015, NFR-004 | `make memcheck` and `make accuracy` reported success while measuring nothing | `go test` treats a missing test name as "no tests to run" and exits zero | Both targets now ask `go test -list` whether the test exists and fail with the task number that will add it | fixed |

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
