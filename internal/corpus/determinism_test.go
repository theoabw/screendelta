package corpus

import (
	"bytes"
	"testing"
)

// TestTheGeneratorIsDeterministic is a regression test for a defect that made every accuracy number in the
// project irreproducible: the sweep case drew its panels by iterating a map, and each drawn panel consumes a
// shade from the random source, so Go's randomised map iteration order gave a different frame for the same
// seed on every run. The measurement still scored F1 1.0000 each time, which is exactly why it went unnoticed:
// a number that is always perfect hides the fact that it is a different number.
func TestTheGeneratorIsDeterministic(t *testing.T) {
	options := Options{Width: 320, Height: 240, Frames: 12, Seed: 20260926, MinLumaDifference: 0.02 * 255}

	generate := func(opts Options) []byte {
		c, err := NewCase("sweep", opts)
		if err != nil {
			t.Fatalf("generating the case failed: %v", err)
		}
		var buffer bytes.Buffer
		for index := 0; index < c.Frames(); index++ {
			buffer.Write(c.Frame(index).Pix)
		}
		return buffer.Bytes()
	}

	first := generate(options)
	second := generate(options)
	if !bytes.Equal(first, second) {
		t.Fatalf("the same seed produced different frames: %d bytes against %d, and the first difference is at %d",
			len(first), len(second), firstDifference(first, second))
	}

	other := options
	other.Seed = options.Seed + 1
	if bytes.Equal(first, generate(other)) {
		t.Fatal("a different seed produced identical frames, so the seed does not reach the generator")
	}
}

// TestEveryCaseIsDeterministic holds the same rule for the named cases rather than for the sweep alone.
func TestEveryCaseIsDeterministic(t *testing.T) {
	for _, name := range Cases() {
		t.Run(name, func(t *testing.T) {
			options := DefaultOptions()
			generate := func() []byte {
				c, err := NewCase(name, options)
				if err != nil {
					t.Fatalf("generating %s failed: %v", name, err)
				}
				var buffer bytes.Buffer
				for index := 0; index < c.Frames(); index++ {
					buffer.Write(c.Frame(index).Pix)
				}
				return buffer.Bytes()
			}
			if !bytes.Equal(generate(), generate()) {
				t.Fatalf("the %s case produced different frames from the same seed", name)
			}
		})
	}
}

func firstDifference(first, second []byte) int {
	limit := len(first)
	if len(second) < limit {
		limit = len(second)
	}
	for index := 1; index < limit; index++ {
		if first[index] != second[index] {
			return index
		}
	}
	return limit
}
