package frame

import (
	"bytes"
	"testing"
)

// FuzzDecodePNG feeds arbitrary bytes to the frame decoder, which is the other place untrusted input arrives.
//
// The property is that no input panics: every byte string either produces an error or produces a frame the rest
// of the engine can use. A review pointed out that the two assertions this target used to make could not fail by
// construction, because the decoder rejects empty geometry and an RGBA image over non-empty bounds always
// allocates pixels. They are kept as documentation of what the decoder guarantees, and the honest statement of
// what the target is for is the absence of a panic.
func FuzzDecodePNG(f *testing.F) {
	// A real one pixel PNG, so the fuzzer starts from something the decoder understands.
	f.Add([]byte{
		0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x02, 0x00, 0x00, 0x00,
		0x90, 0x77, 0x53, 0xde,
	})
	f.Add([]byte("not a png at all"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, body []byte) {
		decoded, err := DecodePNG(1, 1, bytes.NewReader(body))
		if err != nil {
			return
		}
		if decoded.Width <= 0 || decoded.Height <= 0 {
			t.Fatalf("a decoded frame has geometry %dx%d", decoded.Width, decoded.Height)
		}
		if len(decoded.Pixels) == 0 {
			t.Fatal("a decoded frame carries no pixels")
		}
	})
}
