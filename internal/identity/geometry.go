package identity

import (
	"image"

	"github.com/theoabw/screendelta/internal/delta"
)

// The match rules are constants rather than configuration, because each one is a statement about
// what "the same element" means and a caller changing them would be changing the guarantee rather
// than tuning the engine. What a caller does control is how far an element may move and how long it
// may be hidden, both of which are in the configuration document.

const (
	// minimumOverlap is the intersection over union below which two rectangles are not candidates
	// for the same element, however close their centres are.
	minimumOverlap = 0.30
	// minimumAreaRatio and maximumAreaRatio bound how much an element's area may change and still be
	// called the same element. An element that doubles in size is as likely to be a different thing
	// that appeared where the first one was.
	minimumAreaRatio = 0.5
	maximumAreaRatio = 2.0
)

// rectangle is the pixel form of normalized bounds, kept local so the matching below reads in whole
// pixels while the document speaks in fractions of a frame.
type rectangle = image.Rectangle

// rect builds a pixel rectangle from its corners.
func rect(left, top, right, bottom int) rectangle {
	return image.Rect(left, top, right, bottom)
}

func pixelRect(bounds delta.Bounds, frameWidth, frameHeight int) rectangle {
	left := int(bounds.X*float64(frameWidth) + 0.5)
	top := int(bounds.Y*float64(frameHeight) + 0.5)
	return image.Rect(left, top,
		left+int(bounds.W*float64(frameWidth)+0.5),
		top+int(bounds.H*float64(frameHeight)+0.5))
}

// intersectionOverUnion is the overlap of two rectangles, and doubles as the match score.
func intersectionOverUnion(a, b rectangle) float64 {
	overlap := a.Intersect(b)
	if overlap.Empty() {
		return 0
	}
	shared := float64(overlap.Dx() * overlap.Dy())
	union := float64(a.Dx()*a.Dy() + b.Dx()*b.Dy())
	if union <= 0 {
		return 0
	}
	return shared / (union - shared)
}

// withinTolerance reports whether two rectangles are close enough to be the same element, measured
// centre to centre. A gate rather than a penalty, because a distant rectangle that overlaps slightly
// should not be able to win on overlap alone.
func withinTolerance(previous, current rectangle, tolerancePixels int) bool {
	dx := (current.Min.X + current.Max.X) - (previous.Min.X + previous.Max.X)
	dy := (current.Min.Y + current.Max.Y) - (previous.Min.Y + previous.Max.Y)
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	// The centres are compared as doubled coordinates, so halve the allowance once here rather than
	// dividing per axis and losing the halves.
	if tolerancePixels < 1 {
		return dx == 0 && dy == 0
	}
	return dx <= 2*tolerancePixels && dy <= 2*tolerancePixels
}

// similarSize reports whether two rectangles are similar enough in area to be the same element.
func similarSize(previous, current rectangle) bool {
	previousArea := previous.Dx() * previous.Dy()
	currentArea := current.Dx() * current.Dy()
	if previousArea <= 0 || currentArea <= 0 {
		return false
	}
	ratio := float64(currentArea) / float64(previousArea)
	return ratio >= minimumAreaRatio && ratio <= maximumAreaRatio
}

// unionBounds is the smallest normalized rectangle covering both, used when one element is reported
// as more than one region in a frame.
func unionBounds(a, b delta.Bounds) delta.Bounds {
	left := minFloat(a.X, b.X)
	top := minFloat(a.Y, b.Y)
	right := maxFloat(a.X+a.W, b.X+b.W)
	bottom := maxFloat(a.Y+a.H, b.Y+b.H)
	return delta.Bounds{X: left, Y: top, W: right - left, H: bottom - top}
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// coveredFraction is how much of inner the outer rectangle covers, which is the measure used when a
// reported area is part of a tracked element rather than most of it.
func coveredFraction(outer, inner rectangle) float64 {
	if inner.Empty() {
		return 0
	}
	overlap := outer.Intersect(inner)
	if overlap.Empty() {
		return 0
	}
	return float64(overlap.Dx()*overlap.Dy()) / float64(inner.Dx()*inner.Dy())
}

// withinReach reports whether inner lies inside outer grown by the tolerance, which is how an area
// that is part of a tracked element is judged: it may be anywhere inside the element, and the
// tolerance only has to absorb the growth of the element's own footprint between frames.
func withinReach(outer, inner rectangle, tolerancePixels int) bool {
	grown := outer.Inset(-tolerancePixels)
	return inner.Min.X >= grown.Min.X && inner.Min.Y >= grown.Min.Y &&
		inner.Max.X <= grown.Max.X && inner.Max.Y <= grown.Max.Y
}

// evolveFootprint updates an element's footprint from what was observed of it in a frame.
//
// The rule is that a footprint translates with its element and only absorbs ground it did not already
// cover. The observation is where the element showed itself by changing; the footprint is the engine's
// belief about the whole element, which is larger whenever only part of it changed.
//
// The alternative, unioning the observation into the footprint, is what made a change along one edge
// shrink an element to that strip, so a change along the other edge in the next frame looked like a new
// element. Keeping the last full footprint instead loses an element as soon as it moves further than
// the motion tolerance, because the new position falls outside a frozen rectangle. A decaying envelope
// follows but lags by a step every frame and never catches up.
//
// The translation is chosen from nine candidates rather than searched, and the one that best explains
// the observation wins, so the rule stays a handful of rectangle operations and cannot depend on
// anything but the geometry it is given.
func evolveFootprint(footprint, observed rectangle, tolerancePixels int) rectangle {
	if footprint.Empty() {
		return observed
	}
	if observed.Empty() {
		return footprint
	}

	best := footprint
	bestOverlap := intersectionArea(footprint, observed)

	dxs := [...]int{0, observed.Min.X - footprint.Min.X, observed.Max.X - footprint.Max.X}
	dys := [...]int{0, observed.Min.Y - footprint.Min.Y, observed.Max.Y - footprint.Max.Y}

	for _, dy := range dys {
		for _, dx := range dxs {
			if dx == 0 && dy == 0 {
				continue
			}
			if absInt(dx) > tolerancePixels || absInt(dy) > tolerancePixels {
				continue
			}
			moved := footprint.Add(image.Pt(dx, dy))
			overlap := intersectionArea(moved, observed)
			if overlap > bestOverlap {
				best, bestOverlap = moved, overlap
			}
		}
	}

	// Only ground the old footprint did not cover is absorbed, which is what makes a vacated strip
	// disappear from the footprint instead of being kept forever.
	for _, strip := range newGround(observed, footprint) {
		best = best.Union(strip)
	}
	return best
}

// newGround returns the parts of the observation that lie outside a rectangle, one strip per side that
// the observation extends past.
func newGround(observed, footprint rectangle) []rectangle {
	strips := make([]rectangle, 0, 4)
	if observed.Max.X > footprint.Max.X {
		strips = append(strips, rect(footprint.Max.X, observed.Min.Y, observed.Max.X, observed.Max.Y))
	}
	if observed.Min.X < footprint.Min.X {
		strips = append(strips, rect(observed.Min.X, observed.Min.Y, footprint.Min.X, observed.Max.Y))
	}
	if observed.Max.Y > footprint.Max.Y {
		strips = append(strips, rect(observed.Min.X, footprint.Max.Y, observed.Max.X, observed.Max.Y))
	}
	if observed.Min.Y < footprint.Min.Y {
		strips = append(strips, rect(observed.Min.X, observed.Min.Y, observed.Max.X, footprint.Min.Y))
	}
	return strips
}

// intersectionArea is the shared area of two rectangles, in pixels.
func intersectionArea(a, b rectangle) int {
	overlap := a.Intersect(b)
	if overlap.Empty() {
		return 0
	}
	return overlap.Dx() * overlap.Dy()
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// boundsOf converts a pixel rectangle back to normalized bounds, using the frame geometry the caller
// passed. It sits here rather than in the diff package because the identity layer converts footprints
// as well.
func boundsOf(r rectangle, frameWidth, frameHeight int) delta.Bounds {
	if frameWidth <= 0 || frameHeight <= 0 {
		return delta.Bounds{}
	}
	return delta.Bounds{
		X: float64(r.Min.X) / float64(frameWidth),
		Y: float64(r.Min.Y) / float64(frameHeight),
		W: float64(r.Dx()) / float64(frameWidth),
		H: float64(r.Dy()) / float64(frameHeight),
	}
}
