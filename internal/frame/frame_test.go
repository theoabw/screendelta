package frame

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func rawFrame(sequence uint64, width, height int) Frame {
	return Frame{
		Sequence:    sequence,
		Width:       width,
		Height:      height,
		Format:      FormatRGBA8,
		ScaleFactor: 1,
		Pixels:      make([]byte, width*height*4),
	}
}

func TestNewRawAcceptsAValidFrame(t *testing.T) {
	f, err := NewRaw(1, 4, 3, FormatRGBA8, 1.0, make([]byte, 4*3*4))
	if err != nil {
		t.Fatalf("NewRaw returned an error for a valid frame: %v", err)
	}
	if f.Width != 4 || f.Height != 3 {
		t.Fatalf("NewRaw changed the dimensions: %dx%d", f.Width, f.Height)
	}
}

func TestValidateReportsTheBrokenField(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Frame)
		field  string
	}{
		{"sequence zero", func(f *Frame) { f.Sequence = 0 }, "sequence"},
		{"width zero", func(f *Frame) { f.Width = 0; f.Pixels = nil }, "width"},
		{"width too large", func(f *Frame) { f.Width = MaxDimension + 1; f.Pixels = nil }, "width"},
		{"height zero", func(f *Frame) { f.Height = 0; f.Pixels = nil }, "height"},
		{"unsupported format", func(f *Frame) { f.Format = "rgb8" }, "format"},
		{"scale zero", func(f *Frame) { f.ScaleFactor = 0 }, "scaleFactor"},
		{"scale negative", func(f *Frame) { f.ScaleFactor = -1 }, "scaleFactor"},
		{"pixels too short", func(f *Frame) { f.Pixels = make([]byte, 3) }, "pixels"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := rawFrame(7, 4, 3)
			tc.mutate(&f)
			err := f.Validate()
			if err == nil {
				t.Fatalf("Validate accepted a frame with %s broken", tc.field)
			}
			var fieldErr *FieldError
			if !asFieldError(err, &fieldErr) {
				t.Fatalf("Validate returned %T, want *FieldError", err)
			}
			if fieldErr.Field != tc.field {
				t.Fatalf("Validate blamed %q, want %q", fieldErr.Field, tc.field)
			}
			// A frame whose sequence is the thing that is wrong cannot name itself,
			// so the frame reference is only required when the sequence is usable.
			if f.Sequence > 0 && !strings.Contains(err.Error(), "frame 7") {
				t.Fatalf("error does not name the frame: %q", err.Error())
			}
			if f.Sequence == 0 && strings.Contains(err.Error(), "frame ") {
				t.Fatalf("error names a frame it cannot know: %q", err.Error())
			}
		})
	}
}

func TestDecodePNGPreservesPixels(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 2, 1))
	source.Set(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	source.Set(1, 0, color.RGBA{R: 200, G: 100, B: 50, A: 255})

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatalf("could not encode the fixture: %v", err)
	}

	f, err := DecodePNG(3, 1.25, bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatalf("DecodePNG failed on a valid image: %v", err)
	}
	if f.Width != 2 || f.Height != 1 {
		t.Fatalf("decoded size is %dx%d, want 2x1", f.Width, f.Height)
	}
	if f.ScaleFactor != 1.25 {
		t.Fatalf("decoded scaleFactor is %v, want 1.25", f.ScaleFactor)
	}
	if !bytes.Equal(f.Pixels, source.Pix) {
		t.Fatalf("decoded pixels differ from the source: %v vs %v", f.Pixels, source.Pix)
	}
}

func TestDecodePNGRejectsGarbage(t *testing.T) {
	_, err := DecodePNG(1, 1, strings.NewReader("this is not a png"))
	if err == nil {
		t.Fatal("DecodePNG accepted data that is not a PNG")
	}
	if !strings.Contains(err.Error(), "png") {
		t.Fatalf("error does not name the field: %q", err.Error())
	}
}

func TestViewportChanged(t *testing.T) {
	base := rawFrame(1, 10, 10)
	cases := []struct {
		name    string
		current Frame
		want    bool
	}{
		{"identical", rawFrame(2, 10, 10), false},
		{"width differs", rawFrame(2, 11, 10), true},
		{"height differs", rawFrame(2, 10, 11), true},
		{"scale differs", func() Frame { f := rawFrame(2, 10, 10); f.ScaleFactor = 1.5; return f }(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ViewportChanged(base, tc.current); got != tc.want {
				t.Fatalf("ViewportChanged = %v, want %v", got, tc.want)
			}
		})
	}
}

// asFieldError is a tiny local helper so the test does not need errors.As for a
// single concrete type.
func asFieldError(err error, target **FieldError) bool {
	fieldErr, ok := err.(*FieldError)
	if !ok {
		return false
	}
	*target = fieldErr
	return true
}
