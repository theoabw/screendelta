package fingerprint_test

import (
	"testing"

	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/fingerprint"
)

// grid builds a fingerprint whose cells are all the same value, which is enough to test the
// comparison without involving the engine that produces them.
func grid(size int, value int, hash string) delta.Fingerprint {
	cells := make([]int, size*size)
	for index := range cells {
		cells[index] = value
	}
	return delta.Fingerprint{Algorithm: "grid-luma-1", GridSize: size, Cells: cells, StrictHash: hash}
}

func TestIdenticalFingerprintsAreEqualAndStrictlyEqual(t *testing.T) {
	stored := grid(8, 100, "0123456789abcdef")
	current := grid(8, 100, "0123456789abcdef")

	result, err := fingerprint.Compare(stored, current, 2)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if !result.Equal || !result.StrictEqual {
		t.Fatalf("identical fingerprints reported %+v", result)
	}
	if result.DifferingCells != 0 || result.LargestDelta != 0 {
		t.Fatalf("identical fingerprints reported a difference: %+v", result)
	}
}

// TestNoiseWithinTheAllowanceIsEqual is the case the tolerance exists for: a screen captured twice
// differs by a little everywhere and is still the same screen.
func TestNoiseWithinTheAllowanceIsEqual(t *testing.T) {
	stored := grid(8, 100, "aaaaaaaaaaaaaaaa")
	current := grid(8, 100, "aaaaaaaaaaaaaaaa")
	for index := range current.Cells {
		current.Cells[index] += (index % 5) - 2 // between minus two and plus two
	}
	current.StrictHash = "bbbbbbbbbbbbbbbb"

	result, err := fingerprint.Compare(stored, current, 2)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if !result.Equal {
		t.Fatalf("noise inside the allowance was reported as a different screen: %+v", result)
	}
	if result.StrictEqual {
		t.Fatal("fingerprints with different hashes were reported as strictly equal")
	}
	if result.LargestDelta != 2 {
		t.Fatalf("largest delta is %d, want 2", result.LargestDelta)
	}
	if result.Explain() == "" {
		t.Fatal("Explain returned nothing")
	}
}

func TestADifferenceBeyondTheAllowanceIsNotEqual(t *testing.T) {
	stored := grid(8, 100, "aaaaaaaaaaaaaaaa")
	current := grid(8, 100, "aaaaaaaaaaaaaaaa")
	current.Cells[0] += 3 // one level past an allowance of two

	result, err := fingerprint.Compare(stored, current, 2)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if result.Equal {
		t.Fatalf("a difference beyond the allowance was reported as equal: %+v", result)
	}
	if result.DifferingCells != 1 {
		t.Fatalf("differing cells is %d, want 1", result.DifferingCells)
	}
	if result.LargestDelta != 3 {
		t.Fatalf("largest delta is %d, want 3", result.LargestDelta)
	}
}

func TestTheAllowanceBoundaryIsInclusive(t *testing.T) {
	stored := grid(4, 50, "aaaaaaaaaaaaaaaa")

	atBoundary := grid(4, 50, "aaaaaaaaaaaaaaaa")
	atBoundary.Cells[3] += 5
	if result, err := fingerprint.Compare(stored, atBoundary, 5); err != nil {
		t.Fatalf("Compare failed: %v", err)
	} else if !result.Equal {
		t.Fatalf("a difference exactly at the allowance was reported as different: %+v", result)
	}

	beyond := grid(4, 50, "aaaaaaaaaaaaaaaa")
	beyond.Cells[3] += 6
	if result, err := fingerprint.Compare(stored, beyond, 5); err != nil {
		t.Fatalf("Compare failed: %v", err)
	} else if result.Equal {
		t.Fatalf("a difference one level past the allowance was reported as equal: %+v", result)
	}
}

// TestAMateriallyChangedScreenIsDifferent is the other half: the tolerance must not swallow a real
// change. A screen where an element moved has cells that differ by much more than noise.
func TestAMateriallyChangedScreenIsDifferent(t *testing.T) {
	stored := grid(8, 40, "aaaaaaaaaaaaaaaa")
	current := grid(8, 40, "cccccccccccccccc")
	for index := 0; index < 8; index++ {
		current.Cells[index] = 200 // one row of cells turned bright
	}

	result, err := fingerprint.Compare(stored, current, 2)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if result.Equal {
		t.Fatalf("a screen with a bright row was reported as the same screen: %+v", result)
	}
	if result.DifferingCells != 8 {
		t.Fatalf("differing cells is %d, want 8", result.DifferingCells)
	}
}

func TestFingerprintsOfDifferentShapesCannotBeCompared(t *testing.T) {
	cases := []struct {
		name    string
		stored  delta.Fingerprint
		current delta.Fingerprint
		field   string
	}{
		{name: "another algorithm", stored: grid(8, 10, "aa"), current: func() delta.Fingerprint {
			f := grid(8, 10, "aa")
			f.Algorithm = "grid-luma-2"
			return f
		}(), field: "fingerprint.algorithm"},
		{name: "another grid size", stored: grid(8, 10, "aa"), current: grid(16, 10, "aa"), field: "fingerprint.gridSize"},
		{name: "a truncated grid", stored: func() delta.Fingerprint {
			f := grid(8, 10, "aa")
			f.Cells = f.Cells[:10]
			return f
		}(), current: grid(8, 10, "aa"), field: "fingerprint.cells"},
		{name: "a truncated current grid", stored: grid(8, 10, "aa"), current: func() delta.Fingerprint {
			f := grid(8, 10, "aa")
			f.Cells = f.Cells[:10]
			return f
		}(), field: "fingerprint.cells"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := fingerprint.Compare(tc.stored, tc.current, 2); err == nil {
				t.Fatal("two fingerprints that are not measurements of the same thing were compared")
			}
		})
	}
}

func TestANegativeAllowanceIsRejected(t *testing.T) {
	if _, err := fingerprint.Compare(grid(4, 10, "aa"), grid(4, 10, "aa"), -1); err == nil {
		t.Fatal("a negative allowance was accepted")
	}
}

func TestStrictEqualIgnoresTheToleranceAndTheShape(t *testing.T) {
	stored := grid(8, 100, "aaaaaaaaaaaaaaaa")
	same := grid(8, 100, "aaaaaaaaaaaaaaaa")
	if !fingerprint.StrictEqual(stored, same) {
		t.Fatal("identical fingerprints are not strictly equal")
	}

	noisy := grid(8, 100, "aaaaaaaaaaaaaaaa")
	noisy.Cells[0]++
	noisy.StrictHash = "bbbbbbbbbbbbbbbb"
	if fingerprint.StrictEqual(stored, noisy) {
		t.Fatal("a different summary was reported as strictly equal")
	}

	other := grid(16, 100, "aaaaaaaaaaaaaaaa")
	if fingerprint.StrictEqual(stored, other) {
		t.Fatal("fingerprints of different grid sizes were reported as strictly equal")
	}
}
