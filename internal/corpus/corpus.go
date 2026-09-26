// Package corpus generates frame sequences together with the ground truth that
// describes them.
//
// Ground truth is generated rather than labelled by hand, which is the decision recorded
// in research.md: a frame the generator drew knows exactly which pixels differ from the
// frame before it, so the accuracy score cannot be wrong about its own answer key.
//
// Ground truth is the extent of the changed pixels, not the element that moved. A delta
// engine is scored on whether it found the change, and a translated element changes pixels
// where it left and where it arrived; asking for one box around the element would reward a
// different answer than the engine is built to give. Classification is scored separately,
// by tests that know what the sequence did.
package corpus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Region is one rectangle in pixels.
type Region struct {
	Label string `json:"label"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	W     int    `json:"w"`
	H     int    `json:"h"`
}

func (r Region) Rect() (left, top, right, bottom int) {
	return r.X, r.Y, r.X + r.W, r.Y + r.H
}

// Expectation is the ground truth for one frame pair.
type Expectation struct {
	From int `json:"fromFrame"`
	To   int `json:"toFrame"`
	// Changes are the rectangles whose pixels differ between the two frames.
	Changes []Region `json:"changes"`
	// NoChange is true when nothing may be reported at all, which is what the noise case
	// measures.
	NoChange bool `json:"noChange"`
	// Classes states the class expected for each changed rectangle, in the same order as
	// Changes. Empty means the case does not assert classification, which is the case for
	// the sweep: it measures localisation only, and asserting classes there would encode the
	// implementation's opinion rather than the specification's.
	Classes     []string `json:"classes,omitempty"`
	Description string   `json:"description"`
}

// Manifest is written beside the frames and read by the scoring harness.
type Manifest struct {
	Case         string        `json:"case"`
	Width        int           `json:"width"`
	Height       int           `json:"height"`
	ScaleFactor  float64       `json:"scaleFactor"`
	Seed         int64         `json:"seed"`
	Frames       int           `json:"frames"`
	Paths        []string      `json:"paths"`
	Expectations []Expectation `json:"expectations"`
}

// Options controls one generation run.
type Options struct {
	Width  int
	Height int
	Frames int
	Pairs  int
	Seed   int64
	// MinLumaDifference is the difference below which a pixel does not count as changed,
	// on the same 0 to 255 scale the engine's noise floor uses. The answer key has to know
	// it: a change the engine is configured to ignore is not a change it failed to find.
	MinLumaDifference float64
	// NoiseAmplitude is the per-frame perturbation of the noise case. Zero derives it from
	// MinLumaDifference as half the floor, so that the difference between two noisy frames
	// cannot exceed the floor. Generating noise above the floor would make the case demand
	// that a difference the specification calls a change go unreported.
	NoiseAmplitude int
}

// noiseAmplitude is the perturbation the noise case applies per frame.
func (o Options) noiseAmplitude() int {
	if o.NoiseAmplitude > 0 {
		return o.NoiseAmplitude
	}
	amplitude := int(o.MinLumaDifference / 2)
	if amplitude < 1 {
		amplitude = 1
	}
	return amplitude
}

// DefaultOptions are the sizes the committed metrics were produced with.
func DefaultOptions() Options {
	return Options{
		Width:  320,
		Height: 240,
		Frames: 4,
		Pairs:  20,
		Seed:   20260926,
		// The engine's default noise floor is 0.02 of full scale, which is 5.1 levels.
		MinLumaDifference: 0.02 * 255,
	}
}

// Cases lists the case names NewCase and Generate understand.
func Cases() []string {
	return []string{"changed-label", "noise", "moving-button", "occluded-button", "sweep"}
}

// Generate writes a case into outDir as PNG frames plus a manifest, for inspection and for
// the demo. The scored measurement does not use this: it renders the same cases in memory, so
// a score never depends on the filesystem.
func Generate(caseName, outDir string, opts Options) (Manifest, error) {
	c, err := NewCase(caseName, opts)
	if err != nil {
		return Manifest{}, err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("corpus: cannot create %s: %w", outDir, err)
	}

	paths := make([]string, 0, c.Frames())
	for index := 0; index < c.Frames(); index++ {
		p := path(outDir, index+1)
		if err := saveImage(c.Frame(index), p); err != nil {
			return Manifest{}, err
		}
		paths = append(paths, p)
	}

	expectations := make([]Expectation, 0, c.Frames()-1)
	for index := 0; index+1 < c.Frames(); index++ {
		expectations = append(expectations, c.Expectation(index))
	}

	manifest := Manifest{
		Case:         c.Name(),
		Width:        opts.Width,
		Height:       opts.Height,
		ScaleFactor:  1,
		Seed:         opts.Seed,
		Frames:       c.Frames(),
		Paths:        paths,
		Expectations: expectations,
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, fmt.Errorf("corpus: cannot encode the manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "expected.json"), append(encoded, '\n'), 0o644); err != nil {
		return Manifest{}, fmt.Errorf("corpus: cannot write the manifest: %w", err)
	}
	return manifest, nil
}

// Load reads a manifest from a directory written by Generate.
func Load(dir string) (Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "expected.json"))
	if err != nil {
		return Manifest{}, fmt.Errorf("corpus: cannot read the manifest in %s: %w", dir, err)
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("corpus: cannot parse the manifest in %s: %w", dir, err)
	}
	return manifest, nil
}

func path(dir string, index int) string {
	return filepath.Join(dir, fmt.Sprintf("%03d.png", index))
}
