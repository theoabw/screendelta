package frame

import (
	"image"
	"image/draw"
	"image/png"
	"io"
)

// DecodePNG reads a PNG stream and returns an RGBA8 frame at the image's natural
// size. Sequence and scaleFactor come from the caller because neither is a property
// of the file: one is stream position, the other is declared by whoever captured the
// screen.
func DecodePNG(sequence uint64, scaleFactor float64, r io.Reader) (Frame, error) {
	decoded, err := png.Decode(r)
	if err != nil {
		return Frame{}, &FieldError{
			Op:       "frame.DecodePNG",
			Sequence: sequence,
			Field:    "png",
			Problem:  "cannot decode: " + err.Error(),
		}
	}

	bounds := decoded.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return Frame{}, &FieldError{
			Op:       "frame.DecodePNG",
			Sequence: sequence,
			Field:    "png",
			Problem:  "image has no pixels",
		}
	}

	// Convert into a fresh buffer rather than borrowing the decoded image's storage,
	// because the decoder's buffer is not guaranteed to be RGBA8 or tightly packed.
	rgba := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(rgba, rgba.Bounds(), decoded, bounds.Min, draw.Src)

	return NewRaw(sequence, bounds.Dx(), bounds.Dy(), FormatRGBA8, scaleFactor, rgba.Pix)
}
