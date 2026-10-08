// Package client contains the outbound integration with the external
// Go media engine (the component that owns FFmpeg). The video service
// never runs FFmpeg itself; it hands the engine a job and receives the
// result through the internal callback endpoint.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProcessRequest tells the engine what to work on and where to report
// back. The engine receives the source key and an output prefix inside
// the asset's storage directory, never bucket credentials.
type ProcessRequest struct {
	JobID            string         `json:"jobId"`
	MediaAssetID     string         `json:"mediaAssetId"`
	JobType          string         `json:"jobType"`
	SourceStorageKey string         `json:"sourceStorageKey"`
	OutputPrefix     string         `json:"outputPrefix"`
	CallbackURL      string         `json:"callbackUrl"`
	Payload          map[string]any `json:"payload,omitempty"`
}

// MediaEngineClient starts processing on the engine. The interface is
// the mock seam used by unit tests; the HTTP implementation below is
// what production wires up.
type MediaEngineClient interface {
	StartProcessing(ctx context.Context, request ProcessRequest) error
}

// HTTPMediaEngineClient posts jobs to {MEDIA_ENGINE_URL}/v1/jobs.
type HTTPMediaEngineClient struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewHTTPMediaEngineClient builds the production client. The timeout
// only bounds the dispatch call itself — it is never held open while
// FFmpeg runs; results arrive by callback.
func NewHTTPMediaEngineClient(baseURL, token string, timeout time.Duration) *HTTPMediaEngineClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &HTTPMediaEngineClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: timeout},
	}
}

// StartProcessing dispatches one job. A non-2xx response or a transport
// error is returned so the worker can apply its retry policy.
func (c *HTTPMediaEngineClient) StartProcessing(
	ctx context.Context,
	request ProcessRequest,
) error {
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode job: %w", err)
	}

	url := c.baseURL + "/v1/jobs"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	response, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("media engine unreachable: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		// The body belongs to another service; only the status is
		// safe to surface in our error messages.
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))

		return fmt.Errorf("media engine rejected job: HTTP %d", response.StatusCode)
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))

	return nil
}
