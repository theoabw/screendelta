// Package stream owns the frame loop and the session state.
//
// Two properties are decided here rather than in the packages below it. Memory must
// not grow with stream length, so the engine owns two pixel buffers and swaps them
// instead of allocating per frame, and the caller may reuse its own buffer as soon as
// Push returns. And a frame's sequence must increase, because a document names the
// frame it describes and a stream that goes backwards is a caller defect rather than
// content.
package stream

import (
	"github.com/theoabw/screendelta/internal/config"
	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/fielderr"
	"github.com/theoabw/screendelta/internal/frame"
)

// Differ is the comparison the stream delegates to. It is an interface so the loop can
// be tested without the image code, and so a later stage can supply a different
// implementation without touching the loop.
type Differ interface {
	// Compare returns the regions that differ between two frames and any conditions
	// the consumer should know about.
	Compare(previous, current frame.Frame, cfg config.Config) ([]delta.Region, []delta.Condition, error)
	// Fingerprint summarises a frame in a way that is stable under capture noise.
	Fingerprint(current frame.Frame, cfg config.Config) (delta.Fingerprint, error)
}

// viewport describes the geometry a frame was captured at.
type viewport struct {
	width       int
	height      int
	scaleFactor float64
}

func viewportOf(f frame.Frame) viewport {
	return viewport{width: f.Width, height: f.Height, scaleFactor: f.ScaleFactor}
}

func (v viewport) differsFrom(other viewport) bool {
	return v.width != other.width || v.height != other.height || v.scaleFactor != other.scaleFactor
}

// Engine processes a stream of frames. One engine per stream, not safe for
// concurrent use.
type Engine struct {
	cfg    config.Config
	differ Differ

	previous []byte
	scratch  []byte

	previousViewport viewport
	hasPrevious      bool
	lastSequence     uint64

	// bufferAllocations counts how many times the engine has allocated a pixel
	// buffer, so a test can assert that a long stream does not keep allocating.
	bufferAllocations int

	closed bool
}

// New creates an engine for one stream.
func New(cfg config.Config, differ Differ) (*Engine, error) {
	if differ == nil {
		return nil, &fielderr.Error{
			Op:      "stream.New",
			Subject: "engine",
			Field:   "differ",
			Problem: "must not be nil",
		}
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Engine{cfg: cfg, differ: differ}, nil
}

// Push consumes one frame and returns its document.
//
// The caller's pixel buffer is only read, never retained, so the caller may reuse it
// immediately after Push returns.
func (e *Engine) Push(f frame.Frame) (delta.Document, error) {
	if e.closed {
		return delta.Document{}, &fielderr.Error{
			Op:      "stream.Push",
			Subject: "engine",
			Field:   "state",
			Problem: "engine is closed",
		}
	}
	if err := f.Validate(); err != nil {
		return delta.Document{}, err
	}
	if e.hasPrevious && f.Sequence <= e.lastSequence {
		return delta.Document{}, &fielderr.Error{
			Op:       "stream.Push",
			Subject:  "frame",
			Sequence: f.Sequence,
			Field:    "sequence",
			Problem:  "must be greater than the previous frame's sequence",
		}
	}

	e.copyIntoScratch(f)
	current := e.frameFromScratch(f)
	currentViewport := viewportOf(f)

	var (
		regions    []delta.Region
		conditions []delta.Condition
	)
	switch {
	case !e.hasPrevious:
		// There is no predecessor, so there is nothing to be different from.
		conditions = append(conditions, delta.ConditionFirstFrame)
	case currentViewport.differsFrom(e.previousViewport):
		// A display change would otherwise look like a screenful of content changes.
		conditions = append(conditions, delta.ConditionViewportChanged)
	default:
		previous := e.frameFromPrevious(f)
		compared, extra, err := e.differ.Compare(previous, current, e.cfg)
		if err != nil {
			return delta.Document{}, wrapDifferError("compare", f.Sequence, err)
		}
		regions = compared
		conditions = append(conditions, extra...)
	}

	fingerprint, err := e.differ.Fingerprint(current, e.cfg)
	if err != nil {
		return delta.Document{}, wrapDifferError("fingerprint", f.Sequence, err)
	}

	document := delta.Document{
		SchemaVersion: delta.SchemaVersion,
		Frame: delta.FrameRef{
			Sequence:    f.Sequence,
			Width:       f.Width,
			Height:      f.Height,
			ScaleFactor: f.ScaleFactor,
		},
		Fingerprint: ownFingerprint(fingerprint),
		Regions:     ownRegions(regions),
		Conditions:  delta.SortConditions(conditions),
	}
	if err := document.Validate(); err != nil {
		return delta.Document{}, err
	}

	e.swap()
	e.previousViewport = currentViewport
	e.hasPrevious = true
	e.lastSequence = f.Sequence

	return document, nil
}

// Close releases the buffers. It is safe to call more than once.
func (e *Engine) Close() error {
	if e.closed {
		return nil
	}
	e.closed = true
	e.previous = nil
	e.scratch = nil
	return nil
}

// BufferAllocations reports how many pixel buffers the engine has allocated. A stream
// of constant geometry must not increase it.
func (e *Engine) BufferAllocations() int {
	return e.bufferAllocations
}

// BufferBytes reports how many bytes of pixel buffer the engine is holding, which is
// what the memory ceiling is really about.
func (e *Engine) BufferBytes() int {
	return len(e.previous) + len(e.scratch)
}

func (e *Engine) copyIntoScratch(f frame.Frame) {
	needed := len(f.Pixels)
	if len(e.scratch) != needed {
		e.scratch = make([]byte, needed)
		e.bufferAllocations++
	}
	copy(e.scratch, f.Pixels)
}

func (e *Engine) frameFromScratch(template frame.Frame) frame.Frame {
	return frame.Frame{
		Sequence:    template.Sequence,
		Width:       template.Width,
		Height:      template.Height,
		Format:      template.Format,
		ScaleFactor: template.ScaleFactor,
		Pixels:      e.scratch,
	}
}

func (e *Engine) frameFromPrevious(template frame.Frame) frame.Frame {
	return frame.Frame{
		Sequence:    e.lastSequence,
		Width:       e.previousViewport.width,
		Height:      e.previousViewport.height,
		Format:      template.Format,
		ScaleFactor: e.previousViewport.scaleFactor,
		Pixels:      e.previous,
	}
}

func (e *Engine) swap() {
	if len(e.previous) != len(e.scratch) {
		// The geometry changed, so the old buffer is the wrong size. Allocate the new
		// one here rather than carrying two sizes for the rest of the stream.
		e.previous = make([]byte, len(e.scratch))
		e.bufferAllocations++
	}
	e.previous, e.scratch = e.scratch, e.previous
}

// ownRegions takes ownership of what the differ returned.
//
// The engine cannot assume the differ allocates fresh slices per call: a differ that
// pools its own storage is exactly what a streaming implementation should do. So the
// document gets a deep copy, including the pointed-to previous bounds, and the copy is
// sorted rather than the differ's slice, which also stops the engine reordering memory
// it does not own.
func ownRegions(regions []delta.Region) []delta.Region {
	owned := make([]delta.Region, len(regions))
	for index, region := range regions {
		owned[index] = region
		if region.PreviousBounds != nil {
			bounds := *region.PreviousBounds
			owned[index].PreviousBounds = &bounds
		}
	}
	delta.SortRegions(owned)
	return owned
}

// ownFingerprint copies the cell slice for the same reason.
func ownFingerprint(fingerprint delta.Fingerprint) delta.Fingerprint {
	if fingerprint.Cells != nil {
		fingerprint.Cells = append([]int(nil), fingerprint.Cells...)
	}
	return fingerprint
}

// wrapDifferError attributes a differ failure to the operation that failed and to the
// frame being processed, without rewriting the error it was handed: an error value may
// be shared, and mutating it is how a message ends up naming the wrong frame. The cause
// is preserved so a caller can still test for the original error.
func wrapDifferError(operation string, sequence uint64, err error) error {
	if fieldErr, ok := err.(*fielderr.Error); ok {
		// A typed nil satisfies the assertion and then panics on use, which is a
		// differ defect the engine has to survive rather than propagate as a crash.
		if fieldErr == nil {
			return &fielderr.Error{
				Op:       "stream.Push",
				Subject:  "frame",
				Sequence: sequence,
				Field:    operation,
				Problem:  "differ returned a typed nil error",
			}
		}
		context := *fieldErr
		context.Op = "stream.Push"
		// The subject and the number both belong to this engine: it knows which frame
		// it handed over and that the frame, not the differ's own subject, is what a
		// reader needs. A pooled error carrying a stale number, or naming a tile,
		// would send an operator to the wrong place.
		context.Subject = "frame"
		context.Sequence = sequence
		if context.Field == "" {
			context.Field = operation
		} else if context.Field != operation {
			// Keep the differ's own field in the message rather than discarding it.
			context.Problem = context.Field + ": " + context.Problem
			context.Field = operation
		}
		context.Cause = err
		return &context
	}
	return &fielderr.Error{
		Op:       "stream.Push",
		Subject:  "frame",
		Sequence: sequence,
		Field:    operation,
		Problem:  err.Error(),
		Cause:    err,
	}
}
