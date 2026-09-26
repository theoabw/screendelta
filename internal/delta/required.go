package delta

import (
	"bytes"
	"encoding/json"
)

// This file answers a question that a Go struct cannot: was a required field present at all?
//
// Unmarshalling into a struct gives a missing field the same value as an explicitly zero one, so a document
// that omitted identity, identityConfidence or identityUncertain decoded into a valid looking region. The
// engine always writes them, but Decode is also the library's entry point for a document written by
// something else, and a consumer that trusts it should be reading a document the schema accepts.
//
// The check is deliberately separate from Validate. Validate reasons about values and runs on every encode
// as well, which has no raw JSON to inspect; presence can only be answered before the struct is built.

// presence mirrors the schema's required fields as pointers, so nil means the key was absent.
type presence struct {
	SchemaVersion *json.RawMessage `json:"schemaVersion"`
	Frame         *struct {
		Sequence    *json.RawMessage `json:"sequence"`
		Width       *json.RawMessage `json:"width"`
		Height      *json.RawMessage `json:"height"`
		ScaleFactor *json.RawMessage `json:"scaleFactor"`
	} `json:"frame"`
	Fingerprint *struct {
		Algorithm  *json.RawMessage `json:"algorithm"`
		GridSize   *json.RawMessage `json:"gridSize"`
		Cells      *json.RawMessage `json:"cells"`
		StrictHash *json.RawMessage `json:"strictHash"`
	} `json:"fingerprint"`
	Regions *[]struct {
		Identity           *json.RawMessage `json:"identity"`
		Class              *json.RawMessage `json:"class"`
		Bounds             *json.RawMessage `json:"bounds"`
		Magnitude          *json.RawMessage `json:"magnitude"`
		AreaPixels         *json.RawMessage `json:"areaPixels"`
		IdentityConfidence *json.RawMessage `json:"identityConfidence"`
		IdentityUncertain  *json.RawMessage `json:"identityUncertain"`
	} `json:"regions"`
	Conditions *json.RawMessage `json:"conditions"`
}

// checkRequiredFields reports the first required field the document omits.
//
// A field present but null counts as omitted: the schema gives none of the required fields a null type, so a
// null there is a document that does not say what it claims to.
func checkRequiredFields(raw []byte) error {
	var document presence
	if err := json.Unmarshal(raw, &document); err != nil {
		// A document that does not parse is reported by the caller, which is where the parse error belongs.
		return nil
	}

	missing := func(field string) error {
		return &FieldError{
			Op:      "delta.Decode",
			Subject: "document",
			Field:   field,
			Problem: "is required by the schema and is missing or null",
		}
	}
	present := func(field string, raw *json.RawMessage) error {
		if raw == nil || isNull(*raw) {
			return missing(field)
		}
		return nil
	}

	if err := present("schemaVersion", document.SchemaVersion); err != nil {
		return err
	}
	if document.Frame == nil {
		return missing("frame")
	}
	for _, field := range []struct {
		name string
		raw  *json.RawMessage
	}{
		{"frame.sequence", document.Frame.Sequence},
		{"frame.width", document.Frame.Width},
		{"frame.height", document.Frame.Height},
		{"frame.scaleFactor", document.Frame.ScaleFactor},
	} {
		if err := present(field.name, field.raw); err != nil {
			return err
		}
	}
	if document.Fingerprint == nil {
		return missing("fingerprint")
	}
	for _, field := range []struct {
		name string
		raw  *json.RawMessage
	}{
		{"fingerprint.algorithm", document.Fingerprint.Algorithm},
		{"fingerprint.gridSize", document.Fingerprint.GridSize},
		{"fingerprint.cells", document.Fingerprint.Cells},
		{"fingerprint.strictHash", document.Fingerprint.StrictHash},
	} {
		if err := present(field.name, field.raw); err != nil {
			return err
		}
	}
	if document.Regions == nil {
		return missing("regions")
	}
	for index, region := range *document.Regions {
		prefix := "regions[" + itoa(index) + "]."
		for _, field := range []struct {
			name string
			raw  *json.RawMessage
		}{
			{"identity", region.Identity},
			{"class", region.Class},
			{"bounds", region.Bounds},
			{"magnitude", region.Magnitude},
			{"areaPixels", region.AreaPixels},
			{"identityConfidence", region.IdentityConfidence},
			{"identityUncertain", region.IdentityUncertain},
		} {
			if err := present(prefix+field.name, field.raw); err != nil {
				return err
			}
		}
	}
	if err := present("conditions", document.Conditions); err != nil {
		return err
	}
	return nil
}

// isNull reports whether a raw JSON value is the null literal.
func isNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
