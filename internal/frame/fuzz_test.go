package frame

import (
	"bytes"
	"testing"
)

// FuzzDecodePNG feeds arbitrary bytes to the frame decoder, which is the other place untrusted input arrives.
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
		frame, err := DecodePNG(1, 1, bytes.NewReader(body))
		if err != nil {
			return
		}
		// Whatever came back has to describe the pixels it carries.
		if frame.Width <= 0 || frame.Height <= 0 {
			t.Fatalf("a decoded frame has geometry %dx%d", frame.Width, frame.Height)
		}
		if len(frame.Pixels) == 0 {
			t.Fatal("a decoded frame carries no pixels")
		}
	})
}
