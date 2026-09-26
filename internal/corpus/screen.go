package corpus

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
)

// screen is a synthesised frame under construction.
type screen struct {
	img *image.RGBA
}

func fillAll(img *image.RGBA, c color.RGBA) {
	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

// panel draws a filled rectangle with a light border, which is what the diff sees as an
// element: a change of both fill and edge.
func (s *screen) panel(r Region, fillColour color.RGBA) {
	border := color.RGBA{R: 200, G: 205, B: 210, A: 255}
	left, top, right, bottom := r.Rect()
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			if x < 0 || y < 0 || x >= s.img.Rect.Dx() || y >= s.img.Rect.Dy() {
				continue
			}
			if x == left || y == top || x == right-1 || y == bottom-1 {
				s.img.SetRGBA(x, y, border)
				continue
			}
			s.img.SetRGBA(x, y, fillColour)
		}
	}
}

func (s *screen) save(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("corpus: cannot create %s: %w", path, err)
	}
	defer file.Close()
	if err := png.Encode(file, s.img); err != nil {
		return fmt.Errorf("corpus: cannot encode %s: %w", path, err)
	}
	return nil
}

// addNoise perturbs every pixel by up to amplitude levels, which is what makes the noise
// case the measurement of the false-removal rule. Noise that respected element boundaries
// would not test anything.
func addNoise(img *image.RGBA, rng *rand.Rand, amplitude int) {
	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			original := img.RGBAAt(x, y)
			delta := rng.Intn(2*amplitude+1) - amplitude
			img.SetRGBA(x, y, color.RGBA{
				R: clamp(int(original.R) + delta),
				G: clamp(int(original.G) + delta),
				B: clamp(int(original.B) + delta),
				A: 255,
			})
		}
	}
}

func clamp(value int) uint8 {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return uint8(value)
}
