// Package identity tracks elements across the frames of one stream.
//
// The job is to answer one question per reported region: which element is this? A region that
// continues an element from the previous frame keeps that element's identifier, an element that is
// gone is retired, and an element that returns after being occluded gets a new identifier marked
// uncertain.
//
// The last rule is the point of the package. Pixels cannot prove that the thing in a rectangle now
// is the thing that was in it a moment ago, particularly after it was hidden, so the engine refuses
// the match and says so rather than handing a consumer an identifier that silently means something
// else. A consumer that needs a stable handle treats an uncertain identity as new; one that can
// tolerate a wrong match can ignore the flag, and either way the engine has not made the decision
// for it.
//
// Nothing here is learned and nothing is probabilistic beyond the match score. The cost function is
// intersection over union with a gate on how far an element may move and how much its size may
// change, both stated in the configuration.
package identity

import (
	"sort"

	"github.com/theoabw/screendelta/internal/delta"
)

// State is the lifecycle of an identity.
type State string

const (
	// Live means the element was seen within the occlusion window.
	Live State = "live"
	// Retired means the element has been absent longer than the window allows. A retired
	// identity never returns to live and its identifier is never reissued.
	Retired State = "retired"
)

// Element is one tracked identity.
type Element struct {
	ID         uint64
	Bounds     delta.Bounds
	FirstFrame uint64
	LastFrame  uint64
	Confidence float64
	Uncertain  bool
	State      State
	Missed     int

	// pending is the element's footprint in the frame being resolved, accumulated from every
	// region that named it, and pendingSet says whether it was seen at all. One element can be
	// reported as more than one region, so its footprint is the union of them.
	pending    delta.Bounds
	pendingSet bool
}

// Assignment is the identity decision for one reported region.
type Assignment struct {
	ID uint64
	// Confidence is the strength of the evidence for the assignment, from 0 to 1.
	Confidence float64
	// Uncertain is true when the assignment is a reacquisition after retirement rather than a
	// match the engine is willing to claim.
	Uncertain bool
	// PreviousBounds is where the element was before this frame, set when the assignment carried
	// an existing identity over. It is the element's footprint rather than the area that changed,
	// which is what a consumer wants to know when something moved.
	PreviousBounds delta.Bounds
	// Carried is true when PreviousBounds is meaningful.
	Carried bool
}

// Map is the identity table of one stream.
type Map struct {
	next uint64

	elements []Element

	// occlusionFrames is how many consecutive frames an element may be unmatched before it is
	// retired. A larger window tolerates longer occlusion at the cost of keeping stale entries.
	occlusionFrames int
	// motionTolerancePixels gates a match by centroid distance.
	motionTolerancePixels int
}

// New creates a map for one stream.
func New(occlusionFrames, motionTolerancePixels int) *Map {
	if occlusionFrames < 0 {
		occlusionFrames = 0
	}
	if motionTolerancePixels < 0 {
		motionTolerancePixels = 0
	}
	return &Map{occlusionFrames: occlusionFrames, motionTolerancePixels: motionTolerancePixels}
}

// SetRules changes how far an element may move and how long it may be hidden.
//
// The tracked elements are kept, because a rule change says nothing about where anything is: a
// caller that relaxes the motion tolerance is asking the engine to match a move it previously would
// not, not asking it to forget the screen. Rebuilding the table instead would restart the identifier
// sequence mid-session, which is the one thing FR-006 forbids.
func (m *Map) SetRules(occlusionFrames, motionTolerancePixels int) {
	if occlusionFrames < 0 {
		occlusionFrames = 0
	}
	if motionTolerancePixels < 0 {
		motionTolerancePixels = 0
	}
	m.occlusionFrames = occlusionFrames
	m.motionTolerancePixels = motionTolerancePixels
}

// retiredRetentionFrames is how long a retired element is kept for reacquisition.
//
// Retired entries exist for one reason: to recognise that an element which has returned is the one
// that left. That is only meaningful for something that left recently, so a retired entry is dropped
// after this many frames. Dropping it can only turn a reacquisition into an addition, which is the
// safe direction: an addition claims nothing. Keeping every retired entry forever would make the
// table grow with the length of the session, which is the one thing NFR-003 forbids.
const retiredRetentionFrames = 300

// RetireAll ends every live element without touching the identifier counter.
//
// This is what a viewport change means. Bounds are normalised, so a rectangle tracked at one frame
// size says nothing at another, and the engine will not carry an identity across the change. The
// elements are retired rather than forgotten, and the counter keeps rising, so an identifier is
// still never reissued: a consumer that held one learns it is gone rather than seeing the number
// reappear on something else. The viewport-changed condition in the document is what explains the
// absence.
func (m *Map) RetireAll() {
	for index := range m.elements {
		element := &m.elements[index]
		element.pendingSet = false
		if element.State == Live {
			element.State = Retired
			element.Missed = 0
		}
	}
}

// Elements returns the tracked identities, in allocation order, for inspection and tests.
func (m *Map) Elements() []Element {
	return m.elements
}

// Carry returns the identity for a region the engine believes continues an element it is already
// tracking: a region reported as changed or moved.
//
// The element is matched by overlap, gated by how far it may have moved and how much its area may
// have changed. A region that reaches here and matches nothing still gets an identity, because every
// region in a document must carry one; it is a new identity with certain confidence, since the
// engine has no competing interpretation for it.
func (m *Map) Carry(lookup, current delta.Bounds, frameWidth, frameHeight int, sequence uint64) Assignment {
	search := pixelRect(lookup, frameWidth, frameHeight)

	if index, score := m.bestLive(search, frameWidth, frameHeight); index >= 0 {
		element := &m.elements[index]
		where := element.Bounds
		element.observe(current)
		element.LastFrame = sequence
		element.Confidence = score
		element.Missed = 0
		return Assignment{ID: element.ID, Confidence: score, PreviousBounds: where, Carried: true}
	}

	return m.acquire(current, frameWidth, frameHeight, sequence)
}

// observe records the footprint a region gives the element in this frame, keeping the union when
// more than one region names the same element.
func (e *Element) observe(bounds delta.Bounds) {
	if !e.pendingSet {
		e.pending = bounds
		e.pendingSet = true
		return
	}
	e.pending = unionBounds(e.pending, bounds)
}

// Appear returns the identity for a region the engine believes is new: a region reported as added.
//
// A new region where a retired element used to be is the case this package exists for. The engine
// has evidence that something occupied that place before, but not enough to claim it is the same
// element, so the identity is newly allocated and marked uncertain with the observed overlap as its
// confidence. A consumer that needs a stable handle treats it as new, and can see why.
func (m *Map) Appear(bounds delta.Bounds, frameWidth, frameHeight int, sequence uint64) Assignment {
	return m.acquire(bounds, frameWidth, frameHeight, sequence)
}

// acquire allocates a new identity, marked uncertain when a retired element explains the place.
func (m *Map) acquire(bounds delta.Bounds, frameWidth, frameHeight int, sequence uint64) Assignment {
	rect := pixelRect(bounds, frameWidth, frameHeight)
	if _, score := m.bestRetired(rect, frameWidth, frameHeight); score > 0 {
		assignment := m.allocate(bounds, sequence, score)
		assignment.Uncertain = true
		return assignment
	}
	return m.allocate(bounds, sequence, 1)
}

// Vanish retires the identity of a region reported as removed and returns it, so the document can
// name the element that is gone.
//
// An identity the map does not hold cannot be named, so the caller gets a fresh one rather than a
// borrowed identifier that belongs to something else.
func (m *Map) Vanish(bounds delta.Bounds, frameWidth, frameHeight int, sequence uint64) Assignment {
	rect := pixelRect(bounds, frameWidth, frameHeight)

	best, bestScore := -1, 0.0
	for index := range m.elements {
		element := &m.elements[index]
		if element.State != Live {
			continue
		}
		candidate := pixelRect(element.Bounds, frameWidth, frameHeight)
		score := intersectionOverUnion(candidate, rect)
		if score > bestScore {
			best, bestScore = index, score
		}
	}
	if best < 0 || bestScore < minimumOverlap {
		// Nothing live was there to retire, so the document still needs a name for the removed
		// element. It gets a fresh one, and that identity is retired immediately: a removal that
		// named a live identity would tell a consumer that an element it has never seen is gone,
		// while the engine kept matching it.
		assignment := m.allocate(bounds, sequence, 1)
		for index := range m.elements {
			if m.elements[index].ID == assignment.ID {
				m.elements[index].State = Retired
				m.elements[index].Confidence = 0
				break
			}
		}
		assignment.Confidence = 0
		return assignment
	}

	element := &m.elements[best]
	element.pendingSet = false
	element.State = Retired
	element.LastFrame = sequence
	element.Bounds = bounds
	element.Confidence = 0
	return Assignment{ID: element.ID, Confidence: 0}
}

// allocate creates a new identity. The counter only ever increases, which is what makes reuse
// impossible even after retirement.
func (m *Map) allocate(bounds delta.Bounds, sequence uint64, confidence float64) Assignment {
	m.next++
	m.elements = append(m.elements, Element{
		ID:         m.next,
		Bounds:     bounds,
		FirstFrame: sequence,
		LastFrame:  sequence,
		Confidence: confidence,
		State:      Live,
	})
	return Assignment{ID: m.next, Confidence: confidence}
}

// Retire marks the identity of a region the engine reports as removed, and returns it. The caller
// needs the identifier so the document can name the element that is gone.
func (m *Map) Retire(id uint64, sequence uint64, bounds delta.Bounds) Assignment {
	for index := range m.elements {
		element := &m.elements[index]
		if element.ID != id {
			continue
		}
		element.State = Retired
		element.LastFrame = sequence
		element.Bounds = bounds
		element.Confidence = 0
		return Assignment{ID: id, Confidence: 0}
	}
	// An identity the map does not know cannot be named, so the caller gets a fresh one rather than
	// a borrowed identifier that means something else.
	return m.allocate(bounds, sequence, 1)
}

// EndFrame ages the elements that were not matched in the given frame, and drops retired entries
// that are too old to be worth keeping.
//
// It must be called once per frame after every region has been assigned, and it takes the frame
// number rather than remembering the last one, because a frame in which nothing was matched makes no
// other call at all: without the number, an element hidden by an overlay would never age.
//
// changed are the areas reported this frame. An element whose footprint none of them touches is
// still on the screen: nothing there changed, so the element cannot have gone anywhere. Without that
// rule an element that simply sat still for longer than the occlusion window was retired as though
// it had been covered, and the next change to it was reported as a reacquisition of something new.
func (m *Map) EndFrame(sequence uint64, changed []delta.Bounds, frameWidth, frameHeight int) {
	for index := range m.elements {
		element := &m.elements[index]
		if element.pendingSet {
			element.Bounds = element.pending
			element.pendingSet = false
		}
		if element.State != Live || element.LastFrame >= sequence {
			continue
		}
		if !touchesAny(element.Bounds, changed, frameWidth, frameHeight) {
			// Nothing changed where it is, so it is still there.
			element.LastFrame = sequence
			element.Missed = 0
			continue
		}
		element.Missed++
		if element.Missed > m.occlusionFrames {
			element.State = Retired
		}
	}

	m.dropStaleRetired(sequence)
}

// dropStaleRetired removes retired entries that have been gone longer than they are useful for.
func (m *Map) dropStaleRetired(sequence uint64) {
	kept := m.elements[:0]
	for _, element := range m.elements {
		if element.State == Retired && sequence > element.LastFrame+retiredRetentionFrames {
			continue
		}
		kept = append(kept, element)
	}
	m.elements = kept
}

// touchesAny reports whether a footprint overlaps any of the changed areas.
func touchesAny(bounds delta.Bounds, changed []delta.Bounds, frameWidth, frameHeight int) bool {
	if len(changed) == 0 {
		return false
	}
	rect := pixelRect(bounds, frameWidth, frameHeight)
	for _, area := range changed {
		if !rect.Intersect(pixelRect(area, frameWidth, frameHeight)).Empty() {
			return true
		}
	}
	return false
}

// bestLive returns the live element that best explains a rectangle, and the score of that match.
//
// Two ways of explaining it are accepted, because a reported area and a tracked element are not the
// same thing:
//
//   - the element overlaps the area by at least the minimum, which is the ordinary case of an
//     element that changed or moved;
//   - the area lies mostly inside the element, which is what a change within a large element looks
//     like, and what each half of a translation looks like once the element itself is tracked.
//
// Size similarity is required in the first case and not in the second: an area that is part of an
// element is naturally smaller than it, and requiring them to be alike would break exactly the case
// the second rule exists for. The motion tolerance still applies, so a stale element on the other
// side of the frame cannot claim a small change.
func (m *Map) bestLive(rect rectangle, frameWidth, frameHeight int) (int, float64) {
	best, bestScore, bestRank, bestArea := -1, 0.0, 0, 0

	for index := range m.elements {
		element := &m.elements[index]
		if element.State != Live {
			continue
		}
		candidate := pixelRect(element.Bounds, frameWidth, frameHeight)
		area := candidate.Dx() * candidate.Dy()

		// Two kinds of explanation, ranked so that the better one always wins. An overlap match is
		// the ordinary case of the same element changing or moving; a containment match is a change
		// inside a larger element. An overlap match is preferred even when its score is lower,
		// because a large element that happens to contain a small one must not be able to take the
		// small one's identity: before this rule, containment scored a perfect one and tied with the
		// exact match, so the older of the two won and the answer depended on allocation order.
		rank, score := 0, 0.0
		if withinTolerance(candidate, rect, m.motionTolerancePixels) {
			overlap := intersectionOverUnion(candidate, rect)
			if overlap >= minimumOverlap && similarSize(candidate, rect) {
				rank, score = 2, overlap
			}
		}
		if rank == 0 {
			// The area is part of the element rather than most of it. It has to be inside the element
			// and no larger than it, which is what makes this a part rather than a bigger thing that
			// appeared there. No size ratio applies: a part is naturally much smaller than the whole,
			// and requiring them to be alike is what made the two paths disagree at the overlap
			// threshold, where a contained rectangle of three tenths the area lost its identity while
			// one of 0.29 kept it. Distance is measured by containment rather than from centre to
			// centre, because an area inside a large element is far from that element's centre.
			if rect.Dx()*rect.Dy() <= area && withinReach(candidate, rect, m.motionTolerancePixels) {
				contained := coveredFraction(candidate, rect)
				if contained >= minimumOverlap {
					rank, score = 1, contained
				}
			}
		}
		if rank == 0 {
			continue
		}

		better := rank > bestRank ||
			(rank == bestRank && score > bestScore) ||
			(rank == bestRank && score == bestScore && best >= 0 && area < bestArea) ||
			(rank == bestRank && score == bestScore && best >= 0 && area == bestArea && element.ID < m.elements[best].ID)
		if better {
			best, bestScore, bestRank, bestArea = index, score, rank, area
		}
	}
	return best, bestScore
}

// bestRetired returns the retired element whose last position explains a rectangle.
func (m *Map) bestRetired(rect rectangle, frameWidth, frameHeight int) (int, float64) {
	best, bestScore := -1, 0.0
	for index := range m.elements {
		element := &m.elements[index]
		if element.State != Retired {
			continue
		}
		candidate := pixelRect(element.Bounds, frameWidth, frameHeight)
		if !withinTolerance(candidate, rect, m.motionTolerancePixels) {
			continue
		}
		score := intersectionOverUnion(candidate, rect)
		if score < minimumOverlap {
			continue
		}
		if score > bestScore || (score == bestScore && best >= 0 && element.ID < m.elements[best].ID) {
			best, bestScore = index, score
		}
	}
	return best, bestScore
}

// Sorted returns the live elements by identifier, for tests and for a future session summary.
func (m *Map) Sorted() []Element {
	copied := append([]Element(nil), m.elements...)
	sort.Slice(copied, func(i, j int) bool { return copied[i].ID < copied[j].ID })
	return copied
}
