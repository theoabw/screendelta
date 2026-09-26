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
- Appearance-based re-identification after occlusion, which the specification
  deliberately leaves out.
- Any form of capture implementation beyond file and stdin adapters.
