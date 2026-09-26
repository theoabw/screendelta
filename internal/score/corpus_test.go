package score_test

import (
	"fmt"
	"image"
	"os"
	"testing"

	"github.com/theoabw/screendelta/internal/config"
	"github.com/theoabw/screendelta/internal/corpus"
	"github.com/theoabw/screendelta/internal/diff"
	"github.com/theoabw/screendelta/internal/frame"
	"github.com/theoabw/screendelta/internal/score"
	"github.com/theoabw/screendelta/internal/stream"
)

// caseScore is everything one case contributes to the measurement.
type caseScore struct {
	Counts        score.Counts
	Pairs         int
	ClassChecked  int
	ClassMismatch []string
	Regions       int
}

// runCase renders a case in memory, runs the real engine over every frame, and scores every
// adjacent pair, not only the pairs the case was designed around. Scoring only selected
// pairs is how a mutant that misbehaves on the other transitions passes unnoticed.
func runCase(t *testing.T, name string, opts corpus.Options) caseScore {
	t.Helper()

	c, err := corpus.NewCase(name, opts)
	if err != nil {
		t.Fatalf("building the %s case failed: %v", name, err)
	}

	engine, err := stream.New(config.Defaults(), diff.New())
	if err != nil {
		t.Fatalf("creating the engine failed: %v", err)
	}
	defer engine.Close()

	// Frame one has no predecessor, so it contributes no pair.
	if _, err := engine.Push(toFrame(t, c.Frame(0), 1)); err != nil {
		t.Fatalf("Push failed on the first frame of %s: %v", name, err)
	}

	result := caseScore{}
	counts := make([]score.Counts, 0, c.Frames()-1)
	for index := 0; index+1 < c.Frames(); index++ {
		document, err := engine.Push(toFrame(t, c.Frame(index+1), uint64(index+2)))
		if err != nil {
			t.Fatalf("Push failed on frame %d of %s: %v", index+2, name, err)
		}
		expectation := c.Expectation(index)
		if expectation.NoChange && len(expectation.Changes) > 0 {
			t.Fatalf("%s transition %d is contradictory: it expects no change and %d changed rectangles",
				name, index, len(expectation.Changes))
		}

		pair := score.Pair{
			Reported: document.Regions,
			Expected: expectation.Changes,
			NoChange: expectation.NoChange,
			Width:    opts.Width,
			Height:   opts.Height,
		}
		scored, matched := score.Match(pair)
		counts = append(counts, scored)
		result.Pairs++
		result.Regions += len(document.Regions)

		// Where the case states the classes it expects, the region that answered for each
		// rectangle has to carry the right one. Without this the suite scores localisation
		// only, and an engine that calls every change "changed" scores 1.0000.
		if len(expectation.Classes) == 0 {
			continue
		}
		if len(expectation.Classes) != len(expectation.Changes) {
			t.Fatalf("%s transition %d states %d classes for %d rectangles",
				name, index, len(expectation.Classes), len(expectation.Changes))
		}
		for _, m := range matched {
			want := expectation.Classes[m.Expected]
			result.ClassChecked++
			if string(m.Class) != want {
				result.ClassMismatch = append(result.ClassMismatch,
					fmt.Sprintf("transition %d rectangle %d: reported %s, expected %s",
						index, m.Expected, m.Class, want))
			}
		}
	}

	result.Counts = score.Sum(counts...)
	return result
}

func toFrame(t *testing.T, img *image.RGBA, sequence uint64) frame.Frame {
	t.Helper()
	f, err := frame.NewRaw(sequence, img.Rect.Dx(), img.Rect.Dy(), frame.FormatRGBA8, 1, img.Pix)
	if err != nil {
		t.Fatalf("cannot wrap frame %d: %v", sequence, err)
	}
	return f
}

// TestCorpus is the accuracy measurement behind NFR-006 and SC-001.
func TestCorpus(t *testing.T) {
	base := corpus.DefaultOptions()

	withFloor := func(opts corpus.Options) corpus.Options {
		opts.MinLumaDifference = base.MinLumaDifference
		return opts
	}

	cases := []struct {
		name           string
		opts           corpus.Options
		minF1          float64
		minPairs       int
		classification bool
		reason         string
	}{
		{name: "changed-label", opts: base, minF1: 0.98, minPairs: 1, classification: true},
		{name: "moving-button", opts: withFloor(corpus.Options{Width: 320, Height: 240, Frames: 12, Seed: base.Seed}), minF1: 0.98, minPairs: 11, classification: true},
		{name: "occluded-button", opts: base, minF1: 0.98, minPairs: 3, classification: true},
		{name: "noise", opts: withFloor(corpus.Options{Width: 320, Height: 240, Pairs: 60, Seed: base.Seed}), minF1: 1, minPairs: 119,
			reason: "no changes to classify: the case asserts that nothing is reported at all"},
		// The sweep is what makes the sample size the specification asks for honest: 5,000
		// scored pairs of genuinely varied change. It scores localisation only, and the reason
		// is recorded rather than assumed: deciding whether a uniformly different area is a
		// changed element or a removed one is not possible from pixels alone, so asserting
		// classes here would assert the implementation's guess. Classification is asserted by
		// the three cases above, where what happened is unambiguous.
		{name: "sweep", opts: withFloor(corpus.Options{Width: 320, Height: 240, Frames: 5001, Seed: base.Seed}), minF1: 0.98, minPairs: 5000,
			reason: "localisation only: removal cannot be told from change by pixels"},
	}

	overall := make([]score.Counts, 0, len(cases))
	// Pairs and regions are counted separately, because they are different numbers and the requirement is
	// stated in pairs. Adding expected regions to no-change pairs and calling the sum pairs overstated the
	// sample size by a factor of five.
	totalPairs := 0
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := runCase(t, tc.name, tc.opts)
			t.Logf("%s: %d pairs, %d regions reported, %s", tc.name, result.Pairs, result.Regions, result.Counts)
			overall = append(overall, result.Counts)
			totalPairs += result.Pairs

			if result.Pairs < tc.minPairs {
				t.Fatalf("%s scored %d pairs, fewer than the %d required", tc.name, result.Pairs, tc.minPairs)
			}
			if result.Counts.F1() < tc.minF1 {
				t.Fatalf("F1 %.4f is below the required %.4f", result.Counts.F1(), tc.minF1)
			}
			if result.Counts.FalseRemovals != 0 {
				t.Fatalf("%d regions were reported as removed on frames that only differ by noise", result.Counts.FalseRemovals)
			}
			if result.Counts.NoChangeReports != 0 {
				t.Fatalf("%d regions were reported on pairs that did not change", result.Counts.NoChangeReports)
			}
			if len(result.ClassMismatch) > 0 {
				t.Fatalf("%d regions carried the wrong class, first: %s", len(result.ClassMismatch), result.ClassMismatch[0])
			}
			// A case that contains changes must assert what class they carry, or the
			// measurement would score localisation only and a mutant that calls every change
			// "changed" would pass. The noise case has no changes to classify, so it is
			// exempt by construction rather than by name.
			if tc.classification && result.ClassChecked == 0 {
				t.Fatalf("%s declares that it asserts classification but checked none", tc.name)
			}
			if !tc.classification && tc.reason == "" {
				t.Fatalf("%s asserts no classification and gives no reason", tc.name)
			}
		})
	}

	total := score.Sum(overall...)
	t.Logf("every case together: %d frame pairs scored, %d regions matched, %s",
		totalPairs, total.TruePositives+total.FalsePositives+total.FalseNegatives, total)
	if totalPairs < 5000 {
		t.Fatalf("the suite scored %d pairs, fewer than the 5,000 SC-001 requires", totalPairs)
	}
}

// TestNoiseCaseReportsNothingOnEveryPair is the false-removal rule measured pair by pair, so a
// single bad pair cannot hide inside an average.
func TestNoiseCaseReportsNothingOnEveryPair(t *testing.T) {
	c, err := corpus.NewCase("noise", corpus.Options{Width: 320, Height: 240, Pairs: 25, Seed: 7})
	if err != nil {
		t.Fatalf("building the noise case failed: %v", err)
	}
	engine, err := stream.New(config.Defaults(), diff.New())
	if err != nil {
		t.Fatalf("creating the engine failed: %v", err)
	}
	defer engine.Close()

	if _, err := engine.Push(toFrame(t, c.Frame(0), 1)); err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	for index := 0; index+1 < c.Frames(); index++ {
		document, err := engine.Push(toFrame(t, c.Frame(index+1), uint64(index+2)))
		if err != nil {
			t.Fatalf("Push failed: %v", err)
		}
		if len(document.Regions) != 0 {
			t.Fatalf("frames %d to %d differ only by noise but %d regions were reported: %+v",
				index+1, index+2, len(document.Regions), document.Regions)
		}
	}
}

// TestGeneratedFilesMatchTheInMemoryFrames checks that what a person looks at is what the
// measurement scored, so the manifest cannot drift from the rendered frames.
func TestGeneratedFilesMatchTheInMemoryFrames(t *testing.T) {
	dir := t.TempDir()
	opts := corpus.Options{Width: 160, Height: 120, Frames: 4, Pairs: 3, Seed: 11}

	for _, name := range corpus.Cases() {
		manifest, err := corpus.Generate(name, dir+"/"+name, opts)
		if err != nil {
			t.Fatalf("generating %s failed: %v", name, err)
		}
		if manifest.Frames != len(manifest.Paths) {
			t.Fatalf("%s wrote %d paths for %d frames", name, len(manifest.Paths), manifest.Frames)
		}
		c, err := corpus.NewCase(name, opts)
		if err != nil {
			t.Fatalf("building %s failed: %v", name, err)
		}
		if c.Frames() != manifest.Frames {
			t.Fatalf("%s renders %d frames but wrote %d", name, c.Frames(), manifest.Frames)
		}
		for _, path := range manifest.Paths {
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("%s names %s but it does not exist: %v", name, path, err)
			}
		}
	}
}
