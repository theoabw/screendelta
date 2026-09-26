// Command corpusgen writes frame sequences together with the ground truth that
// describes them.
//
// Ground truth is generated rather than labelled by hand, which is the decision
// recorded in research.md: a rendered or synthesised frame knows exactly where every
// element is, so the accuracy score cannot be wrong about its own answer key. Only the
// generator, its seed and the resulting metrics are committed; the images are not,
// because they are reproducible from the command that made them.
//
// Usage:
//
//	corpusgen --case changed-label --out /tmp/frames
//	corpusgen --case noise --pairs 50 --out /tmp/noise
//
// Each case writes 001.png, 002.png and so on for a sequence, plus expected.json
// describing what changed between consecutive frames and, for the noise case, that
// nothing did.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
)

// region is one element placed on a synthesised screen, in pixels.
type region struct {
	Label string `json:"label"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	W     int    `json:"w"`
	H     int    `json:"h"`
}

// expectation is the ground truth for one frame pair.
type expectation struct {
	From        int      `json:"fromFrame"`
	To          int      `json:"toFrame"`
	Changes     []region `json:"changes"`
	NoChange    bool     `json:"noChange"`
	Description string   `json:"description"`
}

// manifest is written beside the frames.
type manifest struct {
	Case         string        `json:"case"`
	Width        int           `json:"width"`
	Height       int           `json:"height"`
	ScaleFactor  float64       `json:"scaleFactor"`
	Seed         int64         `json:"seed"`
	Frames       int           `json:"frames"`
	Expectations []expectation `json:"expectations"`
}

func main() {
	var (
		caseName = flag.String("case", "changed-label", "case to generate: changed-label, noise, moving-button, occluded-button")
		outDir   = flag.String("out", "corpus", "directory to write frames and expected.json into")
		width    = flag.Int("width", 320, "frame width in pixels")
		height   = flag.Int("height", 240, "frame height in pixels")
		frames   = flag.Int("frames", 4, "number of frames for sequence cases")
		pairs    = flag.Int("pairs", 20, "number of pairs for the noise case")
		seed     = flag.Int64("seed", 20260926, "random seed, recorded in the manifest")
	)
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "corpusgen: cannot create %s: %v\n", *outDir, err)
		os.Exit(2)
	}

	options := options{
		width:  *width,
		height: *height,
		frames: *frames,
		pairs:  *pairs,
		seed:   *seed,
	}

	var (
		written      []string
		expectations []expectation
		err          error
	)
	switch *caseName {
	case "changed-label":
		written, expectations, err = generateChangedLabel(*outDir, options)
	case "noise":
		written, expectations, err = generateNoise(*outDir, options)
	case "moving-button":
		written, expectations, err = generateMovingButton(*outDir, options)
	case "occluded-button":
		written, expectations, err = generateOccludedButton(*outDir, options)
	default:
		fmt.Fprintf(os.Stderr, "corpusgen: unknown case %q\n", *caseName)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "corpusgen: %v\n", err)
		os.Exit(2)
	}

	document := manifest{
		Case:         *caseName,
		Width:        *width,
		Height:       *height,
		ScaleFactor:  1,
		Seed:         *seed,
		Frames:       len(written),
		Expectations: expectations,
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "corpusgen: cannot encode the manifest: %v\n", err)
		os.Exit(2)
	}
	manifestPath := filepath.Join(*outDir, "expected.json")
	if err := os.WriteFile(manifestPath, append(encoded, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "corpusgen: cannot write %s: %v\n", manifestPath, err)
		os.Exit(2)
	}

	fmt.Printf("corpusgen: wrote %d frames and %s\n", len(written), manifestPath)
}

type options struct {
	width  int
	height int
	frames int
	pairs  int
	seed   int64
}

// screen is a synthesised frame under construction.
type screen struct {
	img *image.RGBA
}

func newScreen(width, height int) *screen {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	fill(img, color.RGBA{R: 30, G: 34, B: 40, A: 255})
	return &screen{img: img}
}

func fill(img *image.RGBA, c color.RGBA) {
	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

// panel draws a filled rectangle with a light border, which is what the diff sees as
// an element: a change of both fill and edge.
func (s *screen) panel(r region, fillColour color.RGBA) {
	border := color.RGBA{R: 200, G: 205, B: 210, A: 255}
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			if x < 0 || y < 0 || x >= s.img.Rect.Dx() || y >= s.img.Rect.Dy() {
				continue
			}
			edge := x == r.X || y == r.Y || x == r.X+r.W-1 || y == r.Y+r.H-1
			if edge {
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
		return fmt.Errorf("cannot create %s: %w", path, err)
	}
	defer file.Close()
	if err := png.Encode(file, s.img); err != nil {
		return fmt.Errorf("cannot encode %s: %w", path, err)
	}
	return nil
}

func framePath(dir string, index int) string {
	return filepath.Join(dir, fmt.Sprintf("%03d.png", index))
}

// addNoise perturbs every pixel by up to amplitude levels, which is what makes the
// noise case the test of the false-removal rule.
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
