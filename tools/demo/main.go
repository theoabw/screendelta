// Command demo runs the pipeline the engine is one stage of, and reports the rate at which it makes
// decisions.
//
// The pipeline is capture, engine, detector. Capture reads screen frames in order, the engine turns each
// frame into a delta document, and a stub detector turns each document into a decision: what a planner
// would do next. The detector here is deliberately trivial, because the number being measured belongs to
// the pipeline rather than to the planning: what matters is how many decisions per second the engine makes
// possible, on real screen pixels.
//
// The rate is reported two ways, and both are needed. Frames per second is what the engine can consume.
// Decisions per second is what a planner gets, and it is lower whenever a frame carries nothing to decide
// about, which is the honest reason a pipeline can be fast and still not useful.
//
// Usage:
//
//	demo --source corpus/real --passes 20 --assert-rate 20
//	demo --generate 200 --assert-rate 20
//
// The generated source needs no capture and is what CI can run; the captured source is real rendered pixels
// and is what the success criterion asks for.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/theoabw/screendelta/internal/config"
	"github.com/theoabw/screendelta/internal/corpus"
	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/diff"
	"github.com/theoabw/screendelta/internal/frame"
	"github.com/theoabw/screendelta/internal/stream"
)

// decision is what the stub detector concluded from one document.
type decision struct {
	frame    uint64
	action   string
	identity uint64
}

func main() {
	var (
		source     = flag.String("source", "", "a directory of PNG frames, in name order, as the capture source")
		generated  = flag.Int("generate", 0, "instead of a directory, generate this many synthetic frames")
		passes     = flag.Int("passes", 1, "how many times to replay the source, so the rate is measured over enough frames")
		assertRate = flag.Float64("assert-rate", 0, "fail if decisions per second is below this")
		assertFPS  = flag.Float64("assert-fps", 0, "fail if frames per second is below this")
		reportEach = flag.Bool("verbose", false, "print one line per decision")
	)
	flag.Parse()

	if *source == "" && *generated == 0 {
		fmt.Fprintln(os.Stderr, "demo: give --source or --generate")
		os.Exit(1)
	}

	frames, err := capture(*source, *generated)
	if err != nil {
		fmt.Fprintf(os.Stderr, "demo: %v\n", err)
		os.Exit(2)
	}
	if len(frames) < 2 {
		fmt.Fprintln(os.Stderr, "demo: at least two frames are needed to have anything to compare")
		os.Exit(1)
	}

	// The region of interest is the part of the fixture a planner acts on: the entry form. Restricting to it
	// is what the engine is for, and it keeps the clock in the status bar from producing decisions.
	cfg := config.Defaults()
	roi := []config.RegionOfInterest{{Label: "entry form", Bounds: delta.Bounds{X: 0, Y: 0.04, W: 0.55, H: 0.28}}}
	cfg.RegionsOfInterest = &roi

	engine, err := stream.New(cfg, diff.New())
	if err != nil {
		fmt.Fprintf(os.Stderr, "demo: cannot start the engine: %v\n", err)
		os.Exit(2)
	}
	defer engine.Close()

	decisions := make([]decision, 0, len(frames)*(*passes))
	sequence := uint64(0)
	started := time.Now()

	for pass := 0; pass < *passes; pass++ {
		for _, f := range frames {
			sequence++
			f.Sequence = sequence
			document, err := engine.Push(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "demo: the engine failed on frame %d: %v\n", sequence, err)
				os.Exit(2)
			}
			if decided, ok := detect(document); ok {
				decisions = append(decisions, decided)
				if *reportEach {
					fmt.Printf("frame %d: %s (identity %d, %d regions)\n",
						document.Frame.Sequence, decided.action, decided.identity, len(document.Regions))
				}
			}
		}
	}
	elapsed := time.Since(started)

	processed := len(frames) * (*passes)
	framesPerSecond := float64(processed) / elapsed.Seconds()
	decisionsPerSecond := float64(len(decisions)) / elapsed.Seconds()

	fmt.Printf("source %s, %d frames replayed %d times, %d frames processed\n", describe(*source, *generated), len(frames), *passes, processed)
	fmt.Printf("capture %dx%d, elapsed %s\n", frames[0].Width, frames[0].Height, elapsed.Round(time.Millisecond))
	fmt.Printf("frames per second %.1f, decisions %d, decisions per second %.1f\n",
		framesPerSecond, len(decisions), decisionsPerSecond)
	if len(decisions) == 0 {
		fmt.Println("no frame produced a decision, so the rate above is not a decision rate")
	}

	if *assertRate > 0 && decisionsPerSecond < *assertRate {
		fmt.Fprintf(os.Stderr, "demo: %.1f decisions per second is below the required %.1f\n", decisionsPerSecond, *assertRate)
		os.Exit(1)
	}
	if *assertFPS > 0 && framesPerSecond < *assertFPS {
		fmt.Fprintf(os.Stderr, "demo: %.1f frames per second is below the required %.1f\n", framesPerSecond, *assertFPS)
		os.Exit(1)
	}
}

// detect is the stub planner. It reads the document and decides, using only what the contract promises:
// a region inside the region of interest with a certain identity means there is something to type into,
// and a region whose identity is uncertain means the planner must re-orient before acting.
func detect(document delta.Document) (decision, bool) {
	for _, region := range document.Regions {
		if region.Class == delta.ClassRemoved {
			continue
		}
		if region.IdentityUncertain {
			return decision{frame: document.Frame.Sequence, action: "re-orient, the element is not the one that was here", identity: region.Identity}, true
		}
		return decision{frame: document.Frame.Sequence, action: "type into the entry form", identity: region.Identity}, true
	}
	return decision{}, false
}

// capture reads the frames the pipeline will process, either from a directory of PNGs or from the generator.
func capture(source string, generated int) ([]frame.Frame, error) {
	if generated > 0 {
		// The generated source is the corpus sweep at 1080p, which changes several elements per frame and is
		// reproducible from a seed.
		options := corpus.DefaultOptions()
		options.Width, options.Height, options.Frames = 1920, 1080, generated
		c, err := corpus.NewCase("sweep", options)
		if err != nil {
			return nil, err
		}
		frames := make([]frame.Frame, 0, c.Frames())
		for index := 0; index < c.Frames(); index++ {
			img := c.Frame(index)
			f, err := frame.NewRaw(uint64(index+1), img.Rect.Dx(), img.Rect.Dy(), frame.FormatRGBA8, 1, append([]byte(nil), img.Pix...))
			if err != nil {
				return nil, err
			}
			frames = append(frames, f)
		}
		return frames, nil
	}

	entries, err := os.ReadDir(source)
	if err != nil {
		return nil, fmt.Errorf("cannot read the capture directory: %w", err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".png") {
			continue
		}
		paths = append(paths, filepath.Join(source, entry.Name()))
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no PNG frames in %s", source)
	}
	sort.Strings(paths)

	frames := make([]frame.Frame, 0, len(paths))
	for index, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("cannot open %s: %w", path, err)
		}
		f, err := frame.DecodePNG(uint64(index+1), 1, file)
		file.Close()
		if err != nil {
			return nil, fmt.Errorf("cannot decode %s: %w", path, err)
		}
		frames = append(frames, f)
	}
	return frames, nil
}

func describe(source string, generated int) string {
	if generated > 0 {
		return fmt.Sprintf("generated %d frames at 1920x1080", generated)
	}
	return source
}
