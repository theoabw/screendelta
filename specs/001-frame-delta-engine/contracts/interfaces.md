# Interface contracts: CLI and library

Both surfaces expose the same capabilities over the same document model, because
FR-013 requires it and because a pipeline stage that can only be used by shelling
out is a pipeline stage nobody embeds.

## Command line

```
screendelta diff       --previous <frame> --current <frame> [--config <file>] [--out <path>]
screendelta stream     --source <dir|-> [--config <file>] [--out <path>]
screendelta fingerprint --frame <frame> [--config <file>] [--out <path>]
screendelta validate   --config <file>
screendelta version
```

Rules that are part of the contract:

1. **One document per frame.** `diff` writes one JSON document. `stream` writes one
   document per line (`ndjson`), in frame order, and flushes per line so a consumer
   can process incrementally.
2. **Output destination.** `--out` writes to that path only. Without `--out`, the
   document goes to standard output and nothing else does. Diagnostics go to
   standard error.
3. **No network and no stray writes.** The engine never opens a socket and never
   writes outside the declared output path, which FR-016 makes a requirement rather
   than a preference.
4. **Determinism.** The same input and configuration produce byte-identical output,
   independent of thread count, host and run, which is why no timing field exists in
   the document.
5. **Frames.** `--previous` and `--current` accept PNG or raw RGBA. A raw frame
   requires `--width`, `--height` and `--pixel-format`, or it is rejected rather
   than guessed.
6. **Stream source.** `--source -` reads a length-prefixed frame sequence from
   standard input. `--source <dir>` reads files in lexicographic order and treats
   the first frame as the `first-frame` condition case.

### What each subcommand can classify

`diff` compares two frames with no history, so it reports every region as `changed`: it
knows that pixels differ but not whether an element was there before, and calling the
region `added` would assert an absence it cannot see. `moved` and `removed` both need a
baseline, which only a stream provides, so a consumer that needs them must use `stream`.
This is a consequence of the failure semantics, not a limitation of the comparison, and
`quickstart.md` exercises each class where it can actually occur.

### Identity fields on a region

Every region carries `identityConfidence` and `identityUncertain` because FR-007 requires the
engine to report how strong its claim is and to refuse a match it cannot support. The rule a
consumer can rely on:

- a region whose element was matched in the previous frame carries the same `identity` with
  `identityConfidence` equal to the match score and `identityUncertain` false;
- a region that appears where nothing was tracked, or where a retired element used to be,
  carries a newly allocated `identity`; in the second case `identityUncertain` is true, because
  the engine has evidence that something similar was there before but not enough to claim it is
  the same element;
- an `identity` is never reissued, so a consumer can keep a handle forever and learn that it is
  gone from a `removed` region;
- one element may appear as more than one region in a document, all carrying its identity, because a
  translation changes two areas and both belong to the element. A consumer that needs one rectangle
  per element merges the regions sharing an identity, and one that needs the changed area uses them
  as they are.

### Covers and returns

When a larger area covers a tracked element, the element is reported as `removed` under its own
identifier and the covering area as `added` under a new one, so a consumer learns that the handle it
held is gone rather than watching it move to something else. When the covered content shows again, it
is reported as `added` with a newly allocated identifier whose `identityUncertain` is true and whose
`identityConfidence` is greater than zero: the engine has evidence that this is the element that left, and
not enough to claim it, so it reports the evidence and lets the consumer decide. The number measures the
evidence, and a value of one means a perfect appearance match over a perfect overlap, which is still not
proof; the flag is the part that says so.

Three boundaries are stated rather than left to be discovered:

- a cover whose area equals the covered element's footprint leaves no margin to exceed it, so it is
  reported as a change of that element;
- an element that disappears leaving the same footprint is reported as a change, not a removal, because
  pixels cannot tell that apart from a repaint;
- an element that grows and repaints its interior in one frame may be reported as removed and added,
  which costs a consumer an identity on that frame but never hands it a confident wrong one.

### Exit codes

| Code | Meaning |
|---|---|
| 0 | Success, and for `validate`, a configuration that is acceptable |
| 1 | Usage error: unknown flag, missing argument, contradictory options |
| 2 | Input error: unreadable frame, unsupported format, dimensions that do not match a declared raw layout |
| 3 | Contract violation: output that would not satisfy the schema, such as an unsupported schema version request |

Exit code 3 exists so that a pipeline can distinguish "your frames are wrong" from
"this build cannot express what you asked for", which are different operators'
problems.

### Error output

Errors are one line on standard error, prefixed with the subcommand and naming the
offending field or path, for example:

```
screendelta: diff: --config: noiseFloor: value 1.5 is outside 0 to 1
screendelta: stream: frame 41: dimensions 1920x1080 do not match declared raw layout 1920x1200
```

No stack traces, no partial document on standard output when an error occurs, and no
error message that requires reading the source to act on.

## Library

Package layout is in `plan.md`. The contract that matters to a consumer:

```go
// Package delta holds the document model only; it imports no image code, so a
// consumer can decode documents without pulling in a decoder.
package delta

type Document struct { /* schemaVersion, frame, fingerprint, regions, conditions */ }
func (d Document) Encode(w io.Writer, pretty bool) error
func Decode(r io.Reader) (Document, error)      // rejects unknown schemaVersion

// Package stream owns the frame loop and the session state.
package stream

type Options struct { /* the validated configuration */ }
type Engine struct { /* one per stream; holds the previous frame and identity map */ }
func New(opts Options) (*Engine, error)
func (e *Engine) Push(frame frame.Frame) (delta.Document, error)
func (e *Engine) Close() error
```

Contract rules:

1. **`Engine` is stateful per stream and not safe for concurrent use.** One engine
   per stream, as many streams as the caller wants.
2. **`Push` never mutates the caller's frame payload**, and never retains it after
   returning, so the caller may reuse the buffer.
3. **Errors are values**, wrapped with the frame sequence and the field at fault.
   `Push` returns no document on error, so a caller cannot accidentally emit a
   partial one.
4. **Determinism holds across goroutines.** Parallel tiling may be enabled inside
   the engine, but the returned document must be identical to the single-threaded
   result, and a test asserts that by comparing encoded bytes.
5. **`Decode` is strict.** An unknown `schemaVersion` is an error, because silently
   accepting a newer document is how a consumer starts misreading fields.
6. **No global state.** Two engines in one process must not affect each other, which
   rules out package-level buffers and identity counters.

## Versioning

The schema generation is a string constant, `1.0` today. Adding an optional field
stays within the generation. Removing a field, changing a type, changing the meaning
of an existing field or changing region ordering starts a new generation, and the
decoder rejects the old one rather than guessing. Every such change is recorded in
`docs/adr/`.

## Conditions

The document declares its conditions rather than implying them, and three are defined by the schema:

| Condition | Produced by this engine | Meaning |
|---|---|---|
| `first-frame` | yes | There is no predecessor, so the document reports no regions. |
| `viewport-changed` | yes | The geometry or the scale factor changed, so everything the engine remembered about the previous frame's coordinates is meaningless. |
| `out-of-order-timestamp` | no | The frame's timestamp is earlier than its predecessor's. This engine cannot produce it: a frame carries a sequence and no timestamp, by design, because a document that carried a timing field could not be byte-identical between runs. A frame whose sequence does not advance is refused with the sequence in the error, which `stream.Push` documents and a test pins. The condition stays in the published enum for a capture layer that has timestamps of its own and wants to report one through this contract. |
