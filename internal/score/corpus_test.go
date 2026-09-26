package score_test

import (
	"os"
	"testing"

	"github.com/theoabw/screendelta/internal/config"
	"github.com/theoabw/screendelta/internal/corpus"
	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/diff"
	"github.com/theoabw/screendelta/internal/frame"
	"github.com/theoabw/screendelta/internal/score"
	"github.com/theoabw/screendelta/internal/stream"
)

// runCase generates a case, runs the real engine over its frames, and returns the
// documents in frame order together with the manifest.
func runCase(t *testing.T, name string, opts corpus.Options) (corpus.Manifest, []delta.Document) {
	t.Helper()

	dir := t.TempDir()
	manifest, err := corpus.Generate(name, dir, opts)
	if err != nil {
		t.Fatalf("generating %s failed: %v", name, err)
	}

	engine, err := stream.New(config.Defaults(), diff.New())
	if err != nil {
		t.Fatalf("creating the engine failed: %v", err)
	}
	defer engine.Close()

	documents := make([]delta.Document, 0, len(manifest.Paths))
	for index, path := range manifest.Paths {
		file, err := os.Open(path)
		if err != nil {
			t.Fatalf("cannot open %s: %v", path, err)
		}
		f, err := frame.DecodePNG(uint64(index+1), manifest.ScaleFactor, file)
		file.Close()
		if err != nil {
			t.Fatalf("cannot decode %s: %v", path, err)
		}
		document, err := engine.Push(f)
		if err != nil {
			t.Fatalf("Push failed on %s: %v", path, err)
		}
		documents = append(documents, document)
	}
	return manifest, documents
}

// scoreCase scores every expectation of a case against the document that describes the
// later frame of the pair.
func scoreCase(t *testing.T, manifest corpus.Manifest, documents []delta.Document) score.Counts {
	t.Helper()

	counts := make([]score.Counts, 0, len(manifest.Expectations))
	for _, expectation := range manifest.Expectations {
		if expectation.To < 1 || expectation.To > len(documents) {
			t.Fatalf("expectation names frame %d, but the case produced %d documents", expectation.To, len(documents))
		}
		counts = append(counts, score.Match(score.Pair{
			Reported: documents[expectation.To-1].Regions,
			Expected: expectation.Changes,
			NoChange: expectation.NoChange,
			Width:    manifest.Width,
			Height:   manifest.Height,
		}))
	}
	return score.Sum(counts...)
}

// TestCorpus is the accuracy measurement the specification states a target for: region
// detection F1 at or above 0.98, and zero false removals on frames that only differ by
// capture noise.
func TestCorpus(t *testing.T) {
	opts := corpus.DefaultOptions()

	cases := []struct {
		name           string
		minF1          float64
		opts           corpus.Options
		expectNoChange bool
	}{
		{name: "changed-label", minF1: 0.98, opts: opts},
		{name: "moving-button", minF1: 0.98, opts: opts},
		{name: "occluded-button", minF1: 0.98, opts: opts},
		{name: "noise", minF1: 1, opts: corpus.Options{Width: opts.Width, Height: opts.Height, Pairs: 20, Seed: opts.Seed}, expectNoChange: true},
	}

	var overall []score.Counts
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifest, documents := runCase(t, tc.name, tc.opts)
			counts := scoreCase(t, manifest, documents)
			t.Logf("%s: %s", tc.name, counts)
			overall = append(overall, counts)

			if counts.F1() < tc.minF1 {
				t.Fatalf("F1 %.4f is below the required %.4f", counts.F1(), tc.minF1)
			}
			if counts.FalseRemovals != 0 {
				t.Fatalf("%d regions were reported as removed on frames that only differ by noise", counts.FalseRemovals)
			}
			if tc.expectNoChange && counts.NoChangeReports != 0 {
				t.Fatalf("%d regions were reported on pairs that did not change", counts.NoChangeReports)
			}
		})
	}

	t.Logf("all cases: %s", score.Sum(overall...))
}

// TestNoiseCaseReportsNothingOnEveryPair is the false-removal rule measured on its own,
// pair by pair, so a single bad pair cannot hide inside an average.
func TestNoiseCaseReportsNothingOnEveryPair(t *testing.T) {
	manifest, documents := runCase(t, "noise", corpus.Options{Width: 320, Height: 240, Pairs: 25, Seed: 7})

	for _, expectation := range manifest.Expectations {
		regions := documents[expectation.To-1].Regions
		if len(regions) != 0 {
			t.Fatalf("frames %d to %d differ only by noise but %d regions were reported: %+v",
				expectation.From, expectation.To, len(regions), regions)
		}
	}
}
