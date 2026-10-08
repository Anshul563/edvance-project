package service

import "errors"

var (
	// ErrNotFound is returned when the media asset does not exist.
	ErrNotFound = errors.New("media asset not found")
	// ErrGone is returned for deleted assets; callers treat it as 404.
	ErrGone = errors.New("media asset deleted")
	// ErrForbidden is returned when the caller is not the owner.
	ErrForbidden = errors.New("not authorized for this media asset")
	// ErrInvalidIdentifiers guards against malformed uuid paths.
	ErrInvalidIdentifiers = errors.New("invalid identifiers")
	// ErrInvalidUpload is returned for filename/MIME/size validation
	// failures on initiate.
	ErrInvalidUpload = errors.New("invalid upload request")
	// ErrUploadNotPresent is returned on complete when the object
	// never reached the store (or was replaced while signing).
	ErrUploadNotPresent = errors.New("object not present in storage")
	// ErrInvalidCallback is returned for malformed engine callbacks.
	ErrInvalidCallback = errors.New("invalid processing callback")
	// ErrConflict maps to 409 for races the callers cannot retry away.
	ErrConflict = errors.New("conflict")
)
