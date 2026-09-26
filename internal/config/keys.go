package config

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

// The configuration promise is that an unknown key is an error. Go's JSON decoder
// cannot make that promise on its own: it matches field names case-insensitively, so
// "gridsize" is accepted as "gridSize". The key sets below exist so the promise is
// kept exactly, and a mistyped key is reported with the spelling it probably meant.

var (
	topLevelKeys    = []string{"noiseFloor", "minRegionAreaPixels", "motionTolerancePixels", "occlusionFrames", "ignoredAreas", "regionsOfInterest", "output", "fingerprint"}
	outputKeys      = []string{"format", "pretty"}
	fingerprintKeys = []string{"gridSize", "maxCellDelta"}
	boundsKeys      = []string{"x", "y", "w", "h"}
	roiKeys         = []string{"label", "bounds"}
)

// checkMembers walks the raw token stream and rejects two things a map cannot see:
// a member repeated in the same object, because the last one silently wins, and a null
// value, because a typed field has no meaning for null and a reader would rather be
// told than have it treated as absent.
//
// This runs before the map based key check because converting an object to a map
// destroys the evidence of a duplicate.
func checkMembers(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := walkMembers(decoder, "config"); err != nil {
		return err
	}
	// A second document in the same input is a caller mistake, not a configuration.
	if _, err := decoder.Token(); err == nil {
		return &FieldError{
			Op:      "config.Parse",
			Subject: "config",
			Field:   "document",
			Problem: "must contain exactly one configuration object",
		}
	}
	return nil
}

func walkMembers(decoder *json.Decoder, path string) error {
	token, err := decoder.Token()
	if err != nil {
		return &FieldError{Op: "config.Parse", Subject: "config", Field: path, Problem: "cannot parse: " + err.Error()}
	}

	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		if token == nil {
			return &FieldError{Op: "config.Parse", Subject: "config", Field: path, Problem: "must not be null"}
		}
		return nil
	}

	switch delimiter {
	case '{':
		seen := make(map[string]bool)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return &FieldError{Op: "config.Parse", Subject: "config", Field: path, Problem: "cannot parse: " + err.Error()}
			}
			key, _ := keyToken.(string)
			if seen[key] {
				return &FieldError{
					Op:      "config.Parse",
					Subject: "config",
					Field:   path,
					Problem: "duplicate key " + quote(key) + ", the earlier value would be ignored",
				}
			}
			seen[key] = true
			if err := walkMembers(decoder, path+"."+key); err != nil {
				return err
			}
		}
		_, err := decoder.Token()
		return err
	case '[':
		index := 0
		for decoder.More() {
			if err := walkMembers(decoder, path+"["+itoa(index)+"]"); err != nil {
				return err
			}
			index++
		}
		_, err := decoder.Token()
		return err
	}
	return nil
}

// checkKeys walks the raw document and rejects any key that is not in the schema.
func checkKeys(raw []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return &FieldError{
			Op:      "config.Parse",
			Subject: "config",
			Field:   "document",
			Problem: "cannot parse: " + err.Error(),
		}
	}

	if err := checkObject("config", root, topLevelKeys); err != nil {
		return err
	}
	if err := checkNestedObject(root, "output", outputKeys); err != nil {
		return err
	}
	if err := checkNestedObject(root, "fingerprint", fingerprintKeys); err != nil {
		return err
	}
	if err := checkBoundsArray(root, "ignoredAreas"); err != nil {
		return err
	}
	if err := checkRegionsOfInterest(root); err != nil {
		return err
	}
	return nil
}

func checkNestedObject(root map[string]json.RawMessage, key string, allowed []string) error {
	raw, present := root[key]
	if !present {
		return nil
	}
	var nested map[string]json.RawMessage
	if err := json.Unmarshal(raw, &nested); err != nil {
		return &FieldError{Op: "config.Parse", Subject: "config", Field: key, Problem: "must be an object"}
	}
	return checkObject(key, nested, allowed)
}

func checkBoundsArray(root map[string]json.RawMessage, key string) error {
	raw, present := root[key]
	if !present {
		return nil
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return &FieldError{Op: "config.Parse", Subject: "config", Field: key, Problem: "must be an array of bounds"}
	}
	for index, item := range items {
		if err := checkObject(key+"["+itoa(index)+"]", item, boundsKeys); err != nil {
			return err
		}
	}
	return nil
}

func checkRegionsOfInterest(root map[string]json.RawMessage) error {
	raw, present := root["regionsOfInterest"]
	if !present {
		return nil
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return &FieldError{Op: "config.Parse", Subject: "config", Field: "regionsOfInterest", Problem: "must be an array of regions"}
	}
	for index, item := range items {
		where := "regionsOfInterest[" + itoa(index) + "]"
		if err := checkObject(where, item, roiKeys); err != nil {
			return err
		}
		if boundsRaw, ok := item["bounds"]; ok {
			var bounds map[string]json.RawMessage
			if err := json.Unmarshal(boundsRaw, &bounds); err != nil {
				return &FieldError{Op: "config.Parse", Subject: "config", Field: where + ".bounds", Problem: "must be an object"}
			}
			if err := checkObject(where+".bounds", bounds, boundsKeys); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkObject(where string, object map[string]json.RawMessage, allowed []string) error {
	// Sorted so that a document with several unknown keys always reports the same one,
	// rather than whichever the map happened to yield first.
	unknown := make([]string, 0, len(object))
	for key := range object {
		if !contains(allowed, key) {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return &FieldError{
		Op:      "config.Parse",
		Subject: "config",
		Field:   where,
		Problem: unknownKeyProblem(unknown[0], allowed),
	}
}

func unknownKeyProblem(key string, allowed []string) string {
	for _, candidate := range allowed {
		if strings.EqualFold(candidate, key) {
			return "unknown key " + quote(key) + ", did you mean " + quote(candidate) + "?"
		}
	}
	return "unknown key " + quote(key) + ", allowed keys are " + strings.Join(quoteAll(allowed), ", ")
}

func quoteAll(values []string) []string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = quote(value)
	}
	return quoted
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
