package corpus

import (
	"image"
	"image/color"
	"sort"
)

// sweep is the case the accuracy requirement is measured on: thousands of frame pairs in
// which several panels appear, move and disappear per frame, with the answer key computed
// from the rendered pixels.
//
// It exists because SC-001 asks for at least 5,000 frame pairs, and a corpus of hand-written
// cases cannot reach that honestly. The panels are placed on a coarse grid with an inset, so
// neighbouring panels never touch and the answer key stays a set of separate rectangles
// rather than one merged blob. Panels are at least sixteen pixels on a side, which keeps the
// comparison away from the one-pixel case where a two-pixel growth margin costs more
// intersection than any detector can afford.
type sweep struct {
	opts   Options
	frames []*image.RGBA
	cols   int
	rows   int
}

func newSweep(opts Options) *sweep {
	cols := opts.Width / 80
	rows := opts.Height / 60
	if cols < 2 {
		cols = 2
	}
	if rows < 2 {
		rows = 2
	}

	s := &sweep{opts: opts, cols: cols, rows: rows}
	rng := NewRand(opts.Seed)
	for index := 0; index < opts.Frames; index++ {
		s.frames = append(s.frames, s.render(rng))
	}
	return s
}

func (s *sweep) Name() string { return "sweep" }
func (s *sweep) Frames() int  { return len(s.frames) }
func (s *sweep) Frame(index int) *image.RGBA {
	return s.frames[index]
}

// Expectation derives the changed rectangles from the two rendered frames, so no assumption
// about what the panels did can leak into the answer key.
func (s *sweep) Expectation(index int) Expectation {
	return Expectation{
		From:        index + 1,
		To:          index + 2,
		Changes:     Oracle(s.frames[index], s.frames[index+1], s.opts.MinLumaDifference),
		Description: "several panels appeared, moved or vanished, so the answer key is the changed pixels",
	}
}

// render draws between two and four panels in distinct grid cells.
func (s *sweep) render(rng *Rand) *image.RGBA {
	img := blank(s.opts)

	cellWidth := s.opts.Width / s.cols
	cellHeight := s.opts.Height / s.rows
	inset := 8
	if cellWidth < 32 || cellHeight < 32 {
		inset = 2
	}
	if cellWidth-2*inset < 16 || cellHeight-2*inset < 16 {
		// Too small for the panel size the sweep promises, so draw nothing rather than a
		// panel whose answer key would be dominated by the growth margin.
		return img
	}

	wanted := 2 + rng.Intn(3)
	chosen := make(map[int]bool, wanted)
	for attempt := 0; attempt < wanted*8 && len(chosen) < wanted; attempt++ {
		chosen[rng.Intn(s.cols*s.rows)] = true
	}
	// The cells are drawn in sorted order rather than in map order. Iterating the map directly made the
	// case depend on Go's randomised map iteration order, because each drawn panel consumes a shade from
	// the random source: the same seed produced different frames on every run, which made the accuracy
	// measurement irreproducible.
	cells := make([]int, 0, len(chosen))
	for cell := range chosen {
		cells = append(cells, cell)
	}
	sort.Ints(cells)

	for _, cell := range cells {
		column := cell % s.cols
		row := cell / s.cols
		panel := Region{
			X: column*cellWidth + inset,
			Y: row*cellHeight + inset,
			W: cellWidth - 2*inset,
			H: cellHeight - 2*inset,
		}
		shade := uint8(60 + rng.Intn(180))
		drawPanel(img, panel, color.RGBA{R: shade, G: shade, B: shade, A: 255})
	}
	return img
}
