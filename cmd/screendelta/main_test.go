package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePNG builds a small screen with one panel whose colour the caller chooses, which
// is enough to exercise every subcommand through the real command entry point.
func writePNG(t *testing.T, path string, panelValue uint8) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	fill(img, color.RGBA{R: 30, G: 30, B: 30, A: 255})
	for y := 16; y < 32; y++ {
		for x := 16; x < 48; x++ {
			img.SetRGBA(x, y, color.RGBA{R: panelValue, G: panelValue, B: panelValue, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("cannot create %s: %v", path, err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("cannot encode %s: %v", path, err)
	}
}

func fill(img *image.RGBA, c color.RGBA) {
	for y := 0; y < img.Rect.Dy(); y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

// invoke runs the command and returns its exit code and both streams.
func invoke(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestVersionReportsTheToolAndSchema(t *testing.T) {
	code, stdout, _ := invoke(t, "version")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "screendelta") || !strings.Contains(stdout, "schema 1.0") {
		t.Fatalf("version output does not name the tool and schema: %q", stdout)
	}
}

func TestUsageErrorsExitOne(t *testing.T) {
	cases := [][]string{
		{},
		{"nonsense"},
		{"diff"},
		{"diff", "--previous", "a.png"},
		{"stream"},
		{"fingerprint"},
		{"validate"},
		{"diff", "--previous", "a.png", "--current", "b.png", "--unknown-flag"},
	}
	for _, args := range cases {
		code, _, _ := invoke(t, args...)
		if code != 1 {
			t.Fatalf("args %v exited %d, want 1 (usage)", args, code)
		}
	}
}

func TestValidateAcceptsAndRejectsConfigurations(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"noiseFloor":0.05}`), 0o644); err != nil {
		t.Fatalf("cannot write the fixture: %v", err)
	}
	if code, stdout, stderr := invoke(t, "validate", "--config", good); code != 0 {
		t.Fatalf("a valid configuration exited %d: %s", code, stderr)
	} else if !strings.Contains(stdout, "valid") {
		t.Fatalf("validate did not confirm the configuration: %q", stdout)
	}

	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"noiseFlore":0.05}`), 0o644); err != nil {
		t.Fatalf("cannot write the fixture: %v", err)
	}
	code, _, stderr := invoke(t, "validate", "--config", bad)
	if code != 2 {
		t.Fatalf("an invalid configuration exited %d, want 2", code)
	}
	if !strings.Contains(stderr, "noiseFlore") {
		t.Fatalf("the error does not name the offending key: %q", stderr)
	}
}

func TestDiffReportsTheChangedPanel(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.png")
	after := filepath.Join(dir, "after.png")
	writePNG(t, before, 200)
	writePNG(t, after, 90)

	code, stdout, stderr := invoke(t, "diff", "--previous", before, "--current", after)
	if code != 0 {
		t.Fatalf("diff exited %d: %s", code, stderr)
	}

	var document map[string]any
	if err := json.Unmarshal([]byte(stdout), &document); err != nil {
		t.Fatalf("diff did not print one JSON document: %v\n%s", err, stdout)
	}
	if document["schemaVersion"] != "1.0" {
		t.Fatalf("document does not declare the schema version: %v", document["schemaVersion"])
	}
	regions, ok := document["regions"].([]any)
	if !ok || len(regions) != 1 {
		t.Fatalf("expected exactly one region, got %v", document["regions"])
	}
	first, _ := regions[0].(map[string]any)
	if first["class"] != "changed" {
		t.Fatalf("class = %v, want changed", first["class"])
	}
	if _, present := document["duration"]; present {
		t.Fatal("the document carries a timing field, which would break byte-identical output")
	}
}

func TestDiffIdenticalFramesReportsNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "same.png")
	writePNG(t, path, 200)

	code, stdout, stderr := invoke(t, "diff", "--previous", path, "--current", path)
	if code != 0 {
		t.Fatalf("diff exited %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, `"regions":[]`) && !strings.Contains(stdout, `"regions": []`) {
		t.Fatalf("identical frames produced regions: %s", stdout)
	}
}

func TestDiffWritesToTheDeclaredPathOnly(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.png")
	after := filepath.Join(dir, "after.png")
	writePNG(t, before, 200)
	writePNG(t, after, 90)
	out := filepath.Join(dir, "delta.json")

	code, stdout, stderr := invoke(t, "diff", "--previous", before, "--current", after, "--out", out)
	if code != 0 {
		t.Fatalf("diff exited %d: %s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("diff wrote to standard output as well as the file: %q", stdout)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("cannot read the output: %v", err)
	}
	if !strings.Contains(string(body), "\"schemaVersion\"") {
		t.Fatalf("the output file is not a document: %s", body)
	}
}

func TestDiffRejectsMismatchedRawDimensions(t *testing.T) {
	dir := t.TempDir()
	raw := filepath.Join(dir, "frame.raw")
	if err := os.WriteFile(raw, make([]byte, 8*8*4-1), 0o644); err != nil {
		t.Fatalf("cannot write the fixture: %v", err)
	}
	code, _, stderr := invoke(t, "diff", "--previous", raw, "--current", raw, "--width", "8", "--height", "8")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 for a buffer that does not match its dimensions: %s", code, stderr)
	}
	if !strings.Contains(stderr, "pixels") {
		t.Fatalf("the error does not name the field: %q", stderr)
	}
}

func TestStreamWritesOneDocumentPerFrame(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "001.png"), 200)
	writePNG(t, filepath.Join(dir, "002.png"), 90)
	writePNG(t, filepath.Join(dir, "003.png"), 200)

	code, stdout, stderr := invoke(t, "stream", "--source", dir)
	if code != 0 {
		t.Fatalf("stream exited %d: %s", code, stderr)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected one document per frame, got %d lines:\n%s", len(lines), stdout)
	}
	for index, line := range lines {
		var document map[string]any
		if err := json.Unmarshal([]byte(line), &document); err != nil {
			t.Fatalf("line %d is not a JSON document: %v", index+1, err)
		}
		frame := document["frame"].(map[string]any)
		if frame["sequence"].(float64) != float64(index+1) {
			t.Fatalf("line %d names sequence %v", index+1, frame["sequence"])
		}
	}
}

func TestFingerprintIsReportedForOneFrame(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frame.png")
	writePNG(t, path, 200)

	code, stdout, stderr := invoke(t, "fingerprint", "--frame", path)
	if code != 0 {
		t.Fatalf("fingerprint exited %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "grid-luma-1") || !strings.Contains(stdout, "strictHash") {
		t.Fatalf("fingerprint output is not a document with a fingerprint: %s", stdout)
	}
}

func TestRawStreamNeedsDimensions(t *testing.T) {
	code, _, stderr := invoke(t, "stream", "--source", "-")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1: %s", code, stderr)
	}
	if !strings.Contains(stderr, "--width") {
		t.Fatalf("the error does not say what is missing: %q", stderr)
	}
}

func TestReadErrorsExitTwo(t *testing.T) {
	code, _, stderr := invoke(t, "diff", "--previous", "/nonexistent/a.png", "--current", "/nonexistent/b.png")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 for an unreadable frame: %s", code, stderr)
	}
}

// TestCompareReportsEqualAndDifferent covers FR-009 through the command line: a frame compared against
// a fingerprint stored earlier, and the answer on standard output rather than in the exit code,
// because a comparison that ran is a success whichever way it came out.
func TestCompareReportsEqualAndDifferent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frame.png")
	writePNG(t, path, 200)

	stored := filepath.Join(dir, "stored.json")
	code, _, stderr := invoke(t, "fingerprint", "--frame", path, "--out", stored)
	if code != 0 {
		t.Fatalf("fingerprint exited %d: %s", code, stderr)
	}

	code, stdout, stderr := invoke(t, "compare", "--frame", path, "--document", stored)
	if code != 0 {
		t.Fatalf("compare exited %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "equal") {
		t.Fatalf("a frame compared against its own fingerprint said %q", stdout)
	}

	// A materially different screen must come out different.
	other := filepath.Join(dir, "other.png")
	writePNG(t, other, 40)
	code, stdout, stderr = invoke(t, "compare", "--frame", other, "--document", stored)
	if code != 0 {
		t.Fatalf("compare exited %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "different") {
		t.Fatalf("a different screen said %q", stdout)
	}
}

// TestCompareRejectsAnUnknownSchemaVersion covers NFR-010 and FR-015 through the command line: a
// document from a version this build does not understand is a contract problem, not bad input, which
// is what the third exit code is for.
func TestCompareRejectsAnUnknownSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	frame := filepath.Join(dir, "frame.png")
	writePNG(t, frame, 200)

	stored := filepath.Join(dir, "stored.json")
	if code, _, stderr := invoke(t, "fingerprint", "--frame", frame, "--out", stored); code != 0 {
		t.Fatalf("fingerprint exited %d: %s", code, stderr)
	}

	body, err := os.ReadFile(stored)
	if err != nil {
		t.Fatalf("cannot read the stored document: %v", err)
	}
	future := strings.Replace(string(body), `"schemaVersion":"1.0"`, `"schemaVersion":"2.0"`, 1)
	if future == string(body) {
		t.Fatalf("the stored document does not declare the schema version: %s", body)
	}
	futurePath := filepath.Join(dir, "future.json")
	if err := os.WriteFile(futurePath, []byte(future), 0o644); err != nil {
		t.Fatalf("cannot write the fixture: %v", err)
	}

	code, _, stderr := invoke(t, "compare", "--frame", frame, "--document", futurePath)
	if code != 3 {
		t.Fatalf("a document from an unknown schema version exited %d, want 3: %s", code, stderr)
	}
	if !strings.Contains(stderr, "schemaVersion") {
		t.Fatalf("the error does not name the field: %q", stderr)
	}
}

func TestCompareNeedsBothArguments(t *testing.T) {
	code, _, stderr := invoke(t, "compare", "--frame", "a.png")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1: %s", code, stderr)
	}
	if !strings.Contains(stderr, "--document") {
		t.Fatalf("the error does not say what is missing: %q", stderr)
	}
}

func TestCompareReportsAnUnreadableDocumentAsInput(t *testing.T) {
	code, _, _ := invoke(t, "compare", "--frame", "/nonexistent/frame.png", "--document", "/nonexistent/doc.json")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 for a missing file", code)
	}
}
