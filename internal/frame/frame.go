// Package frame holds the input side of the engine: one captured screen image,
// its validation rules, and the comparison the stream loop uses to decide whether
// two frames can be diffed at all.
//
// A Frame borrows its pixel buffer. The stream owns the buffer and may reuse it as
// soon as the engine has consumed the frame, which is what keeps memory flat over a
// long stream.
package frame

import (
	"math"

	"github.com/theoabw/screendelta/internal/fielderr"
)

// PixelFormat names the layout of a frame's pixel buffer.
type PixelFormat string

// FormatRGBA8 is the only format v1 accepts: four bytes per pixel, red, green,
// blue, alpha, row major, top row first.
const FormatRGBA8 PixelFormat = "rgba8"

// MaxDimension bounds a frame's width and height. It exists so that a corrupt
// header cannot ask the engine to allocate an absurd buffer, and it is far above
// any real display resolution.
const MaxDimension = 32768

// operation names this package's operations in error messages.
const operation = "frame.Validate"

// Frame is one captured screen image.
type Frame struct {
	// Sequence is the position in the stream, starting at 1 and strictly
	// increasing. Zero means "not from a stream", which is a validation error
	// because the document reports it.
	Sequence uint64
	Width    int
	Height   int
	Format   PixelFormat
	// ScaleFactor is the declared display scale, for example 1.0 or 1.25. A change
	// between frames is a viewport change, not content.
	ScaleFactor float64
	// Pixels is borrowed, not owned. Length must be Width*Height*4 for RGBA8.
	Pixels []byte
}

// NewRaw builds a frame from an existing buffer after validating it, so an invalid
// frame cannot exist.
func NewRaw(sequence uint64, width, height int, format PixelFormat, scaleFactor float64, pixels []byte) (Frame, error) {
	f := Frame{
		Sequence:    sequence,
		Width:       width,
		Height:      height,
		Format:      format,
		ScaleFactor: scaleFactor,
		Pixels:      pixels,
	}
	if err := f.Validate(); err != nil {
		return Frame{}, err
	}
	return f, nil
}

// Validate reports the first rule the frame breaks, naming the field.
func (f Frame) Validate() error {
	if f.Sequence == 0 {
		// The subject is deliberately unnamed here: a frame with no sequence cannot
		// be identified, and inventing one would make the message misleading.
		return &FieldError{Op: operation, Field: "sequence", Problem: "must be at least 1"}
	}
	if f.Width < 1 || f.Width > MaxDimension {
		return fielderr.Range("width", 1, MaxDimension, f.Width).At(operation, "frame", f.Sequence)
	}
	if f.Height < 1 || f.Height > MaxDimension {
		return fielderr.Range("height", 1, MaxDimension, f.Height).At(operation, "frame", f.Sequence)
	}
	if f.Format != FormatRGBA8 {
		return &FieldError{
			Op:       operation,
			Subject:  "frame",
			Sequence: f.Sequence,
			Field:    "format",
			Problem:  "unsupported pixel format " + string(f.Format) + ", expected " + string(FormatRGBA8),
		}
	}
	if !isFinite(f.ScaleFactor) {
		return &FieldError{
			Op:       operation,
			Subject:  "frame",
			Sequence: f.Sequence,
			Field:    "scaleFactor",
			Problem:  "must be a finite number, not NaN or infinity",
		}
	}
	if f.ScaleFactor <= 0 {
		return &FieldError{
			Op:       operation,
			Subject:  "frame",
			Sequence: f.Sequence,
			Field:    "scaleFactor",
			Problem:  "must be greater than 0",
		}
	}
	expected := f.Width * f.Height * 4
	if len(f.Pixels) != expected {
		return &FieldError{
			Op:       operation,
			Subject:  "frame",
			Sequence: f.Sequence,
			Field:    "pixels",
			Problem:  "length does not match the declared layout, expected " + itoa(expected) + " bytes",
		}
	}
	return nil
}

// ViewportChanged reports whether the two frames describe different viewports, in
// which case their difference is a display change rather than content.
//
// Two NaN scale factors are treated as equal, so the relation stays reflexive: a value
// that is not equal to itself would make every comparison report a viewport change.
// Validated frames never carry a NaN scale in the first place.
func ViewportChanged(previous, current Frame) bool {
	if previous.Width != current.Width || previous.Height != current.Height {
		return true
	}
	if previous.ScaleFactor == current.ScaleFactor {
		return false
	}
	return !(isNaN(previous.ScaleFactor) && isNaN(current.ScaleFactor))
}

// isFinite reports whether a float is a usable number.
func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func isNaN(value float64) bool {
	return math.IsNaN(value)
}
