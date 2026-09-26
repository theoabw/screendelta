// Package config holds the caller's control surface: the thresholds, the ignored
// areas, the optional regions of interest and the output options.
//
// Parsing is strict on purpose. An unknown key is an error rather than a silently
// ignored setting, because a typo that changes nothing is the failure that costs an
// afternoon. Ranges are checked here so no other package has to defend itself
// against a nonsensical threshold.
package config

import (
	"encoding/json"
	"io"
	"os"

	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/fielderr"
)

// Defaults are the values documented in the data model and the configuration
// schema. They are what an empty configuration document means.
const (
	DefaultNoiseFloor            = 0.02
	DefaultMinRegionAreaPixels   = 64
	DefaultMotionTolerancePixels = 8
	DefaultOcclusionFrames       = 3
	DefaultGridSize              = 32
	DefaultMaxCellDelta          = 4
)

// Output formats.
const (
	FormatJSON   = "json"
	FormatNDJSON = "ndjson"
)

// RegionOfInterest restricts reporting to an area, labelled so the caller can tell
// which request a region answers.
type RegionOfInterest struct {
	Label  string       `json:"label"`
	Bounds delta.Bounds `json:"bounds"`
}

// Output controls how documents are written.
type Output struct {
	Format string `json:"format"`
	Pretty bool   `json:"pretty"`
}

// FingerprintOptions controls the screen fingerprint.
type FingerprintOptions struct {
	GridSize     int `json:"gridSize"`
	MaxCellDelta int `json:"maxCellDelta"`
}

// Config is the validated configuration.
//
// RegionsOfInterest is a pointer so that an absent field and an empty list stay
// distinguishable: absent means unrestricted, empty means report nothing. Those are
// different requests and the engine must not conflate them.
type Config struct {
	NoiseFloor            float64             `json:"noiseFloor"`
	MinRegionAreaPixels   int                 `json:"minRegionAreaPixels"`
	MotionTolerancePixels int                 `json:"motionTolerancePixels"`
	OcclusionFrames       int                 `json:"occlusionFrames"`
	IgnoredAreas          []delta.Bounds      `json:"ignoredAreas"`
	RegionsOfInterest     *[]RegionOfInterest `json:"regionsOfInterest,omitempty"`
	Output                Output              `json:"output"`
	Fingerprint           FingerprintOptions  `json:"fingerprint"`
}

// Defaults returns a configuration that passes validation.
func Defaults() Config {
	return Config{
		NoiseFloor:            DefaultNoiseFloor,
		MinRegionAreaPixels:   DefaultMinRegionAreaPixels,
		MotionTolerancePixels: DefaultMotionTolerancePixels,
		OcclusionFrames:       DefaultOcclusionFrames,
		IgnoredAreas:          []delta.Bounds{},
		Output: Output{
			Format: FormatJSON,
			Pretty: false,
		},
		Fingerprint: FingerprintOptions{
			GridSize:     DefaultGridSize,
			MaxCellDelta: DefaultMaxCellDelta,
		},
	}
}

// Parse reads a configuration document over the defaults, so a partial document
// changes only what it names, and rejects anything it does not recognise.
func Parse(r io.Reader) (Config, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return Config{}, &FieldError{
			Op:      "config.Parse",
			Subject: "config",
			Field:   "document",
			Problem: "cannot read: " + err.Error(),
		}
	}

	// The token walk runs first: it catches duplicate members and null values, which a
	// map based check cannot see. Then the exact key check, because the decoder matches
	// keys case-insensitively and would accept "gridsize" for "gridSize".
	if err := checkMembers(raw); err != nil {
		return Config{}, err
	}
	if err := checkKeys(raw); err != nil {
		return Config{}, err
	}

	cfg := Defaults()
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, &FieldError{
			Op:      "config.Parse",
			Subject: "config",
			Field:   "document",
			Problem: "cannot parse: " + err.Error(),
		}
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Load reads a configuration file.
func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, &FieldError{
			Op:      "config.Load",
			Subject: "config",
			Field:   "path",
			Problem: "cannot open " + path + ": " + err.Error(),
		}
	}
	defer file.Close()

	cfg, err := Parse(file)
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks every field against its documented range and names the first one
// that is wrong.
func (c Config) Validate() error {
	if err := inRange("noiseFloor", c.NoiseFloor, 0, 1); err != nil {
		return err
	}
	if err := intInRange("minRegionAreaPixels", c.MinRegionAreaPixels, 1, 1000000); err != nil {
		return err
	}
	if err := intInRange("motionTolerancePixels", c.MotionTolerancePixels, 0, 512); err != nil {
		return err
	}
	if err := intInRange("occlusionFrames", c.OcclusionFrames, 0, 600); err != nil {
		return err
	}
	if c.Output.Format != FormatJSON && c.Output.Format != FormatNDJSON {
		return &FieldError{
			Op:      "config.Validate",
			Subject: "config",
			Field:   "output.format",
			Problem: "unknown format " + quote(c.Output.Format) + ", expected " + quote(FormatJSON) + " or " + quote(FormatNDJSON),
		}
	}
	if err := intInRange("fingerprint.gridSize", c.Fingerprint.GridSize, 8, 256); err != nil {
		return err
	}
	if err := intInRange("fingerprint.maxCellDelta", c.Fingerprint.MaxCellDelta, 0, 255); err != nil {
		return err
	}
	for index, area := range c.IgnoredAreas {
		if err := validateBounds("ignoredAreas["+itoa(index)+"]", area); err != nil {
			return err
		}
	}
	if c.RegionsOfInterest != nil {
		for index, roi := range *c.RegionsOfInterest {
			where := "regionsOfInterest[" + itoa(index) + "]"
			if roi.Label == "" {
				return &FieldError{Op: "config.Validate", Subject: "config", Field: where + ".label", Problem: "must not be empty"}
			}
			if err := validateBounds(where+".bounds", roi.Bounds); err != nil {
				return err
			}
		}
	}
	return nil
}

// Unrestricted reports whether every region is eligible for reporting. An empty but
// present regions-of-interest list is not unrestricted: it means report nothing.
func (c Config) Unrestricted() bool {
	return c.RegionsOfInterest == nil
}

// SuppressesAllRegions reports whether the configuration asks for no regions at all.
func (c Config) SuppressesAllRegions() bool {
	return c.RegionsOfInterest != nil && len(*c.RegionsOfInterest) == 0
}

func validateBounds(field string, b delta.Bounds) error {
	if b.X < 0 || b.Y < 0 || b.W <= 0 || b.H <= 0 {
		return &FieldError{
			Op:      "config.Validate",
			Subject: "config",
			Field:   field,
			Problem: "must have non-negative origin and positive size",
		}
	}
	if b.X+b.W > 1 || b.Y+b.H > 1 {
		return &FieldError{
			Op:      "config.Validate",
			Subject: "config",
			Field:   field,
			Problem: "extends beyond the frame",
		}
	}
	return nil
}

func inRange(field string, value, low, high float64) error {
	if value < low || value > high {
		return &FieldError{
			Op:      "config.Validate",
			Subject: "config",
			Field:   field,
			Problem: "value " + formatFloat(value) + " is outside " + formatFloat(low) + " to " + formatFloat(high),
		}
	}
	return nil
}

func intInRange(field string, value, low, high int) error {
	if value < low || value > high {
		return fielderr.Range(field, low, high, value).At("config.Validate", "config", 0)
	}
	return nil
}
