# Phase 1 data model

The entities the engine owns, the rules that hold for each of them, and the
requirements that force them to exist. Nothing here describes storage: the engine
keeps one previous frame plus a session map, and everything else is a document it
emits.

## Frame

One captured screen image.

| Field | Type | Notes |
|---|---|---|
| `sequence` | unsigned integer | Position in the stream, starting at 1 and strictly increasing |
| `width`, `height` | unsigned integer | Pixels, both greater than zero |
| `pixelFormat` | enum | `rgba8` for v1 |
| `scaleFactor` | number | Declared display scale, for example 1.0 or 1.25 |
| `payload` | bytes | Borrowed from the stream's pool, never owned by the document |

**Invariants**: the payload length matches the dimensions and format; `sequence`
strictly increases within a stream; a frame whose dimensions or `scaleFactor`
differ from its predecessor produces a `viewport-changed` condition and no content
regions.

**Requirements**: FR-001, FR-015, FR-016, NFR-003, NFR-008.

## Region

One rectangular area reported as different from the previous frame.

| Field | Type | Notes |
|---|---|---|
| `identity` | unsigned integer | Reference into the session identity map |
| `class` | enum | `added`, `changed`, `removed`, `moved` |
| `bounds` | object | `x`, `y`, `w`, `h` normalized to the frame extent |
| `previousBounds` | object, optional | Required for `moved` and `removed`, forbidden for `added` |
| `magnitude` | number | 0 to 1, the mean luma difference over the region, scaled |
| `areaPixels` | unsigned integer | Absolute area, for consumers that care about size |
| `identityConfidence` | number | 0 to 1, the strength of the evidence that this region is the element its identity refers to |
| `identityUncertain` | boolean | True when the identity could not be re-established and was re-acquired |

**Multiplicity**: two regions in one document may carry the same `identity`. A translated element
is reported as the area it left and the area it arrived in, and both are the same element, so both
name it. Identities are therefore unique among the elements a document refers to, not among its
rectangles, and a consumer that groups regions by identity gets one group per element per frame.

**Invariants**: all bounds are within the frame, so `x + w <= 1` and `y + h <= 1`;
`magnitude` is inside 0 to 1 inclusive; a `changed` or `added` region's bounds are
the current frame's, a `removed` region's bounds are the previous frame's.

**What previousBounds means**: for a `moved` region it is the engine's belief about where the element
was, taken from the tracked footprint rather than from the area that changed, so it describes the
element and may be larger than the element after it has shrunk. `bounds` is always the measured changed
area and stays exact. A consumer that needs the element's extent should use the footprint the engine
reports, not the changed area.

**Identity fields**: `identityConfidence` is the match score when the identity is carried
over, 1 when the engine has no competing interpretation (a first appearance or a first
comparison), and the observed overlap when an element returned after being occluded.
`identityUncertain` is true only in that last case, because the engine will not claim a match
it cannot support: a consumer that needs a stable handle must treat an uncertain identity as
new, which is why the flag is in the document and not only in the engine.

**Requirements**: FR-002, FR-003, FR-004, FR-005.

## ElementIdentity

A session-scoped handle on something the engine believes is the same element across
frames.

| Field | Type | Notes |
|---|---|---|
| `id` | unsigned integer | Allocated per session, never reused, even after retirement |
| `confidence` | number | 0 to 1, from the match cost |
| `uncertain` | boolean | True when the identity could not be re-established and was re-acquired |
| `firstFrame`, `lastFrame` | unsigned integer | Frame sequence numbers |
| `state` | enum | `live` or `retired` |

**State transitions**:

- An `added` region allocates a new identity in state `live`.
- A matched region keeps its identity and updates `lastFrame` and `confidence`.
- A region unmatched for more than the occlusion window moves to `retired`.
- An element whose footprint has nothing changed inside it is still on the screen, so it is refreshed
  rather than aged: the window counts frames in which the element's own area changed and it was not
  matched, which is the only evidence the engine has.
- A changed area that contains a tracked element and exceeds its footprint by more than the growth
  margin is a cover or a replacement: the tracked element is reported as `removed` and the area as
  `added`, with different identifiers.
- A changed area that overlaps a tracked element, looks unlike it, and looks like an element the engine
  has retired is a return: the tracked element is reported as `removed` and the area as `added` with a
  newly allocated identity marked uncertain whose confidence is the margin between the two appearances
  tempered by how much of the retired element the area covers, so it measures the evidence rather than
  the overlap alone.
- An element whose footprint a changed area contains and reaches past is reported as `removed` only when
  its own pixels changed. An element that merely grew, leaving its interior untouched, is still there and
  is reported as a change or a movement, because a bounding rectangle around a change is not evidence
  that everything inside it changed.
- A region that reappears after retirement receives a **new** identity with
  `uncertain` set, because the engine will not claim a match it cannot support.
- A retired identity never returns to `live` and its `id` is never reissued.

**Requirements**: FR-006, FR-007, NFR-004.

## Fingerprint

A compact, noise-stable summary of a frame.

| Field | Type | Notes |
|---|---|---|
| `algorithm` | string | Versioned name, for example `grid-luma-1` |
| `gridSize` | unsigned integer | Cells per side, default 32 |
| `cells` | array of integers | Quantized luma per cell, row-major |
| `strictHash` | string | Deterministic hash over `cells`, for exact cache keys |

**Invariants**: `strictHash` is a pure function of `algorithm`, `gridSize` and
`cells`, so two identical frames produce identical hashes; equality between
fingerprints is a bounded comparison over `cells`, never a hash comparison, so
sub-threshold noise does not change the answer.

**Requirements**: FR-008, FR-009, FR-014, NFR-010.

## DeltaDocument

The product. One document per frame, self-contained.

| Field | Type | Notes |
|---|---|---|
| `schemaVersion` | string | Constant for a schema generation, `1.0` for v1 |
| `frame` | object | `sequence`, `width`, `height`, `scaleFactor` |
| `fingerprint` | object | As above |
| `regions` | array | Sorted by top edge then left edge, never by map order |
| `conditions` | array of enums | `first-frame`, `viewport-changed`, `out-of-order-timestamp` |

**Invariants**: identical input produces a byte-identical document, so the document
carries **no timings, no durations and no wall-clock fields**; ordering is total and
deterministic; when `conditions` contains `first-frame`, `regions` is empty because
there is no predecessor to compare against; every region references an identity that
exists in the session map.

The absence of timing fields is deliberate and is the reason performance numbers
live in the benchmark and the report rather than in the output.

**Requirements**: FR-010, FR-014, FR-016, NFR-004, NFR-010, SC-006.

## Configuration

The caller's control surface.

| Field | Type | Default | Range |
|---|---|---|---|
| `noiseFloor` | number | 0.02 | 0 to 1 |
| `minRegionAreaPixels` | unsigned integer | 64 | 1 to 1000000 |
| `motionTolerancePixels` | unsigned integer | 8 | 0 to 512 |
| `occlusionFrames` | unsigned integer | 3 | 0 to 600 |
| `ignoredAreas` | array of normalized bounds | empty | within the frame |
| `regionsOfInterest` | array of bounds with an opaque `label` | absent | within the frame |
| `output.format` | enum | `json` | `json`, `ndjson` |
| `fingerprint.gridSize` | unsigned integer | 32 | 8 to 256 |
| `fingerprint.maxCellDelta` | unsigned integer | 4 | 0 to 255 |

**Invariants**: unknown keys are rejected, values outside their range are rejected,
and the error names the field; `regionsOfInterest` absent means unrestricted output,
while present and empty means no regions are reported at all. That distinction is
deliberate, because "I supplied no regions of interest" and "I want nothing" are
different requests.

**Requirements**: FR-011, FR-012, FR-015.

## Relationships

```
Frame ──produces──▶ DeltaDocument ──contains──▶ Region ──references──▶ ElementIdentity
  │                      │
  └──produces────────────┴──▶ Fingerprint
Configuration ──constrains──▶ (Frame comparison, Region reporting)
RegionsOfInterest ──restricts──▶ Region reporting
```

## Requirements to entity map

| Requirement | Entity or rule |
|---|---|
| FR-001 | Frame |
| FR-002, FR-003, FR-004, FR-005 | Region and the matching rules that classify it |
| FR-006, FR-007 | ElementIdentity and its transitions |
| FR-008, FR-009 | Fingerprint |
| FR-010 | DeltaDocument emission |
| FR-011 | RegionsOfInterest and Configuration |
| FR-012, FR-015 | Configuration validation |
| FR-013 | DeltaDocument plus the library and CLI surfaces |
| FR-014, NFR-010 | `schemaVersion` and the versioning rule |
| FR-016 | Frame and DeltaDocument constraints, no network, no stray writes |
| NFR-003 | Frame payload borrowing and stream-owned pools |
| NFR-004 | Ordering and the absence of timing fields |
| NFR-005 | Region invariants plus the noise floor rules |
| NFR-006 | Region bounds and the matching thresholds |
