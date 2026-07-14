// Package apperr carries an HTTP status alongside an error message, mirroring
// the original server's `error.statusCode` convention.
package apperr

import "errors"

// Error is an error with an associated HTTP status code.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

// New builds an *Error.
func New(status int, message string) *Error {
	return &Error{Status: status, Message: message}
}

// StatusOf returns the status carried by err, or def when err carries none.
func StatusOf(err error, def int) int {
	var e *Error
	if errors.As(err, &e) {
		return e.Status
	}
	return def
}
