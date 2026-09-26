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
// The check is deliberately separate from Validate. Validate reasons about values and runs on every encode as
// well, which has no raw JSON to inspect; presence can only be answered before the struct is built.
//
// It enforces the required fields and the objects the schema types as objects. It does not enforce
// additionalProperties, which the encoder tolerates on purpose so that adding an optional field stays within a
// schema generation, and the claim is therefore the narrower one: a consumer that trusts this decoder is not
// reading a document with a missing required field or a null where an object belongs.

// boundsPresence is the schema's bounds object, which requires all four members.
type boundsPresence struct {
	X *json.RawMessage `json:"x"`
	Y *json.RawMessage `json:"y"`
	W *json.RawMessage `json:"w"`
	H *json.RawMessage `json:"h"`
}

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
		Bounds             *boundsPresence  `json:"bounds"`
		PreviousBounds     *boundsPresence  `json:"previousBounds"`
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
			{"magnitude", region.Magnitude},
			{"areaPixels", region.AreaPixels},
			{"identityConfidence", region.IdentityConfidence},
			{"identityUncertain", region.IdentityUncertain},
		} {
			if err := present(prefix+field.name, field.raw); err != nil {
				return err
			}
		}
		if region.Bounds == nil {
			return missing(prefix + "bounds")
		}
		for _, member := range []struct {
			name string
			raw  *json.RawMessage
		}{
			{"x", region.Bounds.X},
			{"y", region.Bounds.Y},
			{"w", region.Bounds.W},
			{"h", region.Bounds.H},
		} {
			if err := present(prefix+"bounds."+member.name, member.raw); err != nil {
				return err
			}
		}
		// Previous bounds are required for a moved or removed region, and when they are there their members are
		// required too. Which classes require the object is a value rule and lives in the validator; this only
		// checks the object's own completeness.
		//
		// A null is not an object, so the schema rejects it wherever it appears, and the pointer cannot tell a
		// null from an absent object: the raw presence of the key is what distinguishes them.
		if region.PreviousBounds != nil {
			for _, member := range []struct {
				name string
				raw  *json.RawMessage
			}{
				{"x", region.PreviousBounds.X},
				{"y", region.PreviousBounds.Y},
				{"w", region.PreviousBounds.W},
				{"h", region.PreviousBounds.H},
			} {
				if err := present(prefix+"previousBounds."+member.name, member.raw); err != nil {
					return err
				}
			}
		}
	}
	if err := present("conditions", document.Conditions); err != nil {
		return err
	}
	return nil
}

// checkNullObjects rejects the objects the schema types as objects when they are present but null.
//
// A pointer field cannot see this: unmarshalling the JSON literal null into a pointer sets it to nil, which is
// exactly what an absent key produces, so a struct-shaped check cannot tell "previousBounds": null from a
// document that never mentions previousBounds. The raw maps keep the distinction, because a map value holds the
// literal bytes of whatever was written there.
func checkNullObjects(raw json.RawMessage) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil
	}

	object := func(field string, value json.RawMessage) error {
		if isNull(value) {
			return &FieldError{
				Op:      "delta.Decode",
				Subject: "document",
				Field:   field,
				Problem: "is null, and the schema requires an object here",
			}
		}
		return nil
	}

	for _, field := range []string{"frame", "fingerprint"} {
		if value, present := top[field]; present {
			if err := object(field, value); err != nil {
				return err
			}
		}
	}

	regions, present := top["regions"]
	if !present || isNull(regions) {
		return nil
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(regions, &entries); err != nil {
		return nil
	}
	for index, entry := range entries {
		for _, field := range []string{"bounds", "previousBounds"} {
			if value, present := entry[field]; present {
				if err := object("regions["+itoa(index)+"]."+field, value); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// isNull reports whether a raw JSON value is the null literal.
func isNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
