# Phase 0 research: frame delta and element identity

Decisions that turn the specification into something implementable. Each entry
records the decision, why it was taken, and what was rejected, so section 4 of the
report can show the reasoning rather than the outcome alone.

## R1 Comparison channel and pixel format

**Decision**: decode every frame to 8-bit RGBA at native resolution, and compute
differences on a luma channel derived with integer weights. Colour is kept only for
region bounds, never for thresholding.

**Rationale**: luma is what changes when text, icons or controls change, and it is
insensitive to the sub-pixel colour fringing that LCD text rendering produces.
Staying at native resolution avoids resampling artefacts that look like changes.

**Alternatives rejected**: per-channel RGB comparison, which doubles the noise
surface; comparing after a fixed downscale, which loses thin strokes such as
one-pixel borders and cursors.

## R2 Change detection strategy

**Decision**: tile the frame into a fixed grid (default 16 by 16 pixels), compute a
per-tile mean absolute luma difference, mark tiles above the threshold as changed,
then merge connected tiles into regions with a union-find pass and grow each region
by a small margin.

**Rationale**: tiling gives a cache-friendly scan, a natural place to parallelise,
and connected components produce regions that match how a person describes a
change. Grown margins keep a region's bounds over the glyphs it contains.

**Alternatives rejected**: per-pixel differencing plus morphological opening, which
is slower and still needs the same merge step; whole-image similarity scores such as
SSIM, which answer "did it change" but not "where".

## R3 Noise floor

**Decision**: two thresholds, a per-tile mean-difference floor and a minimum
region area. Both are configurable, both have documented defaults, and a tile is
only changed when both the mean difference and the area test pass.

**Rationale**: capture noise and lossy compression raise the mean difference
uniformly but in tiny areas, while a real change concentrates energy. Requiring both
tests is what makes NFR-005, zero false removals, achievable rather than hopeful.

**Alternatives rejected**: a single global per-pixel threshold, which cannot satisfy
both a quiet screen and a JPEG-compressed one.

## R4 Region classification

**Decision**: classify by matching current regions against the previous frame's
regions. A current region with no match is `added`; a previous region with no match
is `removed`; a matched pair whose rectangles overlap closely is `changed`; a
matched pair with near-identical size but a small translation inside the motion
tolerance is `moved`, and the document carries both the previous and the current
bounds.

**Rationale**: classification falls out of matching, so there is one place where the
rules live and one place to test.

**Alternatives rejected**: classifying each region in isolation from pixel
statistics, which cannot distinguish a moved button from a deleted one plus an added
one.

## R5 Element identity

**Decision**: a greedy nearest-match over region rectangles with a cost combining
intersection over union, centroid distance and size difference, gated by thresholds.
An unmatched current region receives a new identifier; an unmatched previous region
is retired and its identifier is never reused. A region that reappears after being
absent gets a new identifier and an uncertainty marker.

**Rationale**: greedy matching is deterministic and explainable, which matters more
here than optimality, because the specification demands honesty about uncertainty
rather than the best possible guess. Retirement with no reuse is what makes
identifiers safe as references for a planner.

**Alternatives rejected**: Hungarian assignment, better in crowded scenes but
non-obvious when it fails; appearance descriptors, which need texture work and a
model, and belong to a later pipeline stage.

## R6 Fingerprint

**Decision**: fingerprint by downscaling to a fixed grid, quantising each cell into
a small number of luma buckets, and hashing the quantised grid. Equality uses a
tolerant comparison over the grid (bounded cell differences) with the strict hash
published alongside it for exact-match caches.

**Rationale**: a strict hash of pixels is unstable under noise, and a tolerant
comparison alone cannot be used as a cache key. Publishing both lets a consumer
choose, and keeps FR-008 and FR-009 honest.

**Amendment, 2026-09-26**: a cell holds the mean of a strided sample of its pixels, at a
stride of a quarter of a cell in each axis, rather than the mean of every pixel. The first
implementation summed every pixel of every cell, which was a second full walk over the
frame at 2.8 ms per 1080p pair, and measuring showed it cost more than the comparison the
fingerprint helps. A fingerprint is compared with tolerance by design, so a sixteenth of the
samples measures the same thing: the accuracy corpus is unaffected because it scores regions
rather than fingerprints, and the cell values remain deterministic for a given geometry.
Fusing the accumulation into the luma conversion was tried first and measured slower, 8.2 ms
against 6.7 ms per pair, because the per-pixel cell index cost more than the pass it
replaced.

**Alternatives rejected**: perceptual hashes tuned for images, which are optimised
for visual similarity rather than for "same screen, same plan".

## R7 Streaming and memory

**Decision**: one previous-frame buffer plus a pool of tile and region slices owned
by the stream. The identity map holds only live regions and is pruned when a region
is retired. No buffer is sized by stream length.

**Rationale**: NFR-003 is a ceiling over 10,000 frames, so growth must be impossible
by construction rather than unlikely in practice.

**Alternatives rejected**: keeping a history of frames, which would make occlusion
handling easier and the memory ceiling unreachable.

## R8 Determinism and parallelism

**Decision**: single-threaded by default. Optional parallel tiling aggregates
results in a fixed order, regions are sorted by top then left before emission, and
no output path iterates a map. Determinism is enforced by a test that runs the same
input under one and several workers and compares the documents byte for byte.

**Rationale**: NFR-004, byte-identical output, is cheap to guarantee now and
expensive to retrofit once consumers depend on ordering.

**Alternatives rejected**: relying on Go's randomised map iteration order and
sorting only at the JSON layer, which leaks nondeterminism into region merging.

## R9 Configuration

**Decision**: a JSON document with documented defaults, strict validation that
rejects unknown keys and out-of-range values, and an error that names the offending
field.

**Rationale**: FR-012 requires validation with clear errors. Rejecting unknown keys
turns a typo into a failure instead of a silently ignored setting, which is the
failure mode that costs an afternoon.

**Alternatives rejected**: environment variables, which are awkward for nested
thresholds and invisible in the repository.

## R10 Interfaces

**Decision**: one library API over the versioned document model, and a CLI with
three subcommands: `diff` for two frames, `stream` for newline-delimited documents
from a frame source, and `fingerprint` for a single frame. Exit codes follow the
convention already used in this workspace: 0 success, 1 usage error, 2 input error,
3 contract violation.

**Rationale**: FR-013 requires both surfaces from the same capabilities, and a
stable exit-code convention makes the tool usable as a pipeline stage without
parsing output.

## R11 Ground-truth corpus

**Decision**: generate the corpus. A generator renders local HTML pages in a
headless browser through the DevTools protocol, reads element boxes from the DOM,
and synthesises frame sequences from those boxes with known motion, appearance,
disappearance and occlusion. Only the generator, its seed and the resulting metrics
are committed; the images are not.

**Rationale**: DOM boxes are exact, so ground truth costs nothing and cannot be
wrong. Committing the generator instead of the images keeps the repository small and
makes the corpus reproducible, which section 5 of the report needs.

**Alternatives rejected**: hand-labelling, which does not scale and is itself a
source of error; public screenshot datasets, useful for a sanity check but not
aligned to this engine's specific change semantics.

## R12 Measurement method

**Decision**: accuracy is IoU-matched precision, recall and F1 at an IoU threshold
of 0.5, plus a separate count of false removals on noise-only pairs. Performance is
p50, p95 and p99 per frame pair from a committed benchmark, with `GOMAXPROCS=1`,
memory sampled over a 10,000 frame stream, and results compared with `benchstat`
against the previous commit.

**Rationale**: the specification states thresholds, so the measurement method is
part of the contract, not a detail of the report. Percentiles rather than a mean are
what make the latency requirement meaningful.

## R13 Coordinates, DPI and multiple monitors

**Decision**: bounds are emitted normalized to the frame extent, and every document
records the frame's pixel dimensions and a declared scale factor. A frame whose
dimensions or declared scale differ from its predecessor is reported as an explicit
change of viewport rather than diffed as content.

**Rationale**: a scale change would otherwise produce a screenful of false changes,
which is the single most likely way to make the engine look broken.

## R14 Deferred decisions

Recorded so the plan does not silently assume them:

- The occlusion window after which an identity is retired rather than marked
  uncertain. A starting value is chosen during implementation and then tuned against
  the corpus.
- Whether `moved` should be reported for large translations or split into removed
  and added. The corpus will decide.
- Appearance-based re-identification after occlusion. Deferred until R16, which
  reverses this entry and says why: an identity layer that cannot tell a cover from a
  content change cannot keep the promise FR-007 makes.
- Any form of capture implementation beyond file and stdin adapters.

## R15 One memory of where the elements are

**Decision**: the identity map owns an element's footprint, and the classifier reads it.
The differ stops keeping its own list of the previous frame's changed areas.

**Rationale**: two layers were keeping geometry, and they disagreed. The classifier kept the changed
areas of the last frame; the identity map kept the union of the areas that named each element. Both
were called the element's position. A change confined to one part of a large element therefore
rewrote the identity layer's footprint to that part, and a change to another part in the next frame
looked like a new element. Falsified by the review of 2026-09-26 and recorded as AUD-020.

The identity map is the layer that should own it, for three reasons: it already owns lifecycle, so it
is the only layer that persists across frames; the changed areas are its input rather than a rival
model of the screen; and it already needs them to tell an element that is still visible from one that
has been covered.

**Alternatives rejected**: keeping both memories and reconciling them each frame, which is the defect
with extra steps.

## R16 An element's footprint translates, and only absorbs new ground

**Decision**: when a region names an element, its footprint becomes
`translate(footprint, d) union (observed minus footprint)`, where `observed` is the union of the
regions that named it this frame, and `d` is the translation among at most nine candidates that best
explains the observation, clipped to the motion tolerance, with ties broken deterministically.

**Rationale**: it is the only rule of the four considered that is correct for a moving element, a
growing element and a still element at once.

| Rule | Moving | Growing | Shrinking | Still |
|---|---|---|---|---|
| Keep the last full footprint | lost once the move exceeds the tolerance | lost once the change lands in the grown part | stale | correct |
| Decaying envelope | follows but lags by the step every frame, without bound | correct | stale | correct |
| Translation estimate alone | correct | missed unless zero is allowed, and then growth is not captured | cannot see it | correct |
| Union only (what the code did) | lags one step and reports a union box rather than an element | correct | shrinks to the changed strip, which is AUD-020 | correct |
| Translation plus new ground | exact, the vacated strip is dropped | the new territory is outside the old footprint, so it is absorbed | stale, deliberately | correct |

Shrinkage is the honest failure: an element's extent is only ever revealed by change, so a footprint
that nothing has contradicted is kept rather than guessed away. The division of labour this creates is
written into the data model: `bounds` is the measured changed area and stays exact, while
`previousBounds` is the engine's belief about the element and may be larger than it after a shrink.

**Alternatives rejected**: appearance-based extent estimation, which is the next stage's job and not
this one's; and a shrinking envelope, which invents an extent rather than admitting ignorance.

## R17 A cover cannot be told from a content change in one frame pair

**Decision**: say so, and give the engine one more piece of evidence rather than a guess. Each element
keeps a 16 byte appearance signature: a four by four grid of luma means over its footprint, sampled
with a stride capped so the cost is bounded for any rectangle, compared with a fixed tolerance above
the noise two frames can differ by and below a typical repaint.

With that evidence:

- a changed area that contains at least nine tenths of a tracked element's interior and exceeds its
  footprint by more than the growth margin is a cover or a replacement, so the tracked element is
  reported as `removed` and the area as `added`, with different identifiers. Today the area is called
  `changed` and inherits the covered element's identifier, which is a confident wrong answer;
- a changed area that matches a tracked element geometrically, whose signature differs from that
  element's, while a retired element matches both the geometry and the signature and is materially
  closer in signature, is a return: the tracked element is retired and the area is reported as `added`
  with a new identifier marked uncertain.

**Rationale**: from a single frame pair the question is undecidable, and it is worth saying that
plainly rather than shipping a heuristic that pretends otherwise. A changed area that contains an
element is consistent with a cover, a replacement, a growth that repainted itself, and a full repaint;
colour and size relationships are consistent with all four. What makes it decidable enough is one
extra piece of state per element and the fields the contract already has for recording evidence
instead of proof: `identityUncertain` and `identityConfidence`.

**Residual ambiguity, recorded rather than papered over**: a cover whose footprint equals the
element's leaves no margin to exceed, so it stays `changed` and the covered element keeps its identity,
which is the boundary stated in the interface contract rather than a safety property. A growth that
repaints its interior is reported as removed and added, which costs an identity on that frame. Neither
failure mode hands a consumer a confident wrong match for content it has not seen: the first keeps a
handle on an element that is still where it was and looks the same, the second retires a handle and
allocates a new one.

A third failure mode was found by a review after this section was written and is now closed, though not by
the means first proposed. A return whose content occupies only part of the changed area was not recognised,
because the appearance compared was the appearance of the whole area, which is a mixture as soon as the
content is smaller than the area. Measuring the appearance of the element's old footprint instead was the
first proposal and it does not settle the question either: a partial return leaves that footprint a mixture
too. What closed it is making the comparison relative rather than absolute. The engine no longer asks whether
the area matches the element that left, which a mixture never does, but whether it looks more like that
element than like the one on the screen, and requires the difference to be material.

The remaining boundary is stated rather than hidden: when the mixture is genuinely closer to the element on
the screen than to the one that left, the engine reports a change of the element on the screen. Two greys a
few levels apart with a partial return between them are such a case. Distinguishing it needs sub-region
matching rather than a summary of the area, which this section rejects on cost, so a planner that needs
certainty there must supply a region of interest tight enough to exclude the surrounding content.

**Alternatives rejected**: storing a pixel patch per element, which costs memory and buys no more
discrimination than a coarse mean; comparing against the previous frame's changed areas, which is the
behaviour that makes a cover inherit the covered element's identity; requiring the covering area to be
larger, which is true of any growth and therefore discriminates nothing; and a new schema class or
condition, when the schema is fixed and `identityUncertain` already carries the meaning.

## R18 A disappearance stays a change

**Decision**: an element whose area changed in place, leaving its footprint the same size, stays
reported as `changed`. It is not reported as `removed`.

**Rationale**: the evidence that would justify a removal is that the area no longer resembles the
element and does resemble what surrounds it. The second half fails exactly where it matters: an
element whose fill is close to its neighbourhood, a grey button on a grey dialog, would be called
removed the moment its label changed. That is a false removal, and false removals are what NFR-005 and
SC-001 forbid, on a corpus built to catch them. The engine therefore reports what it can see, and a
consumer reads `magnitude` against the bounds: a value near one over the element's whole footprint is
consistent with a disappearance, a small value with a subtle change. The reasoning is recorded in the
data model and in the report's threats to validity.

**Alternatives rejected**: reporting a removal whenever the area is flat and different, which is a
guess that cannot be told from a repaint, and which would trade a truthful report for a wrong one in
the case the corpus explicitly measures.

## R19 The acceptance test for element identity, restated

**Decision**: user story 2's second acceptance scenario is stated as the two events the engine can
distinguish, rather than as one it cannot:

1. **Cover**: given an element covered by a larger one, the covered element is reported as `removed`
   naming its identifier, and the covering area as `added` with a different identifier, and the
   covered identifier is never reused.
2. **Return**: given the covering area later showing the covered element's content again, the covering
   element is reported as `removed` and the returning content as `added` with a newly allocated
   identifier whose `identityUncertain` is true and whose `identityConfidence` is greater than zero. The
   number measures the evidence and the flag carries the claim: a perfect appearance match over a perfect
   overlap is still not proof of identity.

**Rationale**: the original wording asked for something a pixel-only stage cannot know, which is why
the first implementation could not satisfy it and the review could falsify the claim. A scenario that
names the two events separately is testable, and each part is falsifiable on its own. The identifier
guarantees are unchanged and are the part of the scenario that carries the promise.

**Alternatives rejected**: keeping the original wording and marking the scenario unsatisfied, which
would leave a graded scenario permanently failing rather than a requirement met with a documented
boundary.
