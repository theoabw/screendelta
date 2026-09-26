package frame

import (
	"fmt"
	"strconv"
)

// FieldError is what this package returns instead of a bare error: it carries the
// frame sequence when one is known and the field at fault, so a failure names the
// thing an operator has to change rather than the function that noticed.
type FieldError struct {
	Op       string
	Sequence uint64
	Field    string
	Problem  string
}

func (e *FieldError) Error() string {
	if e.Sequence > 0 {
		return fmt.Sprintf("%s: frame %d: %s: %s", e.Op, e.Sequence, e.Field, e.Problem)
	}
	return fmt.Sprintf("%s: %s: %s", e.Op, e.Field, e.Problem)
}

// problem renders a range violation in one place so every message reads the same.
func problem(low string, high int, got int) string {
	return "value " + itoa(got) + " is outside " + low + " to " + itoa(high)
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
