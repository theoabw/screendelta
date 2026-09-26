package config

import (
	"strconv"

	"github.com/theoabw/screendelta/internal/fielderr"
)

// FieldError is the shared error type, aliased so a caller inspecting a
// configuration failure does not have to import another package.
type FieldError = fielderr.Error

func quote(value string) string {
	return strconv.Quote(value)
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}
