package service

import (
	"errors"

	"github.com/Anshul563/edvance-project/services/content-service/internal/repository"
)

// Typed service errors. Handlers translate these into HTTP status
// codes; raw database errors never reach clients.
var (
	ErrNotFound = errors.New("not found")

	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")

	// ErrNoCreatorProfile means the authenticated user has never
	// onboarded as a creator, so they cannot own content.
	ErrNoCreatorProfile = errors.New("user has no creator profile")

	ErrDuplicate = errors.New("already exists")

	// ErrInvalidStatus is a state conflict (409): the entity exists but
	// its current status forbids the requested transition.
	ErrInvalidStatus = errors.New("invalid status")

	// ErrInvalidVisibility is a bad enum value (400).
	ErrInvalidVisibility = errors.New("invalid visibility")

	// ErrInvalidInput is any client-side validation failure (400).
	ErrInvalidInput = errors.New("invalid input")

	// ErrNotPublishable carries the reasons a publish was rejected.
	ErrNotPublishable = errors.New("content is not publishable")

	// ErrCreatorUnavailable means ownership could not be established
	// because creator-service failed. Reported as 500, never leaked
	// as a transport error.
	ErrCreatorUnavailable = errors.New("creator service unavailable")
)

// mapRepoError converts repository-layer errors into service errors.
// Anything unrecognized is returned as-is and becomes a generic 500 at
// the handler.
func mapRepoError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, repository.ErrVideoNotFound),
		errors.Is(err, repository.ErrShortNotFound),
		errors.Is(err, repository.ErrPostNotFound),
		errors.Is(err, repository.ErrTagNotFound),
		errors.Is(err, repository.ErrCategoryNotFound):
		return ErrNotFound

	case errors.Is(err, repository.ErrVideoSlugTaken),
		errors.Is(err, repository.ErrShortSlugTaken),
		errors.Is(err, repository.ErrTagDuplicate):
		return ErrDuplicate

	case errors.Is(err, repository.ErrVideoInvalidPublish),
		errors.Is(err, repository.ErrShortInvalidPublish),
		errors.Is(err, repository.ErrPostInvalidPublish):
		return ErrNotPublishable

	case errors.Is(err, repository.ErrVideoInvalidStatus),
		errors.Is(err, repository.ErrShortInvalidStatus):
		return ErrInvalidStatus

	default:
		return err
	}
}
