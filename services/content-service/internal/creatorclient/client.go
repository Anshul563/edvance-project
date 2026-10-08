// Package creatorclient is the Content Service's small internal client
// for creator-service. Ownership checks need exactly one fact: which
// creator (if any) the authenticated user owns. The client asks
// creator-service for it over HTTP — it NEVER queries creator-service's
// database, and it never asks for a creator the caller did not
// authenticate as.
package creatorclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrCreatorNotFound means the user has no creator record (never
	// onboarded). Callers map this to 403: the operation requires a
	// creator profile.
	ErrCreatorNotFound = errors.New("creator profile not found")

	// ErrUnavailable means creator-service could not be reached or
	// answered with something unexpected. Callers map this to 500;
	// raw transport errors never reach clients.
	ErrUnavailable = errors.New("creator service unavailable")
)

const maxResponseBytes = 1 << 20 // 1 MiB: /creators/me payloads are tiny

type Client struct {
	baseURL string
	http    *http.Client
}

// New builds a client for creator-service. baseURL is the service root,
// e.g. http://localhost:8083.
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type meResponse struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`
}

// ResolveCreatorID returns the creator ID owned by userID, forwarding
// the caller's own access token so creator-service authenticates the
// request exactly as it would the user's direct /creators/me call.
func (c *Client) ResolveCreatorID(
	ctx context.Context,
	userID uuid.UUID,
	accessToken string,
) (uuid.UUID, error) {
	if accessToken == "" {
		return uuid.Nil, ErrCreatorNotFound
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/creators/me",
		nil,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: build request", ErrUnavailable)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	response, err := c.http.Do(req)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %s", ErrUnavailable, err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: read response", ErrUnavailable)
	}

	switch response.StatusCode {
	case http.StatusOK:
		// continue below

	case http.StatusNotFound:
		return uuid.Nil, ErrCreatorNotFound

	default:
		return uuid.Nil, fmt.Errorf(
			"%w: unexpected status %d",
			ErrUnavailable,
			response.StatusCode,
		)
	}

	var me meResponse

	if err := json.Unmarshal(body, &me); err != nil {
		return uuid.Nil, fmt.Errorf("%w: decode response", ErrUnavailable)
	}

	creatorID, err := uuid.Parse(me.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: missing creator id", ErrUnavailable)
	}

	// Defense in depth: the record must belong to the user we asked
	// about, otherwise a misbehaving upstream could hand us someone
	// else's creator.
	if owner, err := uuid.Parse(me.UserID); err != nil || owner != userID {
		return uuid.Nil, fmt.Errorf(
			"%w: creator does not belong to the requesting user",
			ErrUnavailable,
		)
	}

	return creatorID, nil
}
