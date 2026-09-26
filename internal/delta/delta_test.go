package delta

import (
	"bytes"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func validDocument() Document {
	return Document{
		SchemaVersion: SchemaVersion,
		Frame: FrameRef{
			Sequence:    2,
			Width:       1920,
			Height:      1080,
			ScaleFactor: 1,
		},
		Fingerprint: Fingerprint{
			Algorithm:  "grid-luma-1",
			GridSize:   8,
			Cells:      make([]int, 64),
			StrictHash: "0123456789abcdef",
		},
		Regions: []Region{
			{
				Identity:  1,
				Class:     ClassChanged,
				Bounds:    Bounds{X: 0.1, Y: 0.2, W: 0.3, H: 0.1},
				Magnitude: 0.4,
				// 0.3 of 1920 by 0.1 of 1080 is 576 by 108 pixels.
				AreaPixels: 576 * 108,
			},
		},
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	original := validDocument()

	var encoded bytes.Buffer
	if err := original.Encode(&encoded, false); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if !reflect.DeepEqual(original.Regions, decoded.Regions) {
		t.Fatalf("regions changed across the round trip: %+v vs %+v", original.Regions, decoded.Regions)
	}
	if decoded.Frame != original.Frame {
		t.Fatalf("frame changed across the round trip: %+v", decoded.Frame)
	}
}

func TestEncodeIsByteIdenticalRegardlessOfInputOrder(t *testing.T) {
	first := validDocument()
	second := validDocument()
	second.Regions = []Region{
		{Identity: 3, Class: ClassChanged, Bounds: Bounds{X: 0.6, Y: 0.7, W: 0.1, H: 0.1}, Magnitude: 0.2, AreaPixels: 192 * 108},
		{Identity: 1, Class: ClassChanged, Bounds: Bounds{X: 0.1, Y: 0.2, W: 0.3, H: 0.1}, Magnitude: 0.4, AreaPixels: 576 * 108},
	}
	first.Regions = []Region{second.Regions[1], second.Regions[0]}
	// Two conditions in opposite orders, neither of them first-frame, because a
	// first-frame document may not carry regions.
	second.Conditions = []Condition{ConditionOutOfOrderTimestamp, ConditionViewportChanged}
	first.Conditions = []Condition{ConditionViewportChanged, ConditionOutOfOrderTimestamp, ConditionViewportChanged}

	var a, b bytes.Buffer
	if err := first.Encode(&a, false); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if err := second.Encode(&b, false); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatalf("encoding depends on input order:\n%s\n%s", a.String(), b.String())
	}
}

func TestEncodeCarriesNoTimingField(t *testing.T) {
	var encoded bytes.Buffer
	if err := validDocument().Encode(&encoded, false); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	body := strings.ToLower(encoded.String())
	for _, forbidden := range []string{"time", "duration", "elapsed", "timestamp\""} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("document contains %q, which would break byte-identical output: %s", forbidden, body)
		}
	}
}

func TestValidateRejectsBrokenInvariants(t *testing.T) {
	previous := Bounds{X: 0.1, Y: 0.2, W: 0.3, H: 0.1}

	cases := []struct {
		name   string
		break_ func(*Document)
		field  string
	}{
		{"wrong schema version", func(d *Document) { d.SchemaVersion = "2.0" }, "schemaVersion"},
		{"frame sequence zero", func(d *Document) { d.Frame.Sequence = 0 }, "frame.sequence"},
		{"frame width zero", func(d *Document) { d.Frame.Width = 0 }, "frame.width"},
		{"frame scale zero", func(d *Document) { d.Frame.ScaleFactor = 0 }, "frame.scaleFactor"},
		{"frame scale not finite", func(d *Document) { d.Frame.ScaleFactor = math.NaN() }, "frame.scaleFactor"},
		{"frame scale infinite", func(d *Document) { d.Frame.ScaleFactor = math.Inf(1) }, "frame.scaleFactor"},
		{"algorithm empty", func(d *Document) { d.Fingerprint.Algorithm = "" }, "fingerprint.algorithm"},
		{"grid size out of range", func(d *Document) { d.Fingerprint.GridSize = 4 }, "fingerprint.gridSize"},
		{"cell count mismatch", func(d *Document) { d.Fingerprint.Cells = d.Fingerprint.Cells[:10] }, "fingerprint.cells"},
		{"cell out of range", func(d *Document) { d.Fingerprint.Cells[0] = 300 }, "fingerprint.cells"},
		{"hash too short", func(d *Document) { d.Fingerprint.StrictHash = "abc" }, "fingerprint.strictHash"},
		{"identity zero", func(d *Document) { d.Regions[0].Identity = 0 }, "regions[0].identity"},
		{"unknown class", func(d *Document) { d.Regions[0].Class = "shifted" }, "regions[0].class"},
		{"added with previous bounds", func(d *Document) {
			d.Regions[0].Class = ClassAdded
			d.Regions[0].PreviousBounds = &previous
		}, "regions[0].previousBounds"},
		{"moved without previous bounds", func(d *Document) { d.Regions[0].Class = ClassMoved }, "regions[0].previousBounds"},
		{"bounds beyond frame", func(d *Document) { d.Regions[0].Bounds.X = 0.9 }, "regions[0].bounds"},
		{"negative size", func(d *Document) { d.Regions[0].Bounds.W = 0 }, "regions[0].bounds"},
		{"magnitude above one", func(d *Document) { d.Regions[0].Magnitude = 1.5 }, "regions[0].magnitude"},
		{"magnitude not finite", func(d *Document) { d.Regions[0].Magnitude = math.NaN() }, "regions[0].magnitude"},
		{"bounds not finite", func(d *Document) { d.Regions[0].Bounds.X = math.Inf(1) }, "regions[0].bounds"},
		{"width beyond one", func(d *Document) { d.Regions[0].Bounds.W = 1.0000000005 }, "regions[0].bounds"},
		{"algorithm not an identifier", func(d *Document) { d.Fingerprint.Algorithm = "BAD NAME" }, "fingerprint.algorithm"},
		{"hash not hexadecimal", func(d *Document) { d.Fingerprint.StrictHash = strings.Repeat("z", 16) }, "fingerprint.strictHash"},
		{"hash too long", func(d *Document) { d.Fingerprint.StrictHash = strings.Repeat("a", 129) }, "fingerprint.strictHash"},
		{"area beyond the frame", func(d *Document) { d.Regions[0].AreaPixels = 1920*1080 + 1 }, "regions[0].areaPixels"},
		{"area disagrees with bounds", func(d *Document) { d.Regions[0].AreaPixels = 1000 }, "regions[0].areaPixels"},
		{"area zero", func(d *Document) { d.Regions[0].AreaPixels = 0 }, "regions[0].areaPixels"},
		{"unknown condition", func(d *Document) { d.Conditions = []Condition{"scaled"} }, "conditions"},
		{"first frame with regions", func(d *Document) {
			d.Conditions = []Condition{ConditionFirstFrame}
		}, "regions"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := validDocument()
			tc.break_(&document)
			err := document.Validate()
			if err == nil {
				t.Fatalf("Validate accepted a document with %s broken", tc.field)
			}
			var fieldErr *FieldError
			if !asFieldError(err, &fieldErr) {
				t.Fatalf("Validate returned %T, want *FieldError", err)
			}
			if fieldErr.Field != tc.field {
				t.Fatalf("Validate blamed %q, want %q", fieldErr.Field, tc.field)
			}
			if !strings.Contains(err.Error(), "document") {
				t.Fatalf("error does not name the subject: %q", err.Error())
			}
		})
	}
}

func TestDecodeRejectsAnUnknownVersion(t *testing.T) {
	body := []byte(`{"schemaVersion":"9.9","frame":{"sequence":1,"width":2,"height":2,"scaleFactor":1},"fingerprint":{"algorithm":"grid-luma-1","gridSize":8,"cells":[],"strictHash":"0123456789abcdef"},"regions":[],"conditions":[]}`)
	_, err := Decode(bytes.NewReader(body))
	if err == nil {
		t.Fatal("Decode accepted an unknown schema version")
	}
	if !strings.Contains(err.Error(), "9.9") {
		t.Fatalf("error does not name the version it rejected: %q", err.Error())
	}
}

func TestSortRegionsOrderIsTotal(t *testing.T) {
	regions := []Region{
		{Identity: 9, Bounds: Bounds{X: 0.5, Y: 0.5, W: 0.1, H: 0.1}},
		{Identity: 4, Bounds: Bounds{X: 0.1, Y: 0.5, W: 0.1, H: 0.1}},
		{Identity: 2, Bounds: Bounds{X: 0.5, Y: 0.1, W: 0.1, H: 0.1}},
		{Identity: 1, Bounds: Bounds{X: 0.5, Y: 0.1, W: 0.1, H: 0.1}},
	}
	SortRegions(regions)
	got := []uint64{regions[0].Identity, regions[1].Identity, regions[2].Identity, regions[3].Identity}
	want := []uint64{1, 2, 4, 9}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SortRegions produced %v, want %v", got, want)
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

// TestSharedIdentitiesAreAllowedAndSortTotally states what replaced the old uniqueness rule.
//
// A moved element is reported as the area it left and the area it arrived in, and both are the same
// element, so both carry its identity. Uniqueness per document was wrong for that reason. What the
// ordering must still be is total, so two regions that share a position and an identity as well
// still sort deterministically.
func TestSharedIdentitiesAreAllowedAndSortTotally(t *testing.T) {
	document := validDocument()
	shared := document.Regions[0]
	shared.Class = ClassChanged
	moved := shared
	moved.Bounds = Bounds{X: 0.5, Y: 0.2, W: 0.3, H: 0.1}
	document.Regions = []Region{shared, moved}
	document.Conditions = []Condition{}

	if err := document.Validate(); err != nil {
		t.Fatalf("two regions sharing an identity were rejected: %v", err)
	}

	// The same rectangle twice: the ordering still has to be deterministic.
	twins := []Region{shared, shared}
	sorted := append([]Region(nil), twins...)
	SortRegions(sorted)
	if sorted[0].Identity != sorted[1].Identity {
		t.Fatalf("sorting changed the regions: %+v", sorted)
	}
}

// TestDecodeRejectsAMissingRequiredField closes what the review recorded as an open defect: a document that
// omits a required field decoded successfully, because unmarshalling gives a missing field the same value as
// an explicitly zero one. The engine always writes every field, so the exposure was a consumer reading a
// document written by something else and believing it had values it did not have.
func TestDecodeRejectsAMissingRequiredField(t *testing.T) {
	cells := make([]string, 64)
	for index := range cells {
		cells[index] = "0"
	}
	valid := `{"schemaVersion":"1.0","frame":{"sequence":2,"width":640,"height":480,"scaleFactor":1},` +
		`"fingerprint":{"algorithm":"grid-luma-1","gridSize":8,"cells":[` + strings.Join(cells, ",") + `],"strictHash":"0123456789abcdef"},` +
		`"regions":[{"identity":1,"class":"changed","bounds":{"x":0.1,"y":0.1,"w":0.2,"h":0.2},"magnitude":0.5,"areaPixels":12288,` +
		`"identityConfidence":1,"identityUncertain":false}],"conditions":[]}`

	if _, err := Decode(strings.NewReader(valid)); err != nil {
		t.Fatalf("the valid document was rejected: %v", err)
	}

	cases := []struct {
		name  string
		body  string
		field string
	}{
		{name: "identity confidence", field: "regions[0].identityConfidence",
			body: strings.Replace(valid, `,"identityConfidence":1`, "", 1)},
		{name: "identity uncertainty", field: "regions[0].identityUncertain",
			body: strings.Replace(valid, `,"identityUncertain":false`, "", 1)},
		{name: "identity", field: "regions[0].identity",
			body: strings.Replace(valid, `"identity":1,`, "", 1)},
		{name: "area", field: "regions[0].areaPixels",
			body: strings.Replace(valid, `"areaPixels":12288,`, "", 1)},
		{name: "frame sequence", field: "frame.sequence",
			body: strings.Replace(valid, `"sequence":2,`, "", 1)},
		{name: "frame scale factor", field: "frame.scaleFactor",
			body: strings.Replace(valid, `,"scaleFactor":1`, "", 1)},
		{name: "fingerprint hash", field: "fingerprint.strictHash",
			body: strings.Replace(valid, `,"strictHash":"0123456789abcdef"`, "", 1)},
		{name: "conditions", field: "conditions",
			body: strings.Replace(valid, `,"conditions":[]`, "", 1)},
		{name: "null identity confidence", field: "regions[0].identityConfidence",
			body: strings.Replace(valid, `"identityConfidence":1`, `"identityConfidence":null`, 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.body == valid {
				t.Fatalf("the fixture for %s did not change the document, so this test would prove nothing", tc.name)
			}
			_, err := Decode(strings.NewReader(tc.body))
			if err == nil {
				t.Fatalf("a document missing %s was accepted", tc.field)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("the error does not name %s: %v", tc.field, err)
			}
		})
	}
}

// goldenDocument is the fixed document the golden file records: one changed region, one moved region and one
// condition, so the file exercises ordering, the class-specific rules and the absence of timing fields.
func goldenDocument() Document {
	previous := Bounds{X: 0.1, Y: 0.2, W: 0.2, H: 0.1}
	document := Document{
		SchemaVersion: SchemaVersion,
		Frame:         FrameRef{Sequence: 7, Width: 1920, Height: 1080, ScaleFactor: 1},
		Fingerprint: Fingerprint{
			Algorithm:  "grid-luma-1",
			GridSize:   8,
			Cells:      []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63},
			StrictHash: "0123456789abcdef",
		},
		Regions: []Region{
			{Identity: 1, Class: ClassChanged, Bounds: Bounds{X: 0.25, Y: 0.3, W: 0.1, H: 0.05}, Magnitude: 0.42, AreaPixels: 192 * 54, IdentityConfidence: 1},
			{Identity: 2, Class: ClassMoved, Bounds: Bounds{X: 0.6, Y: 0.7, W: 0.05, H: 0.04}, PreviousBounds: &previous, Magnitude: 0.31, AreaPixels: 96 * 43, IdentityConfidence: 0.64, IdentityUncertain: true},
		},
		Conditions: []Condition{ConditionViewportChanged},
	}
	SortRegions(document.Regions)
	return document
}

// TestTheEncoderReproducesTheGoldenDocument pins the wire format. A change to field order, to a key name or to
// the way a value is written shows up here as a failing test rather than as a consumer that stops parsing,
// and the file can be regenerated deliberately with GENERATE_GOLDEN=1.
func TestTheEncoderReproducesTheGoldenDocument(t *testing.T) {
	expected, err := os.ReadFile("../../testdata/golden/document.json")
	if err != nil {
		t.Fatalf("cannot read the golden document: %v", err)
	}

	var encoded bytes.Buffer
	if err := goldenDocument().Encode(&encoded, true); err != nil {
		t.Fatalf("encoding failed: %v", err)
	}
	if encoded.String() != string(expected) {
		t.Fatalf("the encoded document differs from the golden file:\n--- encoded ---\n%s\n--- golden ---\n%s",
			encoded.String(), string(expected))
	}

	// The golden document has to be one the engine would accept, or it is a fixture for a format nothing else
	// agrees with.
	decoded, err := Decode(strings.NewReader(string(expected)))
	if err != nil {
		t.Fatalf("the golden document does not decode: %v", err)
	}
	if len(decoded.Regions) != 2 {
		t.Fatalf("the golden document decoded into %d regions", len(decoded.Regions))
	}
}

// TestGenerateGolden regenerates the golden document, so the file can be refreshed deliberately rather than
// by hand when the wire format changes on purpose.
func TestGenerateGolden(t *testing.T) {
	if os.Getenv("GENERATE_GOLDEN") != "1" {
		t.Skip("set GENERATE_GOLDEN=1 to regenerate testdata/golden/document.json")
	}
	file, err := os.Create("../../testdata/golden/document.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := goldenDocument().Encode(file, true); err != nil {
		t.Fatal(err)
	}
}
