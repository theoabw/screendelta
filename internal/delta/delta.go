// Package delta holds the document model and nothing else.
//
// It imports no image code on purpose: a consumer that only reads documents, or a
// test that only checks their shape, should not drag a decoder into the build. That
// separation is what lets a second consumer be written against the schema alone.
package delta

import (
	"math"
	"regexp"
	"sort"

	"github.com/theoabw/screendelta/internal/fielderr"
)

// SchemaVersion is the document generation this package produces and the only one
// it accepts. Removing a field, changing a type or changing region ordering starts a
// new generation.
const SchemaVersion = "1.0"

// boundsTolerance absorbs floating point error when checking that a region fits
// inside the frame, so a region that lands exactly on an edge is not rejected.
const boundsTolerance = 1e-9

// areaTolerancePercent is the slack allowed between the area a consumer is told and
// the area its bounds imply, because a region is rounded to whole pixels somewhere and
// the two cannot be expected to agree to the byte.
const areaTolerancePercent = 1

// The schema states these patterns, so validation has to enforce them: a document that
// encodes successfully must not be rejectable by an independent consumer's validator.
var (
	algorithmPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	hashPattern      = regexp.MustCompile(`^[0-9a-f]{16,128}$`)
)

// Bounds is a rectangle normalized to the frame extent, so a consumer is
// independent of resolution.
type Bounds struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// FrameRef identifies the frame a document describes.
type FrameRef struct {
	Sequence    uint64  `json:"sequence"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	ScaleFactor float64 `json:"scaleFactor"`
}

// Fingerprint is a compact, noise-stable summary of a frame.
type Fingerprint struct {
	Algorithm  string `json:"algorithm"`
	GridSize   int    `json:"gridSize"`
	Cells      []int  `json:"cells"`
	StrictHash string `json:"strictHash"`
}

// RegionClass says how a region differs from the previous frame.
type RegionClass string

const (
	ClassAdded   RegionClass = "added"
	ClassChanged RegionClass = "changed"
	ClassRemoved RegionClass = "removed"
	ClassMoved   RegionClass = "moved"
)

// Region is one rectangular area reported as different.
type Region struct {
	// Identity is session-scoped and never reused, even after retirement.
	Identity uint64      `json:"identity"`
	Class    RegionClass `json:"class"`
	Bounds   Bounds      `json:"bounds"`
	// PreviousBounds is required for moved and removed regions and forbidden for
	// added ones, because an added region has no previous position by definition.
	PreviousBounds *Bounds `json:"previousBounds,omitempty"`
	Magnitude      float64 `json:"magnitude"`
	AreaPixels     int     `json:"areaPixels"`
	// IdentityConfidence is the strength of the evidence that this region is the element
	// its identity refers to. It is required rather than optional so a consumer never has to
	// tell "certain" apart from "not stated".
	IdentityConfidence float64 `json:"identityConfidence"`
	// IdentityUncertain is true when the identity could not be re-established and was
	// re-acquired, which is the case for an element that returns after being occluded. A
	// consumer that needs a stable handle treats such an identity as new.
	IdentityUncertain bool `json:"identityUncertain"`
}

// Condition explains why a document reports what it does, or does not.
type Condition string

const (
	ConditionFirstFrame          Condition = "first-frame"
	ConditionViewportChanged     Condition = "viewport-changed"
	ConditionOutOfOrderTimestamp Condition = "out-of-order-timestamp"
)

// conditionRank fixes the order conditions appear in, so encoding is deterministic
// regardless of the order the engine discovered them.
var conditionRank = map[Condition]int{
	ConditionFirstFrame:          0,
	ConditionViewportChanged:     1,
	ConditionOutOfOrderTimestamp: 2,
}

// Document is the product: one self-contained document per frame.
//
// There is no timing field anywhere in this struct, and adding one would break
// NFR-004. Performance numbers belong to the benchmark and the report.
type Document struct {
	SchemaVersion string      `json:"schemaVersion"`
	Frame         FrameRef    `json:"frame"`
	Fingerprint   Fingerprint `json:"fingerprint"`
	Regions       []Region    `json:"regions"`
	Conditions    []Condition `json:"conditions"`
}

// SortRegions puts regions in the order the contract requires: top edge, left edge, identity, then size.
//
// The size keys are part of the order rather than decoration. Two regions can share a position and an
// identity, because one element can be reported as more than one region, and a key of position and identity
// alone leaves those two in whatever order they arrived in. A fuzz target found this: the comment claimed a
// total order while the comparison had three keys, so the claim was false for exactly the case the order
// exists to settle. Adding the size makes the comparison total, which is what makes the encoded document
// independent of the order the classifier discovered its regions in.
func SortRegions(regions []Region) {
	sort.SliceStable(regions, func(i, j int) bool {
		a, b := regions[i], regions[j]
		if a.Bounds.Y != b.Bounds.Y {
			return a.Bounds.Y < b.Bounds.Y
		}
		if a.Bounds.X != b.Bounds.X {
			return a.Bounds.X < b.Bounds.X
		}
		if a.Identity != b.Identity {
			return a.Identity < b.Identity
		}
		if a.Bounds.W != b.Bounds.W {
			return a.Bounds.W < b.Bounds.W
		}
		return a.Bounds.H < b.Bounds.H
	})
}

// Ordered reports whether regions are already in the contract's order. Validate uses it, so a document that
// arrives out of order from somewhere else is rejected rather than accepted and re-ordered silently.
func Ordered(regions []Region) bool {
	for index := 1; index < len(regions); index++ {
		if regionLess(regions[index], regions[index-1]) {
			return false
		}
	}
	return true
}

// regionLess is the contract's ordering as a predicate, so SortRegions, Ordered and the validator cannot
// drift apart.
func regionLess(a, b Region) bool {
	if a.Bounds.Y != b.Bounds.Y {
		return a.Bounds.Y < b.Bounds.Y
	}
	if a.Bounds.X != b.Bounds.X {
		return a.Bounds.X < b.Bounds.X
	}
	if a.Identity != b.Identity {
		return a.Identity < b.Identity
	}
	if a.Bounds.W != b.Bounds.W {
		return a.Bounds.W < b.Bounds.W
	}
	return a.Bounds.H < b.Bounds.H
}

// SortConditions puts conditions in their canonical order and removes duplicates,
// so two runs that discovered the same conditions in different orders encode the
// same bytes.
func SortConditions(conditions []Condition) []Condition {
	if len(conditions) == 0 {
		return nil
	}
	seen := make(map[Condition]bool, len(conditions))
	unique := make([]Condition, 0, len(conditions))
	for _, condition := range conditions {
		if seen[condition] {
			continue
		}
		seen[condition] = true
		unique = append(unique, condition)
	}
	sort.SliceStable(unique, func(i, j int) bool {
		return conditionRank[unique[i]] < conditionRank[unique[j]]
	})
	return unique
}

// Validate checks the invariants a JSON Schema cannot express, and the ones it can,
// so an invalid document cannot be encoded.
func (d Document) Validate() error {
	if d.SchemaVersion != SchemaVersion {
		return &FieldError{
			Op:      "delta.Validate",
			Subject: "document",
			Field:   "schemaVersion",
			Problem: "unsupported version " + quote(d.SchemaVersion) + ", expected " + quote(SchemaVersion),
		}
	}
	if err := validateFrame(d.Frame); err != nil {
		return err
	}
	if err := validateFingerprint(d.Fingerprint); err != nil {
		return err
	}
	// A first frame has no predecessor, so it cannot report content changes.
	if hasCondition(d.Conditions, ConditionFirstFrame) && len(d.Regions) > 0 {
		return &FieldError{
			Op:      "delta.Validate",
			Subject: "document",
			Field:   "regions",
			Problem: "must be empty when the first-frame condition is present",
		}
	}

	for index, region := range d.Regions {
		if err := validateRegion(index, region, d.Frame.Width, d.Frame.Height); err != nil {
			return err
		}
	}
	for _, condition := range d.Conditions {
		if _, known := conditionRank[condition]; !known {
			return &FieldError{
				Op:      "delta.Validate",
				Subject: "document",
				Field:   "conditions",
				Problem: "unknown condition " + quote(string(condition)),
			}
		}
	}
	return nil
}

func validateFrame(f FrameRef) error {
	if f.Sequence == 0 {
		return &FieldError{Op: "delta.Validate", Subject: "document", Field: "frame.sequence", Problem: "must be at least 1"}
	}
	if f.Width < 1 {
		return &FieldError{Op: "delta.Validate", Subject: "document", Field: "frame.width", Problem: "must be at least 1"}
	}
	if f.Height < 1 {
		return &FieldError{Op: "delta.Validate", Subject: "document", Field: "frame.height", Problem: "must be at least 1"}
	}
	if !isFinite(f.ScaleFactor) {
		return &FieldError{Op: "delta.Validate", Subject: "document", Field: "frame.scaleFactor", Problem: "must be a finite number, not NaN or infinity"}
	}
	if f.ScaleFactor <= 0 {
		return &FieldError{Op: "delta.Validate", Subject: "document", Field: "frame.scaleFactor", Problem: "must be greater than 0"}
	}
	return nil
}

func validateFingerprint(fp Fingerprint) error {
	if !algorithmPattern.MatchString(fp.Algorithm) {
		return &FieldError{
			Op:      "delta.Validate",
			Subject: "document",
			Field:   "fingerprint.algorithm",
			Problem: "must be a lower case identifier such as " + quote("grid-luma-1"),
		}
	}
	if fp.GridSize < 8 || fp.GridSize > 256 {
		return fielderr.Range("fingerprint.gridSize", 8, 256, fp.GridSize).At("delta.Validate", "document", 0)
	}
	if len(fp.Cells) != fp.GridSize*fp.GridSize {
		return &FieldError{
			Op:      "delta.Validate",
			Subject: "document",
			Field:   "fingerprint.cells",
			Problem: "length must be gridSize squared",
		}
	}
	for _, cell := range fp.Cells {
		if cell < 0 || cell > 255 {
			return fielderr.Range("fingerprint.cells", 0, 255, cell).At("delta.Validate", "document", 0)
		}
	}
	if !hashPattern.MatchString(fp.StrictHash) {
		return &FieldError{
			Op:      "delta.Validate",
			Subject: "document",
			Field:   "fingerprint.strictHash",
			Problem: "must be 16 to 128 lower case hexadecimal characters",
		}
	}
	return nil
}

func validateRegion(index int, region Region, frameWidth, frameHeight int) error {
	where := "regions[" + itoa(index) + "]"
	if region.Identity == 0 {
		return &FieldError{Op: "delta.Validate", Subject: "document", Field: where + ".identity", Problem: "must be at least 1"}
	}
	switch region.Class {
	case ClassAdded:
		if region.PreviousBounds != nil {
			return &FieldError{Op: "delta.Validate", Subject: "document", Field: where + ".previousBounds", Problem: "must be absent for an added region"}
		}
	case ClassChanged:
	case ClassRemoved, ClassMoved:
		if region.PreviousBounds == nil {
			return &FieldError{Op: "delta.Validate", Subject: "document", Field: where + ".previousBounds", Problem: "is required for a " + string(region.Class) + " region"}
		}
	default:
		return &FieldError{Op: "delta.Validate", Subject: "document", Field: where + ".class", Problem: "unknown class " + quote(string(region.Class))}
	}
	if err := validateBounds(where+".bounds", region.Bounds); err != nil {
		return err
	}
	if region.PreviousBounds != nil {
		if err := validateBounds(where+".previousBounds", *region.PreviousBounds); err != nil {
			return err
		}
	}
	if !isFinite(region.IdentityConfidence) || region.IdentityConfidence < 0 || region.IdentityConfidence > 1 {
		return &FieldError{
			Op:      "delta.Validate",
			Subject: "document",
			Field:   where + ".identityConfidence",
			Problem: "must be a finite number between 0 and 1",
		}
	}
	if !isFinite(region.Magnitude) || region.Magnitude < 0 || region.Magnitude > 1 {
		return &FieldError{Op: "delta.Validate", Subject: "document", Field: where + ".magnitude", Problem: "must be a finite number between 0 and 1"}
	}
	if region.AreaPixels < 1 {
		return &FieldError{Op: "delta.Validate", Subject: "document", Field: where + ".areaPixels", Problem: "must be at least 1"}
	}
	framePixels := frameWidth * frameHeight
	if region.AreaPixels > framePixels {
		return &FieldError{
			Op:      "delta.Validate",
			Subject: "document",
			Field:   where + ".areaPixels",
			Problem: "exceeds the " + itoa(framePixels) + " pixels of the frame it describes",
		}
	}
	// The area a consumer is given has to agree with the area the bounds imply, or the
	// document contradicts itself.
	implied := int(math.Round(region.Bounds.W*float64(frameWidth))) * int(math.Round(region.Bounds.H*float64(frameHeight)))
	slack := implied * areaTolerancePercent / 100
	if slack < 1 {
		slack = 1
	}
	if difference := region.AreaPixels - implied; difference > slack || difference < -slack {
		return &FieldError{
			Op:      "delta.Validate",
			Subject: "document",
			Field:   where + ".areaPixels",
			Problem: "is " + itoa(region.AreaPixels) + " but the bounds imply " + itoa(implied),
		}
	}
	return nil
}

func validateBounds(field string, b Bounds) error {
	for _, value := range []float64{b.X, b.Y, b.W, b.H} {
		if !isFinite(value) {
			return &FieldError{Op: "delta.Validate", Subject: "document", Field: field, Problem: "must be finite, not NaN or infinity"}
		}
	}
	// Each coordinate is checked against the schema's own limits, separately from the
	// tolerance used for the sum below: a width slightly over one is still a width the
	// schema rejects.
	if b.X < 0 || b.X > 1 || b.Y < 0 || b.Y > 1 || b.W <= 0 || b.W > 1 || b.H <= 0 || b.H > 1 {
		return &FieldError{Op: "delta.Validate", Subject: "document", Field: field, Problem: "must have an origin and size inside 0 to 1"}
	}
	if b.X+b.W > 1+boundsTolerance || b.Y+b.H > 1+boundsTolerance {
		return &FieldError{Op: "delta.Validate", Subject: "document", Field: field, Problem: "extends beyond the frame"}
	}
	return nil
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func hasCondition(conditions []Condition, want Condition) bool {
	for _, condition := range conditions {
		if condition == want {
			return true
		}
	}
	return false
}
