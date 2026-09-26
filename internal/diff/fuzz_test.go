package diff

import (
	"testing"

	"github.com/theoabw/screendelta/internal/config"
	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/frame"
)

// FuzzCompare asserts the invariant that matters more than any individual rule: whatever the comparison
// produces from arbitrary pixels, the document built from it passes the validator the contract publishes.
//
// A unit test checks the cases someone thought of. This checks the cases nobody thought of, and the property
// it checks is the one a consumer depends on: the engine never reports a region that its own schema rejects,
// a bound outside the frame, a magnitude off its scale, an area smaller than the configured floor, or two
// regions in an order the contract does not allow.
func FuzzCompare(f *testing.F) {
	// Seeds: a blank pair, a pair differing in one pixel, a pair differing along an edge, and a pair whose
	// bytes differ everywhere.
	f.Add([]byte{0x00, 0x00, 0x00, 0xff}, 4, 4, uint8(0))
	f.Add([]byte{0xff, 0xff, 0xff, 0xff}, 8, 8, uint8(1))
	f.Add([]byte{0x40, 0x80, 0xc0, 0xff}, 16, 9, uint8(2))

	f.Fuzz(func(t *testing.T, data []byte, width, height int, variant uint8) {
		// Keep the frames small enough that the fuzzer explores the rules rather than the allocator, and
		// refuse the degenerate geometry the API rejects anyway.
		if width < 1 || height < 1 || width > 64 || height > 64 {
			t.Skip()
		}
		if len(data) == 0 {
			t.Skip()
		}

		pixels := func(seed uint8) []byte {
			buffer := make([]byte, width*height*4)
			for index := range buffer {
				buffer[index] = data[(index+int(seed))%len(data)]
			}
			return buffer
		}

		previous, err := frame.NewRaw(1, width, height, frame.FormatRGBA8, 1, pixels(0))
		if err != nil {
			t.Skip()
		}
		current, err := frame.NewRaw(2, width, height, frame.FormatRGBA8, 1, pixels(variant))
		if err != nil {
			t.Skip()
		}

		differ := New()
		regions, conditions, err := differ.Compare(previous, current, config.Defaults())
		if err != nil {
			return
		}

		// The comparison returns regions in the order it discovered them; the document layer sorts them. What
		// has to hold is that the sorted result does not depend on that discovery order, which is the property
		// the contract's "total order" claim exists to provide and which a three key comparison did not
		// provide: the fuzz target found two regions sharing a position and an identity and the order came out
		// differently depending on which was seen first.
		forward := append([]delta.Region(nil), regions...)
		backward := append([]delta.Region(nil), regions...)
		for left, right := 0, len(backward)-1; left < right; left, right = left+1, right-1 {
			backward[left], backward[right] = backward[right], backward[left]
		}
		delta.SortRegions(forward)
		delta.SortRegions(backward)
		if !sameOrder(forward, backward) {
			t.Fatalf("the region order depends on the order they were discovered in:\nforward:  %+v\nbackward: %+v", forward, backward)
		}

		// The contract's own rules, checked through the validator the encoder uses.
		document := delta.Document{
			SchemaVersion: delta.SchemaVersion,
			Frame:         delta.FrameRef{Sequence: 2, Width: width, Height: height, ScaleFactor: 1},
			Fingerprint:   delta.Fingerprint{Algorithm: "grid-luma-1", GridSize: 8, Cells: make([]int, 64), StrictHash: "0123456789abcdef"},
			Regions:       forward,
			Conditions:    conditions,
		}
		if err := document.Validate(); err != nil {
			t.Fatalf("the comparison produced a document the contract rejects: %v\nframe %dx%d", err, width, height)
		}

		for index, region := range regions {
			if region.Identity == 0 {
				t.Fatalf("region %d carries identity zero", index)
			}
			// The schema's class rules, which are narrower than "everything that is not added needs previous
			// bounds": a changed region is left unconstrained, because a change may or may not have a
			// previous position to name. The first version of this target asserted the wider rule and failed
			// on its own seed, which is the fuzzer doing its job on the test rather than on the engine.
			if region.Class == delta.ClassAdded && region.PreviousBounds != nil {
				t.Fatalf("region %d is added and carries previous bounds", index)
			}
			if (region.Class == delta.ClassMoved || region.Class == delta.ClassRemoved) && region.PreviousBounds == nil {
				t.Fatalf("region %d is %s and carries no previous bounds", index, region.Class)
			}
		}
		if !delta.Ordered(forward) {
			t.Fatalf("the sorted regions are not in the contract's order: %+v", forward)
		}
	})
}

// sameOrder compares two region sequences by the contract's key, so the check does not depend on the
// predicate the engine uses to sort.
func sameOrder(a, b []delta.Region) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index].Bounds != b[index].Bounds || a[index].Identity != b[index].Identity || a[index].Class != b[index].Class {
			return false
		}
	}
	return true
}
