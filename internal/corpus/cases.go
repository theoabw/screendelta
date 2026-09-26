package corpus

import (
	"fmt"
	"image"
	"image/color"
)

var (
	panelFill   = color.RGBA{R: 90, G: 120, B: 170, A: 255}
	panelAlt    = color.RGBA{R: 170, G: 110, B: 90, A: 255}
	overlayFill = color.RGBA{R: 70, G: 70, B: 76, A: 255}
	backdrop    = color.RGBA{R: 30, G: 34, B: 40, A: 255}
)

// Case renders a sequence in memory and states what changed between consecutive frames.
//
// Rendering in memory is what lets the accuracy measurement cover thousands of pairs: the
// same cases write PNGs for inspection, but a scored run never touches the filesystem, so
// nothing about the score depends on disk.
type Case interface {
	Name() string
	Frames() int
	Frame(index int) *image.RGBA
	Expectation(index int) Expectation
}

// NewCase builds a case by name.
func NewCase(name string, opts Options) (Case, error) {
	if opts.Width <= 0 || opts.Height <= 0 {
		return nil, fmt.Errorf("corpus: width and height must be positive")
	}
	if opts.Frames < 2 {
		opts.Frames = 2
	}
	switch name {
	case "changed-label":
		return &changedLabel{opts: opts}, nil
	case "noise":
		return newNoise(opts), nil
	case "moving-button":
		return &movingButton{opts: opts}, nil
	case "occluded-button":
		return newOccluded(opts), nil
	case "sweep":
		return newSweep(opts), nil
	default:
		return nil, fmt.Errorf("corpus: unknown case %q, known cases are %v", name, Cases())
	}
}

// baseLayout places three panels. The middle one is the element most cases change; the
// others exist so that a change is not the only thing on the screen.
func baseLayout(opts Options) []Region {
	panelW := opts.Width / 4
	panelH := opts.Height / 8
	if panelW < 16 {
		panelW = 16
	}
	if panelH < 16 {
		panelH = 16
	}
	return []Region{
		{Label: "toolbar", X: opts.Width / 16, Y: opts.Height / 16, W: panelW * 2, H: panelH},
		{Label: "button", X: opts.Width / 4, Y: opts.Height / 2, W: panelW, H: panelH},
		{Label: "footer", X: opts.Width / 16, Y: opts.Height - opts.Height/8 - panelH, W: panelW * 3, H: panelH},
	}
}

func blank(opts Options) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, opts.Width, opts.Height))
	fillAll(img, backdrop)
	return img
}

// drawBase draws the shared layout, replacing the button when asked to.
func drawBase(opts Options, layout []Region, button *Region) *image.RGBA {
	img := blank(opts)
	for _, r := range layout {
		if r.Label == "button" {
			if button == nil {
				continue
			}
			drawPanel(img, *button, panelFill)
			continue
		}
		drawPanel(img, r, panelFill)
	}
	return img
}

// changedLabel changes one panel's colour in place, which is the simplest case: same
// footprint, different pixels.
type changedLabel struct{ opts Options }

func (c *changedLabel) Name() string { return "changed-label" }
func (c *changedLabel) Frames() int  { return 2 }
func (c *changedLabel) Expectation(index int) Expectation {
	return Expectation{From: 1, To: 2, Changes: c.changes(), Classes: []string{"changed"},
		Description: "one panel changed colour in place, so the changed pixels are the panel"}
}

func (c *changedLabel) changes() []Region {
	return Oracle(c.Frame(0), c.Frame(1), c.opts.MinLumaDifference)
}

func (c *changedLabel) Frame(index int) *image.RGBA {
	layout := baseLayout(c.opts)
	if index == 0 {
		return drawBase(c.opts, layout, &layout[1])
	}
	img := blank(c.opts)
	for _, r := range layout {
		if r.Label == "button" {
			drawPanel(img, r, panelAlt)
			continue
		}
		drawPanel(img, r, panelFill)
	}
	return img
}

// noise perturbs every pixel of an otherwise unchanged screen, which is the measurement of
// the false-removal rule. Its expectations are deliberately not derived from pixels: noise
// does change pixels, and the rule the case tests is that capture noise may not be reported
// at all.
type noiseCase struct {
	opts   Options
	frames []*image.RGBA
}

func newNoise(opts Options) *noiseCase {
	layout := baseLayout(opts)
	base := drawBase(opts, layout, &layout[1])
	rng := NewRand(opts.Seed)

	amplitude := opts.noiseAmplitude()
	frames := make([]*image.RGBA, 0, opts.Pairs*2)
	for pair := 0; pair < opts.Pairs; pair++ {
		for variant := 0; variant < 2; variant++ {
			clone := image.NewRGBA(base.Rect)
			copy(clone.Pix, base.Pix)
			addNoise(clone, rng, amplitude)
			frames = append(frames, clone)
		}
	}
	return &noiseCase{opts: opts, frames: frames}
}

func (n *noiseCase) Name() string { return "noise" }
func (n *noiseCase) Frames() int  { return len(n.frames) }
func (n *noiseCase) Frame(index int) *image.RGBA {
	return n.frames[index]
}

func (n *noiseCase) Expectation(index int) Expectation {
	return Expectation{
		From:     index + 1,
		To:       index + 2,
		NoChange: true,
		Description: "capture noise at half the floor on each frame, so no region may be reported: " +
			"the difference between two noisy frames cannot exceed the floor",
	}
}

// movingButton translates one panel by a step per frame. The changed pixels are the strip
// the panel left and the strip it arrived in, which the oracle computes rather than the
// generator guessing, because at larger steps the two strips do not touch and at steps wider
// than the panel they include the gap between them.
type movingButton struct {
	opts    Options
	layouts []Region
}

func (m *movingButton) Name() string { return "moving-button" }
func (m *movingButton) Frames() int  { return m.opts.Frames }

func (m *movingButton) step() int {
	step := m.opts.Width / 40
	if step < 2 {
		step = 2
	}
	return step
}

func (m *movingButton) button(index int) Region {
	button := baseLayout(m.opts)[1]
	button.X = button.X + index*m.step()
	if button.X+button.W > m.opts.Width {
		button.X = m.opts.Width - button.W
	}
	return button
}

func (m *movingButton) Frame(index int) *image.RGBA {
	layout := baseLayout(m.opts)
	button := m.button(index)
	return drawBase(m.opts, layout, &button)
}

func (m *movingButton) Expectation(index int) Expectation {
	expectation := Expectation{
		From:        index + 1,
		To:          index + 2,
		Changes:     Oracle(m.Frame(index), m.Frame(index+1), m.opts.MinLumaDifference),
		Description: "the panel translated, so the changed pixels are where it left and where it arrived",
	}
	// The first comparison of a stream has no baseline, so the engine reports changed. Later
	// comparisons know the footprint moved.
	if index == 0 {
		expectation.Classes = repeatClass("changed", len(expectation.Changes))
	} else {
		expectation.Classes = repeatClass("moved", len(expectation.Changes))
	}
	return expectation
}

// occluded places a panel, covers it, and uncovers it. The transition where the button stays
// covered is the case that caught a region being reported as removed for a frame pair that
// did not change at all.
type occluded struct {
	opts   Options
	frames []*image.RGBA
	states []bool
}

func newOccluded(opts Options) *occluded {
	layout := baseLayout(opts)
	button := layout[1]
	overlay := Region{Label: "overlay", X: button.X - 4, Y: button.Y - 4, W: button.W + 8, H: button.H + 8}

	states := []struct {
		button  bool
		overlay bool
	}{
		{button: true},
		{overlay: true},
		{overlay: true},
		{button: true},
	}

	c := &occluded{opts: opts}
	for _, state := range states {
		img := blank(opts)
		for _, r := range layout {
			if r.Label == "button" && !state.button {
				continue
			}
			drawPanel(img, r, panelFill)
		}
		if state.overlay {
			drawPanel(img, overlay, overlayFill)
		}
		c.frames = append(c.frames, img)
		c.states = append(c.states, state.overlay)
	}
	return c
}

func (o *occluded) Name() string { return "occluded-button" }
func (o *occluded) Frames() int  { return len(o.frames) }
func (o *occluded) Frame(index int) *image.RGBA {
	return o.frames[index]
}

func (o *occluded) Expectation(index int) Expectation {
	if o.states[index] == o.states[index+1] && o.states[index] {
		return Expectation{
			From:        index + 1,
			To:          index + 2,
			NoChange:    true,
			Description: "still covered, so nothing changed",
		}
	}
	changes := Oracle(o.frames[index], o.frames[index+1], o.opts.MinLumaDifference)
	return Expectation{
		From:        index + 1,
		To:          index + 2,
		Changes:     changes,
		Classes:     repeatClass("changed", len(changes)),
		Description: "the overlay appeared or disappeared, so its pixels changed",
	}
}

func repeatClass(class string, count int) []string {
	if count == 0 {
		return nil
	}
	classes := make([]string, count)
	for index := range classes {
		classes[index] = class
	}
	return classes
}
