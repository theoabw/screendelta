# ScreenDelta Constitution

A component that reports what changed between screen frames, and which element is
which across a stream of frames, so that a slow planner can look at a delta
instead of a whole screen. The system name is provisional.

## Core Principles

### I. The Specification Is the Source of Truth

Every behaviour in the implementation traces to a requirement identifier in
`specs/001-frame-delta-engine/spec.md`. No code is written for behaviour that no
requirement covers. Identifiers are stable once approved, withdrawn requirements
keep their identifier and are marked, and a change to behaviour starts as a change
to the specification in its own commit.

### II. Verification Is Machine-Checked

Every requirement is verified by a test, a measurement or a generated corpus, and
the evidence is committed. No requirement is marked verified on the strength of a
demonstration or a screenshot alone. Where a property can be stated, it is checked
by property-based or differential testing rather than by example. Performance
requirements are verified by a committed benchmark that reports percentiles, not
by a single timing.

### III. Never Fabricate a Result

The engine reports uncertainty instead of guessing. A region that vanishes below
the noise floor is not reported as removed. An identifier is never reused within a
session. An element whose identity cannot be re-established after occlusion is
given a new identifier and an explicit uncertainty marker, never a silent match.
Failing loudly on unusable input is preferred to producing a plausible document
from data that does not support one.

### IV. Bounded and Streamable by Design

The engine streams. Memory does not grow with stream length, thresholds and
ignored regions are configuration rather than code, and the same input produces
byte-identical output regardless of host, thread count or scheduling. Work that
cannot be expressed within the memory ceiling is refused with a clear error rather
than accepted and thrashed.

### V. Chainable Interface First

The delta document is a versioned, documented schema, and it is the product. Any
capture source can feed the engine and any detector, OCR stage or planner can
consume it, so no stage may depend on another stage's internals. Externally
supplied regions of interest are first-class input, because the later pipeline
will want to constrain the engine rather than replace it. Adding a consumer must
never require changing the document format outside the versioning rules.

## Quality and Performance Standards

- Latency: 95th percentile at or below 12 ms and 99th percentile at or below 25 ms
  per 1080p frame pair on one CPU core, measured by the committed benchmark on the
  reference machine, which is recorded with the results.
- Throughput: at least 30 frames per second sustained at 1080p on one core.
- Memory: at most 128 MB resident while streaming 10,000 frames, with no growth
  attributable to stream length.
- Accuracy: region detection F1 at or above 0.98 and zero false removals against
  the generated ground-truth corpus.
- Portability: CPU only, no GPU, no network access at runtime, no writes outside
  the declared output path, runs on Linux and Windows.
- Determinism: byte-identical output for identical input, enforced by a test that
  runs the same input under different thread counts.

The implementation language is not fixed by this constitution. It is chosen in the
plan with a decision record, against these constraints: single-binary or
single-command distribution, predictable numeric behaviour, and a native image
processing path fast enough for the latency budget.

## Development Workflow

1. Specification, then plan, then tasks, then code, in that order, with the gates in
   `CONTRIBUTING.md` respected.
2. `make check` passes before every commit, and `make check-strict` from the moment
   a specification exists.
3. Every requirement identifier appears in `docs/traceability.md` with the task
   that implements it and the test that verifies it.
4. Every substantive agent session is recorded in `docs/prompt-log/`, including
   what was rejected and why.
5. Review happens per merge request: a self-review against `CONTRIBUTING.md` and an
   independent agent review briefed to falsify the change.
6. Performance and accuracy claims in the README are regenerated from the committed
   benchmark and corpus, never typed by hand from an earlier run.

## Governance

This constitution supersedes local convenience. Amendments require a decision
record in `docs/adr/`, a stated reason and a migration note for anything already
built against the previous version. Compliance is checked by `make check` and by
review, and a deviation found in review is either fixed or recorded as an accepted
deviation in the audit log. Complexity that cannot be traced to a requirement is
removed rather than documented.

**Version**: 1.0.0 | **Ratified**: 2026-09-26 | **Last Amended**: 2026-09-26
