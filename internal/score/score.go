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

// Match scores one pair and returns the counts.
func Match(pair Pair) Counts {
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

	usedReported := make([]bool, len(pair.Reported))
	for _, expected := range pair.Expected {
		expectedRect := rectOf(expected, pair.Width, pair.Height)
		best, bestOverlap := -1, 0.0
		for index, region := range pair.Reported {
			if usedReported[index] {
				continue
			}
			overlap := intersectionOverUnion(pixelRect(region.Bounds, pair.Width, pair.Height), expectedRect)
			if overlap > bestOverlap {
				bestOverlap = overlap
				best = index
			}
		}
		if best >= 0 && bestOverlap >= threshold {
			usedReported[best] = true
			counts.TruePositives++
			continue
		}
		counts.FalseNegatives++
	}

	for index := range pair.Reported {
		if !usedReported[index] {
			counts.FalsePositives++
		}
	}
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

func pixelRect(bounds delta.Bounds, width, height int) image.Rectangle {
	left := int(bounds.X * float64(width))
	top := int(bounds.Y * float64(height))
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
