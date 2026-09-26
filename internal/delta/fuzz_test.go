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

// FuzzEncodeDecodeRoundTrip is the round-trip property made reachable.
//
// A review measured the other target and found that its round trip was reached by one input in a million: random
// bytes almost never form a valid document, so the property was effectively the seed corpus. This target builds a
// document from structured input instead.
//
// A second review measured this one and found its comment wider than its behaviour, so the numbers are here
// rather than a claim: of about a million executions, roughly nine percent reach the decode, seventy four percent
// return because the validator refused the document, and eighteen percent skip on geometry the API rejects. One
// execution in eleven is not "every execution", and the comment says which it is.
//
// The property is that anything the engine can be asked to write, it can read back, with the region's fields
// unchanged. The assertions cover every field of the region, which the first version did not: it checked the
// frame, the class and the identity, and an identity of one cannot fail because the validator requires at least
// one and one is the only value written.
func FuzzEncodeDecodeRoundTrip(f *testing.F) {
	f.Add(uint64(1), 64, 48, 1.0, uint8(0), 0.25, 0.1, 0.5, 0.2, 1.0, 4096, true)
	f.Add(uint64(7), 1920, 1080, 2.0, uint8(1), 0.0, 0.0, 1.0, 1.0, 0.0, 1, false)

	f.Fuzz(func(t *testing.T, sequence uint64, width, height int, scale float64, classIndex uint8,
		x, y, w, h, confidence float64, area int, uncertain bool) {
		if width < 1 || height < 1 || width > 4096 || height > 4096 {
			t.Skip()
		}
		if sequence < 1 {
			t.Skip()
		}
		classes := []RegionClass{ClassChanged, ClassAdded, ClassMoved, ClassRemoved}
		class := classes[int(classIndex)%len(classes)]

		region := Region{
			Identity:           1,
			Class:              class,
			Bounds:             Bounds{X: x, Y: y, W: w, H: h},
			Magnitude:          0.5,
			AreaPixels:         area,
			IdentityConfidence: confidence,
			IdentityUncertain:  uncertain,
		}
		if class == ClassMoved || class == ClassRemoved {
			previous := Bounds{X: 0.1, Y: 0.1, W: 0.1, H: 0.1}
			region.PreviousBounds = &previous
		}

		document := Document{
			SchemaVersion: SchemaVersion,
			Frame:         FrameRef{Sequence: sequence, Width: width, Height: height, ScaleFactor: scale},
			Fingerprint: Fingerprint{
				Algorithm:  "grid-luma-1",
				GridSize:   8,
				Cells:      make([]int, 64),
				StrictHash: "0123456789abcdef",
			},
			Regions: []Region{region},
		}

		var encoded bytes.Buffer
		if err := document.Encode(&encoded, false); err != nil {
			// A document the validator rejects is a legitimate outcome of arbitrary input.
			return
		}
		decoded, err := Decode(bytes.NewReader(encoded.Bytes()))
		if err != nil {
			t.Fatalf("the engine's own output does not decode: %v\noutput: %s", err, encoded.String())
		}
		if decoded.Frame.Sequence != sequence || decoded.Frame.Width != width || decoded.Frame.Height != height {
			t.Fatalf("the frame survived the round trip changed: %+v", decoded.Frame)
		}
		if len(decoded.Regions) != 1 {
			t.Fatalf("the round trip produced %d regions", len(decoded.Regions))
		}
		back := decoded.Regions[0]
		if back.Class != class || back.Identity != 1 {
			t.Fatalf("the region survived the round trip changed: %+v", back)
		}
		if back.Bounds != region.Bounds {
			t.Fatalf("the bounds survived the round trip changed: %+v became %+v", region.Bounds, back.Bounds)
		}
		if back.AreaPixels != region.AreaPixels || back.Magnitude != region.Magnitude {
			t.Fatalf("the area or magnitude survived the round trip changed: %+v", back)
		}
		if back.IdentityConfidence != region.IdentityConfidence || back.IdentityUncertain != region.IdentityUncertain {
			t.Fatalf("the identity evidence survived the round trip changed: %+v", back)
		}
		switch {
		case region.PreviousBounds == nil && back.PreviousBounds != nil:
			t.Fatalf("previous bounds appeared in the round trip: %+v", back)
		case region.PreviousBounds != nil && back.PreviousBounds == nil:
			t.Fatalf("previous bounds were lost in the round trip: %+v", back)
		case region.PreviousBounds != nil && *back.PreviousBounds != *region.PreviousBounds:
			t.Fatalf("previous bounds survived the round trip changed: %+v", back.PreviousBounds)
		}
	})
}
