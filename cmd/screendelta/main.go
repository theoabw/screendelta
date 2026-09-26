// Command screendelta reports what changed between screen frames and which element is
// which across a stream of them.
//
// The command is a thin shell over the library: it parses arguments, reads frames,
// writes documents and maps failures to exit codes. Every behaviour it exposes is
// defined by specs/001-frame-delta-engine/contracts/interfaces.md, and the exit codes are
// part of that contract:
//
//	0 success
//	1 usage error: unknown flag, missing argument, contradictory options
//	2 input error: unreadable frame, unsupported format, mismatched dimensions
//	3 contract violation: the request cannot be expressed by this build
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/theoabw/screendelta/internal/config"
	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/diff"
	"github.com/theoabw/screendelta/internal/fielderr"
	"github.com/theoabw/screendelta/internal/fingerprint"
	"github.com/theoabw/screendelta/internal/frame"
	"github.com/theoabw/screendelta/internal/stream"
)

// Version is the tool version, set at build time with -ldflags when wanted.
var Version = "0.1.0-dev"

const usage = `screendelta: report what changed between screen frames.

Usage:
  screendelta diff        --previous <frame> --current <frame> [--config <file>] [--out <path>]
  screendelta stream      --source <dir|->        [--config <file>] [--out <path>]
  screendelta fingerprint --frame <frame>         [--config <file>] [--out <path>]
  screendelta compare     --frame <frame> --document <json> [--config <file>]
  screendelta validate    --config <file>
  screendelta version

screendelta compare reports whether a frame is the same screen as a stored fingerprint
document, allowing each cell to differ by the configured tolerance. It prints equal or
different and exits 0 either way, because a comparison that ran is a success: the answer is
on standard output. A stored document the build does not understand is exit 3.

Frame options:
  --width N --height N   required for raw rgba8 input, rejected for PNG
  --pixel-format F       only rgba8 is supported

Exit codes:
  0 success, 1 usage, 2 input error, 3 contract violation
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is the whole command, with its streams passed in so a test can drive it.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 1
	}

	subcommand, rest := args[0], args[1:]
	switch subcommand {
	case "compare":
		return runCompare(rest, stdout, stderr)
	case "diff":
		return runDiff(rest, stdout, stderr)
	case "stream":
		return runStream(rest, stdin, stdout, stderr)
	case "fingerprint":
		return runFingerprint(rest, stdout, stderr)
	case "validate":
		return runValidate(rest, stdout, stderr)
	case "version":
		fmt.Fprintf(stdout, "screendelta %s (schema %s)\n", Version, delta.SchemaVersion)
		return 0
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "screendelta: unknown subcommand %q\n", subcommand)
		fmt.Fprint(stderr, usage)
		return 1
	}
}

// frameFlags holds the options shared by the subcommands that read frames.
type frameFlags struct {
	configPath  string
	outputPath  string
	width       int
	height      int
	pixelFormat string
	pretty      bool
}

func addFrameFlags(fs *flag.FlagSet, ff *frameFlags) {
	fs.StringVar(&ff.configPath, "config", "", "configuration document to load")
	fs.StringVar(&ff.outputPath, "out", "", "write the document here instead of standard output")
	fs.IntVar(&ff.width, "width", 0, "frame width for raw input")
	fs.IntVar(&ff.height, "height", 0, "frame height for raw input")
	fs.StringVar(&ff.pixelFormat, "pixel-format", string(frame.FormatRGBA8), "pixel format for raw input")
	fs.BoolVar(&ff.pretty, "pretty", false, "indent the JSON output")
}

// runCompare answers FR-009: compare a frame against a fingerprint stored earlier. It is also where
// an unsupported schema version is rejected with the exit code that means a contract problem rather
// than an input problem, because a document the build does not understand is not bad data.
func runCompare(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		ff       frameFlags
		frameArg string
		document string
	)
	addFrameFlags(fs, &ff)
	fs.StringVar(&frameArg, "frame", "", "the frame to fingerprint")
	fs.StringVar(&document, "document", "", "a stored document whose fingerprint to compare against")
	if err := fs.Parse(args); err != nil {
		return usageError(stderr, "compare", err)
	}
	if frameArg == "" || document == "" {
		fmt.Fprintln(stderr, "screendelta: compare: --frame and --document are both required")
		return 1
	}

	cfg, code := loadConfig(ff, stderr, "compare")
	if code != 0 {
		return code
	}

	stored, code := readDocument(document, stderr, "compare")
	if code != 0 {
		return code
	}

	f, code := readFrame(frameArg, 1, ff, stderr, "compare")
	if code != 0 {
		return code
	}
	current, err := diff.New().Fingerprint(f, cfg)
	if err != nil {
		return reportError(stderr, "compare", err)
	}

	result, err := fingerprint.Compare(stored.Fingerprint, current, cfg.Fingerprint.MaxCellDelta)
	if err != nil {
		return reportError(stderr, "compare", err)
	}

	verdict := "different"
	if result.Equal {
		verdict = "equal"
	}
	fmt.Fprintf(stdout, "%s: %s (allowance %d, differing cells %d, largest delta %d)\n",
		verdict, result.Explain(), cfg.Fingerprint.MaxCellDelta, result.DifferingCells, result.LargestDelta)
	return 0
}

// readDocument reads a stored document, rejecting one this build does not understand. The distinction
// matters: an unreadable file is bad input, and a document from a newer schema is a contract problem.
func readDocument(path string, stderr io.Writer, subcommand string) (delta.Document, int) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(stderr, "screendelta: %s: cannot open %s: %v\n", subcommand, path, err)
		return delta.Document{}, 2
	}
	defer file.Close()

	document, err := delta.Decode(file)
	if err != nil {
		var fieldErr *fielderr.Error
		if errors.As(err, &fieldErr) && fieldErr.Field == "schemaVersion" {
			fmt.Fprintf(stderr, "screendelta: %s: %s\n", subcommand, err.Error())
			return delta.Document{}, 3
		}
		return delta.Document{}, reportError(stderr, subcommand, err)
	}
	return document, 0
}

func runDiff(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		ff       frameFlags
		previous string
		current  string
	)
	addFrameFlags(fs, &ff)
	fs.StringVar(&previous, "previous", "", "the earlier frame")
	fs.StringVar(&current, "current", "", "the later frame")
	if err := fs.Parse(args); err != nil {
		return usageError(stderr, "diff", err)
	}
	if previous == "" || current == "" {
		fmt.Fprintln(stderr, "screendelta: diff: --previous and --current are both required")
		return 1
	}

	cfg, code := loadConfig(ff, stderr, "diff")
	if code != 0 {
		return code
	}

	before, code := readFrame(previous, 1, ff, stderr, "diff")
	if code != 0 {
		return code
	}
	after, code := readFrame(current, 2, ff, stderr, "diff")
	if code != 0 {
		return code
	}

	engine, err := stream.New(cfg, diff.New())
	if err != nil {
		return reportError(stderr, "diff", err)
	}
	if _, err := engine.Push(before); err != nil {
		return reportError(stderr, "diff", err)
	}
	document, err := engine.Push(after)
	if err != nil {
		return reportError(stderr, "diff", err)
	}

	return writeDocument(document, ff, stdout, stderr, "diff")
}

func runStream(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("stream", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		ff     frameFlags
		source string
	)
	addFrameFlags(fs, &ff)
	fs.StringVar(&source, "source", "", "a directory of PNG frames in order, or - for raw frames on standard input")
	if err := fs.Parse(args); err != nil {
		return usageError(stderr, "stream", err)
	}
	if source == "" {
		fmt.Fprintln(stderr, "screendelta: stream: --source is required")
		return 1
	}

	cfg, code := loadConfig(ff, stderr, "stream")
	if code != 0 {
		return code
	}

	engine, err := stream.New(cfg, diff.New())
	if err != nil {
		return reportError(stderr, "stream", err)
	}
	defer engine.Close()

	output, closeOutput, code := openOutput(ff.outputPath, stdout, stderr, "stream")
	if code != 0 {
		return code
	}
	defer closeOutput()

	// One document per line, flushed as it is written, so a consumer can act on each
	// frame without waiting for the stream to end.
	emit := func(document delta.Document) int {
		if err := document.Encode(output, ff.pretty); err != nil {
			return reportError(stderr, "stream", err)
		}
		if flusher, ok := output.(interface{ Flush() error }); ok {
			_ = flusher.Flush()
		}
		if syncer, ok := output.(*os.File); ok {
			_ = syncer.Sync()
		}
		return 0
	}

	sequence := uint64(0)
	handle := func(f frame.Frame) int {
		document, err := engine.Push(f)
		if err != nil {
			return reportError(stderr, "stream", err)
		}
		return emit(document)
	}

	if source == "-" {
		if ff.width <= 0 || ff.height <= 0 {
			fmt.Fprintln(stderr, "screendelta: stream: raw frames on standard input need --width and --height")
			return 1
		}
		size := ff.width * ff.height * 4
		buffer := make([]byte, size)
		for {
			read, err := io.ReadFull(stdin, buffer)
			if err == io.EOF {
				break
			}
			if err == io.ErrUnexpectedEOF {
				fmt.Fprintf(stderr, "screendelta: stream: frame %d is %d bytes, expected %d\n", sequence+1, read, size)
				return 2
			}
			if err != nil {
				fmt.Fprintf(stderr, "screendelta: stream: cannot read frame %d: %v\n", sequence+1, err)
				return 2
			}
			sequence++
			pixels := make([]byte, size)
			copy(pixels, buffer)
			f, err := frame.NewRaw(sequence, ff.width, ff.height, frame.FormatRGBA8, 1, pixels)
			if err != nil {
				return reportError(stderr, "stream", err)
			}
			if code := handle(f); code != 0 {
				return code
			}
		}
		return 0
	}

	paths, err := framePaths(source)
	if err != nil {
		fmt.Fprintf(stderr, "screendelta: stream: %v\n", err)
		return 2
	}
	for index, path := range paths {
		f, code := readFrame(path, uint64(index+1), ff, stderr, "stream")
		if code != 0 {
			return code
		}
		if code := handle(f); code != 0 {
			return code
		}
	}
	return 0
}

func runFingerprint(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fingerprint", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		ff   frameFlags
		path string
	)
	addFrameFlags(fs, &ff)
	fs.StringVar(&path, "frame", "", "the frame to summarise")
	if err := fs.Parse(args); err != nil {
		return usageError(stderr, "fingerprint", err)
	}
	if path == "" {
		fmt.Fprintln(stderr, "screendelta: fingerprint: --frame is required")
		return 1
	}

	cfg, code := loadConfig(ff, stderr, "fingerprint")
	if code != 0 {
		return code
	}
	f, code := readFrame(path, 1, ff, stderr, "fingerprint")
	if code != 0 {
		return code
	}

	fingerprint, err := diff.New().Fingerprint(f, cfg)
	if err != nil {
		return reportError(stderr, "fingerprint", err)
	}
	document := delta.Document{
		SchemaVersion: delta.SchemaVersion,
		Frame: delta.FrameRef{
			Sequence:    f.Sequence,
			Width:       f.Width,
			Height:      f.Height,
			ScaleFactor: f.ScaleFactor,
		},
		Fingerprint: fingerprint,
		Regions:     []delta.Region{},
		Conditions:  []delta.Condition{},
	}
	return writeDocument(document, ff, stdout, stderr, "fingerprint")
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var path string
	fs.StringVar(&path, "config", "", "configuration document to check")
	if err := fs.Parse(args); err != nil {
		return usageError(stderr, "validate", err)
	}
	if path == "" {
		fmt.Fprintln(stderr, "screendelta: validate: --config is required")
		return 1
	}

	cfg, err := config.Load(path)
	if err != nil {
		return reportError(stderr, "validate", err)
	}
	fmt.Fprintf(stdout, "config is valid: noiseFloor %g, gridSize %d, %s output\n",
		cfg.NoiseFloor, cfg.Fingerprint.GridSize, cfg.Output.Format)
	return 0
}

func loadConfig(ff frameFlags, stderr io.Writer, subcommand string) (config.Config, int) {
	if ff.configPath == "" {
		cfg := config.Defaults()
		cfg.Output.Pretty = ff.pretty
		return cfg, 0
	}
	cfg, err := config.Load(ff.configPath)
	if err != nil {
		return config.Config{}, reportError(stderr, subcommand, err)
	}
	if ff.pretty {
		cfg.Output.Pretty = true
	}
	return cfg, 0
}

// readFrame reads one frame, either as PNG or as raw rgba8 when dimensions are given.
func readFrame(path string, sequence uint64, ff frameFlags, stderr io.Writer, subcommand string) (frame.Frame, int) {
	if ff.width > 0 || ff.height > 0 {
		return readRawFrame(path, sequence, ff, stderr, subcommand)
	}

	file, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(stderr, "screendelta: %s: cannot open %s: %v\n", subcommand, path, err)
		return frame.Frame{}, 2
	}
	defer file.Close()

	f, err := frame.DecodePNG(sequence, 1, file)
	if err != nil {
		return frame.Frame{}, reportError(stderr, subcommand, err)
	}
	return f, 0
}

func readRawFrame(path string, sequence uint64, ff frameFlags, stderr io.Writer, subcommand string) (frame.Frame, int) {
	if ff.width <= 0 || ff.height <= 0 {
		fmt.Fprintf(stderr, "screendelta: %s: raw input needs both --width and --height\n", subcommand)
		return frame.Frame{}, 1
	}
	if frame.PixelFormat(ff.pixelFormat) != frame.FormatRGBA8 {
		fmt.Fprintf(stderr, "screendelta: %s: unsupported pixel format %q, only %s is supported\n",
			subcommand, ff.pixelFormat, frame.FormatRGBA8)
		return frame.Frame{}, 1
	}

	pixels, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "screendelta: %s: cannot read %s: %v\n", subcommand, path, err)
		return frame.Frame{}, 2
	}
	f, err := frame.NewRaw(sequence, ff.width, ff.height, frame.FormatRGBA8, 1, pixels)
	if err != nil {
		return frame.Frame{}, reportError(stderr, subcommand, err)
	}
	return f, 0
}

// framePaths lists the frames of a directory in the order they were captured.
func framePaths(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", dir, err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".png") {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no PNG frames in %s", dir)
	}
	sort.Strings(paths)
	return paths, nil
}

func openOutput(path string, stdout io.Writer, stderr io.Writer, subcommand string) (io.Writer, func(), int) {
	if path == "" {
		return stdout, func() {}, 0
	}
	file, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(stderr, "screendelta: %s: cannot write %s: %v\n", subcommand, path, err)
		return nil, func() {}, 2
	}
	return file, func() { file.Close() }, 0
}

func writeDocument(document delta.Document, ff frameFlags, stdout, stderr io.Writer, subcommand string) int {
	output, closeOutput, code := openOutput(ff.outputPath, stdout, stderr, subcommand)
	if code != 0 {
		return code
	}
	defer closeOutput()

	if err := document.Encode(output, ff.pretty); err != nil {
		return reportError(stderr, subcommand, err)
	}
	return 0
}

// usageError reports a flag parsing failure as a usage error rather than an input error,
// because the operator can fix it by reading the help.
func usageError(stderr io.Writer, subcommand string, err error) int {
	fmt.Fprintf(stderr, "screendelta: %s: %v\n", subcommand, err)
	fmt.Fprint(stderr, usage)
	return 1
}

// reportError maps an error to the exit code its kind deserves and prints one line.
func reportError(stderr io.Writer, subcommand string, err error) int {
	if err == nil {
		return 0
	}
	message := err.Error()
	if !strings.HasPrefix(message, "screendelta") {
		message = fmt.Sprintf("%s: %s", subcommand, message)
	}
	fmt.Fprintf(stderr, "screendelta: %s\n", message)

	var fieldErr *fielderr.Error
	if errors.As(err, &fieldErr) {
		return 2
	}
	return 2
}
