// Package fingerprint compares a frame's fingerprint against one stored earlier.
//
// Computing a fingerprint belongs to the comparison that reads a frame, so that a frame is walked
// once for both the delta and the fingerprint. Deciding whether two fingerprints describe the same
// screen is a separate question with its own rules, and it is here. The split matters because the two
// halves answer to different requirements: FR-008 is about how a fingerprint is built and FR-009 is
// about what a consumer may conclude from two of them.
//
// Two answers are reported rather than one, because a consumer needs different things from them. The
// tolerant answer is what a plan cache wants: is this screen the same screen, allowing for capture
// noise. The strict answer is what an exact cache key wants: are these the same bytes of summary. A
// caller that needs a key uses the hash, and a caller that needs a decision uses the comparison, and
// neither has to guess which one the engine meant.
package fingerprint

import (
	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/fielderr"
)

// Result is what comparing two fingerprints concluded.
type Result struct {
	// Equal is the tolerant answer: no cell differs by more than the allowed delta.
	Equal bool
	// StrictEqual is the exact answer: the two hashes are identical.
	StrictEqual bool
	// DifferingCells counts the cells that differ by more than the allowed delta.
	DifferingCells int
	// LargestDelta is the largest single cell difference found, whatever the allowance.
	LargestDelta int
}

// Explain returns a short human readable reason, for a report or a log line.
func (r Result) Explain() string {
	switch {
	case r.Equal && r.StrictEqual:
		return "same screen, same summary"
	case r.Equal:
		return "same screen within the tolerance, different summary"
	case r.DifferingCells == 1:
		return "one cell differs by more than the allowance"
	default:
		return "the grids differ"
	}
}

// Compare decides whether two fingerprints describe the same screen, allowing each cell to differ by
// at most the configured delta.
//
// The allowance is per cell rather than in total, which is the decision recorded in research.md: a
// total budget would let one large difference hide behind many equal cells, and capture noise is a
// per-cell phenomenon.
func Compare(stored, current delta.Fingerprint, maxCellDelta int) (Result, error) {
	if err := comparable(stored, current); err != nil {
		return Result{}, err
	}
	if maxCellDelta < 0 {
		return Result{}, &fielderr.Error{
			Op:      "fingerprint.Compare",
			Subject: "config",
			Field:   "fingerprint.maxCellDelta",
			Problem: "must not be negative",
		}
	}

	result := Result{StrictEqual: stored.StrictHash == current.StrictHash}
	for index := range stored.Cells {
		difference := stored.Cells[index] - current.Cells[index]
		if difference < 0 {
			difference = -difference
		}
		if difference > result.LargestDelta {
			result.LargestDelta = difference
		}
		if difference > maxCellDelta {
			result.DifferingCells++
		}
	}
	result.Equal = result.DifferingCells == 0
	return result, nil
}

// StrictEqual reports whether two fingerprints are identical summaries, which is what a cache key
// needs. A hash collision is not a concern here because the hash is over the whole grid.
func StrictEqual(stored, current delta.Fingerprint) bool {
	if stored.Algorithm != current.Algorithm || stored.GridSize != current.GridSize {
		return false
	}
	return stored.StrictHash == current.StrictHash
}

// comparable rejects two fingerprints that cannot be compared at all.
//
// A fingerprint taken with another algorithm or another grid size describes the screen in a way this
// comparison does not understand, and reporting "different" for it would be a lie: the two summaries
// are not measurements of the same thing. That is a contract problem for the caller rather than a
// difference between screens.
func comparable(stored, current delta.Fingerprint) error {
	if stored.Algorithm != current.Algorithm {
		return &fielderr.Error{
			Op:      "fingerprint.Compare",
			Subject: "fingerprint",
			Field:   "algorithm",
			Problem: "cannot compare " + quote(stored.Algorithm) + " with " + quote(current.Algorithm),
		}
	}
	if stored.GridSize != current.GridSize {
		return &fielderr.Error{
			Op:      "fingerprint.Compare",
			Subject: "fingerprint",
			Field:   "gridSize",
			Problem: "cannot compare a grid of " + itoa(stored.GridSize) + " with one of " + itoa(current.GridSize),
		}
	}
	if len(stored.Cells) != stored.GridSize*stored.GridSize {
		return &fielderr.Error{
			Op:      "fingerprint.Compare",
			Subject: "fingerprint",
			Field:   "cells",
			Problem: "the stored fingerprint has " + itoa(len(stored.Cells)) + " cells for a grid of " + itoa(stored.GridSize),
		}
	}
	if len(current.Cells) != current.GridSize*current.GridSize {
		return &fielderr.Error{
			Op:      "fingerprint.Compare",
			Subject: "fingerprint",
			Field:   "cells",
			Problem: "the current fingerprint has " + itoa(len(current.Cells)) + " cells for a grid of " + itoa(current.GridSize),
		}
	}
	return nil
}

func quote(value string) string { return "\"" + value + "\"" }

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	digits := make([]byte, 0, 12)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	if negative {
		digits = append(digits, '-')
	}
	for left, right := 0, len(digits)-1; left < right; left, right = left+1, right-1 {
		digits[left], digits[right] = digits[right], digits[left]
	}
	return string(digits)
}
