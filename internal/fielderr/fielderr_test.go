package fielderr

import (
	"strings"
	"testing"
)

func TestErrorRendersOperationSubjectAndField(t *testing.T) {
	err := (&Error{Field: "width", Problem: "value 0 is outside 1 to 32768"}).At("frame.Validate", "frame", 7)
	want := "frame.Validate: frame 7: width: value 0 is outside 1 to 32768"
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestErrorOmitsAnUnknownSequence(t *testing.T) {
	err := (&Error{Field: "sequence", Problem: "must be at least 1"}).At("frame.Validate", "", 0)
	if got := err.Error(); strings.Contains(got, "frame ") {
		t.Fatalf("Error() invented a subject it cannot know: %q", got)
	}
	if got := err.Error(); got != "frame.Validate: sequence: must be at least 1" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestRangeRendersTheBounds(t *testing.T) {
	err := Range("noiseFloor", 1, 100, 150).At("config.Validate", "", 0)
	if got := err.Error(); !strings.Contains(got, "value 150 is outside 1 to 100") {
		t.Fatalf("Range() = %q", got)
	}
}
