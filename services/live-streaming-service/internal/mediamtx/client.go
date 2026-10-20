// Package mediamtx is a minimal client for the MediaMTX control API.
// The API is only reachable on private networking (docker-compose does not
// publish its port); it is used to reconcile stream status and to kick a
// publisher when a creator ends a session.
//
// Schema notes (MediaMTX v1.21): the connected publisher is exposed as
// path.source {id, type} — older versions used path.publisher. RTMP
// publishers are kicked through /v3/rtmp/conns/kick/{id} (older versions
// used /v3/rtmpconns/kick/{id}).
package mediamtx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PathInfo is the subset of a MediaMTX path record this service needs.
type PathInfo struct {
	Name   string      `json:"name"`
	Ready  bool        `json:"ready"`
	Online bool        `json:"online"`
	Source *PathSource `json:"source"`
}

// PathSource identifies the client currently feeding a path (an RTMP
// connection for OBS publishers).
type PathSource struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// Client talks to the MediaMTX control API over HTTP.
type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}
}

// ListActivePaths returns every path currently known to MediaMTX that has
// a publisher connected.
func (c *Client) ListActivePaths(ctx context.Context) (map[string]PathInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v3/paths/list", nil)
	if err != nil {
		return nil, err
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mediamtx: list paths: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mediamtx: list paths: unexpected status %d", res.StatusCode)
	}

	var payload struct {
		Items []PathInfo `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("mediamtx: decode paths: %w", err)
	}

	out := make(map[string]PathInfo, len(payload.Items))
	for _, item := range payload.Items {
		if item.Source != nil && item.Source.ID != "" {
			out[item.Name] = item
		}
	}

	return out, nil
}

// KickRTMPConnections disconnects the publisher of a path. This is
// best-effort: the session lifecycle is updated regardless, and the
// publish-authorization hook prevents the kicked publisher from
// reconnecting once the session has ended.
func (c *Client) KickRTMPConnections(ctx context.Context, path PathInfo) error {
	if path.Source == nil || path.Source.ID == "" {
		return nil
	}

	return c.kick(ctx, path.Source.ID)
}

func (c *Client) kick(ctx context.Context, connectionID string) error {
	endpoint := c.baseURL + "/v3/rtmp/conns/kick/" + url.PathEscape(connectionID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("mediamtx: kick %s: %w", connectionID, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("mediamtx: kick %s: unexpected status %d", connectionID, res.StatusCode)
	}

	return nil
}
