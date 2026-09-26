package delta

import (
	"encoding/json"
	"io"
	"strconv"

	"github.com/theoabw/screendelta/internal/fielderr"
)

// FieldError is the shared error type, aliased so a caller inspecting a document
// failure does not have to import another package.
type FieldError = fielderr.Error

// Encode writes the document as JSON.
//
// Regions and conditions are sorted first and the copy is shallow, so encoding does
// not mutate the caller's document. Identical input therefore produces byte-identical
// output, which is what NFR-004 requires and what the determinism test asserts.
func (d Document) Encode(w io.Writer, pretty bool) error {
	// The document is canonicalised before it is validated, because the validator enforces the contract's
	// region order and a caller may legitimately build a document in any order. Validating first made the
	// encoder reject documents it is supposed to canonicalise, which a test caught: it builds the same
	// document twice with the regions in opposite orders and requires identical bytes.
	normalised := d
	normalised.Regions = append([]Region(nil), d.Regions...)
	SortRegions(normalised.Regions)
	normalised.Conditions = SortConditions(d.Conditions)
	if normalised.Regions == nil {
		normalised.Regions = []Region{}
	}
	if normalised.Conditions == nil {
		normalised.Conditions = []Condition{}
	}
	if err := normalised.Validate(); err != nil {
		return err
	}

	encoder := json.NewEncoder(w)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	// HTML escaping would rewrite characters that appear in labels and hashes for no
	// benefit, since this is not embedded in a page.
	encoder.SetEscapeHTML(false)
	return encoder.Encode(normalised)
}

// Decode reads a document and rejects one it does not understand.
//
// Unknown fields are tolerated, because adding an optional field stays within a
// schema generation; an unknown schemaVersion is not tolerated, because silently
// reading a newer document is how a consumer starts misreading fields.
func Decode(r io.Reader) (Document, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return Document{}, &FieldError{Op: "delta.Decode", Subject: "document", Field: "input", Problem: "cannot read: " + err.Error()}
	}

	// Presence is checked before the struct is built, because after that a missing required field is
	// indistinguishable from one that was written as a zero value.
	if err := checkRequiredFields(raw); err != nil {
		return Document{}, err
	}
	if err := checkNullObjects(raw); err != nil {
		return Document{}, err
	}

	var document Document
	if err := json.Unmarshal(raw, &document); err != nil {
		return Document{}, &FieldError{Op: "delta.Decode", Subject: "document", Field: "json", Problem: "cannot parse: " + err.Error()}
	}
	if document.SchemaVersion != SchemaVersion {
		return Document{}, &FieldError{
			Op:      "delta.Decode",
			Subject: "document",
			Field:   "schemaVersion",
			Problem: "unsupported version " + quote(document.SchemaVersion) + ", expected " + quote(SchemaVersion),
		}
	}
	if err := document.Validate(); err != nil {
		return Document{}, err
	}
	return document, nil
}

func quote(value string) string {
	return strconv.Quote(value)
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
