package main

import (
	"image"
	"image/color"
	"math/rand"
)

// The cases below are the ones the tests consume: a single changed element, noise
// alone, an element that moves, and an element that is covered and returns. Each one
// knows exactly what changed, because it placed the elements itself.

var (
	panelFill   = color.RGBA{R: 90, G: 120, B: 170, A: 255}
	panelAlt    = color.RGBA{R: 170, G: 110, B: 90, A: 255}
	overlayFill = color.RGBA{R: 70, G: 70, B: 76, A: 255}
)

// baseLayout places three panels. The middle one is the element most cases change; the
// others exist so that a change is not the only thing on the screen.
func baseLayout(opts options) []region {
	panelW := opts.width / 4
	panelH := opts.height / 8
	if panelW < 8 {
		panelW = 8
	}
	if panelH < 8 {
		panelH = 8
	}
	return []region{
		{Label: "toolbar", X: opts.width / 16, Y: opts.height / 16, W: panelW * 2, H: panelH},
		{Label: "button", X: opts.width / 4, Y: opts.height / 2, W: panelW, H: panelH},
		{Label: "footer", X: opts.width / 16, Y: opts.height - opts.height/8, W: panelW * 3, H: panelH / 2},
	}
}

func drawLayout(layout []region, opts options, replace func(region) (region, bool)) *screen {
	s := newScreen(opts.width, opts.height)
	for _, r := range layout {
		if replacement, swapped := replace(r); swapped {
			s.panel(replacement, panelFill)
			continue
		}
		s.panel(r, panelFill)
	}
	return s
}

func generateChangedLabel(outDir string, opts options) ([]string, []expectation, error) {
	layout := baseLayout(opts)

	// Frame one is the normal screen. Frame two keeps the geometry and changes the
	// button's fill, which is a label change as far as the diff is concerned.
	first := drawLayout(layout, opts, func(r region) (region, bool) { return r, false })
	firstPath := framePath(outDir, 1)
	if err := first.save(firstPath); err != nil {
		return nil, nil, err
	}

	button := layout[1]
	second := newScreen(opts.width, opts.height)
	for _, r := range layout {
		if r.Label == "button" {
			second.panel(r, panelAlt)
			continue
		}
		second.panel(r, panelFill)
	}
	secondPath := framePath(outDir, 2)
	if err := second.save(secondPath); err != nil {
		return nil, nil, err
	}

	return []string{firstPath, secondPath}, []expectation{{
		From:        1,
		To:          2,
		Changes:     []region{button},
		Description: "one panel changed colour, so exactly one region should be reported",
	}}, nil
}

func generateNoise(outDir string, opts options) ([]string, []expectation, error) {
	layout := baseLayout(opts)
	base := drawLayout(layout, opts, func(r region) (region, bool) { return r, false })
	rng := rand.New(rand.NewSource(opts.seed))

	var (
		written      []string
		expectations []expectation
	)
	for pair := 1; pair <= opts.pairs; pair++ {
		for variant := 0; variant < 2; variant++ {
			noisy := copyPixels(base)
			addNoise(noisy, rng, 3)
			path := framePath(outDir, (pair-1)*2+variant+1)
			if err := (&screen{img: noisy}).save(path); err != nil {
				return nil, nil, err
			}
			written = append(written, path)
		}
		expectations = append(expectations, expectation{
			From:        (pair-1)*2 + 1,
			To:          (pair-1)*2 + 2,
			NoChange:    true,
			Description: "capture noise only, so no region may be reported",
		})
	}
	return written, expectations, nil
}

func generateMovingButton(outDir string, opts options) ([]string, []expectation, error) {
	layout := baseLayout(opts)
	button := layout[1]

	step := opts.width / 40
	if step < 2 {
		step = 2
	}

	var (
		written      []string
		expectations []expectation
	)
	for index := 1; index <= opts.frames; index++ {
		moved := button
		moved.X = button.X + (index-1)*step
		s := drawLayout(layout, opts, func(r region) (region, bool) {
			if r.Label == "button" {
				return moved, true
			}
			return r, false
		})
		path := framePath(outDir, index)
		if err := s.save(path); err != nil {
			return nil, nil, err
		}
		written = append(written, path)

		if index > 1 {
			expectations = append(expectations, expectation{
				From:        index - 1,
				To:          index,
				Changes:     []region{moved},
				Description: "one panel translated by one step, so it should be reported as moved",
			})
		}
	}
	return written, expectations, nil
}

func generateOccludedButton(outDir string, opts options) ([]string, []expectation, error) {
	layout := baseLayout(opts)
	button := layout[1]
	overlay := region{Label: "overlay", X: button.X - 4, Y: button.Y - 4, W: button.W + 8, H: button.H + 8}

	states := []struct {
		drawButton  bool
		drawOverlay bool
		description string
	}{
		{drawButton: true, description: "button visible"},
		{drawOverlay: true, description: "button covered by an overlay"},
		{drawOverlay: true, description: "still covered"},
		{drawButton: true, description: "button visible again, so its identity must be new and marked uncertain"},
	}

	var (
		written      []string
		expectations []expectation
	)
	for index, state := range states {
		s := newScreen(opts.width, opts.height)
		for _, r := range layout {
			if r.Label == "button" {
				if state.drawButton {
					s.panel(button, panelFill)
				}
				continue
			}
			s.panel(r, panelFill)
		}
		if state.drawOverlay {
			s.panel(overlay, overlayFill)
		}
		path := framePath(outDir, index+1)
		if err := s.save(path); err != nil {
			return nil, nil, err
		}
		written = append(written, path)

		if index > 0 {
			expectations = append(expectations, expectation{
				From:        index,
				To:          index + 1,
				Description: state.description,
			})
		}
	}
	return written, expectations, nil
}

// copyPixels clones a frame so noise can be applied without disturbing the original.
func copyPixels(s *screen) *image.RGBA {
	clone := image.NewRGBA(s.img.Rect)
	copy(clone.Pix, s.img.Pix)
	return clone
}
