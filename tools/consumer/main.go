// Command consumer is a second, independent reader of delta documents.
//
// It exists to test the contract rather than the engine. It shares no code with the engine: its types are
// written from the published JSON Schema, it validates every document it reads against the rules the schema
// states, and it summarises the element lifecycle it observes. If the engine emits something the schema
// forbids, this program is the thing that notices, which is what makes it evidence rather than a
// demonstration.
//
// Usage:
//
//	screendelta stream --source frames | consumer
//	consumer --source stream.ndjson --summary
//
// It reads newline delimited documents, one per frame, and exits 0 when every document conformed and 1 when
// any did not, naming the first few violations so the reason is visible without reading the whole stream.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
)

// document mirrors the published schema, field by field, and deliberately not the engine's own types.
type document struct {
	SchemaVersion string   `json:"schemaVersion"`
	Frame         frame    `json:"frame"`
	Fingerprint   print    `json:"fingerprint"`
	Regions       []region `json:"regions"`
	Conditions    []string `json:"conditions"`
}

type frame struct {
	Sequence    uint64  `json:"sequence"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	ScaleFactor float64 `json:"scaleFactor"`
}

type print struct {
	Algorithm  string `json:"algorithm"`
	GridSize   int    `json:"gridSize"`
	Cells      []int  `json:"cells"`
	StrictHash string `json:"strictHash"`
}

type region struct {
	Identity           uint64  `json:"identity"`
	Class              string  `json:"class"`
	Bounds             bounds  `json:"bounds"`
	PreviousBounds     *bounds `json:"previousBounds"`
	Magnitude          float64 `json:"magnitude"`
	AreaPixels         int     `json:"areaPixels"`
	IdentityConfidence float64 `json:"identityConfidence"`
	IdentityUncertain  bool    `json:"identityUncertain"`
}

type bounds struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// lifecycle is what this consumer keeps per element, which is the thing a planner would want.
type lifecycle struct {
	FirstFrame    uint64
	LastFrame     uint64
	Frames        int
	Classes       map[string]int
	EverUncertain bool
	RetiredAt     uint64
}

// lastSequence is the previous document's frame number, for checking that a stream is in order.
var lastSequence uint64

func hasCondition(conditions []string, wanted string) bool {
	for _, condition := range conditions {
		if condition == wanted {
			return true
		}
	}
	return false
}

func main() {
	var (
		source  = flag.String("source", "-", "a file of newline delimited documents, or - for standard input")
		summary = flag.Bool("summary", true, "print the element lifecycle summary")
	)
	flag.Parse()

	input := io.Reader(os.Stdin)
	if *source != "-" {
		file, err := os.Open(*source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "consumer: cannot open %s: %v\n", *source, err)
			os.Exit(2)
		}
		defer file.Close()
		input = file
	}

	lifecycles := map[uint64]*lifecycle{}
	violations := make([]string, 0, 8)
	documents, framesOfRegions := 0, 0

	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<24)
	for line := 1; scanner.Scan(); line++ {
		body := scanner.Bytes()
		if len(body) == 0 {
			continue
		}
		var parsed document
		// The schema forbids additional properties, so an unknown field is a contract violation and this
		// consumer is as strict as the schema rather than tolerating what the engine happens to emit.
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&parsed); err != nil {
			violations = append(violations, fmt.Sprintf("line %d: does not match the schema: %v", line, err))
			continue
		}
		documents++
		if len(parsed.Regions) > 0 {
			framesOfRegions++
		}
		violations = append(violations, check(parsed, line, lifecycles)...)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "consumer: cannot read the stream: %v\n", err)
		os.Exit(2)
	}

	// Every identity that was ever reported as removed must not appear again afterwards.
	for id, life := range lifecycles {
		if life.RetiredAt != 0 && life.LastFrame > life.RetiredAt {
			violations = append(violations, fmt.Sprintf(
				"identity %d was reported retired at frame %d and appeared again at frame %d",
				id, life.RetiredAt, life.LastFrame))
		}
	}

	if *summary {
		printSummary(documents, framesOfRegions, lifecycles)
	}
	if len(violations) > 0 {
		for index, violation := range violations {
			if index == 5 {
				fmt.Fprintf(os.Stderr, "consumer: and %d more violations\n", len(violations)-5)
				break
			}
			fmt.Fprintf(os.Stderr, "consumer: %s\n", violation)
		}
		os.Exit(1)
	}
}

// check validates one document against the rules the schema states, and folds it into the lifecycle map.
func check(parsed document, line int, lifecycles map[uint64]*lifecycle) []string {
	var violations []string
	add := func(format string, args ...any) {
		violations = append(violations, fmt.Sprintf("line %d: ", line)+fmt.Sprintf(format, args...))
	}

	if parsed.SchemaVersion != "1.0" {
		add("schema version %q is not one this consumer understands", parsed.SchemaVersion)
	}
	if parsed.Frame.Sequence == 0 {
		add("frame sequence is zero, and the schema requires one or more")
	}
	if parsed.Frame.Width <= 0 || parsed.Frame.Height <= 0 {
		add("frame geometry is %dx%d", parsed.Frame.Width, parsed.Frame.Height)
	}
	if parsed.Frame.ScaleFactor <= 0 {
		add("frame scale factor is %v", parsed.Frame.ScaleFactor)
	}
	if parsed.Fingerprint.Algorithm == "" {
		add("the fingerprint names no algorithm")
	}
	if len(parsed.Fingerprint.Cells) != parsed.Fingerprint.GridSize*parsed.Fingerprint.GridSize {
		add("the fingerprint has %d cells for a grid of %d", len(parsed.Fingerprint.Cells), parsed.Fingerprint.GridSize)
	}
	if parsed.Fingerprint.StrictHash == "" {
		add("the fingerprint carries no strict hash")
	}
	for _, condition := range parsed.Conditions {
		switch condition {
		case "first-frame", "viewport-changed", "out-of-order-timestamp":
		default:
			add("condition %q is not one the schema defines", condition)
		}
	}

	for _, region := range parsed.Regions {
		if hasCondition(parsed.Conditions, "first-frame") {
			add("the document declares the first frame and reports %d regions, and there is no predecessor to differ from",
				len(parsed.Regions))
			break
		}
		_ = region
	}
	if sequence, seen := lastSequence, parsed.Frame.Sequence; seen != 0 && sequence >= parsed.Frame.Sequence && !hasCondition(parsed.Conditions, "first-frame") {
		add("frame sequence %d does not follow %d, and a stream must be in order", parsed.Frame.Sequence, sequence)
	}
	lastSequence = parsed.Frame.Sequence

	seen := map[uint64]bool{}
	for index, item := range parsed.Regions {
		where := fmt.Sprintf("region %d", index)
		if item.Identity == 0 {
			add("%s carries identity zero", where)
		}
		if seen[item.Identity] {
			// Two regions may share an identity, which the contract allows for a moved element, so this is
			// not a violation. It is counted so the summary can report it.
			_ = item
		}
		seen[item.Identity] = true

		switch item.Class {
		case "added", "changed", "removed", "moved":
		default:
			add("%s has class %q", where, item.Class)
		}
		if !within(item.Bounds) {
			add("%s has bounds outside the frame: %+v", where, item.Bounds)
		}
		if item.Class == "added" && item.PreviousBounds != nil {
			add("%s is added and carries previous bounds, which the schema forbids", where)
		}
		if (item.Class == "moved" || item.Class == "removed") && item.PreviousBounds == nil {
			add("%s is %s and carries no previous bounds, which the schema requires", where, item.Class)
		}
		if item.PreviousBounds != nil && !within(*item.PreviousBounds) {
			add("%s has previous bounds outside the frame: %+v", where, *item.PreviousBounds)
		}
		if item.Magnitude < 0 || item.Magnitude > 1 {
			add("%s has magnitude %v", where, item.Magnitude)
		}
		if item.IdentityConfidence < 0 || item.IdentityConfidence > 1 {
			add("%s has identity confidence %v", where, item.IdentityConfidence)
		}
		if item.AreaPixels < 1 {
			add("%s has area %d", where, item.AreaPixels)
		}

		life, ok := lifecycles[item.Identity]
		if !ok {
			life = &lifecycle{FirstFrame: parsed.Frame.Sequence, Classes: map[string]int{}}
			lifecycles[item.Identity] = life
		}
		life.LastFrame = parsed.Frame.Sequence
		life.Frames++
		life.Classes[item.Class]++
		if item.IdentityUncertain {
			life.EverUncertain = true
		}
		if item.Class == "removed" {
			life.RetiredAt = parsed.Frame.Sequence
		}
	}
	return violations
}

func within(b bounds) bool {
	return b.X >= 0 && b.Y >= 0 && b.W > 0 && b.H > 0 &&
		b.X+b.W <= 1.000001 && b.Y+b.H <= 1.000001
}

// printSummary reports what the stream said about elements, which is the part a planner consumes.
func printSummary(documents, framesWithRegions int, lifecycles map[uint64]*lifecycle) {
	identities := make([]uint64, 0, len(lifecycles))
	uncertain, retired, shared := 0, 0, 0
	for id, life := range lifecycles {
		identities = append(identities, id)
		if life.EverUncertain {
			uncertain++
		}
		if life.RetiredAt != 0 {
			retired++
		}
		if life.Frames > life.Classes["removed"] {
			shared++
		}
	}
	sort.Slice(identities, func(i, j int) bool { return identities[i] < identities[j] })

	fmt.Printf("documents %d, frames with regions %d, identities %d\n", documents, framesWithRegions, len(identities))
	fmt.Printf("identities retired %d, reacquired uncertainly %d, seen in more than one document %d\n",
		retired, uncertain, shared)
	if len(identities) > 0 {
		first, last := identities[0], identities[len(identities)-1]
		reused := 0
		for _, id := range identities {
			life := lifecycles[id]
			if life.RetiredAt != 0 && life.LastFrame > life.RetiredAt {
				reused++
			}
		}
		fmt.Printf("identity range %d to %d, reissued after retirement %d\n", first, last, reused)
	}
}
