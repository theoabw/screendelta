package corpus

import (
	"image"
	"image/color"
	"math/rand"
)

// Oracle computes ground truth from rendered pixels rather than from what the generator
// intended.
//
// This exists because the first version of the corpus recorded the element the generator
// moved, and the review showed that definition is wrong in three ways: it demanded regions
// for frames that only differ by a translation larger than the element, it missed clipped
// overlays at the frame edge, and it asked for one box where the pixels changed in two
// places. Deriving the expectation from the rendered pair removes every one of those
// assumptions: the answer key is computed from the same images the engine sees, by a method
// that shares no code with the engine.
//
// The method is deliberately simpler than the engine's: any channel difference counts, the
// connectivity is four-way rather than eight-way, and there is no noise floor, no minimum
// area and no growth margin. Two different definitions of the same thing agree on realistic
// material and disagree on the margins, which is what an independent oracle is for.
func Oracle(before, after *image.RGBA, minLumaDifference float64) []Region {
	bounds := before.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if after.Bounds() != bounds {
		return nil
	}

	changed := make([]bool, width*height)
	any := false
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			b := before.RGBAAt(bounds.Min.X+x, bounds.Min.Y+y)
			a := after.RGBAAt(bounds.Min.X+x, bounds.Min.Y+y)
			if a == b {
				continue
			}
			if difference := abs(lumaOf(a) - lumaOf(b)); float64(difference) <= minLumaDifference {
				continue
			}
			changed[y*width+x] = true
			any = true
		}
	}
	if !any {
		return nil
	}

	// Flood fill with an explicit stack so a large changed area cannot exhaust the
	// goroutine stack, and label in scan order so the result is deterministic.
	visited := make([]bool, width*height)
	regions := make([]Region, 0, 8)
	stack := make([]int, 0, 64)

	for start := 0; start < len(changed); start++ {
		if !changed[start] || visited[start] {
			continue
		}
		visited[start] = true
		stack = append(stack[:0], start)
		left, top := width, height
		right, bottom := 0, 0

		for len(stack) > 0 {
			index := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			x, y := index%width, index/width
			if x < left {
				left = x
			}
			if y < top {
				top = y
			}
			if x+1 > right {
				right = x + 1
			}
			if y+1 > bottom {
				bottom = y + 1
			}
			for _, neighbour := range []int{index - 1, index + 1, index - width, index + width} {
				if neighbour < 0 || neighbour >= len(changed) || visited[neighbour] || !changed[neighbour] {
					continue
				}
				// Keep horizontal neighbours on the same row.
				if (neighbour == index-1 || neighbour == index+1) && neighbour/width != y {
					continue
				}
				visited[neighbour] = true
				stack = append(stack, neighbour)
			}
		}
		regions = append(regions, Region{Label: "changed", X: left, Y: top, W: right - left, H: bottom - top})
	}
	return regions
}

// lumaOf converts one pixel to the single channel the engine compares, using the same integer
// weights, so the answer key and the engine cannot disagree about what a level of difference is.
func lumaOf(c color.RGBA) int {
	return (299*int(c.R) + 587*int(c.G) + 114*int(c.B)) / 1000
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// Rand is a small deterministic source the sweep cases use, so a run is reproducible from
// its seed alone.
type Rand struct{ *rand.Rand }

// NewRand returns a deterministic source.
func NewRand(seed int64) *Rand { return &Rand{rand.New(rand.NewSource(seed))} }

// Intn is a convenience wrapper so callers do not import math/rand themselves.
func (r *Rand) Intn(n int) int { return r.Rand.Intn(n) }
