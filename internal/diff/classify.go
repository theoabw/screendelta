package diff

import (
	"sort"

	"github.com/theoabw/screendelta/internal/config"
	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/frame"
	"github.com/theoabw/screendelta/internal/identity"
)

// candidate is a changed area in pixels, after filtering and clipping.
type candidate struct {
	left, top, right, bottom int
	magnitude                float64

	// signature is how the area looks, measured once from the frame's luma plane. The identity layer
	// needs it to tell a cover from a content change, which geometry alone cannot do.
	appearance    identity.Signature
	hasAppearance bool
}

// signature returns the area's appearance, or nil when it could not be measured, in which case the
// identity layer falls back to geometry alone.
func (c candidate) signature() *identity.Signature {
	if !c.hasAppearance {
		return nil
	}
	copied := c.appearance
	return &copied
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
	changedBoundsFirst := make([]delta.Bounds, 0, len(candidates))
	for _, c := range candidates {
		changedBoundsFirst = append(changedBoundsFirst, boundsOf(rect(c.left, c.top, c.right, c.bottom), current))
	}

	if !d.hasPrevious {
		regions := make([]delta.Region, 0, len(candidates))
		for _, c := range candidates {
			regions = append(regions, d.regionFor(delta.ClassChanged, c, nil, current))
		}
		d.identities.EndFrame(current.Sequence, changedBoundsFirst, current.Width, current.Height)
		sortRegions(regions)
		return regions
	}

	// The areas reported this frame, in normalized form, are what the identity map needs to tell an
	// element that is still on the screen from one that has been covered: nothing changed where it
	// is, so it is still there.
	changedBounds := make([]delta.Bounds, 0, len(candidates))
	for _, c := range candidates {
		changedBounds = append(changedBounds, boundsOf(rect(c.left, c.top, c.right, c.bottom), current))
	}

	// The elements the engine is tracking, read from the layer that owns them.
	d.liveElements = d.identities.LiveElements()
	usedPrevious := make([]bool, len(d.liveElements))

	// Which tracked elements had their own pixels change is a fact about the frame rather than about any one
	// changed area, so it is measured once. Measuring it per area instead cost the large-change profile
	// twenty milliseconds a frame.
	goneElements := make(map[uint64]bool, len(d.liveElements))
	for _, live := range d.liveElements {
		if d.changedFraction(live.Bounds, current) >= coverInteriorFraction {
			goneElements[live.ID] = true
		}
	}
	regions := make([]delta.Region, 0, len(candidates)+len(d.liveElements))

	matchedCandidate := make([]bool, len(candidates))

	// Pass zero: a return, or a cover. Both are cases where a tracked element is not what is there any
	// more, and both would otherwise be reported as a change of that element and inherit its identity.
	for index := range candidates {
		if matchedCandidate[index] {
			continue
		}
		c := candidates[index]

		// Which live elements this area has taken the place of: those it contains and reaches past, whose
		// own pixels changed. The pixel question is asked here because the mask is here.
		area := boundsOf(rect(c.left, c.top, c.right, c.bottom), current)

		taken := make([]identity.CoveredElement, 0, 2)
		for _, enclosed := range d.identities.Enclosed(area, current.Width, current.Height) {
			if !goneElements[enclosed.ID] {
				// The element is still there and only its edge moved: this is growth, not a cover.
				continue
			}
			taken = append(taken, enclosed)
		}

		// A return is the case where the area looks more like something the engine retired than like what
		// is on the screen, and the element it overlaps is one whose own pixels changed.
		returned := identity.ReturnDecision{}
		if len(goneElements) > 0 {
			returned = d.identities.Return(area, c.signature(), current.Width, current.Height, current.Sequence, goneElements)
		}

		if len(taken) == 0 && !returned.IsReturn {
			// Nothing has taken anything's place, so the ordinary passes decide.
			continue
		}

		// Every element the area took the place of is retired and named, whether the return identified one
		// of them or not: leaving one live would hand a consumer a handle that later identifies something
		// unrelated.
		for _, element := range taken {
			if returned.IsReturn && element.ID == returned.CoveredID {
				continue
			}
			d.identities.RetireByID(element.ID)
			if index := d.snapshotIndex(element.ID); index >= 0 {
				usedPrevious[index] = true
			}
			regions = append(regions, d.regionRemoved(element.ID, element.Bounds, previous, current))
		}
		if returned.IsReturn {
			if index := d.snapshotIndex(returned.CoveredID); index >= 0 {
				usedPrevious[index] = true
			}
			// The element the return displaced is named as removed, exactly as a cover names it: what is on
			// the screen there is no longer that element, which is the whole reason the return was called.
			regions = append(regions, d.regionRemoved(returned.CoveredID, returned.CoveredBounds, previous, current))
			regions = append(regions, d.regionWithAssignment(delta.ClassAdded, c, returned.Assignment, nil, current))
		} else {
			regions = append(regions, d.regionFor(delta.ClassAdded, c, nil, current))
		}
		matchedCandidate[index] = true
	}

	// Pass one: unchanged footprints that changed content.
	for index, c := range candidates {
		if matchedCandidate[index] {
			// A previous pass already answered for this area, which happens when a cover or a return
			// consumed it.
			continue
		}
		best, bestOverlap := -1, 0.0
		for previousIndex, state := range d.liveElements {
			if usedPrevious[previousIndex] {
				continue
			}
			overlap := intersectionOverUnion(rect(c.left, c.top, c.right, c.bottom), pixelRectOf(state.Bounds, current))
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
	for previousIndex, state := range d.liveElements {
		if usedPrevious[previousIndex] {
			continue
		}
		previousRect := pixelRectOf(state.Bounds, current)
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
		for previousIndex, state := range d.liveElements {
			if usedPrevious[previousIndex] {
				continue
			}
			previousRect := pixelRectOf(state.Bounds, current)
			// How much of the previous element the new area covers, not the other way
			// round: the question is whether the element that was there is gone.
			if coveredFraction(rect(c.left, c.top, c.right, c.bottom), previousRect) < 0.5 {
				continue
			}
			// And the element's own pixels have to have changed. A bounding rectangle around a change
			// is not evidence that everything inside it changed, and without this an element that grew
			// outward was retired as though something had taken its place.
			if false {
				continue
			}
			replaced = previousIndex
			break
		}
		if replaced >= 0 {
			state := d.liveElements[replaced]
			usedPrevious[replaced] = true
			previousBounds := boundsOf(pixelRectOf(state.Bounds, current), current)
			removedCandidate := candidate{
				left:      pixelRectOf(state.Bounds, current).Min.X,
				top:       pixelRectOf(state.Bounds, current).Min.Y,
				right:     pixelRectOf(state.Bounds, current).Max.X,
				bottom:    pixelRectOf(state.Bounds, current).Max.Y,
				magnitude: regionMagnitude(previous, current, pixelRectOf(state.Bounds, current)),
			}
			regions = append(regions, d.regionFor(delta.ClassRemoved, removedCandidate, &previousBounds, current))
		}
		regions = append(regions, d.regionFor(delta.ClassAdded, c, nil, current))
	}

	// There is deliberately no pass that reports every unmatched previous element as
	// removed. An element whose pixels did not change is still on the screen; it is simply
	// absent from the delta, and calling it removed would invent a disappearance out of a
	// frame pair that only differs by noise.
	// One EndFrame per comparison, after every region has been assigned: the occlusion window counts
	// frames, and an element hidden by an overlay is unmatched in the frames that cover it.
	d.identities.EndFrame(current.Sequence, changedBounds, current.Width, current.Height)

	// Elements whose pixels did not change are still on the screen, so they carry over.
	unchanged := make([]identity.LiveElement, 0, len(d.liveElements))
	for index, state := range d.liveElements {
		if !usedPrevious[index] {
			unchanged = append(unchanged, state)
		}
	}
	sortRegions(regions)
	return regions
}

// changedFraction is how much of a footprint's pixels changed this frame. The mask marks every pixel
// that differed by more than the noise floor, so it answers the question a bounding rectangle cannot:
// whether the element itself changed or only the ground around it.
func (d *Differ) changedFraction(bounds delta.Bounds, current frame.Frame) float64 {
	// Without a mask there is no pixel question to ask, so the geometry decides and the answer is one.
	// That happens when a caller drives the classifier directly rather than through a comparison.
	if len(d.mask) < current.Width*current.Height {
		return 1
	}
	r := pixelRectOf(bounds, current)
	// The footprint an element is remembered with includes the growth margin every reported region gets, and
	// that ring belongs to the background rather than to the element, so it never changes and would make an
	// element that changed entirely look as though a tenth of it had stayed still. Measuring the element is
	// measuring inside that margin.
	inner := r.Inset(growthMargin)
	if !inner.Empty() {
		r = inner
	}
	if r.Empty() {
		return 0
	}
	left := max(r.Min.X, 0)
	top := max(r.Min.Y, 0)
	right := min(r.Max.X, current.Width)
	bottom := min(r.Max.Y, current.Height)
	if right <= left || bottom <= top {
		return 0
	}

	changed, total := 0, 0
	for y := top; y < bottom; y++ {
		row := y * current.Width
		for x := left; x < right; x++ {
			total++
			if d.mask[row+x] != 0 {
				changed++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(changed) / float64(total)
}

// snapshotIndex finds an element in the snapshot the passes iterate, so the caller can mark it used and
// keep the later passes from matching geometry that is already gone.
func (d *Differ) snapshotIndex(id uint64) int {
	for index, element := range d.liveElements {
		if element.ID == id {
			return index
		}
	}
	return -1
}

// regionRemoved builds a region for an element the engine has retired, naming the identity it held.
func (d *Differ) regionRemoved(id uint64, bounds delta.Bounds, previous, current frame.Frame) delta.Region {
	where := bounds
	// The magnitude describes how much changed where the element was, because a removal is still a
	// change on the screen and a zero there would tell a consumer nothing happened.
	magnitude := regionMagnitude(previous, current, pixelRectOf(bounds, current))
	return delta.Region{
		Identity:           id,
		Class:              delta.ClassRemoved,
		Bounds:             where,
		PreviousBounds:     &where,
		Magnitude:          clampMagnitude(magnitude),
		AreaPixels:         areaOf(bounds, current),
		IdentityConfidence: 0,
	}
}

// regionWithAssignment builds a region around an identity the map already decided, which is how a
// reacquisition keeps the uncertainty the map recorded for it.
func (d *Differ) regionWithAssignment(class delta.RegionClass, c candidate, assignment identity.Assignment, previous *delta.Bounds, current frame.Frame) delta.Region {
	bounds := boundsOf(rect(c.left, c.top, c.right, c.bottom), current)
	return delta.Region{
		Identity:           assignment.ID,
		Class:              class,
		Bounds:             bounds,
		PreviousBounds:     previous,
		Magnitude:          clampMagnitude(c.magnitude),
		AreaPixels:         c.area(),
		IdentityConfidence: clampMagnitude(assignment.Confidence),
		IdentityUncertain:  assignment.Uncertain,
	}
}

// regionFor builds a document region, asking the identity map which element it is.
//
// The identity decision follows the class, because the classifier has already decided what
// happened to the geometry: a region that continues an element carries its identity over, a region
// that appeared gets a new one, and a region that is gone retires the identity it names. An element
// that returns after being occluded is the interesting case, and the map marks it uncertain rather
// than claiming a match it cannot support.
func (d *Differ) regionFor(class delta.RegionClass, c candidate, previousBounds *delta.Bounds, current frame.Frame) delta.Region {
	bounds := boundsOf(rect(c.left, c.top, c.right, c.bottom), current)
	if previousBounds == nil && class == delta.ClassRemoved {
		copied := bounds
		previousBounds = &copied
	}

	var assignment identity.Assignment
	switch {
	case class == delta.ClassRemoved:
		assignment = d.identities.Vanish(bounds, current.Width, current.Height, current.Sequence)
	case class == delta.ClassAdded:
		assignment = d.identities.Appear(bounds, current.Width, current.Height, current.Sequence, c.signature())
	case previousBounds != nil:
		// A moved region is one of the areas a translation changed, so the element is looked up
		// where it was rather than where this area is. Looking it up by the area would give each
		// half of a movement its own identity, which is what happened before this was fixed.
		assignment = d.identities.Carry(*previousBounds, bounds, current.Width, current.Height, current.Sequence, c.signature())
		if assignment.Carried {
			// The element's own footprint is a better answer than the area that changed, and the
			// identity map is the only layer that knows it.
			carried := assignment.PreviousBounds
			previousBounds = &carried
		}
	default:
		assignment = d.identities.Carry(bounds, bounds, current.Width, current.Height, current.Sequence, c.signature())
	}

	return delta.Region{
		Identity:           assignment.ID,
		Class:              class,
		Bounds:             bounds,
		PreviousBounds:     previousBounds,
		Magnitude:          clampMagnitude(c.magnitude),
		AreaPixels:         c.area(),
		IdentityConfidence: assignment.Confidence,
		IdentityUncertain:  assignment.Uncertain,
	}
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
		if plane := d.plane(current, cfg.Fingerprint.GridSize); plane != nil {
			if appearance, ok := signatureOf(plane, rect(left, top, right, bottom), current.Width, current.Height); ok {
				c.appearance, c.hasAppearance = appearance, true
			}
		}

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
				left:          clipped.Min.X,
				top:           clipped.Min.Y,
				right:         clipped.Max.X,
				bottom:        clipped.Max.Y,
				magnitude:     c.magnitude,
				appearance:    c.appearance,
				hasAppearance: c.hasAppearance,
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
