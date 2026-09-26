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
	NoChange    bool   `json:"noChange"`
	Description string `json:"description"`
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
}

// DefaultOptions are the sizes the committed metrics were produced with.
func DefaultOptions() Options {
	return Options{Width: 320, Height: 240, Frames: 4, Pairs: 20, Seed: 20260926}
}

// Cases lists the case names Generate understands.
func Cases() []string {
	return []string{"changed-label", "noise", "moving-button", "occluded-button"}
}

// Generate writes a case into outDir and returns the manifest.
func Generate(caseName, outDir string, opts Options) (Manifest, error) {
	if opts.Width <= 0 || opts.Height <= 0 {
		return Manifest{}, fmt.Errorf("corpus: width and height must be positive")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("corpus: cannot create %s: %w", outDir, err)
	}

	var (
		paths        []string
		expectations []Expectation
		err          error
	)
	switch caseName {
	case "changed-label":
		paths, expectations, err = generateChangedLabel(outDir, opts)
	case "noise":
		paths, expectations, err = generateNoise(outDir, opts)
	case "moving-button":
		paths, expectations, err = generateMovingButton(outDir, opts)
	case "occluded-button":
		paths, expectations, err = generateOccludedButton(outDir, opts)
	default:
		return Manifest{}, fmt.Errorf("corpus: unknown case %q, known cases are %v", caseName, Cases())
	}
	if err != nil {
		return Manifest{}, err
	}

	manifest := Manifest{
		Case:         caseName,
		Width:        opts.Width,
		Height:       opts.Height,
		ScaleFactor:  1,
		Seed:         opts.Seed,
		Frames:       len(paths),
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
