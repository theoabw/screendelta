package frame

import (
	"strconv"

	"github.com/theoabw/screendelta/internal/fielderr"
)

// FieldError is the shared error type, aliased here so a caller inspecting a frame
// failure does not have to import another package to do it.
type FieldError = fielderr.Error

func itoa(value int) string {
	return strconv.Itoa(value)
}
