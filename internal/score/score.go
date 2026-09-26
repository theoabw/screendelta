// Package score measures how well the engine's regions match ground truth.
//
// The measure is deliberately the plain one: a reported region counts as a detection when
// it overlaps an expected rectangle by at least the threshold, each expected rectangle can
// be detected once, and everything left over is counted against the engine. That is the
// intersection-over-union convention the specification names, and it is computed here
// rather than inside the engine so the thing being measured is not also the thing doing
// the measuring.
package score

import (
	"fmt"
	"image"

	"github.com/theoabw/screendelta/internal/corpus"
	"github.com/theoabw/screendelta/internal/delta"
)

// DefaultIOUThreshold is the overlap at which a reported region counts as a detection.
const DefaultIOUThreshold = 0.5

// Counts is the outcome of scoring one or more frame pairs.
type Counts struct {
	Reported        int
	Expected        int
	TruePositives   int
	FalsePositives  int
	FalseNegatives  int
	FalseRemovals   int
	NoChangePairs   int
	NoChangeReports int
}

// Precision is the share of reported regions that were expected.
func (c Counts) Precision() float64 {
	if c.Reported == 0 {
		return 1
	}
	return float64(c.TruePositives) / float64(c.Reported)
}

// Recall is the share of expected changes that were reported.
func (c Counts) Recall() float64 {
	if c.Expected == 0 {
		return 1
	}
	return float64(c.TruePositives) / float64(c.Expected)
}

// F1 is the harmonic mean of precision and recall.
func (c Counts) F1() float64 {
	precision, recall := c.Precision(), c.Recall()
	if precision+recall == 0 {
		return 0
	}
	return 2 * precision * recall / (precision + recall)
}

func (c Counts) String() string {
	return fmt.Sprintf("precision %.4f recall %.4f F1 %.4f (tp %d fp %d fn %d, false removals %d, reports on no-change pairs %d)",
		c.Precision(), c.Recall(), c.F1(), c.TruePositives, c.FalsePositives, c.FalseNegatives,
		c.FalseRemovals, c.NoChangeReports)
}

// Pair is what to score for one frame pair.
type Pair struct {
	Reported  []delta.Region
	Expected  []corpus.Region
	NoChange  bool
	Width     int
	Height    int
	Threshold float64
}

// Matched records one expected rectangle and the region that answered for it, so a case can
// also assert what the engine said the change was.
type Matched struct {
	Expected   int
	Reported   int
	Class      delta.RegionClass
	ReportedOK bool
}

// Match scores one pair and returns the counts together with the matching itself.
//
// The matching maximises the number of expected rectangles that get an answer, using
// augmenting paths rather than taking the first acceptable region in order. Greedy matching
// would let the order of two overlapping regions decide the score, which would make the
// measurement depend on something the requirement does not mention.
func Match(pair Pair) (Counts, []Matched) {
	threshold := pair.Threshold
	if threshold <= 0 {
		threshold = DefaultIOUThreshold
	}

	counts := Counts{Reported: len(pair.Reported), Expected: len(pair.Expected)}
	if pair.NoChange {
		counts.NoChangePairs = 1
		counts.NoChangeReports = len(pair.Reported)
		for _, region := range pair.Reported {
			if region.Class == delta.ClassRemoved {
				counts.FalseRemovals++
			}
		}
	}

	reportedRects := make([]image.Rectangle, len(pair.Reported))
	for index, region := range pair.Reported {
		reportedRects[index] = pixelRect(region.Bounds, pair.Width, pair.Height)
	}

	// Overlap above the threshold is the only edge allowed in the graph.
	allowed := make([][]bool, len(pair.Expected))
	for i, expected := range pair.Expected {
		expectedRect := rectOf(expected, pair.Width, pair.Height)
		allowed[i] = make([]bool, len(pair.Reported))
		for j, reported := range reportedRects {
			allowed[i][j] = intersectionOverUnion(reported, expectedRect) >= threshold
		}
	}

	assignedExpected := make([]int, len(pair.Reported)) // region -> expected, or -1
	for index := range assignedExpected {
		assignedExpected[index] = -1
	}
	assignedReported := make([]int, len(pair.Expected)) // expected -> region, or -1
	for index := range assignedReported {
		assignedReported[index] = -1
	}

	var augment func(expected int, seen []bool) bool
	augment = func(expected int, seen []bool) bool {
		for region := range pair.Reported {
			if !allowed[expected][region] || seen[region] {
				continue
			}
			seen[region] = true
			if assignedExpected[region] == -1 || augment(assignedExpected[region], seen) {
				assignedExpected[region] = expected
				assignedReported[expected] = region
				return true
			}
		}
		return false
	}

	for expected := range pair.Expected {
		augment(expected, make([]bool, len(pair.Reported)))
	}

	matched := make([]Matched, 0, len(pair.Expected))
	for expected, region := range assignedReported {
		if region < 0 {
			counts.FalseNegatives++
			continue
		}
		counts.TruePositives++
		matched = append(matched, Matched{
			Expected:   expected,
			Reported:   region,
			Class:      pair.Reported[region].Class,
			ReportedOK: true,
		})
	}
	counts.FalsePositives = counts.Reported - counts.TruePositives
	return counts, matched
}

// Score is Match for callers that only want the numbers.
func Score(pair Pair) Counts {
	counts, _ := Match(pair)
	return counts
}

// Sum adds counts together, which is how a case is scored across its pairs.
func Sum(all ...Counts) Counts {
	total := Counts{}
	for _, counts := range all {
		total.Reported += counts.Reported
		total.Expected += counts.Expected
		total.TruePositives += counts.TruePositives
		total.FalsePositives += counts.FalsePositives
		total.FalseNegatives += counts.FalseNegatives
		total.FalseRemovals += counts.FalseRemovals
		total.NoChangePairs += counts.NoChangePairs
		total.NoChangeReports += counts.NoChangeReports
	}
	return total
}

func rectOf(r corpus.Region, width, height int) image.Rectangle {
	left, top, right, bottom := r.Rect()
	if right > width {
		right = width
	}
	if bottom > height {
		bottom = height
	}
	return image.Rect(left, top, right, bottom)
}

// pixelRect converts normalized bounds back to pixels using the same rounding in both
// directions, so a rectangle that the engine derived from whole pixels converts back to
// exactly those pixels rather than one pixel short.
func pixelRect(bounds delta.Bounds, width, height int) image.Rectangle {
	left := int(bounds.X*float64(width) + 0.5)
	top := int(bounds.Y*float64(height) + 0.5)
	return image.Rect(left, top,
		left+int(bounds.W*float64(width)+0.5),
		top+int(bounds.H*float64(height)+0.5))
}

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
