# The vibe-coded pass

This directory holds the first pass at the project idea, written before the specification existed and without one to
check it against. It is kept because the course asks for the same idea to be built twice, once by prompting with no
specification and once under Spec Kit, and because the difference between the two is the point of the exercise.

The prompt, in full: "Write a Go program that takes two screenshots and prints the areas that changed as JSON."

That is the whole brief. There was no requirement list, no contract, no schema, no noise floor, no configuration, no
identity and no test. The result is 148 lines in one file, and it does the one thing the prompt asks.

## What it does

It decodes two PNGs of the same size, converts each pixel to a luma value with the standard integer weights, takes the
absolute difference, thresholds it, flood fills the changed pixels into 4-connected components, drops components under
a minimum area, and prints their bounding boxes as JSON with a class of `changed`.

```bash
go run ./vibe --min-area 40 before.png after.png
```

## What it does not do, measured against the engine

Both were run on the same eight real 1280 by 720 screenshots on the reference machine. The numbers are the ones this
pass produced rather than estimates.

| Property | The vibe-coded pass | The specification-driven engine |
|---|---|---|
| Document contract | Missing four of five required top-level fields and five of seven required region fields | Passes the published schema completely |
| An empty result | `{"regions":null}` | A document with an empty list and a fingerprint |
| Bounds | Integer pixels | Fractions of the frame, so the document does not depend on the frame size |
| Identity | None | An identifier per element, stable across a stream, never reused |
| Classification | Everything is `changed` | `added`, `removed`, `moved`, `changed` decided from evidence |
| Conditions | None | The declared conditions, including the first frame and a viewport change |
| Noise handling | A fixed threshold of 25, chosen by the prompt's author | A configurable noise floor, measured against the corpus |
| Determinism | Identical output on repeated runs, because the loop happens to be over a slice | Guaranteed by a total order on the regions, and asserted by a test |
| Region count on the seven consecutive real pairs | 3, 7, 4, 4, 5, 8, 12 | 3, 6, 4, 4, 5, 8, 10 |
| Lines | 148 | 1,229 in the comparison stage alone, 5,784 in the engine |
| Tests | None | 6,884 lines |
| Time per pair, including process start | 64 to 70 ms | 35 to 41 ms |

Three of those results are worth naming because they are not the ones a first pass would predict.

The vibe pass is **slower**, by roughly a factor of two, on the one operation both implement. Fewer lines is not less
work: the engine's comparison stage is longer because it measures a local noise floor, works on word-aligned words
instead of one pixel at a time, and reuses buffers, and the specification is what asked for those things.

The vibe pass is **deterministic** on these inputs, although nothing in it guarantees that. The region loop happens to
range over a slice, so the output order follows the scan order. A reviewer looking for a defect would find none here,
and would be right to call the property luck rather than design, which is exactly what a specification moves from luck
to a test.

The vibe pass **passes the noise pair** it was tried on, reporting nothing, because its fixed threshold of 25 sits
above the generated noise. It passes for a reason no one wrote down, and the engine's noise floor is the same property
with a value, a measurement and a test behind it.

## Where the rest of the comparison lives

`docs/vv/evidence/vibe-2026-09-28.txt` holds the commands and their output. Section 3 of the project report discusses
what the two passes say about prompting with and without a specification.
