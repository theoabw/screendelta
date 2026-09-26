# Implementation Plan: Frame Delta and Element Identity Engine

**Branch**: `001-frame-delta-engine` | **Date**: 2026-09-26 | **Spec**: `specs/001-frame-delta-engine/spec.md`

**Input**: Feature specification from `/specs/001-frame-delta-engine/spec.md`

## Summary

A Go library and single-binary CLI that turns a stream of screen frames into one
delta document per frame: changed, added, removed and moved rectangular regions with
normalized bounds and a change magnitude, session-stable element identifiers with
explicit uncertainty, and a noise-stable screen fingerprint. Deterministic,
single-core, memory-bounded, and chainable through a versioned JSON schema. No
model, no OCR, no element classification: those are later pipeline stages.

## Technical Context

**Language/Version**: Go 1.24 or newer, with the toolchain version pinned in `go.mod`

**Primary Dependencies**: standard library (`image/png`, `encoding/json`, `flag`, `hash`), `golang.org/x/image` for JPEG and other decoders, `pgregory.net/rapid` for property tests, `golang.org/x/perf/cmd/benchstat` for benchmark comparison. No third-party dependency on the hot path.

**Storage**: none. The engine is stateless between frames except for the identity map it owns for the life of a stream. Fingerprints can be written to a caller-specified file for cache keys; no database.

**Testing**: `go test` with table-driven unit tests, property tests for identity and moved-region rules, golden fixtures for document shape, a generated accuracy corpus, and `go test -bench` measured with `benchstat`. Coverage via `go test -cover` on the geometry and identity packages.

**Target Platform**: Linux and Windows, x86-64, CPU only, no GPU. Cross-compiled with `GOOS`/`GOARCH`.

**Project Type**: library plus CLI in one module.

**Performance Goals**: p95 at or below 12 ms and p99 at or below 25 ms per 1080p frame pair on one core; at least 30 frames per second sustained end to end; at most 128 MB resident over 10,000 frames.

**Constraints**: no network access at runtime, no writes outside the declared output path, byte-identical output for identical input, `GOMAXPROCS=1` for measured runs, allocation per frame must not grow with stream length.

**Scale/Scope**: frames up to 4K, streams of 10,000 frames and beyond, up to a few hundred regions per frame, one screen per stream.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | How this plan satisfies it |
|---|---|
| I. The specification is the source of truth | Every package and test in the structure below maps to requirement identifiers. The plan adds no behaviour that `spec.md` does not require. |
| II. Verification is machine-checked | Accuracy comes from a generated corpus with known ground truth, identity and moved-region rules get property tests, performance comes from committed benchmarks reported as percentiles, and the README numbers are regenerated rather than typed. |
| III. Never fabricate a result | The noise floor, the identifier rule and the uncertainty marker are implemented as invariants with tests, not as heuristics described in prose. NFR-005 is a zero-tolerance test over the noise corpus. |
| IV. Bounded and streamable | One frame pair in memory plus fixed pools; the identity map is the only per-stream growth and it is bounded by the number of live regions. No buffer is sized by stream length. |
| V. Chainable interface first | The delta document is the product and lives in its own package with its own tests, so a second consumer can be written against it without importing the engine internals. Schema version is a field in the document. |

No violations. The one deliberate trade-off, Go's collector against the p99 target,
is recorded in `docs/adr/0003-go-implementation.md` with its mitigation and its
fallback.

## Project Structure

### Documentation (this feature)

```text
specs/001-frame-delta-engine/
├── plan.md              # This file
├── research.md          # Phase 0: image comparison, identity and fingerprint decisions
├── data-model.md        # Phase 1: document, region, identity and configuration model
├── quickstart.md        # Phase 1: runnable validation scenarios
├── contracts/           # Phase 1: delta document schema, CLI contract, library contract
├── checklists/          # requirements quality checklist, maintained by the specify and clarify steps
└── tasks.md             # Phase 2: created by the tasks step
```

### Source Code (repository root)

```text
cmd/
└── screendelta/
    └── main.go                 # thin CLI entry point, argument parsing only

internal/
├── frame/                      # decode, validate, downscale, viewport metadata
├── diff/                       # tile comparison, connected components, region classification
├── identity/                   # session-stable identifiers, matching, uncertainty
├── fingerprint/                # noise-stable screen fingerprint
├── delta/                      # the versioned document model and its JSON encoding
├── config/                     # configuration document, defaults, validation
└── stream/                     # frame source adapters (files, stdin), stream loop, pools

testdata/
├── corpus/                     # generated ground-truth corpus metadata, not the images
├── golden/                     # golden delta documents for shape and determinism tests
└── noise/                      # noise-only frame pairs for the zero false removal test

tools/
└── corpusgen/                  # generator: synthetic frame sequences and DOM-derived boxes

docs/
└── vv/evidence/                # committed benchmark and accuracy output
```

**Structure Decision**: one Go module with `internal/` packages split by
responsibility, because the constitution makes the delta document a product in its
own right and the geometry and identity rules the two areas that carry the accuracy
requirements. `delta` and `config` are importable without the image code, which is
what lets a second consumer be written against the schema alone. The corpus
generator is a separate command under `tools/` so that test data creation never
becomes a runtime dependency, and the generated images themselves are not committed:
only the generator, its seed and the resulting metrics are, which keeps the
repository small and the corpus reproducible.

## Performance Methodology

Performance claims are only credible if the method is fixed before the first
measurement, so the method is part of the plan:

1. Measure on the reference machine recorded in `docs/vv/evidence/benchmark-host.txt`,
   with `GOMAXPROCS=1`, no other load, and the CPU governor set to performance.
2. Report p50, p95 and p99 per frame pair, never a mean alone, from
   `go test -bench` plus `benchstat` across at least three runs.
3. Record resident memory over a 10,000 frame stream, sampled from the process, and
   fail the run when it exceeds the ceiling.
4. Compare against the previous commit's numbers, so a regression is visible in the
   pull request rather than discovered in the report.
5. Regenerate every number quoted in the README from the committed benchmark output.

## Complexity Tracking

No constitution violations. Two decisions worth watching:

| Decision | Why needed | Simpler alternative rejected because |
|---|---|---|
| Buffer pools owned by the stream | The p99 target and the memory ceiling are both threatened by naive per-frame allocation, and Go's collector makes tail latency the binding constraint | Allocating per frame is simpler, but it moves the risk into the measurement the project is graded on, and the pools are a few hundred lines with their own tests |
| Separate `identity` package with property tests | Identity across occlusion is the hardest rule in the specification and the one most likely to be wrong in a way that looks plausible | Folding matching into `diff` would make the rule untestable in isolation and couple an accuracy requirement to image code |
