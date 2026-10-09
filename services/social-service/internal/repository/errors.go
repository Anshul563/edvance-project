package repository

import "errors"

var (
	// ErrNotFound is returned when a row does not exist.
	ErrNotFound = errors.New("not found")
	// ErrDuplicate is returned when a UNIQUE constraint rejects a write.
	ErrDuplicate = errors.New("duplicate")
	// ErrInvalidInput is returned when a CHECK constraint rejects a write.
	ErrInvalidInput = errors.New("invalid input")
	// ErrForbidden is returned when an operation is rejected by ownership.
	ErrForbidden = errors.New("forbidden")
)
