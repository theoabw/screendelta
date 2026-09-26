package diff

import (
	"image"
	"math"

	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/frame"
)

// Geometry in pixels uses image.Rectangle because it is the standard representation of a
// half-open rectangle and its Intersect and Union methods already match the semantics
// wanted here. Only the conversion to and from the document's normalized bounds is local.

func rect(left, top, right, bottom int) image.Rectangle {
	return image.Rect(left, top, right, bottom)
}

// pixelRectOf converts normalized document bounds to pixels for a given frame.
func pixelRectOf(bounds delta.Bounds, current frame.Frame) image.Rectangle {
	left := int(bounds.X * float64(current.Width))
	top := int(bounds.Y * float64(current.Height))
	right := left + int(bounds.W*float64(current.Width)+0.5)
	bottom := top + int(bounds.H*float64(current.Height)+0.5)
	return rect(left, top, right, bottom)
}

// boundsOf converts a pixel rectangle back to normalized bounds.
//
// The rectangle stays in whole pixels, so the area the document reports and the area its
// bounds imply agree exactly, which the document validation requires.
func boundsOf(r image.Rectangle, current frame.Frame) delta.Bounds {
	return delta.Bounds{
		X: float64(r.Min.X) / float64(current.Width),
		Y: float64(r.Min.Y) / float64(current.Height),
		W: float64(r.Dx()) / float64(current.Width),
		H: float64(r.Dy()) / float64(current.Height),
	}
}

// expand grows a rectangle on every side by the given number of pixels.
func expand(r image.Rectangle, by int) image.Rectangle {
	return rect(r.Min.X-by, r.Min.Y-by, r.Max.X+by, r.Max.Y+by)
}

// contains reports whether inner lies entirely inside outer.
func contains(outer, inner image.Rectangle) bool {
	return inner.Min.X >= outer.Min.X && inner.Min.Y >= outer.Min.Y &&
		inner.Max.X <= outer.Max.X && inner.Max.Y <= outer.Max.Y
}

func unionRect(a, b image.Rectangle) image.Rectangle {
	return a.Union(b)
}

// intersect returns the overlap of two rectangles, and whether there is one.
func intersect(a, b image.Rectangle) (image.Rectangle, bool) {
	overlap := a.Intersect(b)
	if overlap.Empty() {
		return image.Rectangle{}, false
	}
	return overlap, true
}

// intersectionOverUnion is the standard overlap measure: the shared area divided by the
// area either rectangle covers.
func intersectionOverUnion(a, b image.Rectangle) float64 {
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

// coveredFraction reports how much of inner is covered by outer, which is how a
// replacement is recognised: the old element's footprint is mostly inside the new area.
func coveredFraction(outer, inner image.Rectangle) float64 {
	if inner.Empty() {
		return 0
	}
	overlap := outer.Intersect(inner)
	if overlap.Empty() {
		return 0
	}
	return float64(overlap.Dx()*overlap.Dy()) / float64(inner.Dx()*inner.Dy())
}

// regionMagnitude is the mean absolute luma difference over a rectangle between two
// frames, on the same 0 to 1 scale the tile magnitudes use. It is what gives a removed
// region a magnitude rather than a placeholder.
func regionMagnitude(previous, current frame.Frame, r image.Rectangle) float64 {
	left := max(r.Min.X, 0)
	top := max(r.Min.Y, 0)
	right := min(r.Max.X, current.Width)
	bottom := min(r.Max.Y, current.Height)
	if right <= left || bottom <= top {
		return 0
	}

	sum, count := 0, 0
	for y := top; y < bottom; y++ {
		rowOffset := y * current.Width * 4
		for x := left; x < right; x++ {
			offset := rowOffset + x*4
			sum += absInt(luma(current.Pixels[offset:]) - luma(previous.Pixels[offset:]))
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return float64(sum) / float64(count) / 255
}

// refine narrows a tile bounding box to the pixels that actually differ, and returns the
// mean absolute luma difference over that narrower box.
//
// A tile grid is cheap to scan and cheap to merge, but a tile is coarse: a 30 pixel tall
// element straddles two tile rows, so its tile box can be almost twice its height. The
// refinement pass costs one scan over the tiles that changed, which is a small fraction of
// the frame, and it is what lets the reported bounds describe the element rather than the
// grid.
//
// If no single pixel passes the threshold, the tile box is returned: the tiles changed
// because their mean did, and discarding that would lose a real change.
func refine(previous, current frame.Frame, group component, threshold float64) (image.Rectangle, float64) {
	left, top := group.left, group.top
	right, bottom := group.right, group.bottom

	// Constructed rather than built with image.Rect, which normalises its corners: an
	// inverted rectangle meant as "nothing seen yet" would come back as the whole frame.
	refined := image.Rectangle{Min: image.Point{X: math.MaxInt32, Y: math.MaxInt32}}
	sum, count := 0, 0

	for y := top; y < min(bottom, current.Height); y++ {
		rowOffset := y * current.Width * 4
		for x := left; x < min(right, current.Width); x++ {
			offset := rowOffset + x*4
			difference := absInt(luma(current.Pixels[offset:]) - luma(previous.Pixels[offset:]))
			if float64(difference) <= threshold {
				continue
			}
			if x < refined.Min.X {
				refined.Min.X = x
			}
			if y < refined.Min.Y {
				refined.Min.Y = y
			}
			if x+1 > refined.Max.X {
				refined.Max.X = x + 1
			}
			if y+1 > refined.Max.Y {
				refined.Max.Y = y + 1
			}
			sum += difference
			count++
		}
	}

	if count == 0 {
		return rect(left, top, right, bottom), group.magnitude
	}
	return refined, float64(sum) / float64(count) / 255
}
