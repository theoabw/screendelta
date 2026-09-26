// Package fielderr provides the error type every package in this module returns.
//
// The point is that a failure names what an operator has to change, not the
// function that noticed: the operation, the subject it was working on, the field at
// fault and what was wrong with it. Message wording is part of the contract because
// the CLI prints these verbatim and the tests assert on them.
package fielderr

import "strconv"

// Error is a failure that can identify where it happened.
type Error struct {
	// Op is the operation, for example "frame.Validate".
	Op string
	// Subject names the thing being validated, for example "frame", and is empty
	// when there is nothing useful to name, such as when the subject's own
	// identifier is invalid.
	Subject string
	// Sequence is an optional ordinal within the subject, for example a frame
	// number. It is rendered only when it is greater than zero.
	Sequence uint64
	// Field is the field or option at fault.
	Field string
	// Problem states what is wrong, in a form an operator can act on.
	Problem string
	// Cause is the underlying failure, when there is one. It is kept so that
	// errors.Is and errors.As still work through the context this type adds.
	Cause error
}

func (e *Error) Error() string {
	message := e.Op
	if e.Subject != "" {
		message += ": " + e.Subject
		if e.Sequence > 0 {
			message += " " + strconv.FormatUint(e.Sequence, 10)
		}
	}
	return message + ": " + e.Field + ": " + e.Problem
}

// Unwrap exposes the cause, so a caller can still test for the original error.
func (e *Error) Unwrap() error {
	return e.Cause
}

// With returns a copy carrying the operation, subject and sequence. A copy, not a
// mutation: an error value may be shared, and rewriting someone else's context is
// how a message ends up naming the wrong frame.
func (e Error) With(op, subject string, sequence uint64) *Error {
	e.Op = op
	e.Subject = subject
	e.Sequence = sequence
	return &e
}

// Range renders a range violation in one place so every message reads the same.
func Range(field string, low, high, got int) *Error {
	return &Error{
		Field:   field,
		Problem: "value " + strconv.Itoa(got) + " is outside " + strconv.Itoa(low) + " to " + strconv.Itoa(high),
	}
}

// At sets the operation and subject on an error built elsewhere, in place. It is kept
// for the simple case where the value was just constructed and cannot be shared; use
// With when the error came from somewhere else.
func (e *Error) At(op, subject string, sequence uint64) *Error {
	e.Op = op
	e.Subject = subject
	e.Sequence = sequence
	return e
}
