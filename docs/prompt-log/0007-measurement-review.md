# 0007 Review of the measurement, and what it falsified

- Date: 2026-09-26
- Author: Theo Wilenius (owner), with an agent as authoring agent
- Agent and model: dsh agent session, codex review subagent at low reasoning effort for the
  two independent review passes
- Spec Kit command: `/speckit.implement` for the measurement tasks, `/speckit.analyze`
  captured in `docs/analysis/`
- Artifacts produced: `internal/corpus` with an in-memory case API and a pixel oracle,
  `internal/score` with maximum-cardinality matching, `internal/stream/memory_test.go`
  measuring resident memory, the fix that removed the tile grid from `internal/diff`, and
  the revised evidence under `docs/vv/evidence/`
- Related requirement IDs: FR-002, FR-003, FR-005, NFR-003, NFR-005, NFR-006, SC-001, SC-003
- Related commit: 91ff96b, 914fe49, aeb503f, 7d4d0d1

## Prompt (verbatim)

The prompt that produced this round was the owner's request to turn the project's promises
into measurements:

> I want the accuracy and memory claims measured, not asserted. Build the harnesses, run
> them, and record the numbers as evidence.

The review prompt was the standing one used for an independent pass:

> Your job is to falsify the claims below, not to comment on style. For each, either
> demonstrate a counterexample in a scratch copy or report that you could not and say what
> you tried.

## Intent

The intent was to replace assertion with measurement before writing the report, on the
principle that a claim nobody measured is a claim the report cannot defend. The review
prompt was phrased as falsification rather than review because the first round had already
shown that a read-only pass over code finds different things than an attempt to break it.

## What happened

The measurement was built and it passed, and that was the problem. The first revision scored
27 frame pairs and 9 expected regions, and reported F1 1.0000. The independent review then
falsified the measurement itself rather than the engine:

1. An engine that classified every region as `changed` still scored F1 1.0000, because the
   suite scored localisation only and never checked what class a region carried.
2. A mutant that emitted spurious `removed` regions on the transitions the expectations did
   not cover passed both scoring tests, because only the designed pairs were scored.
3. `make accuracy` claimed 5,000 pairs while scoring 27.
4. The answer key was stated by the generator rather than derived from the images, and the
   review showed it demanded regions for frame pairs where nothing had changed, missed an
   overlay clipped at the frame edge, and asked for one box where the pixels changed in two
   places.
5. An engine retaining 512 bytes per frame passed both memory tests, because the growth
   allowance was 8 MB and only the heap was sampled.

The consequence was not a patch to the harness but a change of definition: ground truth now
comes from the rendered pixels, every adjacent pair is scored, the sample size is enforced,
classes are asserted where they are unambiguous, and the memory bound is one megabyte of
resident growth, which the demonstrated leak cannot survive.

The revised measurement then found three real defects in the diff core, which the first one
had been hiding: a tile grid that cut genuine changes at tile boundaries and reported one
change twice, tiles that merged two elements into one region, and remembered geometry that
was erased by a frame pair that changed nothing.

## What was corrected in the specification and the documents

- `specs/001-frame-delta-engine/contracts/interfaces.md` now states that a two frame `diff`
  can only report `changed`, because `moved` and `removed` both need a stream's history.
- `docs/vv/results.md` withdrew the first measurement rather than quietly replacing it, and
  records it as AUD-007. A measurement that reported success without measuring is the failure
  this chapter exists to catch, so removing the evidence of it would have defeated the point.
- The reason the sweep scores localisation without classes is recorded in the test itself:
  whether a uniformly different area is a changed element or a removed one cannot be decided
  from pixels, so asserting classes there would assert the implementation's guess rather than
  the specification's requirement.

## What the agent got wrong, and what changed in the prompts

The agent treated a passing test as evidence. Both the harness and the engine passed, and the
numbers were written into the V&V record as verified, when what had actually been verified was
that the harness agreed with itself. The prompt that produced this round has been changed for
subsequent rounds: a measurement is not done until an attempt to pass it with a deliberately
wrong implementation has failed. That mutation step is now part of the round, and it is what
turned all five findings above into visible failures.

A second prompt change: the review prompt now asks explicitly whether the measurement can be
gamed, not only whether the code is correct. The first review round asked about correctness and
found code defects; this round asked whether the evidence was real and found that it was not.

## Verification

```text
make accuracy   27,203 pairs, precision 1.0000, recall 1.0000, F1 1.0000,
                0 false removals, every asserted class correct
make memcheck   10,000 frames at 1080p in 2.46 s, peak resident 22.5 MiB against a 128 MiB
                ceiling, heap identical at frame 1,000 and frame 10,000
go test ./...   all packages pass
make check-strict  32 requirements, 32 matrix rows, 5 verified, 0 errors
```

The leak check was verified in both directions: with 512 bytes per frame retained inside the
engine both memory tests fail and name the growth, and after the change is removed both pass.
