package delta

import (
	"bytes"
	"strings"
	"testing"
)

// The fuzz targets in this file ask one question the unit tests cannot: does any input make the engine panic
// or produce something it would not accept from itself?
//
// FR-015 says the engine fails explicitly rather than emitting a partial document, and a panic is the opposite
// of an explicit failure: it takes the process down without naming the field. The decoders are the place where
// untrusted input arrives, so they are where the question belongs.

// FuzzDecode feeds arbitrary bytes to the document decoder.
//
// Two properties, both of which have held for every input tried: no input panics, and any input that decodes
// has to survive the validator and re-encode into something that decodes again. The second property is the
// strong one: a document the engine accepts but cannot reproduce is a document that would break a consumer.
func FuzzDecode(f *testing.F) {
	seeds := []string{
		`{"schemaVersion":"1.0","frame":{"sequence":1,"width":64,"height":48,"scaleFactor":1},` +
			`"fingerprint":{"algorithm":"grid-luma-1","gridSize":8,"cells":[` + strings.Repeat("0,", 63) + `0],"strictHash":"0123456789abcdef"},` +
			`"regions":[],"conditions":["first-frame"]}`,
		`{"schemaVersion":"2.0"}`,
		`{"schemaVersion":"1.0","frame":null,"fingerprint":{},"regions":[{}],"conditions":[]}`,
		`{"schemaVersion":"1.0","frame":{"sequence":0,"width":-1,"height":1e999,"scaleFactor":0},` +
			`"fingerprint":{"algorithm":"GRID","gridSize":8,"cells":[],"strictHash":"x"},` +
			`"regions":[{"identity":0,"class":"nonsense","bounds":{"x":-1,"y":2,"w":0,"h":0},` +
			`"magnitude":5,"areaPixels":-1,"identityConfidence":2,"identityUncertain":true}],"conditions":["nope"]}`,
		`[]`,
		`null`,
		``,
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, body []byte) {
		document, err := Decode(bytes.NewReader(body))
		if err != nil {
			return
		}
		if err := document.Validate(); err != nil {
			t.Fatalf("a document that decoded does not validate: %v\ninput: %s", err, body)
		}
		var encoded bytes.Buffer
		if err := document.Encode(&encoded, false); err != nil {
			t.Fatalf("a document that decoded and validated does not encode: %v", err)
		}
		again, err := Decode(bytes.NewReader(encoded.Bytes()))
		if err != nil {
			t.Fatalf("the engine's own output does not decode: %v\noutput: %s", err, encoded.String())
		}
		if len(again.Regions) != len(document.Regions) {
			t.Fatalf("re-encoding lost regions: %d became %d", len(document.Regions), len(again.Regions))
		}
	})
}
