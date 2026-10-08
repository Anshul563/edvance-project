package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/creatorclient"
)

// Actor is the authenticated caller. UserID comes from the JWT subject
// and AccessToken is the raw bearer token, forwarded to creator-service
// so ownership is always derived from the caller's own credentials —
// never from a creator_id the client sent.
type Actor struct {
	UserID      uuid.UUID
	AccessToken string
}

// Anonymous is the no-credentials actor used by public read endpoints.
func Anonymous() Actor {
	return Actor{}
}

// Authenticated reports whether this actor carries a verified identity.
func (a Actor) Authenticated() bool {
	return a.UserID != uuid.Nil
}

// CreatorResolver maps an authenticated user to the creator they own.
// Implemented by creatorclient.Client in production; tests inject fakes
// to prove the service enforces whatever the resolver decides.
type CreatorResolver interface {
	ResolveCreatorID(
		ctx context.Context,
		userID uuid.UUID,
		accessToken string,
	) (uuid.UUID, error)
}

// resolveCreatorID establishes ownership for write operations. A user
// without a creator profile cannot own content, and an unreachable
// creator-service fails the request instead of guessing.
func resolveCreatorID(
	ctx context.Context,
	creators CreatorResolver,
	actor Actor,
) (uuid.UUID, error) {
	if !actor.Authenticated() {
		return uuid.Nil, ErrUnauthorized
	}

	creatorID, err := creators.ResolveCreatorID(
		ctx,
		actor.UserID,
		actor.AccessToken,
	)
	if err != nil {
		switch {
		case errors.Is(err, creatorclient.ErrCreatorNotFound):
			return uuid.Nil, ErrNoCreatorProfile

		case errors.Is(err, creatorclient.ErrUnavailable):
			return uuid.Nil, ErrCreatorUnavailable

		default:
			return uuid.Nil, fmt.Errorf("resolve creator: %w", err)
		}
	}

	return creatorID, nil
}

// viewerCreatorID establishes ownership scope for read operations.
// Reads degrade to anonymous instead of failing: a creator-service
// outage must not take the public catalog down. Callers treat
// uuid.Nil as "not the owner".
func viewerCreatorID(
	ctx context.Context,
	creators CreatorResolver,
	actor Actor,
) uuid.UUID {
	if !actor.Authenticated() {
		return uuid.Nil
	}

	creatorID, err := creators.ResolveCreatorID(
		ctx,
		actor.UserID,
		actor.AccessToken,
	)
	if err != nil {
		slog.Warn(
			"creator lookup failed while scoping a read; treating viewer as anonymous",
			"error", err,
			"user_id", actor.UserID,
		)

		return uuid.Nil
	}

	return creatorID
}

// owns reports whether the viewer is the creator of a content row.
func owns(viewerCreatorID uuid.UUID, contentCreatorID uuid.UUID) bool {
	return viewerCreatorID != uuid.Nil && viewerCreatorID == contentCreatorID
}

// visibleTo implements the read rule shared by every content type:
// published+public rows are for everyone; everything else is owner-only
// and reads as not-found to everyone else.
func visibleTo(
	viewerCreatorID uuid.UUID,
	contentCreatorID uuid.UUID,
	isPublished bool,
	isPublic bool,
) bool {
	if isPublished && isPublic {
		return true
	}

	return owns(viewerCreatorID, contentCreatorID)
}
