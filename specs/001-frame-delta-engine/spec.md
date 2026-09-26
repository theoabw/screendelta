# Feature Specification: Frame Delta and Element Identity Engine

**Feature Branch**: `001-frame-delta-engine`

**Created**: 2026-09-26

**Status**: Draft

**Input**: User description: "A deterministic frame delta and element identity engine for legacy GUI navigation"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Report what changed between two frames (Priority: P1)

A pipeline stage holds the previous screen and receives the current one. It needs
to know which regions changed and by how much, so that it can pass a small delta to
a slow planner instead of a whole screenshot.

**Why this priority**: This is the core value. Without a trustworthy delta there is
no product, and every later stage depends on it.

**Independent Test**: Give the engine two frames with known differences and check
that the reported regions match the known changed areas, that identical frames
produce an empty delta, and that a difference below the configured noise floor
produces no region at all.

**Acceptance Scenarios**:

1. **Given** two byte-identical frames, **When** the delta is computed, **Then** the document contains no regions and reports that nothing changed.
2. **Given** a previous frame and a current frame in which one button changed its label, **When** the delta is computed, **Then** exactly one region is reported as changed, its bounds enclose the button, and its magnitude is above the configured floor.
3. **Given** two frames of different dimensions, **When** the delta is computed, **Then** the engine fails with a clear error and produces no document.

---

### User Story 2 - Keep element identity across a stream of frames (Priority: P2)

A planner that acted on an element one step ago needs to refer to the same element
in the next step, even though the screen was redrawn and the element moved slightly.

**Why this priority**: It is what makes multi-step navigation possible, and it is
the part no existing tool does with explicit honesty about failure.

**Independent Test**: Feed a generated sequence in which a region persists, moves,
is occluded and returns, and check that identifiers persist while the region is
tracked, are never reused, and that a re-appearance after occlusion is marked
uncertain rather than silently matched.

**Acceptance Scenarios**:

1. **Given** a sequence of frames in which one region moves by a few pixels per frame, **When** the stream is processed, **Then** that region keeps one identifier throughout and the identifier is never assigned to another region.
2. **Given** an element that is covered by a larger one and later shows its content again, **When** the stream is processed, **Then** the covered element is reported as removed naming its identifier, the covering area as added naming a different one, and the returning content as added with a newly allocated identifier marked uncertain and a confidence greater than zero, where the flag rather than the number is what says the engine is not claiming the match. A cover whose area equals the covered element's footprint leaves no margin to exceed it and is reported as a change, which is recorded as a boundary of the stage rather than a defect.
3. **Given** a stream of one thousand frames in which a region appears and disappears repeatedly, **When** the stream is processed, **Then** no identifier is reused and memory does not grow with stream length.

---

### User Story 3 - Fingerprint a screen so a plan can be cached (Priority: P3)

A planner that solved a screen once wants to recognise that screen again cheaply,
and to be told when the screen is not the one it cached, rather than acting on a
stale plan.

**Why this priority**: Valuable, but a pipeline can run without it, so it follows
the delta core.

**Independent Test**: Fingerprint a frame, compare it against itself, against a
noise-perturbed copy and against a materially changed copy, and check equal, equal
and different respectively.

**Acceptance Scenarios**:

1. **Given** a frame and a copy of it with sub-threshold noise, **When** the fingerprint is compared, **Then** the result is equal.
2. **Given** a frame and a copy with one region materially changed, **When** the fingerprint is compared, **Then** the result is different and the changed region is reported.

---

### Edge Cases

- Identical frames, and frames that differ only by capture noise or lossy compression.
- A whole-screen change, such as a theme or dark-mode switch, which must produce a
  bounded region set rather than thousands of individual tiles.
- A window dragged across the screen, which is a bulk translation of many regions.
- A changed display scale or DPI between frames, which must be reported as an
  explicit condition rather than silently rescaled.
- A moving cursor or blinking caret, which must be ignorable by configuration.
- A modal overlay covering part of the screen, while underlying regions are still
  tracked.
- Partial occlusion of a tracked region.
- Frames arriving out of order or with a non-monotonic timestamp.
- Very large frames, exceeding the memory ceiling, which must be refused explicitly.
- An empty stream, and a stream containing exactly one frame, which has no
  predecessor to compare against.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The engine MUST accept two frames as image files or raw pixel buffers, with dimensions and a declared pixel format.
- **FR-002**: The engine MUST report changed regions with bounds normalized to the frame extent, so that consumers are independent of resolution.
- **FR-003**: The engine MUST classify each reported region as added, changed, removed or moved.
- **FR-004**: The engine MUST report a change magnitude per region on a documented scale, and MUST support a configurable floor below which no region is reported.
- **FR-005**: The engine MUST report no regions when two frames differ only below the configured noise floor.
- **FR-006**: The engine MUST assign element identifiers that are stable across a frame stream and never reused within a session.
- **FR-007**: The engine MUST report identity confidence per tracked region, and MUST mark an identity as uncertain rather than silently matching a region whose identity cannot be re-established.

  A single frame pair cannot decide whether a changed area that contains a tracked element is a cover, a
  replacement or a repaint of that element, so the engine decides on evidence rather than on a guess: it
  keeps a coarse appearance signature per element and reports a reacquisition only when the returning
  content looks like an element it has retired and unlike the one that covered it. Where the evidence is
  absent the engine reports a change, which is truthful about the pixels. A same-footprint disappearance
  is therefore reported as a change rather than as a removal, because the evidence that would justify a
  removal also fires on a subtle repaint of an element whose fill resembles its surroundings, and false
  removals are forbidden by NFR-005 and SC-001.
- **FR-008**: The engine MUST compute a per-frame fingerprint that is stable under sub-threshold noise.
- **FR-009**: The engine MUST support comparing a frame against a previously stored fingerprint and MUST report equal or different.
- **FR-010**: The engine MUST emit one self-contained document per frame when streaming, so that a consumer can process frames independently and in order.
- **FR-011**: The engine MUST accept externally supplied regions of interest and MUST be able to restrict its reported output to those regions.
- **FR-012**: The engine MUST accept a configuration document covering at least the noise floor, motion tolerance, ignored areas and output options, MUST apply documented defaults, and MUST reject invalid configuration with a clear error.
- **FR-013**: The engine MUST expose the same capabilities through a command line interface and a library interface, so that the pipeline can embed it without shelling out.
- **FR-014**: Every emitted document MUST declare a schema version.
- **FR-015**: The engine MUST fail explicitly, without emitting a partial document, on unusable input such as mismatched dimensions, undecodable data, or an unsupported schema version.
- **FR-016**: The engine MUST operate without network access and MUST NOT write outside its declared output path.

### Non-Functional Requirements

- **NFR-001** (performance): 95th percentile delta latency at or below 12 ms and 99th percentile at or below 25 ms per 1080p frame pair on one CPU core, measured by the committed benchmark on the recorded reference machine.
- **NFR-002** (performance): At least 30 frames per second sustained throughput at 1080p on one CPU core, end to end including document emission.
- **NFR-003** (scalability): At most 128 MB resident memory while streaming 10,000 frames at 1080p, with no growth attributable to stream length.
- **NFR-004** (reliability): Identical input MUST produce byte-identical output, independent of host, thread count and scheduling.
- **NFR-005** (reliability): Zero false removals on the noise corpus: a region that is still present MUST NOT be reported as removed because of capture noise.
- **NFR-006** (accuracy): Region detection F1 at or above 0.98, and moved-region attribution accuracy at or above 0.95, against the generated ground-truth corpus.
- **NFR-007** (usability): A new user MUST be able to produce a delta document for their own screenshots within two minutes using only the README.
- **NFR-008** (portability): CPU only, no GPU, no network access at runtime, and the engine MUST run on both Linux and Windows.
- **NFR-009** (maintainability): At least 80 percent line coverage on the geometry and identity modules, with coverage reported by the standard tool for the chosen stack.
- **NFR-010** (compatibility): Schema changes MUST be versioned, and a consumer MUST be able to reject a document whose version it does not understand.

### Key Entities

- **Frame**: one captured screen image at a point in time, with dimensions, pixel format and a monotonic timestamp.
- **Region**: a rectangular area of interest with normalized bounds, a change class, a change magnitude and an identity reference.
- **Element Identity**: a session-scoped identifier attached to a tracked region, with a confidence value and an uncertainty flag.
- **Fingerprint**: a compact, noise-stable summary of a frame, usable for equality comparison and cache keys.
- **Delta Document**: the versioned output for one frame, containing the frame reference, the fingerprint, the regions and any condition flags.
- **Configuration**: the tunable input controlling thresholds, motion tolerance, ignored areas and output behaviour.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: On a generated corpus of at least 5,000 frame pairs with known changes, region detection F1 is at or above 0.98 and false removals are zero.
- **SC-002**: The committed benchmark reports p95 at or below 12 ms and p99 at or below 25 ms per 1080p frame pair on the reference machine.
- **SC-003**: A 10,000 frame stream completes within the 128 MB memory ceiling, with no identifier reuse and byte-identical output across two runs with different thread counts.
- **SC-004**: A demonstration pipeline of capture, engine and a stub detector sustains at least 20 decisions per second wall clock on real legacy desktop screenshots.
- **SC-005**: A person unfamiliar with the project produces a delta document for their own screenshot using only the README, in under five minutes.
- **SC-006**: The public interface is usable from a second consumer written independently against the schema, without changes to the engine.

## Assumptions

- Frames are captured at 60 frames per second or slower, and consecutive frames
  depict the same viewport unless a change of scale is explicitly declared.
- Ground truth comes from generated content: rendered pages whose element boxes are
  read from the DOM, and synthesized frame sequences with known motion and
  appearance. Hand-labelled data is used only for a small sanity check.
- The engine is one stage of a larger pipeline. Capture, element classification,
  text recognition and action planning are out of scope for this specification.
- Real legacy desktop screenshots are available for a sanity check, but they are not
  the primary accuracy corpus.
- A single machine profile is used for performance claims, and it is recorded
  alongside the benchmark results.
