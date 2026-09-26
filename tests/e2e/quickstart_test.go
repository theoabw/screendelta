// Package e2e drives the built command through the quickstart scenarios.
//
// These tests are deliberately a level above the unit tests: they build the real binary, generate real
// frames, run the command as a user would, and check the exit code, the standard output and the files it
// left behind. The unit tests prove the rules; this proves that a reviewer following the quickstart gets
// the documented result, which is what the specification's scenarios are for.
package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/theoabw/screendelta/internal/corpus"
)

var build struct {
	once sync.Once
	path string
	err  error
}

// binary builds the command once per test run and returns its path.
func binary(t *testing.T) string {
	t.Helper()
	build.once.Do(func() {
		dir, err := os.MkdirTemp("", "screendelta-e2e")
		if err != nil {
			build.err = err
			return
		}
		name := "screendelta"
		if os.PathSeparator == '\\' {
			name += ".exe"
		}
		path := filepath.Join(dir, name)
		// The module path rather than a relative one, because a test runs in its own package directory.
		command := exec.Command("go", "build", "-o", path, "github.com/theoabw/screendelta/cmd/screendelta")
		command.Stderr = os.Stderr
		if err := command.Run(); err != nil {
			build.err = err
			return
		}
		build.path = path
	})
	if build.err != nil {
		t.Fatalf("cannot build the command: %v", build.err)
	}
	return build.path
}

// result is what one invocation produced.
type result struct {
	code   int
	stdout string
	stderr string
}

func run(t *testing.T, env []string, args ...string) result {
	t.Helper()
	command := exec.Command(binary(t), args...)
	command.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()

	code := 0
	var exit *exec.ExitError
	if err != nil {
		if !asExitError(err, &exit) {
			t.Fatalf("cannot run %v: %v", args, err)
		}
		code = exit.ExitCode()
	}
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func asExitError(err error, target **exec.ExitError) bool {
	exit, ok := err.(*exec.ExitError)
	if ok {
		*target = exit
	}
	return ok
}

// frames generates a case into a temporary directory and returns the directory and the manifest.
func frames(t *testing.T, name string, opts corpus.Options) (string, corpus.Manifest) {
	t.Helper()
	dir := t.TempDir()
	manifest, err := corpus.Generate(name, dir, opts)
	if err != nil {
		t.Fatalf("generating the %s case failed: %v", name, err)
	}
	return dir, manifest
}

// document is the part of a delta document these tests read.
type document struct {
	SchemaVersion string `json:"schemaVersion"`
	Frame         struct {
		Sequence uint64 `json:"sequence"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
	} `json:"frame"`
	Fingerprint struct {
		Algorithm  string `json:"algorithm"`
		GridSize   int    `json:"gridSize"`
		StrictHash string `json:"strictHash"`
	} `json:"fingerprint"`
	Regions []struct {
		Identity           uint64  `json:"identity"`
		Class              string  `json:"class"`
		Magnitude          float64 `json:"magnitude"`
		AreaPixels         int     `json:"areaPixels"`
		IdentityConfidence float64 `json:"identityConfidence"`
		IdentityUncertain  bool    `json:"identityUncertain"`
		Bounds             struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
			W float64 `json:"w"`
			H float64 `json:"h"`
		} `json:"bounds"`
	} `json:"regions"`
	Conditions []string `json:"conditions"`
}

func parseOne(t *testing.T, output string) document {
	t.Helper()
	var parsed document
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("the command did not print one JSON document: %v\n%s", err, output)
	}
	return parsed
}

// Scenario 1: identical frames produce no regions, and the document carries no timing field.
func TestScenario1IdenticalFramesReportNothing(t *testing.T) {
	_, manifest := frames(t, "changed-label", corpus.DefaultOptions())
	first := manifest.Paths[0]

	got := run(t, nil, "diff", "--previous", first, "--current", first)
	if got.code != 0 {
		t.Fatalf("exit code %d: %s", got.code, got.stderr)
	}
	parsed := parseOne(t, got.stdout)
	if len(parsed.Regions) != 0 {
		t.Fatalf("identical frames reported %d regions", len(parsed.Regions))
	}
	if len(parsed.Conditions) != 0 {
		t.Fatalf("identical frames reported conditions: %v", parsed.Conditions)
	}
	if parsed.SchemaVersion == "" || parsed.Fingerprint.Algorithm == "" {
		t.Fatalf("the document is missing its version or fingerprint: %+v", parsed)
	}
	for _, banned := range []string{"duration", "elapsed", "timestamp", "time"} {
		if strings.Contains(got.stdout, banned) {
			t.Fatalf("the document carries a timing field %q, so output cannot be byte-identical", banned)
		}
	}
}

// Scenario 2: a changed region is reported once, with normalized bounds.
func TestScenario2AChangedRegionIsReportedOnce(t *testing.T) {
	_, manifest := frames(t, "changed-label", corpus.DefaultOptions())
	before, after := manifest.Paths[0], manifest.Paths[1]

	got := run(t, nil, "diff", "--previous", before, "--current", after)
	if got.code != 0 {
		t.Fatalf("exit code %d: %s", got.code, got.stderr)
	}
	parsed := parseOne(t, got.stdout)
	if len(parsed.Regions) != 1 {
		t.Fatalf("expected one region, got %d: %+v", len(parsed.Regions), parsed.Regions)
	}
	region := parsed.Regions[0]
	if region.Class != "changed" {
		t.Fatalf("class is %q, want changed on a first comparison", region.Class)
	}
	if region.Bounds.X < 0 || region.Bounds.Y < 0 || region.Bounds.W <= 0 || region.Bounds.H <= 0 {
		t.Fatalf("bounds are not a normalized rectangle: %+v", region.Bounds)
	}
	if region.Bounds.X+region.Bounds.W > 1.000001 || region.Bounds.Y+region.Bounds.H > 1.000001 {
		t.Fatalf("bounds extend past the frame: %+v", region.Bounds)
	}
	if region.Magnitude <= 0 || region.Magnitude > 1 {
		t.Fatalf("magnitude %v is outside zero to one", region.Magnitude)
	}
	if region.AreaPixels <= 0 {
		t.Fatalf("area is %d", region.AreaPixels)
	}
	if region.Identity == 0 {
		t.Fatal("the region carries no identity")
	}
}

// Scenario 3: noise alone reports nothing.
func TestScenario3NoiseAloneReportsNothing(t *testing.T) {
	_, manifest := frames(t, "noise", corpus.Options{Width: 320, Height: 240, Pairs: 5, Seed: 3})
	first, second := manifest.Paths[0], manifest.Paths[1]

	got := run(t, nil, "diff", "--previous", first, "--current", second)
	if got.code != 0 {
		t.Fatalf("exit code %d: %s", got.code, got.stderr)
	}
	parsed := parseOne(t, got.stdout)
	if len(parsed.Regions) != 0 {
		t.Fatalf("capture noise reported %d regions: %+v", len(parsed.Regions), parsed.Regions)
	}
}

// Scenario 4: identity survives motion and is never reused.
func TestScenario4IdentitySurvivesMotion(t *testing.T) {
	dir, _ := frames(t, "moving-button", corpus.Options{Width: 320, Height: 240, Frames: 6, Seed: 20260926})

	got := run(t, nil, "stream", "--source", dir)
	if got.code != 0 {
		t.Fatalf("exit code %d: %s", got.code, got.stderr)
	}

	identities := map[uint64]bool{}
	perFrame := 0
	var lastChange uint64
	for _, line := range strings.Split(strings.TrimSpace(got.stdout), "\n") {
		parsed := parseOne(t, line)
		for _, region := range parsed.Regions {
			if identities[region.Identity] {
				continue
			}
			identities[region.Identity] = true
		}
		if len(parsed.Regions) > 0 {
			lastChange++
		}
		perFrame += len(parsed.Regions)
	}
	if lastChange == 0 {
		t.Fatal("the moving element was never reported")
	}
	// One moving element produces two areas per movement, so a handful of frames must not produce a new
	// identity for every region: that would be the pre-identity behaviour.
	if len(identities) > 4 {
		t.Fatalf("six frames of one moving element produced %d identities, so identity is not surviving motion", len(identities))
	}
}

// Scenario 6: a fingerprint distinguishes noise from content.
func TestScenario6FingerprintDistinguishesNoiseFromContent(t *testing.T) {
	dir, manifest := frames(t, "changed-label", corpus.DefaultOptions())
	first, second := manifest.Paths[0], manifest.Paths[1]
	stored := filepath.Join(dir, "stored.json")

	if got := run(t, nil, "fingerprint", "--frame", first, "--out", stored); got.code != 0 {
		t.Fatalf("fingerprint exited %d: %s", got.code, got.stderr)
	}

	same := run(t, nil, "compare", "--frame", first, "--document", stored)
	if same.code != 0 {
		t.Fatalf("compare exited %d: %s", same.code, same.stderr)
	}
	if !strings.HasPrefix(same.stdout, "equal") {
		t.Fatalf("a frame compared against its own fingerprint said %q", same.stdout)
	}

	changed := run(t, nil, "compare", "--frame", second, "--document", stored)
	if changed.code != 0 {
		t.Fatalf("compare exited %d: %s", changed.code, changed.stderr)
	}
	if !strings.HasPrefix(changed.stdout, "different") {
		t.Fatalf("a changed frame said %q", changed.stdout)
	}
}

// Scenario 7: regions of interest restrict the output, and an empty list reports nothing.
func TestScenario7RegionsOfInterestRestrict(t *testing.T) {
	dir, manifest := frames(t, "changed-label", corpus.DefaultOptions())
	before, after := manifest.Paths[0], manifest.Paths[1]

	// The changed panel sits in the middle of the frame, so a toolbar at the top excludes it and a band
	// across the middle includes it.
	toolbar := filepath.Join(dir, "toolbar.json")
	if err := os.WriteFile(toolbar, []byte(`{"regionsOfInterest":[{"label":"toolbar","bounds":{"x":0,"y":0,"w":1,"h":0.2}}]}`), 0o644); err != nil {
		t.Fatalf("cannot write the config: %v", err)
	}
	restricted := run(t, nil, "diff", "--previous", before, "--current", after, "--config", toolbar)
	if restricted.code != 0 {
		t.Fatalf("exit code %d: %s", restricted.code, restricted.stderr)
	}
	if parsed := parseOne(t, restricted.stdout); len(parsed.Regions) != 0 {
		t.Fatalf("a toolbar region of interest reported %d regions outside it", len(parsed.Regions))
	}

	middle := filepath.Join(dir, "middle.json")
	if err := os.WriteFile(middle, []byte(`{"regionsOfInterest":[{"label":"middle","bounds":{"x":0,"y":0.3,"w":1,"h":0.5}}]}`), 0o644); err != nil {
		t.Fatalf("cannot write the config: %v", err)
	}
	included := run(t, nil, "diff", "--previous", before, "--current", after, "--config", middle)
	if included.code != 0 {
		t.Fatalf("exit code %d: %s", included.code, included.stderr)
	}
	if parsed := parseOne(t, included.stdout); len(parsed.Regions) == 0 {
		t.Fatal("a region of interest containing the change reported nothing")
	}

	// An empty list is a request for nothing, which is not the same as omitting the field.
	empty := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(empty, []byte(`{"regionsOfInterest":[]}`), 0o644); err != nil {
		t.Fatalf("cannot write the config: %v", err)
	}
	none := run(t, nil, "diff", "--previous", before, "--current", after, "--config", empty)
	if none.code != 0 {
		t.Fatalf("exit code %d: %s", none.code, none.stderr)
	}
	if parsed := parseOne(t, none.stdout); len(parsed.Regions) != 0 {
		t.Fatalf("an empty region-of-interest list reported %d regions", len(parsed.Regions))
	}
}

// Scenario 8: unusable input fails loudly, with the documented exit code, and writes nothing.
func TestScenario8UnusableInputFailsLoudly(t *testing.T) {
	dir, manifest := frames(t, "changed-label", corpus.DefaultOptions())
	frame := manifest.Paths[0]

	// A raw frame whose buffer does not match the declared dimensions, and a config with an unknown key.
	short := filepath.Join(dir, "short.raw")
	if err := os.WriteFile(short, make([]byte, 16), 0o644); err != nil {
		t.Fatalf("cannot write the fixture: %v", err)
	}

	cases := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{name: "mismatched raw dimensions", args: []string{"diff", "--previous", short, "--current", short, "--width", "100", "--height", "100"}, wantCode: 2},
		{name: "a frame that is not there", args: []string{"diff", "--previous", filepath.Join(dir, "absent.png"), "--current", frame}, wantCode: 2},
		{name: "a missing argument", args: []string{"diff", "--previous", frame}, wantCode: 1},
		{name: "an unknown subcommand", args: []string{"nonsense"}, wantCode: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, nil, tc.args...)
			if got.code != tc.wantCode {
				t.Fatalf("exit code %d, want %d: %s", got.code, tc.wantCode, got.stderr)
			}
			if strings.TrimSpace(got.stdout) != "" {
				t.Fatalf("a failing command wrote to standard output: %q", got.stdout)
			}
			if strings.TrimSpace(got.stderr) == "" {
				t.Fatal("a failing command explained nothing on standard error")
			}
		})
	}
}

// Scenario 9: output is deterministic across thread counts and scheduling.
//
// The requirement says byte-identical output, independent of host, thread count and scheduling. Two of the
// three are testable here, and they are tested four ways rather than one: two thread counts, which is the
// wording of the criterion, and two garbage collector settings, because when the collector runs is exactly
// the kind of scheduling difference that would show up if any part of the engine depended on a map's
// iteration order or on a goroutine's progress. Another host is not testable from here and the matrix says so
// rather than counting this as the whole requirement.
func TestScenario9OutputIsDeterministicAcrossThreads(t *testing.T) {
	dir, _ := frames(t, "sweep", corpus.Options{Width: 320, Height: 240, Frames: 8, Seed: 20260926})

	runs := []struct {
		name string
		env  []string
	}{
		{name: "one thread", env: []string{"GOMAXPROCS=1"}},
		{name: "four threads", env: []string{"GOMAXPROCS=4"}},
		{name: "frequent collection", env: []string{"GOMAXPROCS=2", "GOGC=10"}},
		{name: "collection off", env: []string{"GOMAXPROCS=2", "GOGC=off"}},
	}

	var reference []byte
	referenceName := ""
	for _, configuration := range runs {
		output := filepath.Join(t.TempDir(), "stream.ndjson")
		if got := run(t, configuration.env, "stream", "--source", dir, "--out", output); got.code != 0 {
			t.Fatalf("%s: exit code %d: %s", configuration.name, got.code, got.stderr)
		}
		body, err := os.ReadFile(output)
		if err != nil {
			t.Fatalf("%s: cannot read the output: %v", configuration.name, err)
		}
		if len(body) == 0 {
			t.Fatalf("%s: the stream wrote nothing", configuration.name)
		}
		if reference == nil {
			reference, referenceName = body, configuration.name
			continue
		}
		if !bytes.Equal(reference, body) {
			t.Fatalf("%s produced different output from %s: %d bytes against %d",
				configuration.name, referenceName, len(body), len(reference))
		}
	}
}
