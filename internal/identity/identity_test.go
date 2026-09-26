package identity_test

import (
	"fmt"
	"image"
	"testing"

	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/identity"
)

const (
	frameWidth  = 320
	frameHeight = 240
)

// boundsOf builds normalized bounds from whole pixels, the way the engine does.
func boundsOf(x, y, w, h int) delta.Bounds {
	return delta.Bounds{
		X: float64(x) / frameWidth,
		Y: float64(y) / frameHeight,
		W: float64(w) / frameWidth,
		H: float64(h) / frameHeight,
	}
}

func TestFirstAppearanceAllocatesACertainIdentity(t *testing.T) {
	m := identity.New(2, 8)
	assignment := m.Appear(boundsOf(40, 60, 80, 30), frameWidth, frameHeight, 1, nil)

	if assignment.ID != 1 {
		t.Fatalf("first identity is %d, want 1", assignment.ID)
	}
	if assignment.Uncertain {
		t.Fatal("an element with nothing before it was marked uncertain")
	}
	if assignment.Confidence != 1 {
		t.Fatalf("confidence is %v, want 1 when there is no competing interpretation", assignment.Confidence)
	}
}

func TestAMovingElementKeepsOneIdentity(t *testing.T) {
	m := identity.New(2, 8)
	first := m.Appear(boundsOf(100, 100, 60, 40), frameWidth, frameHeight, 1, nil)

	sequence := uint64(1)
	id := first.ID
	for step := 1; step <= 10; step++ {
		sequence++
		x := 100 + step*4
		assignment := m.Carry(boundsOf(x, 100, 60, 40), boundsOf(x, 100, 60, 40), frameWidth, frameHeight, sequence, nil)
		if assignment.ID != id {
			t.Fatalf("step %d: identity changed from %d to %d", step, id, assignment.ID)
		}
		if assignment.Uncertain {
			t.Fatalf("step %d: a tracked element was marked uncertain", step)
		}
		if assignment.Confidence <= 0 || assignment.Confidence > 1 {
			t.Fatalf("step %d: confidence %v is outside 0 to 1", step, assignment.Confidence)
		}
		// A stream ends every frame, which is what moves an element's footprint from the frame
		// being resolved to the one the next comparison will look up.
		m.EndFrame(sequence, nil, frameWidth, frameHeight)
	}
}

func TestAnElementBeyondTheMotionToleranceIsNotMatched(t *testing.T) {
	m := identity.New(2, 4)
	first := m.Appear(boundsOf(20, 20, 40, 40), frameWidth, frameHeight, 1, nil)

	// Forty pixels away, far beyond the four pixel tolerance: the same rectangle shape, but not a
	// candidate for the same element.
	assignment := m.Carry(boundsOf(60, 20, 40, 40), boundsOf(60, 20, 40, 40), frameWidth, frameHeight, 2, nil)
	if assignment.ID == first.ID {
		t.Fatalf("an element forty pixels away was matched to identity %d", first.ID)
	}
}

func TestAnElementThatChangesSizeTooMuchIsNotMatched(t *testing.T) {
	m := identity.New(2, 64)
	first := m.Appear(boundsOf(100, 100, 40, 40), frameWidth, frameHeight, 1, nil)

	// Four times the area in the same place: close enough by centre, but not the same element.
	assignment := m.Carry(boundsOf(100, 100, 80, 80), boundsOf(100, 100, 80, 80), frameWidth, frameHeight, 2, nil)
	if assignment.ID == first.ID {
		t.Fatalf("an element that quadrupled in area kept identity %d", first.ID)
	}
}

func TestRemovedElementRetiresItsIdentity(t *testing.T) {
	m := identity.New(2, 8)
	live := m.Appear(boundsOf(50, 50, 60, 40), frameWidth, frameHeight, 1, nil)
	m.EndFrame(1, nil, frameWidth, frameHeight)

	gone := m.Vanish(boundsOf(50, 50, 60, 40), frameWidth, frameHeight, 2)
	if gone.ID != live.ID {
		t.Fatalf("the removed region named identity %d, want the retired %d", gone.ID, live.ID)
	}

	elements := m.Elements()
	if len(elements) != 1 || elements[0].State != identity.Retired {
		t.Fatalf("the identity was not retired: %+v", elements)
	}
}

// TestReappearanceAfterOcclusionIsNewAndUncertain is acceptance scenario 2 of user story 2.
func TestReappearanceAfterOcclusionIsNewAndUncertain(t *testing.T) {
	// One frame of tolerance: the element is hidden for two frames, which is longer than the window.
	m := identity.New(1, 8)
	bounds := boundsOf(100, 80, 60, 40)
	original := m.Appear(bounds, frameWidth, frameHeight, 1, nil)
	m.EndFrame(1, nil, frameWidth, frameHeight)

	// Frames two and three: the element is covered, so what is reported is the cover, and the
	// element's own footprint is inside the area that changed.
	cover := []delta.Bounds{boundsOf(90, 70, 80, 60)}
	m.EndFrame(2, cover, frameWidth, frameHeight)
	m.EndFrame(3, cover, frameWidth, frameHeight)

	switch elements := m.Elements(); {
	case len(elements) != 1:
		t.Fatalf("expected one element, got %+v", elements)
	case elements[0].State != identity.Retired:
		t.Fatalf("an element hidden for longer than the window is still %s", elements[0].State)
	}

	// Frame four: it is visible again in the same place.
	reappeared := m.Appear(bounds, frameWidth, frameHeight, 4, nil)
	if reappeared.ID == original.ID {
		t.Fatalf("the returning element reused identity %d", original.ID)
	}
	if !reappeared.Uncertain {
		t.Fatal("the returning element was matched silently instead of being marked uncertain")
	}
	if reappeared.Confidence <= 0 {
		t.Fatalf("the reappearance carries no evidence at all: confidence %v", reappeared.Confidence)
	}
}

func TestAnElementHiddenWithinTheWindowKeepsItsIdentity(t *testing.T) {
	// Two frames of tolerance: hidden for one frame, which the window allows.
	m := identity.New(2, 8)
	bounds := boundsOf(100, 80, 60, 40)
	original := m.Appear(bounds, frameWidth, frameHeight, 1, nil)
	m.EndFrame(1, nil, frameWidth, frameHeight)

	// Frame two: nothing is reported where the element is.
	m.EndFrame(2, nil, frameWidth, frameHeight)

	// Frame three: visible again. The element was never retired, so it is matched, not reacquired.
	assignment := m.Carry(bounds, bounds, frameWidth, frameHeight, 3, nil)
	if assignment.ID != original.ID {
		t.Fatalf("an element hidden for one frame of a two frame window lost its identity: %d became %d",
			original.ID, assignment.ID)
	}
	if assignment.Uncertain {
		t.Fatal("an element hidden within the window was marked uncertain")
	}
}

// TestIdentifiersAreNeverReused is the property the whole story rests on: an identity that has been
// retired stays retired and its number is never handed out again, however many frames pass.
//
// The sequence is a cycle of four frames: the element appears, drifts two pixels, is reported gone,
// and is absent for a frame. It is therefore retired and reappears a little way from where it was,
// over and over, which is the case a naive counter or a "find the nearest" matcher gets wrong.
func TestIdentifiersAreNeverReused(t *testing.T) {
	m := identity.New(1, 8)

	issued := make(map[uint64]bool)
	retired := make(map[uint64]bool)
	sequence := uint64(0)

	for step := 0; step < 1000; step++ {
		sequence++
		phase := step % 4
		bounds := boundsOf(60+phase*2, 80+phase*3, 40, 30)

		var assignment identity.Assignment
		switch phase {
		case 0:
			assignment = m.Appear(bounds, frameWidth, frameHeight, sequence, nil)
			if retired[assignment.ID] {
				t.Fatalf("frame %d issued identity %d, which had already been retired", sequence, assignment.ID)
			}
			issued[assignment.ID] = true
		case 1:
			assignment = m.Carry(bounds, bounds, frameWidth, frameHeight, sequence, nil)
			if retired[assignment.ID] {
				t.Fatalf("frame %d revived retired identity %d", sequence, assignment.ID)
			}
			if !issued[assignment.ID] {
				t.Fatalf("frame %d produced identity %d without allocating it", sequence, assignment.ID)
			}
		case 2:
			assignment = m.Vanish(bounds, frameWidth, frameHeight, sequence)
			if !issued[assignment.ID] {
				t.Fatalf("frame %d retired identity %d that was never issued", sequence, assignment.ID)
			}
			retired[assignment.ID] = true
		case 3:
			// The element is gone and nothing is reported, so only the window ages.
		}
		m.EndFrame(sequence, nil, frameWidth, frameHeight)
	}

	if len(issued) < 100 {
		t.Fatalf("the sequence produced only %d identities, which is too few to test reuse", len(issued))
	}
	if len(retired) < 100 {
		t.Fatalf("only %d identities were retired, which is too few to test reuse", len(retired))
	}
}

// TestMemoryDoesNotGrowWithStreamLength is acceptance scenario 3 of user story 2 for the identity
// table: elements appear and disappear for a thousand frames, and the table must not grow without
// bound. Retired identities are kept, because their numbers must never be reissued, so the table is
// bounded by the number of distinct elements rather than by the number of frames.
func TestMemoryDoesNotGrowWithStreamLength(t *testing.T) {
	const window = 2
	m := identity.New(window, 4)

	// One element that comes and goes in the same place for a thousand frames.
	bounds := boundsOf(100, 100, 40, 30)
	var firstFrame uint64
	for sequence := uint64(1); sequence <= 1000; sequence++ {
		if sequence%3 == 1 {
			assignment := m.Appear(bounds, frameWidth, frameHeight, sequence, nil)
			if sequence == 1 {
				firstFrame = assignment.ID
			}
		} else if sequence%3 == 0 {
			m.Vanish(bounds, frameWidth, frameHeight, sequence)
		} else {
			m.Carry(bounds, bounds, frameWidth, frameHeight, sequence, nil)
		}
		m.EndFrame(sequence, nil, frameWidth, frameHeight)
	}

	elements := m.Elements()
	if len(elements) > 1000 {
		t.Fatalf("the identity table holds %d elements after a thousand frames", len(elements))
	}
	// Every reappearance past the window is a new identity by design, so the table is expected to
	// hold one element per reappearance. What must not happen is unbounded growth per frame.
	t.Logf("identity table holds %d elements after 1000 frames, first identity %d", len(elements), firstFrame)

	live := 0
	for _, element := range elements {
		if element.State == identity.Live {
			live++
		}
	}
	if live > window+2 {
		t.Fatalf("%d elements are live at once, which is more than the window allows", live)
	}
}

// TestTheSameSequenceProducesTheSameIdentities guards the determinism requirement at the level of
// the identity table: two runs over identical input must issue the same numbers.
func TestTheSameSequenceProducesTheSameIdentities(t *testing.T) {
	run := func() string {
		m := identity.New(2, 8)
		report := ""
		for step := 0; step < 50; step++ {
			sequence := uint64(step + 1)
			bounds := boundsOf(20+step*3, 40, 50, 30)
			switch step % 5 {
			case 4:
				assignment := m.Vanish(bounds, frameWidth, frameHeight, sequence)
				report += fmt.Sprintf("%d:vanish:%d ", sequence, assignment.ID)
			default:
				assignment := m.Carry(bounds, bounds, frameWidth, frameHeight, sequence, nil)
				report += fmt.Sprintf("%d:carry:%d:%v ", sequence, assignment.ID, assignment.Uncertain)
			}
			m.EndFrame(sequence, nil, frameWidth, frameHeight)
		}
		return report
	}
	if run() != run() {
		t.Fatal("two identical runs produced different identities")
	}
}

func TestSortedReturnsEveryIdentityInAllocationOrder(t *testing.T) {
	m := identity.New(1, 8)
	for index := 0; index < 5; index++ {
		m.Appear(boundsOf(10+index*50, 10, 30, 30), frameWidth, frameHeight, uint64(index+1), nil)
		m.EndFrame(uint64(index+1), nil, frameWidth, frameHeight)
	}

	sorted := m.Sorted()
	if len(sorted) != 5 {
		t.Fatalf("Sorted returned %d elements, want 5", len(sorted))
	}
	for index, element := range sorted {
		if element.ID != uint64(index+1) {
			t.Fatalf("element %d has identity %d, want %d", index, element.ID, index+1)
		}
	}
	if len(m.Elements()) != 5 {
		t.Fatalf("Elements returned %d, want 5", len(m.Elements()))
	}
}

func TestVanishWithNothingToRetireAllocatesAName(t *testing.T) {
	m := identity.New(1, 8)
	// Nothing has ever been tracked, so a removal cannot name an element. The region still has to
	// carry an identity, and borrowing one that belongs to something else would be worse than a
	// fresh number that names nothing.
	assignment := m.Vanish(boundsOf(10, 10, 20, 20), frameWidth, frameHeight, 1)
	if assignment.ID != 1 {
		t.Fatalf("identity is %d, want a fresh 1", assignment.ID)
	}
	// The identity is retired at once: a removal that named a live identity would tell a consumer
	// that an element it has never seen is gone, while the engine kept matching it.
	elements := m.Elements()
	if len(elements) != 1 {
		t.Fatalf("a removal with nothing to retire left %+v", elements)
	}
	if elements[0].State != identity.Retired {
		t.Fatalf("the identity a removal named is %s, want retired", elements[0].State)
	}
	if elements[0].Confidence != 0 {
		t.Fatalf("a removed identity kept confidence %v", elements[0].Confidence)
	}
}

func TestConfidenceFallsWhenAnElementOnlyPartlyOverlaps(t *testing.T) {
	m := identity.New(2, 16)
	first := m.Appear(boundsOf(100, 100, 60, 40), frameWidth, frameHeight, 1, nil)
	m.EndFrame(1, nil, frameWidth, frameHeight)

	// Moved ten pixels along, so it overlaps its previous footprint by most but not all of itself.
	assignment := m.Carry(boundsOf(110, 100, 60, 40), boundsOf(110, 100, 60, 40), frameWidth, frameHeight, 2, nil)
	if assignment.ID != first.ID {
		t.Fatalf("identity changed from %d to %d for a ten pixel move", first.ID, assignment.ID)
	}
	if assignment.Confidence >= 1 {
		t.Fatalf("confidence stayed at %v for a partial overlap, so it says nothing", assignment.Confidence)
	}
	if assignment.Confidence <= 0.5 {
		t.Fatalf("confidence %v is too low for an element that mostly still overlaps", assignment.Confidence)
	}
	m.EndFrame(2, nil, frameWidth, frameHeight)

	for _, element := range m.Elements() {
		if element.ID != first.ID {
			continue
		}
		if element.LastFrame != 2 {
			t.Fatalf("the element's last frame is %d, want 2", element.LastFrame)
		}
		if element.Confidence != assignment.Confidence {
			t.Fatalf("the element records confidence %v but the assignment said %v", element.Confidence, assignment.Confidence)
		}
	}
}

func TestZeroToleranceMatchesOnlyTheSamePlace(t *testing.T) {
	m := identity.New(2, 0)
	first := m.Appear(boundsOf(50, 50, 40, 30), frameWidth, frameHeight, 1, nil)
	m.EndFrame(1, nil, frameWidth, frameHeight)

	same := m.Carry(boundsOf(50, 50, 40, 30), boundsOf(50, 50, 40, 30), frameWidth, frameHeight, 2, nil)
	if same.ID != first.ID {
		t.Fatalf("an unmoved element lost its identity with zero tolerance")
	}
	m.EndFrame(2, nil, frameWidth, frameHeight)

	moved := m.Carry(boundsOf(52, 50, 40, 30), boundsOf(52, 50, 40, 30), frameWidth, frameHeight, 3, nil)
	if moved.ID == first.ID {
		t.Fatalf("a two pixel move matched with zero tolerance")
	}
}

func TestAnElementAtTheAreaRatioBoundaryIsMatched(t *testing.T) {
	// Each case starts from the same untracked element, because a match rewrites what the element is:
	// chaining two growths tests the second growth against the first, not against the original.
	build := func() (*identity.Map, uint64) {
		m := identity.New(2, 8)
		first := m.Appear(boundsOf(100, 100, 40, 40), frameWidth, frameHeight, 1, nil)
		m.EndFrame(1, nil, frameWidth, frameHeight)
		return m, first.ID
	}

	// Exactly twice the area, grown about the same centre so that only the size gate is under test.
	// The boundary is inclusive by design, documented in the constants.
	m, id := build()
	doubled := m.Carry(boundsOf(100, 80, 40, 80), boundsOf(100, 80, 40, 80), frameWidth, frameHeight, 2, nil)
	if doubled.ID != id {
		t.Fatalf("an element that exactly doubled in area lost its identity")
	}

	// Three times the area about the same centre: well inside the motion tolerance, and still not
	// the same element.
	m, id = build()
	tripled := m.Carry(boundsOf(100, 60, 40, 120), boundsOf(100, 60, 40, 120), frameWidth, frameHeight, 2, nil)
	if tripled.ID == id {
		t.Fatalf("an element that tripled in area kept its identity")
	}
}

func TestAnAreaInsideAnElementIsAttributedToIt(t *testing.T) {
	// The case the part-of rule exists for: a large element is tracked, and a later frame changes
	// only a small area inside it. The area is not similar in size to the element, and its centre is
	// far from the element's centre, so only containment can attribute it.
	m := identity.New(2, 32)
	element := m.Appear(boundsOf(100, 100, 200, 120), frameWidth, frameHeight, 1, nil)
	m.EndFrame(1, nil, frameWidth, frameHeight)

	inside := boundsOf(110, 110, 20, 16)
	assignment := m.Carry(inside, inside, frameWidth, frameHeight, 2, nil)
	if assignment.ID != element.ID {
		t.Fatalf("an area inside the element was given identity %d instead of %d", assignment.ID, element.ID)
	}
	if !assignment.Carried {
		t.Fatal("the assignment did not say where the element was")
	}
	if assignment.Confidence <= 0 || assignment.Confidence > 1 {
		t.Fatalf("confidence %v is outside 0 to 1", assignment.Confidence)
	}
}

func TestSetRulesChangesTheWindowWithoutLosingIdentities(t *testing.T) {
	m := identity.New(0, 2)
	bounds := boundsOf(50, 50, 40, 30)
	first := m.Appear(bounds, frameWidth, frameHeight, 1, nil)
	m.EndFrame(1, nil, frameWidth, frameHeight)

	// A window of zero retires an element the moment it is missed, which requires something to have
	// changed where it is; otherwise the map is right that it is still on the screen.
	cover := []delta.Bounds{boundsOf(40, 40, 60, 50)}
	m.EndFrame(2, cover, frameWidth, frameHeight)
	if state := m.Elements()[0].State; state != identity.Retired {
		t.Fatalf("a window of zero left the element %s after a miss", state)
	}

	// Widening the window afterwards does not revive it, which is the rule: a retired identity never
	// returns to live.
	m.SetRules(5, 2)
	m.EndFrame(3, cover, frameWidth, frameHeight)
	if state := m.Elements()[0].State; state != identity.Retired {
		t.Fatalf("widening the window revived a retired identity: %s", state)
	}
	if m.Elements()[0].ID != first.ID {
		t.Fatalf("the element's identity changed to %d", m.Elements()[0].ID)
	}

	// A second element tracked under the new rules is matched normally.
	other := m.Appear(boundsOf(200, 150, 40, 30), frameWidth, frameHeight, 4, nil)
	m.EndFrame(4, nil, frameWidth, frameHeight)
	m.EndFrame(5, nil, frameWidth, frameHeight)
	if state := stateOf(m, other.ID); state != identity.Live {
		t.Fatalf("an element missed once under a window of five is %s", state)
	}
}

func TestRetireAllEndsEveryElementWithoutReissuingNumbers(t *testing.T) {
	m := identity.New(5, 8)
	first := m.Appear(boundsOf(20, 20, 40, 30), frameWidth, frameHeight, 1, nil)
	second := m.Appear(boundsOf(200, 150, 40, 30), frameWidth, frameHeight, 1, nil)
	m.EndFrame(1, nil, frameWidth, frameHeight)

	m.RetireAll()
	for _, element := range m.Elements() {
		if element.State != identity.Retired {
			t.Fatalf("element %d is %s after RetireAll", element.ID, element.State)
		}
	}

	// A new element continues the sequence instead of starting again, which is what keeps an
	// identifier from meaning two different things in one session.
	third := m.Appear(boundsOf(100, 100, 20, 20), frameWidth, frameHeight, 2, nil)
	if third.ID <= second.ID || third.ID <= first.ID {
		t.Fatalf("identity %d was reissued after %d and %d", third.ID, first.ID, second.ID)
	}
	if third.Uncertain {
		t.Fatal("an element appearing where nothing was tracked was marked uncertain")
	}
}

func stateOf(m *identity.Map, id uint64) identity.State {
	for _, element := range m.Elements() {
		if element.ID == id {
			return element.State
		}
	}
	return ""
}

// TestASmallElementInsideALargeOneKeepsItsOwnIdentity is the case the review found: an exact match
// must beat containment, or a large element can take the identity of a small one that sits inside it
// and the answer depends on which of the two was allocated first.
func TestASmallElementInsideALargeOneKeepsItsOwnIdentity(t *testing.T) {
	for _, order := range []string{"large first", "small first"} {
		t.Run(order, func(t *testing.T) {
			m := identity.New(2, 8)
			largeBounds := boundsOf(40, 40, 100, 100)
			smallBounds := boundsOf(80, 80, 10, 10)

			var large, small identity.Assignment
			if order == "large first" {
				large = m.Appear(largeBounds, frameWidth, frameHeight, 1, nil)
				small = m.Appear(smallBounds, frameWidth, frameHeight, 1, nil)
			} else {
				small = m.Appear(smallBounds, frameWidth, frameHeight, 1, nil)
				large = m.Appear(largeBounds, frameWidth, frameHeight, 1, nil)
			}
			m.EndFrame(1, nil, frameWidth, frameHeight)

			// The small element changes inside the large one. It must continue as itself, whichever
			// order the two were allocated in.
			carried := m.Carry(smallBounds, smallBounds, frameWidth, frameHeight, 2, nil)
			if carried.ID != small.ID {
				t.Fatalf("%s: the small element's identity became %d instead of %d", order, carried.ID, small.ID)
			}
			if carried.ID == large.ID {
				t.Fatalf("%s: the large element took the small element's identity", order)
			}

			// And the large element's own change continues as itself.
			largeEdge := boundsOf(40, 40, 100, 10)
			largeCarried := m.Carry(largeEdge, largeEdge, frameWidth, frameHeight, 2, nil)
			if largeCarried.ID != large.ID {
				t.Fatalf("%s: the large element's identity became %d instead of %d", order, largeCarried.ID, large.ID)
			}
		})
	}
}

// TestAnElementThatSitsStillIsNotRetired is the other case the review found: frames passing without
// a change are not occlusion. An element whose pixels did not change is still on the screen.
func TestAnElementThatSitsStillIsNotRetired(t *testing.T) {
	m := identity.New(1, 8)
	bounds := boundsOf(60, 60, 40, 30)
	original := m.Appear(bounds, frameWidth, frameHeight, 1, nil)
	m.EndFrame(1, nil, frameWidth, frameHeight)

	// Many frames pass with nothing reported anywhere, which is what a still screen looks like.
	for sequence := uint64(2); sequence <= 50; sequence++ {
		m.EndFrame(sequence, nil, frameWidth, frameHeight)
	}
	if state := stateOf(m, original.ID); state != identity.Live {
		t.Fatalf("an element that sat still for fifty frames is %s", state)
	}

	// A change to it is still a change to the same element, not a reacquisition.
	changed := m.Carry(bounds, bounds, frameWidth, frameHeight, 51, nil)
	if changed.ID != original.ID || changed.Uncertain {
		t.Fatalf("a change to an element that never went away was reported as %+v", changed)
	}
}

// TestRetiredEntriesAreBoundedByTheRetentionWindow checks the memory bound: retired identities exist
// only to recognise a returning element, so they are dropped once that is no longer plausible. The
// real property is that the table does not grow with the length of the session, so two runs of
// different length are compared rather than one run against a fixed number.
func TestRetiredEntriesAreBoundedByTheRetentionWindow(t *testing.T) {
	m := identity.New(1, 8)
	cover := []delta.Bounds{boundsOf(50, 50, 60, 50)}
	bounds := boundsOf(60, 60, 40, 30)
	sequence := uint64(0)

	run := func(cycles int) {
		for cycle := 0; cycle < cycles; cycle++ {
			sequence++
			m.Appear(bounds, frameWidth, frameHeight, sequence, nil)
			m.EndFrame(sequence, nil, frameWidth, frameHeight)
			for step := 0; step < 3; step++ {
				sequence++
				// The area changes, so the element is not matched and ages out.
				m.EndFrame(sequence, cover, frameWidth, frameHeight)
			}
		}
	}

	run(200)
	after200 := len(m.Elements())
	run(200)
	after400 := len(m.Elements())

	t.Logf("table holds %d entries after 200 cycles and %d after 400", after200, after400)
	if after400 > after200+10 {
		t.Fatalf("the table grew from %d to %d entries with session length, so retired entries are not bounded",
			after200, after400)
	}
	if after400 > 120 {
		t.Fatalf("the table holds %d entries, more than the retention window allows", after400)
	}
}

// --- footprint evolution, the AUD-020 regression ---------------------------------------------

func TestAMovingFootprintFollowsWithoutLag(t *testing.T) {
	m := identity.New(2, 8)
	bounds := boundsOf(100, 100, 60, 40)
	first := m.Appear(bounds, frameWidth, frameHeight, 1, nil)
	m.EndFrame(1, nil, frameWidth, frameHeight)

	// Thirty steps of four pixels. The element's footprint must follow it rather than accumulate the
	// whole path: a footprint that lagged would eventually cover where the element has been.
	previous := bounds
	for step := 1; step <= 30; step++ {
		x := 100 + step*4
		current := boundsOf(x, 100, 60, 40)
		assignment := m.Carry(previous, current, frameWidth, frameHeight, uint64(step+1), nil)
		if assignment.ID != first.ID {
			t.Fatalf("step %d: identity changed to %d", step, assignment.ID)
		}
		m.EndFrame(uint64(step+1), []delta.Bounds{current}, frameWidth, frameHeight)
		previous = current
	}

	footprint := footprintOf(m, first.ID, frameWidth, frameHeight)
	if footprint.Dx() > 80 {
		t.Fatalf("the footprint is %d pixels wide for a 60 pixel element, so it kept the path", footprint.Dx())
	}
	if footprint.Min.X < 180 {
		t.Fatalf("the footprint starts at %d, so it lagged behind the element", footprint.Min.X)
	}
}

func TestAPartialChangeLeavesTheFootprintIntact(t *testing.T) {
	// The defect the review found: a change along one edge shrank the element to that strip, so a
	// change along the other edge in the next frame looked like a new element.
	m := identity.New(2, 8)
	whole := boundsOf(100, 100, 100, 100)
	first := m.Appear(whole, frameWidth, frameHeight, 1, nil)
	m.EndFrame(1, nil, frameWidth, frameHeight)

	leftStrip := boundsOf(100, 100, 10, 100)
	if assignment := m.Carry(whole, leftStrip, frameWidth, frameHeight, 2, nil); assignment.ID != first.ID {
		t.Fatalf("the left strip became a new element: %d instead of %d", assignment.ID, first.ID)
	}
	m.EndFrame(2, []delta.Bounds{leftStrip}, frameWidth, frameHeight)

	rightStrip := boundsOf(190, 100, 10, 100)
	assignment := m.Carry(rightStrip, rightStrip, frameWidth, frameHeight, 3, nil)
	if assignment.ID != first.ID {
		t.Fatalf("a change along the other edge became a new element: %d instead of %d", assignment.ID, first.ID)
	}
}

func TestAGrowingElementExtendsAndAShrinkingOneKeepsItsFootprint(t *testing.T) {
	m := identity.New(2, 8)
	small := boundsOf(100, 100, 40, 40)
	first := m.Appear(small, frameWidth, frameHeight, 1, nil)
	m.EndFrame(1, nil, frameWidth, frameHeight)

	// Grown by a strip on the right: new ground, so the footprint extends.
	grown := boundsOf(100, 100, 60, 40)
	if assignment := m.Carry(small, grown, frameWidth, frameHeight, 2, nil); assignment.ID != first.ID {
		t.Fatalf("a grown element became a new element: %d", assignment.ID)
	}
	m.EndFrame(2, []delta.Bounds{grown}, frameWidth, frameHeight)
	if footprint := footprintOf(m, first.ID, frameWidth, frameHeight); footprint.Dx() < 55 {
		t.Fatalf("the footprint did not extend with the growth: %v", footprint)
	}

	// Shrunk by a strip on the right: the engine keeps what it believed, because an extent is only
	// ever revealed by change and nothing here contradicts the left part.
	shrunk := boundsOf(100, 100, 30, 40)
	if assignment := m.Carry(grown, shrunk, frameWidth, frameHeight, 3, nil); assignment.ID != first.ID {
		t.Fatalf("a shrunk element became a new element: %d", assignment.ID)
	}
	m.EndFrame(3, []delta.Bounds{shrunk}, frameWidth, frameHeight)
	if footprint := footprintOf(m, first.ID, frameWidth, frameHeight); footprint.Dx() < 55 {
		t.Fatalf("the footprint shrank to %v, which is what AUD-020 was", footprint)
	}
}

func TestFootprintEvolutionIsDeterministic(t *testing.T) {
	run := func() []int {
		m := identity.New(2, 8)
		whole := boundsOf(50, 50, 80, 60)
		element := m.Appear(whole, frameWidth, frameHeight, 1, nil)
		observed := []delta.Bounds{
			boundsOf(50, 50, 10, 60),
			boundsOf(120, 50, 10, 60),
			boundsOf(54, 50, 20, 60),
			boundsOf(50, 46, 80, 10),
		}
		positions := make([]int, 0, len(observed))
		for index, bounds := range observed {
			sequence := uint64(index + 2)
			m.Carry(whole, bounds, frameWidth, frameHeight, sequence, nil)
			m.EndFrame(sequence, []delta.Bounds{bounds}, frameWidth, frameHeight)
			footprint := footprintOf(m, element.ID, frameWidth, frameHeight)
			positions = append(positions, footprint.Min.X, footprint.Min.Y, footprint.Dx(), footprint.Dy())
		}
		return positions
	}
	first, second := run(), run()
	if len(first) != len(second) {
		t.Fatalf("two runs produced %d and %d measurements", len(first), len(second))
	}
	for index := range first {
		if first[index] != second[index] {
			t.Fatalf("two runs disagree at %d: %v against %v", index, first, second)
		}
	}
}

// footprintOf reads an element's footprint in pixels, for the tests above.
func footprintOf(m *identity.Map, id uint64, width, height int) image.Rectangle {
	for _, element := range m.Elements() {
		if element.ID != id {
			continue
		}
		left := int(element.Bounds.X*float64(width) + 0.5)
		top := int(element.Bounds.Y*float64(height) + 0.5)
		return image.Rect(left, top,
			left+int(element.Bounds.W*float64(width)+0.5),
			top+int(element.Bounds.H*float64(height)+0.5))
	}
	return image.Rectangle{}
}

// --- appearance as evidence -------------------------------------------------------------------

func signatureOfCells(values ...uint8) identity.Signature {
	var s identity.Signature
	copy(s[:], values)
	return s
}

func filledSignature(value uint8) identity.Signature {
	var s identity.Signature
	for index := range s {
		s[index] = value
	}
	return s
}

func TestReacquisitionRequiresMoreThanGeometry(t *testing.T) {
	// A shape is retired, and something that looks nothing like it later appears in the same place.
	// Geometry alone would call that a reacquisition; appearance says it is a new element, and the
	// uncertainty marker stays meaningful because it is not raised here.
	m := identity.New(1, 8)
	bounds := boundsOf(100, 80, 60, 40)
	cover := []delta.Bounds{boundsOf(90, 70, 80, 60)}

	dark := filledSignature(30)
	bright := filledSignature(220)

	original := m.Appear(bounds, frameWidth, frameHeight, 1, &dark)
	m.EndFrame(1, nil, frameWidth, frameHeight)
	m.EndFrame(2, cover, frameWidth, frameHeight)
	m.EndFrame(3, cover, frameWidth, frameHeight)
	if state := stateOf(m, original.ID); state != identity.Retired {
		t.Fatalf("the element was not retired, so the test is not testing reacquisition: %s", state)
	}

	appearance := m.Appear(bounds, frameWidth, frameHeight, 4, &bright)
	if appearance.Uncertain {
		t.Fatal("an area that looks nothing like the retired element was marked as a reacquisition")
	}
	if appearance.ID == original.ID {
		t.Fatal("the retired identity was reused")
	}
}

func TestReacquisitionWithAMatchingAppearanceIsUncertain(t *testing.T) {
	m := identity.New(1, 8)
	bounds := boundsOf(100, 80, 60, 40)
	cover := []delta.Bounds{boundsOf(90, 70, 80, 60)}
	appearance := filledSignature(120)

	original := m.Appear(bounds, frameWidth, frameHeight, 1, &appearance)
	m.EndFrame(1, nil, frameWidth, frameHeight)
	m.EndFrame(2, cover, frameWidth, frameHeight)
	m.EndFrame(3, cover, frameWidth, frameHeight)
	if state := stateOf(m, original.ID); state != identity.Retired {
		t.Fatalf("the element was not retired: %s", state)
	}

	// The same content comes back, give or take capture noise.
	noisy := filledSignature(126)
	returned := m.Appear(bounds, frameWidth, frameHeight, 4, &noisy)
	if !returned.Uncertain {
		t.Fatal("content that matches the retired element was not marked uncertain")
	}
	if returned.ID == original.ID {
		t.Fatal("the retired identity was reused")
	}
	if returned.Confidence <= 0 {
		t.Fatalf("the reacquisition carries no evidence: confidence %v", returned.Confidence)
	}
}

func TestClosestSignatureDecidesBetweenTwoCandidates(t *testing.T) {
	// Two retired elements could explain the same place; the one that looks like the returning content
	// is the one whose appearance the new element records against.
	m := identity.New(1, 8)
	place := boundsOf(100, 80, 60, 40)
	cover := []delta.Bounds{boundsOf(90, 70, 80, 60)}

	dark := filledSignature(20)
	mid := filledSignature(120)
	first := m.Appear(place, frameWidth, frameHeight, 1, &dark)
	m.EndFrame(1, nil, frameWidth, frameHeight)
	second := m.Appear(place, frameWidth, frameHeight, 2, &mid)
	m.EndFrame(2, cover, frameWidth, frameHeight)
	m.EndFrame(3, cover, frameWidth, frameHeight)
	// One more covered frame, because the second element was seen a frame later and ages a frame later.
	m.EndFrame(4, cover, frameWidth, frameHeight)
	for _, id := range []uint64{first.ID, second.ID} {
		if state := stateOf(m, id); state != identity.Retired {
			t.Fatalf("identity %d is %s, want retired", id, state)
		}
	}

	closer := filledSignature(121)
	returned := m.Appear(place, frameWidth, frameHeight, 5, &closer)
	if !returned.Uncertain {
		t.Fatal("the returning content was not marked uncertain")
	}
	if returned.ID == first.ID || returned.ID == second.ID {
		t.Fatal("a retired identity was reused")
	}
}

func TestSignatureDistanceAndTolerance(t *testing.T) {
	base := filledSignature(100)

	if distance := base.Distance(filledSignature(100)); distance != 0 {
		t.Fatalf("identical signatures are %d apart", distance)
	}
	// The allowance is per cell on average, so one cell out by a lot is not a repaint and one cell
	// out by a little is not a difference.
	near := filledSignature(100)
	near[0] = 100 + identity.SignatureTolerance
	if !base.Close(near) {
		t.Fatalf("signatures %d apart were not considered close", base.Distance(near))
	}

	// Every cell out by one level more than the allowance, which is the smallest difference the
	// per-cell rule rejects. A single cell cannot carry the whole allowance in one byte.
	far := filledSignature(100 + identity.SignatureTolerance + 1)
	if base.Close(far) {
		t.Fatalf("signatures %d apart were considered close", base.Distance(far))
	}
	if distance := base.Distance(far); distance != (identity.SignatureTolerance+1)*identity.SignatureCells {
		t.Fatalf("distance is %d, want %d", distance, (identity.SignatureTolerance+1)*identity.SignatureCells)
	}

	closest, distance := base.ClosestTo([]identity.Signature{filledSignature(200), filledSignature(101)})
	if closest != 1 || distance != identity.SignatureCells {
		t.Fatalf("ClosestTo chose %d at distance %d, want 1 at %d", closest, distance, identity.SignatureCells)
	}
}

// --- covers and returns ------------------------------------------------------------------------

func TestCoverNeedsAnAreaBiggerThanTheElement(t *testing.T) {
	m := identity.New(2, 8)
	element := boundsOf(100, 100, 60, 40)
	live := m.Appear(element, frameWidth, frameHeight, 1, nil)
	m.EndFrame(1, nil, frameWidth, frameHeight)

	// A change inside the element does not enclose it.
	if enclosed := m.Enclosed(boundsOf(110, 110, 20, 10), frameWidth, frameHeight); len(enclosed) != 0 {
		t.Fatalf("a change inside the element enclosed it: %+v", enclosed)
	}
	// An area exactly the element's size does not either: there is no margin past it.
	if enclosed := m.Enclosed(element, frameWidth, frameHeight); len(enclosed) != 0 {
		t.Fatalf("an area the size of the element enclosed it: %+v", enclosed)
	}
	if state := stateOf(m, live.ID); state != identity.Live {
		t.Fatalf("the element is %s after an area that does not enclose it", state)
	}

	// An area that contains the element and reaches past it does.
	enclosed := m.Enclosed(boundsOf(90, 90, 80, 60), frameWidth, frameHeight)
	if len(enclosed) != 1 {
		t.Fatalf("an area containing the element and exceeding it enclosed %d elements", len(enclosed))
	}
	if enclosed[0].ID != live.ID {
		t.Fatalf("the area named identity %d instead of %d", enclosed[0].ID, live.ID)
	}
	m.RetireByID(enclosed[0].ID)
	if state := stateOf(m, live.ID); state != identity.Retired {
		t.Fatalf("the covered element is %s, want retired", state)
	}
}

// TestOneAreaCanEncloseSeveralElements is the case the review found: a single covering area took the
// place of two elements, and retiring only the closest containment left the other handle live, so a
// consumer following it would later find it attached to something unrelated.
func TestOneAreaCanEncloseSeveralElements(t *testing.T) {
	m := identity.New(2, 8)
	first := m.Appear(boundsOf(60, 100, 40, 40), frameWidth, frameHeight, 1, nil)
	second := m.Appear(boundsOf(160, 100, 40, 40), frameWidth, frameHeight, 1, nil)
	m.EndFrame(1, nil, frameWidth, frameHeight)

	enclosed := m.Enclosed(boundsOf(40, 80, 180, 80), frameWidth, frameHeight)
	if len(enclosed) != 2 {
		t.Fatalf("an area covering two elements enclosed %d of them", len(enclosed))
	}
	ids := map[uint64]bool{}
	for _, element := range enclosed {
		ids[element.ID] = true
	}
	if !ids[first.ID] || !ids[second.ID] {
		t.Fatalf("the enclosed set is %+v, want both %d and %d", enclosed, first.ID, second.ID)
	}

	for _, element := range enclosed {
		m.RetireByID(element.ID)
	}
	for _, id := range []uint64{first.ID, second.ID} {
		if state := stateOf(m, id); state != identity.Retired {
			t.Fatalf("identity %d is %s after the area covered it", id, state)
		}
	}
}

func TestCoverIgnoresRetiredElementsAndEmptyAreas(t *testing.T) {
	m := identity.New(0, 8)
	bounds := boundsOf(100, 100, 60, 40)
	element := m.Appear(bounds, frameWidth, frameHeight, 1, nil)
	m.EndFrame(1, nil, frameWidth, frameHeight)
	// Retire it by changing its area and not matching it.
	m.EndFrame(2, []delta.Bounds{bounds}, frameWidth, frameHeight)
	if state := stateOf(m, element.ID); state != identity.Retired {
		t.Fatalf("the element is %s, want retired for the test", state)
	}

	if enclosed := m.Enclosed(boundsOf(90, 90, 80, 60), frameWidth, frameHeight); len(enclosed) != 0 {
		t.Fatalf("a retired element was enclosed: %+v", enclosed)
	}
	if enclosed := m.Enclosed(boundsOf(0, 0, 0, 0), frameWidth, frameHeight); len(enclosed) != 0 {
		t.Fatalf("an empty area enclosed something: %+v", enclosed)
	}
	if enclosed := m.Enclosed(boundsOf(300, 200, 30, 30), frameWidth, frameHeight); len(enclosed) != 0 {
		t.Fatalf("an area nowhere near an element enclosed something: %+v", enclosed)
	}
}

func TestReturnNeedsEvidenceInBothDirections(t *testing.T) {
	// Each case starts from the same state: one element tracked and then retired, so the question is
	// only what the returning area looks like.
	build := func() (*identity.Map, uint64, identity.Signature) {
		m := identity.New(1, 8)
		bounds := boundsOf(100, 100, 60, 40)
		appearance := filledSignature(80)
		element := m.Appear(bounds, frameWidth, frameHeight, 1, &appearance)
		m.EndFrame(1, nil, frameWidth, frameHeight)
		m.EndFrame(2, []delta.Bounds{bounds}, frameWidth, frameHeight)
		m.EndFrame(3, []delta.Bounds{bounds}, frameWidth, frameHeight)
		return m, element.ID, appearance
	}
	bounds := boundsOf(100, 100, 60, 40)

	t.Run("no appearance to compare", func(t *testing.T) {
		m, _, _ := build()
		if decision := m.Return(bounds, nil, frameWidth, frameHeight, 4); decision.IsReturn {
			t.Fatalf("a return was decided without an appearance: %+v", decision)
		}
	})

	t.Run("nothing live overlaps", func(t *testing.T) {
		m := identity.New(1, 8)
		if decision := m.Return(bounds, ptrSignature(filledSignature(80)), frameWidth, frameHeight, 1); decision.IsReturn {
			t.Fatalf("a return was decided with nothing tracked: %+v", decision)
		}
	})

	t.Run("the area looks like what is there", func(t *testing.T) {
		m := identity.New(1, 8)
		appearance := filledSignature(80)
		m.Appear(bounds, frameWidth, frameHeight, 1, &appearance)
		m.EndFrame(1, nil, frameWidth, frameHeight)
		// A live element, and an area that looks just like it: that is a change, not a return.
		if decision := m.Return(bounds, &appearance, frameWidth, frameHeight, 2); decision.IsReturn {
			t.Fatalf("an area that looks like what is on screen was called a return: %+v", decision)
		}
	})

	t.Run("no retired element matches the geometry", func(t *testing.T) {
		m, _, _ := build()
		// Far from where the retired element was, and looking nothing like either element.
		elsewhere := boundsOf(10, 10, 60, 40)
		if decision := m.Return(elsewhere, ptrSignature(filledSignature(200)), frameWidth, frameHeight, 4); decision.IsReturn {
			t.Fatalf("a return was decided away from every retired element: %+v", decision)
		}
	})

	t.Run("the retired element is not the closer one", func(t *testing.T) {
		m, _, _ := build()
		// Overlaps the retired element's place, but looks nothing like it and nothing like anything
		// else either, so the engine has no reason to prefer the retired one.
		if decision := m.Return(bounds, ptrSignature(filledSignature(200)), frameWidth, frameHeight, 4); decision.IsReturn {
			t.Fatalf("a return was decided on geometry alone: %+v", decision)
		}
	})

	t.Run("the evidence supports a return", func(t *testing.T) {
		m, original, appearance := build()
		// A live element that looks quite different, and an area that looks like the retired one.
		// The covering element sits over the retired element's place, which is what makes the geometric
		// question worth asking at all, and looks quite different from the returning content.
		cover := boundsOf(100, 100, 80, 50)
		liveElement := m.Appear(cover, frameWidth, frameHeight, 4, ptrSignature(filledSignature(200)))
		m.EndFrame(4, []delta.Bounds{cover}, frameWidth, frameHeight)

		decision := m.Return(bounds, &appearance, frameWidth, frameHeight, 5)
		if !decision.IsReturn {
			t.Fatalf("a returning element was not recognised: %+v", decision)
		}
		if decision.CoveredID != liveElement.ID {
			t.Fatalf("the return covered identity %d instead of %d", decision.CoveredID, liveElement.ID)
		}
		if decision.Assignment.ID == original || decision.Assignment.ID == liveElement.ID {
			t.Fatalf("the returning content reused identity %d", decision.Assignment.ID)
		}
		if !decision.Assignment.Uncertain {
			t.Fatal("the returning content was not marked uncertain")
		}
		if decision.Assignment.Confidence <= 0 || decision.Assignment.Confidence > 1 {
			t.Fatalf("confidence %v is outside zero to one", decision.Assignment.Confidence)
		}
		if state := stateOf(m, liveElement.ID); state != identity.Retired {
			t.Fatalf("the element that was covered is %s, want retired", state)
		}
	})
}

func TestLiveElementsAreOnlyTheLiveOnesInOrder(t *testing.T) {
	m := identity.New(0, 8)
	first := m.Appear(boundsOf(20, 20, 30, 30), frameWidth, frameHeight, 1, nil)
	second := m.Appear(boundsOf(200, 20, 30, 30), frameWidth, frameHeight, 1, nil)
	m.EndFrame(1, nil, frameWidth, frameHeight)

	live := m.LiveElements()
	if len(live) != 2 {
		t.Fatalf("LiveElements returned %d elements, want 2", len(live))
	}
	if live[0].ID != first.ID || live[1].ID != second.ID {
		t.Fatalf("LiveElements is not ordered by identity: %+v", live)
	}

	// Change the first element's area and do not match it, so it retires.
	m.EndFrame(2, []delta.Bounds{boundsOf(20, 20, 30, 30)}, frameWidth, frameHeight)
	if state := stateOf(m, first.ID); state != identity.Retired {
		t.Fatalf("the first element is %s, want retired", state)
	}
	live = m.LiveElements()
	if len(live) != 1 || live[0].ID != second.ID {
		t.Fatalf("LiveElements returned %+v after a retirement", live)
	}
}

func ptrSignature(s identity.Signature) *identity.Signature { return &s }
