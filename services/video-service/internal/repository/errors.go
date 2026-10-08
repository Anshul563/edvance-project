package repository

import "errors"

var (
	// ErrAssetNotFound is returned when a media asset row does not exist.
	ErrAssetNotFound = errors.New("media asset not found")
	// ErrAssetConflict is returned when a conditional asset write misses
	// its expected status guard (the row moved underneath us).
	ErrAssetConflict = errors.New("media asset status conflict")
	// ErrJobNotFound is returned when a processing job row does not exist.
	ErrJobNotFound = errors.New("processing job not found")
	// ErrJobConflict is returned when a conditional job update missed.
	ErrJobConflict = errors.New("processing job status conflict")
	// ErrCaptionNotFound is returned when a caption row does not exist.
	ErrCaptionNotFound = errors.New("caption not found")
	// ErrThumbnailNotFound is returned when a thumbnail row does not exist.
	ErrThumbnailNotFound = errors.New("thumbnail not found")
	// ErrUniqueConflict is returned when a partial unique index
	// (one primary thumbnail, one default caption) rejects the write.
	ErrUniqueConflict = errors.New("unique constraint violation")
)
