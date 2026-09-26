package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/theoabw/screendelta/internal/delta"
)

func TestDefaultsAreValid(t *testing.T) {
	cfg := Defaults()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Defaults do not validate: %v", err)
	}
	if cfg.NoiseFloor != DefaultNoiseFloor || cfg.Fingerprint.GridSize != DefaultGridSize {
		t.Fatalf("Defaults do not carry the documented values: %+v", cfg)
	}
	if !cfg.Unrestricted() {
		t.Fatal("Defaults should be unrestricted: no regions-of-interest list at all")
	}
}

func TestParseOverlaysPartialDocumentsOnDefaults(t *testing.T) {
	cfg, err := Parse(strings.NewReader(`{"noiseFloor":0.1,"output":{"pretty":true}}`))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if cfg.NoiseFloor != 0.1 {
		t.Fatalf("noiseFloor = %v, want 0.1", cfg.NoiseFloor)
	}
	if !cfg.Output.Pretty {
		t.Fatal("output.pretty was not applied")
	}
	if cfg.Output.Format != FormatJSON {
		t.Fatalf("output.format = %q, want the default %q", cfg.Output.Format, FormatJSON)
	}
	if cfg.Fingerprint.GridSize != DefaultGridSize {
		t.Fatalf("an unrelated nested default was lost: %+v", cfg.Fingerprint)
	}
}

func TestParseRejectsUnknownKeys(t *testing.T) {
	cases := []struct {
		name string
		body string
		key  string
	}{
		{"top level", `{"noiseFlore":0.1}`, "noiseFlore"},
		{"nested", `{"output":{"prety":true}}`, "prety"},
		{"fingerprint", `{"fingerprint":{"gridsize":32}}`, "gridsize"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tc.body))
			if err == nil {
				t.Fatalf("Parse accepted the unknown key %q", tc.key)
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("error does not name the unknown key: %q", err.Error())
			}
		})
	}
}

func TestValidateRejectsOutOfRangeValues(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		field string
	}{
		{"noise floor above one", `{"noiseFloor":1.5}`, "noiseFloor"},
		{"noise floor negative", `{"noiseFloor":-0.1}`, "noiseFloor"},
		{"area below one", `{"minRegionAreaPixels":0}`, "minRegionAreaPixels"},
		{"motion tolerance too large", `{"motionTolerancePixels":513}`, "motionTolerancePixels"},
		{"occlusion frames too large", `{"occlusionFrames":601}`, "occlusionFrames"},
		{"unknown format", `{"output":{"format":"yaml"}}`, "output.format"},
		{"grid too small", `{"fingerprint":{"gridSize":4}}`, "fingerprint.gridSize"},
		{"cell delta too large", `{"fingerprint":{"maxCellDelta":256}}`, "fingerprint.maxCellDelta"},
		{"ignored area beyond frame", `{"ignoredAreas":[{"x":0.9,"y":0,"w":0.2,"h":0.1}]}`, "ignoredAreas[0]"},
		{"ignored area zero size", `{"ignoredAreas":[{"x":0,"y":0,"w":0,"h":0.1}]}`, "ignoredAreas[0]"},
		{"label empty", `{"regionsOfInterest":[{"label":"","bounds":{"x":0,"y":0,"w":0.5,"h":0.5}}]}`, "regionsOfInterest[0].label"},
		{"roi beyond frame", `{"regionsOfInterest":[{"label":"x","bounds":{"x":0.5,"y":0,"w":0.9,"h":0.1}}]}`, "regionsOfInterest[0].bounds"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tc.body))
			if err == nil {
				t.Fatalf("Parse accepted %s", tc.name)
			}
			var fieldErr *FieldError
			if !asFieldError(err, &fieldErr) {
				t.Fatalf("Parse returned %T, want *FieldError", err)
			}
			if fieldErr.Field != tc.field {
				t.Fatalf("Parse blamed %q, want %q", fieldErr.Field, tc.field)
			}
		})
	}
}

func TestAbsentAndEmptyRegionsOfInterestMeanDifferentThings(t *testing.T) {
	absent, err := Parse(strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if !absent.Unrestricted() || absent.SuppressesAllRegions() {
		t.Fatalf("absent list should be unrestricted: %+v", absent)
	}

	empty, err := Parse(strings.NewReader(`{"regionsOfInterest":[]}`))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if empty.Unrestricted() {
		t.Fatal("an empty list should not be unrestricted")
	}
	if !empty.SuppressesAllRegions() {
		t.Fatal("an empty list should suppress every region")
	}

	provided, err := Parse(strings.NewReader(`{"regionsOfInterest":[{"label":"toolbar","bounds":{"x":0,"y":0,"w":1,"h":0.2}}]}`))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	want := []RegionOfInterest{{Label: "toolbar", Bounds: delta.Bounds{X: 0, Y: 0, W: 1, H: 0.2}}}
	if !reflect.DeepEqual(*provided.RegionsOfInterest, want) {
		t.Fatalf("regions of interest = %+v, want %+v", *provided.RegionsOfInterest, want)
	}
}

func TestValidateRejectsAMalformedDocument(t *testing.T) {
	_, err := Parse(strings.NewReader(`{"noiseFloor":`))
	if err == nil {
		t.Fatal("Parse accepted a truncated document")
	}
	if !strings.Contains(err.Error(), "config") {
		t.Fatalf("error does not name the subject: %q", err.Error())
	}
}

func asFieldError(err error, target **FieldError) bool {
	fieldErr, ok := err.(*FieldError)
	if !ok {
		return false
	}
	*target = fieldErr
	return true
}

func TestParseRejectsAMiscasedKeyWithASuggestion(t *testing.T) {
	// Go's JSON decoder matches field names case-insensitively, so this case is the
	// one the exact key check exists for.
	_, err := Parse(strings.NewReader(`{"fingerprint":{"gridsize":32}}`))
	if err == nil {
		t.Fatal("Parse accepted a miscased key")
	}
	if !strings.Contains(err.Error(), "did you mean") || !strings.Contains(err.Error(), "gridSize") {
		t.Fatalf("error does not suggest the intended key: %q", err.Error())
	}
}

func TestParseRejectsUnknownKeysInsideArrays(t *testing.T) {
	cases := []string{
		`{"ignoredAreas":[{"x":0,"y":0,"w":0.1,"h":0.1,"z":1}]}`,
		`{"regionsOfInterest":[{"label":"a","bounds":{"x":0,"y":0,"w":0.1,"h":0.1},"extra":1}]}`,
		`{"regionsOfInterest":[{"label":"a","bounds":{"x":0,"y":0,"w":0.1,"depth":0.1}}]}`,
	}
	for _, body := range cases {
		if _, err := Parse(strings.NewReader(body)); err == nil {
			t.Fatalf("Parse accepted an unknown key inside an array: %s", body)
		}
	}
}
