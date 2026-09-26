// Command corpusgen writes frame sequences together with the ground truth that describes
// them. The generation itself lives in internal/corpus so that the scoring harness can
// drive it without shelling out; this command is the operator's way in.
//
// Usage:
//
//	corpusgen --case changed-label --out /tmp/frames
//	corpusgen --case noise --pairs 50 --out /tmp/noise
//
// Each case writes 001.png, 002.png and so on, plus expected.json describing the pixels
// that changed between consecutive frames. Only the generator, its seed and the resulting
// metrics belong in the repository, so the images are written where the caller asks.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/theoabw/screendelta/internal/corpus"
)

func main() {
	defaults := corpus.DefaultOptions()
	var (
		caseName = flag.String("case", "changed-label", "case to generate: "+strings.Join(corpus.Cases(), ", "))
		outDir   = flag.String("out", "corpus", "directory to write frames and expected.json into")
		width    = flag.Int("width", defaults.Width, "frame width in pixels")
		height   = flag.Int("height", defaults.Height, "frame height in pixels")
		frames   = flag.Int("frames", defaults.Frames, "number of frames for sequence cases")
		pairs    = flag.Int("pairs", defaults.Pairs, "number of pairs for the noise case")
		seed     = flag.Int64("seed", defaults.Seed, "random seed, recorded in the manifest")
	)
	flag.Parse()

	manifest, err := corpus.Generate(*caseName, *outDir, corpus.Options{
		Width:  *width,
		Height: *height,
		Frames: *frames,
		Pairs:  *pairs,
		Seed:   *seed,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "corpusgen: %v\n", err)
		os.Exit(2)
	}
	fmt.Printf("corpusgen: wrote %d frames and %s/expected.json\n", manifest.Frames, *outDir)
}
