package diff

import (
	"sort"

	"github.com/theoabw/screendelta/internal/config"
	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/frame"
)

// candidate is a changed area in pixels, after filtering and clipping.
type candidate struct {
	left, top, right, bottom int
	magnitude                float64
}

func (c candidate) width() int  { return c.right - c.left }
func (c candidate) height() int { return c.bottom - c.top }
func (c candidate) area() int   { return c.width() * c.height() }

// classify turns changed areas into document regions.
//
// The four classes are decided by comparing each area with where elements were in the
// previous frame, because a pixel difference alone cannot tell a changed element from a
// replaced one:
//
//   - overlap of at least half the union means the same element with different content,
//     so "changed";
//   - a previous element whose footprint has simply shifted, within the configured
//     motion tolerance, means "moved", and the reported bounds are the union of where it
//     was and where it is now, because the pixels changed in both places;
//   - an area that covers a previous element but does not match its size means the
//     element is gone and something else occupies its place, so the caller is told both
//     "removed" for what is gone and "added" for what arrived;
//   - anything else is "added".
//
// A "removed" region is therefore only reported when the engine can point at the element
// that is gone, which is what keeps the class from being invented for a changed element.
func (d *Differ) classify(candidates []candidate, previous, current frame.Frame, cfg config.Config) []delta.Region {
	// The first comparison of a stream has no baseline: the differ knows that pixels
	// changed but not whether the element was there before. Reporting "changed" is the
	// honest answer, because "added" would assert an absence the engine cannot see.
	if !d.hasPrevious {
		regions := make([]delta.Region, 0, len(candidates))
		for _, c := range candidates {
			regions = append(regions, d.regionFor(delta.ClassChanged, c, nil, current))
		}
		d.remember(regions, current)
		sortRegions(regions)
		return regions
	}

	usedPrevious := make([]bool, len(d.previous))
	regions := make([]delta.Region, 0, len(candidates)+len(d.previous))

	// Pass one: unchanged footprints that changed content.
	matchedCandidate := make([]bool, len(candidates))
	for index, c := range candidates {
		best, bestOverlap := -1, 0.0
		for previousIndex, state := range d.previous {
			if usedPrevious[previousIndex] {
				continue
			}
			overlap := intersectionOverUnion(rect(c.left, c.top, c.right, c.bottom), pixelRectOf(state.bounds, current))
			if overlap > bestOverlap {
				bestOverlap = overlap
				best = previousIndex
			}
		}
		if best >= 0 && bestOverlap >= 0.5 {
			usedPrevious[best] = true
			matchedCandidate[index] = true
			regions = append(regions, d.regionFor(delta.ClassChanged, c, nil, current))
		}
	}

	// Pass two: a previous element whose footprint moved. A translation changes two areas,
	// the part of the old position the element no longer covers and the part of the new
	// position it did not cover before, so both are reported separately and both are
	// classified as moved with the element's earlier position as their origin. Reporting one
	// box around the whole movement instead would describe pixels that did not change and
	// miss the two areas that did.
	for previousIndex, state := range d.previous {
		if usedPrevious[previousIndex] {
			continue
		}
		previousRect := pixelRectOf(state.bounds, current)
		expanded := expand(previousRect, cfg.MotionTolerancePixels)

		absorbed := make([]int, 0, 2)
		for index, c := range candidates {
			if matchedCandidate[index] {
				continue
			}
			if !contains(expanded, rect(c.left, c.top, c.right, c.bottom)) {
				continue
			}
			absorbed = append(absorbed, index)
		}
		if len(absorbed) == 0 {
			continue
		}

		usedPrevious[previousIndex] = true
		previousBounds := boundsOf(previousRect, current)
		for _, index := range absorbed {
			matchedCandidate[index] = true
			regions = append(regions, d.regionFor(delta.ClassMoved, candidates[index], &previousBounds, current))
		}
	}

	// Pass three: what is left is new, or replaces something of a different size.
	for index, c := range candidates {
		if matchedCandidate[index] {
			continue
		}
		replaced := -1
		for previousIndex, state := range d.previous {
			if usedPrevious[previousIndex] {
				continue
			}
			previousRect := pixelRectOf(state.bounds, current)
			// How much of the previous element the new area covers, not the other way
			// round: the question is whether the element that was there is gone.
			if coveredFraction(rect(c.left, c.top, c.right, c.bottom), previousRect) >= 0.5 {
				replaced = previousIndex
				break
			}
		}
		if replaced >= 0 {
			state := d.previous[replaced]
			usedPrevious[replaced] = true
			previousBounds := boundsOf(pixelRectOf(state.bounds, current), current)
			removedCandidate := candidate{
				left:      pixelRectOf(state.bounds, current).Min.X,
				top:       pixelRectOf(state.bounds, current).Min.Y,
				right:     pixelRectOf(state.bounds, current).Max.X,
				bottom:    pixelRectOf(state.bounds, current).Max.Y,
				magnitude: regionMagnitude(previous, current, pixelRectOf(state.bounds, current)),
			}
			regions = append(regions, d.regionFor(delta.ClassRemoved, removedCandidate, &previousBounds, current))
		}
		regions = append(regions, d.regionFor(delta.ClassAdded, c, nil, current))
	}

	// There is deliberately no pass that reports every unmatched previous element as
	// removed. An element whose pixels did not change is still on the screen; it is simply
	// absent from the delta, and calling it removed would invent a disappearance out of a
	// frame pair that only differs by noise.
	d.remember(regions, current)
	sortRegions(regions)
	return regions
}

// regionFor builds a document region, allocating an identity that this stream has never
// used before. Identity stability across frames is the next user story; what holds here
// is the guarantee that an identity is never reused.
func (d *Differ) regionFor(class delta.RegionClass, c candidate, previousBounds *delta.Bounds, current frame.Frame) delta.Region {
	d.nextIdentity++
	bounds := boundsOf(rect(c.left, c.top, c.right, c.bottom), current)
	if previousBounds == nil && class == delta.ClassRemoved {
		copied := bounds
		previousBounds = &copied
	}
	return delta.Region{
		Identity:       d.nextIdentity,
		Class:          class,
		Bounds:         bounds,
		PreviousBounds: previousBounds,
		Magnitude:      clampMagnitude(c.magnitude),
		AreaPixels:     c.area(),
	}
}

// remember stores the geometry of the screen as it now stands, so the next comparison
// can tell what changed, moved and disappeared. Removed regions are not part of it: they
// are absent from the screen.
func (d *Differ) remember(regions []delta.Region, current frame.Frame) {
	d.previous = d.previous[:0]
	for _, region := range regions {
		if region.Class == delta.ClassRemoved {
			continue
		}
		pixel := pixelRectOf(region.Bounds, current)
		d.previous = append(d.previous, regionState{
			bounds: region.Bounds,
			width:  pixel.Max.X - pixel.Min.X,
			height: pixel.Max.Y - pixel.Min.Y,
		})
	}
	d.hasPrevious = true
	d.previousWidth, d.previousHeight = current.Width, current.Height
}

// filter grows, discards and clips the changed areas according to the configuration.
func (d *Differ) filter(components []component, current frame.Frame, cfg config.Config) []candidate {
	candidates := make([]candidate, 0, len(components))
	for _, group := range components {
		left := max(group.left-growthMargin, 0)
		top := max(group.top-growthMargin, 0)
		right := min(group.right+growthMargin, current.Width)
		bottom := min(group.bottom+growthMargin, current.Height)

		if (right-left)*(bottom-top) < cfg.MinRegionAreaPixels {
			continue
		}
		if ignored(cfg, left, top, right, bottom, current) {
			continue
		}

		c := candidate{left: left, top: top, right: right, bottom: bottom, magnitude: group.magnitude}

		if cfg.Unrestricted() {
			candidates = append(candidates, c)
			continue
		}
		// A configured list of regions of interest restricts what is reported, and each
		// reported area is clipped to the region that admitted it.
		for _, roi := range *cfg.RegionsOfInterest {
			roiRect := pixelRectOf(roi.Bounds, current)
			clipped, ok := intersect(rect(c.left, c.top, c.right, c.bottom), roiRect)
			if !ok {
				continue
			}
			candidates = append(candidates, candidate{
				left:      clipped.Min.X,
				top:       clipped.Min.Y,
				right:     clipped.Max.X,
				bottom:    clipped.Max.Y,
				magnitude: c.magnitude,
			})
			break
		}
	}
	return candidates
}

// ignored reports whether an area's centre falls inside a configured ignored area, which
// is how a clock or a cursor is kept out of the output without hiding real changes
// elsewhere.
func ignored(cfg config.Config, left, top, right, bottom int, current frame.Frame) bool {
	if len(cfg.IgnoredAreas) == 0 {
		return false
	}
	centreX := (left + right) / 2
	centreY := (top + bottom) / 2
	for _, area := range cfg.IgnoredAreas {
		areaRect := pixelRectOf(area, current)
		if centreX >= areaRect.Min.X && centreX < areaRect.Max.X && centreY >= areaRect.Min.Y && centreY < areaRect.Max.Y {
			return true
		}
	}
	return false
}

func sortComponents(components []component) {
	sort.SliceStable(components, func(i, j int) bool {
		if components[i].top != components[j].top {
			return components[i].top < components[j].top
		}
		return components[i].left < components[j].left
	})
}

func sortRegions(regions []delta.Region) {
	sort.SliceStable(regions, func(i, j int) bool {
		if regions[i].Bounds.Y != regions[j].Bounds.Y {
			return regions[i].Bounds.Y < regions[j].Bounds.Y
		}
		if regions[i].Bounds.X != regions[j].Bounds.X {
			return regions[i].Bounds.X < regions[j].Bounds.X
		}
		return regions[i].Identity < regions[j].Identity
	})
}

func clampMagnitude(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
