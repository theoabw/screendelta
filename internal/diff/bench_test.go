package diff

import (
	"image"
	"image/color"
	"testing"

	"github.com/theoabw/screendelta/internal/config"
	"github.com/theoabw/screendelta/internal/frame"
)

// These benchmarks exist to show where a frame pair's cost goes, so an optimisation can be aimed
// at the part that actually costs rather than the part that looks expensive.

const (
	benchWidth  = 1920
	benchHeight = 1080
)

func benchFrames() (frame.Frame, frame.Frame) {
	a := image.NewRGBA(image.Rect(0, 0, benchWidth, benchHeight))
	b := image.NewRGBA(image.Rect(0, 0, benchWidth, benchHeight))
	for y := 0; y < benchHeight; y++ {
		for x := 0; x < benchWidth; x++ {
			shade := uint8(30 + (x/64+y/64)%160)
			a.SetRGBA(x, y, colorOf(shade))
			if x > 900 && x < 1200 && y > 400 && y < 600 {
				b.SetRGBA(x, y, colorOf(shade/2))
				continue
			}
			b.SetRGBA(x, y, colorOf(shade))
		}
	}
	return frame.Frame{Sequence: 1, Width: benchWidth, Height: benchHeight, Format: frame.FormatRGBA8, ScaleFactor: 1, Pixels: a.Pix},
		frame.Frame{Sequence: 2, Width: benchWidth, Height: benchHeight, Format: frame.FormatRGBA8, ScaleFactor: 1, Pixels: b.Pix}
}

func colorOf(shade uint8) color.RGBA {
	return color.RGBA{R: shade, G: shade, B: shade, A: 255}
}

// BenchmarkPlanePass measures the conversion of a frame into the luma plane, which is the single
// largest cost in the engine because it touches every pixel once.
func BenchmarkPlanePass(b *testing.B) {
	first, _ := benchFrames()
	d := New()
	buffer := make([]byte, benchWidth*benchHeight)
	b.SetBytes(benchWidth * benchHeight * 4)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.fillPlane(buffer, first, 32)
	}
}

// BenchmarkGridFromPlane measures deriving the fingerprint grid from a plane already in hand.
func BenchmarkGridFromPlane(b *testing.B) {
	first, _ := benchFrames()
	d := New()
	plane := make([]byte, benchWidth*benchHeight)
	d.fillPlane(plane, first, 32)
	d.prepareCells(32)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.gridFrom(plane, first, 32)
	}
}

func BenchmarkComponentsWithPlanesHeld(b *testing.B) {
	first, second := benchFrames()
	cfg := config.Defaults()
	d := New()
	d.plane(first, config.Defaults().Fingerprint.GridSize)
	d.plane(second, config.Defaults().Fingerprint.GridSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.components(first, second, cfg)
	}
}

func BenchmarkFingerprint(b *testing.B) {
	_, second := benchFrames()
	cfg := config.Defaults()
	d := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		second.Sequence = uint64(i + 1)
		if _, err := d.Fingerprint(second, cfg); err != nil {
			b.Fatal(err)
		}
	}
}
