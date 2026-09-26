package corpus

import (
	"image"
	"image/color"
	"math/rand"
)

var (
	panelFill   = color.RGBA{R: 90, G: 120, B: 170, A: 255}
	panelAlt    = color.RGBA{R: 170, G: 110, B: 90, A: 255}
	overlayFill = color.RGBA{R: 70, G: 70, B: 76, A: 255}
)

// baseLayout places three panels. The middle one is the element most cases change; the
// others exist so that a change is not the only thing on the screen.
func baseLayout(opts Options) []Region {
	panelW := opts.Width / 4
	panelH := opts.Height / 8
	if panelW < 8 {
		panelW = 8
	}
	if panelH < 8 {
		panelH = 8
	}
	return []Region{
		{Label: "toolbar", X: opts.Width / 16, Y: opts.Height / 16, W: panelW * 2, H: panelH},
		{Label: "button", X: opts.Width / 4, Y: opts.Height / 2, W: panelW, H: panelH},
		{Label: "footer", X: opts.Width / 16, Y: opts.Height - opts.Height/8, W: panelW * 3, H: panelH / 2},
	}
}

func newScreen(opts Options) *screen {
	img := image.NewRGBA(image.Rect(0, 0, opts.Width, opts.Height))
	fillAll(img, color.RGBA{R: 30, G: 34, B: 40, A: 255})
	return &screen{img: img}
}

func drawLayout(opts Options, layout []Region, replace func(Region) (Region, bool)) *screen {
	s := newScreen(opts)
	for _, r := range layout {
		if replacement, swapped := replace(r); swapped {
			s.panel(replacement, panelFill)
			continue
		}
		s.panel(r, panelFill)
	}
	return s
}

func generateChangedLabel(outDir string, opts Options) ([]string, []Expectation, error) {
	layout := baseLayout(opts)
	button := layout[1]

	first := drawLayout(opts, layout, func(r Region) (Region, bool) { return r, false })
	firstPath := path(outDir, 1)
	if err := first.save(firstPath); err != nil {
		return nil, nil, err
	}

	second := newScreen(opts)
	for _, r := range layout {
		if r.Label == "button" {
			second.panel(r, panelAlt)
			continue
		}
		second.panel(r, panelFill)
	}
	secondPath := path(outDir, 2)
	if err := second.save(secondPath); err != nil {
		return nil, nil, err
	}

	return []string{firstPath, secondPath}, []Expectation{{
		From:        1,
		To:          2,
		Changes:     []Region{button},
		Description: "one panel changed colour, so exactly the panel's pixels should be reported",
	}}, nil
}

func generateNoise(outDir string, opts Options) ([]string, []Expectation, error) {
	layout := baseLayout(opts)
	base := drawLayout(opts, layout, func(r Region) (Region, bool) { return r, false })
	rng := rand.New(rand.NewSource(opts.Seed))

	var (
		paths        []string
		expectations []Expectation
	)
	for pair := 1; pair <= opts.Pairs; pair++ {
		for variant := 0; variant < 2; variant++ {
			noisy := copyPixels(base)
			addNoise(noisy, rng, 3)
			p := path(outDir, (pair-1)*2+variant+1)
			if err := (&screen{img: noisy}).save(p); err != nil {
				return nil, nil, err
			}
			paths = append(paths, p)
		}
		expectations = append(expectations, Expectation{
			From:        (pair-1)*2 + 1,
			To:          (pair-1)*2 + 2,
			NoChange:    true,
			Description: "capture noise only, so no region may be reported",
		})
	}
	return paths, expectations, nil
}

func generateMovingButton(outDir string, opts Options) ([]string, []Expectation, error) {
	layout := baseLayout(opts)
	button := layout[1]

	step := opts.Width / 40
	if step < 2 {
		step = 2
	}

	var (
		paths        []string
		expectations []Expectation
	)
	for index := 1; index <= opts.Frames; index++ {
		moved := button
		moved.X = button.X + (index-1)*step
		s := drawLayout(opts, layout, func(r Region) (Region, bool) {
			if r.Label == "button" {
				return moved, true
			}
			return r, false
		})
		p := path(outDir, index)
		if err := s.save(p); err != nil {
			return nil, nil, err
		}
		paths = append(paths, p)

		if index > 1 {
			previous := button
			previous.X = button.X + (index-2)*step
			expectations = append(expectations, Expectation{
				From:        index - 1,
				To:          index,
				Changes:     translationStrips(previous, moved),
				Description: "the panel translated, so the pixels that changed are where it left and where it arrived",
			})
		}
	}
	return paths, expectations, nil
}

// translationStrips returns the two strips a horizontal translation changes: the part of
// the old position the panel no longer covers, and the part of the new position it did not
// cover before.
func translationStrips(previous, current Region) []Region {
	if current.X == previous.X {
		return nil
	}
	if current.X > previous.X {
		width := current.X - previous.X
		return []Region{
			{Label: "vacated", X: previous.X, Y: previous.Y, W: width, H: previous.H},
			{Label: "occupied", X: previous.X + previous.W, Y: previous.Y, W: width, H: previous.H},
		}
	}
	width := previous.X - current.X
	return []Region{
		{Label: "vacated", X: current.X + current.W, Y: previous.Y, W: width, H: previous.H},
		{Label: "occupied", X: current.X, Y: previous.Y, W: width, H: previous.H},
	}
}

func generateOccludedButton(outDir string, opts Options) ([]string, []Expectation, error) {
	layout := baseLayout(opts)
	button := layout[1]
	overlay := Region{Label: "overlay", X: button.X - 4, Y: button.Y - 4, W: button.W + 8, H: button.H + 8}

	states := []struct {
		drawButton  bool
		drawOverlay bool
		description string
		noChange    bool
	}{
		{drawButton: true, description: "button visible"},
		{drawOverlay: true, description: "button covered, so the overlay's pixels changed"},
		{drawOverlay: true, description: "still covered, so nothing changed", noChange: true},
		{drawButton: true, description: "button visible again, so the overlay's pixels changed back"},
	}

	var (
		paths        []string
		expectations []Expectation
	)
	for index, state := range states {
		s := newScreen(opts)
		for _, r := range layout {
			if r.Label == "button" {
				if state.drawButton {
					s.panel(button, panelFill)
				} else {
					s.panel(button, color.RGBA{R: 30, G: 34, B: 40, A: 255})
				}
				continue
			}
			s.panel(r, panelFill)
		}
		if state.drawOverlay {
			s.panel(overlay, overlayFill)
		}
		p := path(outDir, index+1)
		if err := s.save(p); err != nil {
			return nil, nil, err
		}
		paths = append(paths, p)

		if index > 0 {
			expectation := Expectation{
				From:        index,
				To:          index + 1,
				Description: state.description,
			}
			if state.noChange {
				expectation.NoChange = true
			} else {
				expectation.Changes = []Region{overlay}
				if states[index-1].drawOverlay == state.drawOverlay {
					expectation.Changes = nil
					expectation.NoChange = true
				}
			}
			expectations = append(expectations, expectation)
		}
	}
	return paths, expectations, nil
}

func copyPixels(s *screen) *image.RGBA {
	clone := image.NewRGBA(s.img.Rect)
	copy(clone.Pix, s.img.Pix)
	return clone
}
