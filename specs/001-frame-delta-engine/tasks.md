# Tasks: Frame Delta and Element Identity Engine

**Input**: `specs/001-frame-delta-engine/` (spec, plan, research, data model, contracts, quickstart)

**Format**: `[ID] [P?] [Story] Description` where `[P]` marks a task that can run in parallel with its neighbours because it touches different files. Tests appear inside the story they verify rather than as a separate phase, because the specification states thresholds that only a test can settle.

## Phase 1: Setup

- [ ] T001 Create the Go module with the toolchain version pinned (`go.mod`)
- [ ] T002 Add build, test, lint, vet, bench, corpus, accuracy and memcheck targets (`Makefile`)
- [ ] T003 Add CI jobs for traceability, tests and lint (`.gitlab-ci.yml`)
- [ ] T004 Add cross-compilation for `linux/amd64` and `windows/amd64` (`scripts/build.sh`)

## Phase 2: Foundational

- [ ] T005 Decode PNG and raw RGBA frames with dimension and format validation (`internal/frame`) (FR-001)
- [ ] T006 Validate frame invariants and detect a viewport change against the predecessor (`internal/frame`) (FR-015)
- [ ] T007 Define error values that carry the frame sequence and the offending field (`internal/frame`) (FR-015)
- [ ] T008 Implement the document model and JSON encoding with `schemaVersion` (`internal/delta`) (FR-014)
- [ ] T009 Implement configuration defaults, parsing and strict validation (`internal/config`) (FR-012)
- [ ] T010 Add the CLI skeleton: subcommands, flags and exit codes (`cmd/screendelta`) (FR-013)
- [ ] T011 Implement the stream loop with stream-owned buffer pools (`internal/stream`) (FR-010)
- [ ] T012 Build the corpus generator: render pages over CDP, read DOM boxes, synthesise sequences (`tools/corpusgen`)
- [ ] T013 Add the determinism test comparing one worker against several (`internal/stream`) (NFR-004)

## Phase 3: User Story 1, report what changed (P1)

- [ ] T014 Implement tiling and normalized region bounds (`internal/diff`) (FR-002)
- [ ] T015 Implement the luma channel and per-tile mean difference (`internal/diff`) (FR-002)
- [ ] T016 Implement the two-part noise floor and per-region magnitude (`internal/diff`) (FR-004)
- [ ] T017 Implement connected-component merging and region growth (`internal/diff`) (FR-002)
- [ ] T018 Implement classification of added, changed, removed and moved (`internal/diff`) (FR-003)
- [ ] T019 Add noise-only frame pairs and the zero-false-removal test (`testdata/noise`) (FR-005, NFR-005)
- [ ] T020 Add the memory ceiling test over 10,000 frames (`internal/stream`) (NFR-003)
- [ ] T021 [P] Add property tests for bounds invariants: inside the frame, magnitude in range (`internal/diff`) (FR-002, FR-004)
- [ ] T022 Add the IoU matching harness that scores regions against ground truth (`internal/score`) (NFR-006)
- [ ] T023 Add the accuracy test over the generated corpus, reporting F1 and false removals (`internal/score`) (NFR-006, SC-001)
- [ ] T024 Add golden documents for the diff shape and ordering (`testdata/golden`) (FR-002, NFR-004)

## Phase 4: User Story 2, keep identity across frames (P2)

- [x] T025 Implement the identity map with allocation, retirement and no reuse (`internal/identity`) (FR-006)
- [x] T026 Implement greedy nearest matching on IoU, centroid distance and size (`internal/identity`) (FR-006)
- [x] T027 [P] Add property tests for identifier stability and non-reuse across a stream (`internal/identity`) (FR-006)
- [x] T028 Implement occlusion handling and the uncertainty marker (`internal/identity`) (FR-007)
- [x] T029 Add the occluded-element corpus case and its recovery test (`tools/corpusgen`, `internal/identity`) (FR-007)
- [x] T030 Implement newline-delimited streaming with a flush per document (`internal/stream`) (FR-010)
- [x] T031 Implement region-of-interest restriction and clipping, including empty-list semantics (`internal/diff`) (FR-011)
- [x] T032 Add the region-of-interest test, including the difference between absent and empty (`internal/diff`) (FR-011)
- [x] T033 Add the throughput check at 1080p, asserting at least 30 frames per second (`internal/stream`) (NFR-002)
- [x] T034 Add the allocation-growth guard: allocations per frame must not rise with stream length (`internal/stream`) (NFR-003)

## Phase 5: User Story 3, fingerprint a screen (P3)

- [x] T035 Implement grid downscaling and luma quantisation (`internal/fingerprint`) (FR-008)
- [x] T036 Implement strict hashing over the quantized grid (`internal/fingerprint`) (FR-008)
- [x] T037 Implement tolerant comparison bounded by `maxCellDelta` (`internal/fingerprint`) (FR-009)
- [x] T038 Add fingerprint tests: a noisy copy is equal, a materially changed copy is different (`internal/fingerprint`) (FR-008, FR-009)
- [x] T039 Implement the fingerprint subcommand and decode-time rejection of an unknown schema version (`cmd/screendelta`) (FR-009, FR-014, NFR-010)

## Phase 6: Polish and cross-cutting concerns

- [ ] T040 Add the benchmark harness reporting p50, p95 and p99 per frame pair (`internal/diff`) (NFR-001)
- [ ] T041 Run the benchmark on the reference machine and commit the output (`docs/vv/evidence`) (NFR-001, SC-002)
- [ ] T042 Add memory sampling to the stream benchmark and fail when the ceiling is exceeded (`internal/stream`) (NFR-003, SC-003)
- [x] T043 Add end-to-end tests for every quickstart scenario (`tests/e2e`) (FR-005, FR-015)
- [x] T044 Verify the no-network and no-stray-write behaviours by test (`internal/stream`) (FR-016)
- [ ] T045 Add the Windows cross-build check and platform smoke test (`.gitlab-ci.yml`) (NFR-008)
- [ ] T046 Add coverage reporting for the geometry and identity packages (`Makefile`) (NFR-009)
- [ ] T047 Add the pipeline demonstration: capture, engine, stub detector, decision rate (`tools/demo`) (SC-004)
- [x] T048 Write an independent second consumer against the schema alone (`tools/consumer`) (SC-006, FR-013)
- [ ] T049 Add the CLI usability pass: help text, error wording, README quickstart (`cmd/screendelta`, `README.md`) (NFR-007, SC-005)
- [ ] T050 Report benchmark, coverage and memory results into the V&V record (`docs/vv/results.md`) (SC-001, SC-002, SC-003)
- [ ] T051 Write the acceptance script for external testers and run it (`docs/vv/acceptance.md`) (SC-005)
- [ ] T052 Move traceability rows to verified with their test and evidence (`docs/traceability.md`) (FR-013)
- [ ] T053 Regenerate every README number from committed benchmark output (`README.md`) (NFR-001, NFR-002)

## Dependencies and execution order

- Phase 1 has no dependencies.
- Phase 2 depends on Phase 1, and T008 and T009 must land before any story phase, because both the document and the configuration are contracts the stories build on.
- T012, the corpus generator, must land before T019, T023 and T029, because those tests consume its output.
- Phase 3 depends on Phase 2. Phase 4 depends on T014 to T018, because identity is defined over regions. Phase 5 depends only on Phase 2, so it can run in parallel with Phases 3 and 4.
- Phase 6 depends on the story phases it measures. T040 to T042 need the code they benchmark; T050 needs T041 and T042; T052 needs everything verified.

## Parallel opportunities

- T021 and T027 are property tests over different packages.
- Phase 5 can be built by a second worker while Phase 3 is in progress, because the fingerprint shares only `internal/frame` with the diff path.
- Within Phase 6, T045, T046 and T048 touch CI, the Makefile and a new tool respectively, and do not overlap.

## Implementation strategy

The smallest useful increment is Phase 1, Phase 2 and Phase 3: an engine that
reports what changed between two frames, verified against the noise corpus and the
accuracy harness. That is the MVP, and it is demonstrable on its own. Identity
follows, because a planner needs it, and the fingerprint follows last, because a
pipeline can run without it.

Every task ends with `make check-strict` green and its traceability row updated in
the same commit, per `CONTRIBUTING.md`.

## Requirement coverage

Every requirement maps to at least one task, which is what the traceability check
enforces once this file exists.

| Requirement | Tasks |
|---|---|
| FR-001 | T005, T006 |
| FR-002 | T014, T015, T017, T021, T024 |
| FR-003 | T018 |
| FR-004 | T016, T021 |
| FR-005 | T019, T043 |
| FR-006 | T025, T026, T027 |
| FR-007 | T028, T029 |
| FR-008 | T035, T036, T038 |
| FR-009 | T037, T038, T039 |
| FR-010 | T011, T030 |
| FR-011 | T031, T032 |
| FR-012 | T009 |
| FR-013 | T010, T048, T052 |
| FR-014 | T008, T039 |
| FR-015 | T006, T007, T043 |
| FR-016 | T044 |
| NFR-001 | T040, T041, T053 |
| NFR-002 | T033, T053 |
| NFR-003 | T020, T034, T042 |
| NFR-004 | T013, T024 |
| NFR-005 | T019 |
| NFR-006 | T022, T023 |
| NFR-007 | T049 |
| NFR-008 | T004, T045 |
| NFR-009 | T046 |
| NFR-010 | T039 |
| SC-001 | T023, T050 |
| SC-002 | T041, T050 |
| SC-003 | T042, T050 |
| SC-004 | T047 |
| SC-005 | T049, T051 |
| SC-006 | T048 |
