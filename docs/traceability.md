# Requirement traceability

The report is graded on the specification being the source of truth for the
implementation. This file is the evidence: every requirement defined in `specs/`
appears here, with the task that implements it, the test that verifies it and the
evidence that the test ran.

Run `make check` after every change. `scripts/check_traceability.py` fails when:

- a requirement defined in a specification has no row here;
- a row names a specification that does not define its identifier, or omits the
  specification that does, or traverses outside the repository;
- a row appears twice for the same specification and identifier;
- a specification defines the same identifier twice, or an HTML comment is left
  unterminated;
- a status is not one of the allowed values;
- a row marked `in-progress` or `verified` names no task, names something other
  than an identifier such as `T012`, names a task that is not a task entry in the
  feature's `tasks.md`, or names a task while another task entry is the one that
  references the requirement;
- a row marked `verified` names no test or no evidence, or names a path that does
  not exist, is a directory, is absolute, or resolves outside the repository;
- a requirement of a feature that has a `tasks.md` is not referenced there, unless
  its row is `deferred` or `withdrawn`;
- a matrix row's identifier cell looks like an identifier but is malformed.

It warns, rather than fails, in two cases where it cannot judge a value: an
external URL used as evidence, or a value such as `pytest -q` that looks like a
command rather than a file path. A warning is a prompt to look, not a pass, and it
never appears for a path that could have been checked and does not exist.

A requirement is owned by a task when the task entry for that identifier, together
with its continuation lines, references the identifier exactly. `NFR-001` does not
count as `FR-001`, and an entry defined as a checkbox line, a numbered line or a
heading is required: plain bullet prose is not a task entry.

What it cannot check, and what therefore remains a reviewer's job: that the named
test actually exercises the requirement, and that the evidence shows what the row
claims. The check rejects a verification claim whose test or evidence file does not
exist, and rejects a task that is not a real task entry; it cannot tell a relevant
test from an irrelevant one that happens to exist.

Requirement definitions inside fenced code blocks are ignored, because a fence in a
specification is nearly always an example. Never put a real requirement inside a
fence: it will not be checked, and it will not appear in the coverage counts.

A `planned` row may name no task and no test, by design, so a green run is a floor
rather than a statement that everything is implemented. While `specs/` holds no
specification at all, the check warns and succeeds; `make check-strict` runs the
same check with `--require-specs`, which also fails when the specifications define
no requirement at all. Use `make check-strict` from the moment the first
specification exists, and switch the `.gitlab-ci.yml` job to it at the same time.

## Identifier namespaces

Identifiers are namespaced by specification. `FR-001` in `specs/001-alpha/spec.md`
and `FR-001` in `specs/002-beta/spec.md` are different requirements, and a row is
identified by its `Spec` cell plus its `ID` cell. A specification may cite another
specification's identifier, written as `specs/002-beta/spec.md#FR-001`; only bullet
lines of the form `- **FR-001**: ...` count as definitions.

## Status values

| Status | Meaning |
|---|---|
| `planned` | Written in the spec. May name no task yet, and needs no test. |
| `in-progress` | Task started. Must name a task listed in `tasks.md`; needs no test yet. |
| `verified` | Test passes and its output is recorded. Must name a task, an existing test file and existing evidence. |
| `deferred` | Deliberately out of scope for this iteration, with the reason in the report. |
| `withdrawn` | Requirement removed after approval. The identifier is kept and never reused. |

## Conventions

- `Task` holds a task identifier from the feature's `tasks.md`, for example `T012`.
- `Test` holds a path plus a test name, for example
  `tests/test_auth.py::test_rejects_short_password`. The part before `::` must
  exist.
- `Evidence` holds a repository path, for example `docs/vv/results.md` or a file
  under `docs/vv/evidence/`. An anchor such as `docs/vv/results.md#2026-10-12` is
  allowed. Prefer repository paths over external links, because a link cannot be
  checked after the project ends.
- One row per requirement. When several tests cover a requirement, name the most
  decisive one here and list the rest in `docs/vv/results.md`.

## Matrix

Rows are `planned` until a task implements them. `Task` and `Test` are filled in
when `/speckit.tasks` produces `tasks.md` and the tests exist. The example row has
been removed now that the first specification defines real requirements.

| ID | Requirement | Spec | User story | Task | Test | Evidence | Status |
|---|---|---|---|---|---|---|---|
| FR-001 | Accept two frames as files or raw buffers with declared dimensions and format | specs/001-frame-delta-engine/spec.md | US1 | T005 | `internal/frame/frame_test.go::TestNewRawAcceptsAValidFrame` | | in-progress |
| FR-002 | Report changed regions with normalized bounds | specs/001-frame-delta-engine/spec.md | US1 | T014 | `internal/diff/diff_test.go::TestChangedPanelIsReportedOnceWithCoveringBounds` | | in-progress |
| FR-003 | Classify regions as added, changed, removed or moved | specs/001-frame-delta-engine/spec.md | US1 | T018 | `internal/diff/diff_test.go::TestMovedElementIsReportedAsMoved` | | in-progress |
| FR-004 | Report change magnitude per region with a configurable reporting floor | specs/001-frame-delta-engine/spec.md | US1 | T016 | `internal/diff/diff_test.go::TestChangedPanelIsReportedOnceWithCoveringBounds` | | in-progress |
| FR-005 | Report no regions when differences stay below the noise floor | specs/001-frame-delta-engine/spec.md | US1 | T019 | `internal/diff/diff_test.go::TestNoiseBelowTheFloorReportsNothing` | | in-progress |
| FR-006 | Assign element identifiers stable across a stream and never reused in a session | specs/001-frame-delta-engine/spec.md | US2 | T025 | `internal/identity/identity_test.go::TestIdentifiersAreNeverReused` | `docs/vv/evidence/identity-2026-09-26.txt` | in-progress |
| FR-007 | Report identity confidence and mark uncertain identity instead of guessing | specs/001-frame-delta-engine/spec.md | US2 | T028 | `internal/identity/identity_test.go::TestReappearanceAfterOcclusionIsNewAndUncertain` | `docs/vv/evidence/identity-2026-09-26.txt` | in-progress |
| FR-008 | Compute a per-frame fingerprint stable under sub-threshold noise | specs/001-frame-delta-engine/spec.md | US3 | T035 | `internal/diff/diff_test.go::TestFingerprintTracksContentNotNoise` | | in-progress |
| FR-009 | Compare a frame against a stored fingerprint and report equal or different | specs/001-frame-delta-engine/spec.md | US3 | T037 |  |  | planned |
| FR-010 | Emit one self-contained document per frame when streaming | specs/001-frame-delta-engine/spec.md | US2 | T011 | `internal/stream/stream_test.go::TestFirstFrameCarriesTheConditionAndNoRegions` | | in-progress |
| FR-011 | Accept externally supplied regions of interest and restrict output to them | specs/001-frame-delta-engine/spec.md | US2 | T031 | `internal/diff/diff_test.go::TestRegionsOfInterestRestrictAndSuppress` | | in-progress |
| FR-012 | Accept a validated configuration document with documented defaults | specs/001-frame-delta-engine/spec.md | US1 | T009 | `internal/config/config_test.go::TestParseRejectsAMiscasedKeyWithASuggestion` |  | in-progress |
| FR-013 | Expose the same capabilities through CLI and library interfaces | specs/001-frame-delta-engine/spec.md | US1 | T010 | `cmd/screendelta/main_test.go::TestStreamWritesOneDocumentPerFrame` |  | in-progress |
| FR-014 | Declare a schema version in every emitted document | specs/001-frame-delta-engine/spec.md | US3 | T008 | `internal/delta/delta_test.go::TestDecodeRejectsAnUnknownVersion` | | in-progress |
| FR-015 | Fail explicitly without partial output on unusable input | specs/001-frame-delta-engine/spec.md | US1 | T006 | `internal/frame/frame_test.go::TestDecodePNGRejectsGarbage` |  | in-progress |
| FR-016 | Operate without network access and write only to the declared output path | specs/001-frame-delta-engine/spec.md | US1 | T044 |  |  | planned |
| NFR-001 | p95 delta latency at or below 12 ms per 1080p frame pair on one CPU core | specs/001-frame-delta-engine/spec.md | US1 | T040 | `internal/perf/perf_test.go::TestLatencyPercentiles` | `docs/vv/evidence/perf-2026-09-26.txt` | verified |
| NFR-002 | At least 30 frames per second sustained at 1080p on one CPU core | specs/001-frame-delta-engine/spec.md | US2 | T033 | `internal/perf/perf_test.go::TestThroughput` | `docs/vv/evidence/perf-2026-09-26.txt` | verified |
| NFR-003 | At most 128 MB resident memory streaming 10,000 frames, no growth with length | specs/001-frame-delta-engine/spec.md | US2 | T042 | `internal/stream/memory_test.go::TestMemoryCeiling` | `docs/vv/evidence/memory-2026-09-26.txt` | verified |
| NFR-004 | Byte-identical output for identical input, independent of thread count | specs/001-frame-delta-engine/spec.md | US2 | T013 | `internal/delta/delta_test.go::TestEncodeIsByteIdenticalRegardlessOfInputOrder` | | in-progress |
| NFR-005 | Zero false removals on the noise corpus | specs/001-frame-delta-engine/spec.md | US1 | T019 | `internal/diff/diff_test.go::TestNoiseFloorSeparatesNoiseFromChange` | `docs/vv/evidence/accuracy-2026-09-26.txt` | verified |
| NFR-006 | Region detection F1 at or above 0.98 on the generated corpus | specs/001-frame-delta-engine/spec.md | US1 | T022 | `internal/score/corpus_test.go::TestCorpus` | `docs/vv/evidence/accuracy-2026-09-26.txt` | verified |
| NFR-007 | A new user produces a delta document from the README within two minutes | specs/001-frame-delta-engine/spec.md | US1 | T049 |  |  | planned |
| NFR-008 | CPU only, no GPU, no network, runs on Linux and Windows | specs/001-frame-delta-engine/spec.md | US1 | T004 |  |  | planned |
| NFR-009 | At least 80 percent line coverage on the geometry and identity modules | specs/001-frame-delta-engine/spec.md | US2 | T046 | `internal/identity/identity_test.go::TestSetRulesChangesTheWindowWithoutLosingIdentities` | `docs/vv/evidence/coverage-2026-09-26.txt` | verified |
| NFR-010 | Versioned schema, and consumers can reject unknown versions | specs/001-frame-delta-engine/spec.md | US3 | T039 | | | planned |
| SC-001 | Detection F1 at or above 0.98 and zero false removals on 5,000 generated frame pairs | specs/001-frame-delta-engine/spec.md | US1 | T023 | `internal/score/corpus_test.go::TestCorpus` | `docs/vv/evidence/accuracy-2026-09-26.txt` | verified |
| SC-002 | Benchmark reports p95 at or below 12 ms and p99 at or below 25 ms | specs/001-frame-delta-engine/spec.md | US1 | T041 | `internal/perf/perf_test.go::TestLatencyPercentiles` | `docs/vv/evidence/perf-2026-09-26.txt` | verified |
| SC-003 | 10,000 frame stream within the memory ceiling with no identifier reuse | specs/001-frame-delta-engine/spec.md | US2 | T042 | `internal/stream/memory_test.go::TestRealPipelineRetention` | `docs/vv/evidence/memory-2026-09-26.txt` | verified |
| SC-004 | Capture, engine and stub detector sustain at least 20 decisions per second | specs/001-frame-delta-engine/spec.md | US2 | T047 |  |  | planned |
| SC-005 | An unfamiliar user reproduces a delta document from the README in under five minutes | specs/001-frame-delta-engine/spec.md | US1 | T049 |  |  | planned |
| SC-006 | A second consumer written independently against the schema works unchanged | specs/001-frame-delta-engine/spec.md | US3 | T048 |  |  | planned |
