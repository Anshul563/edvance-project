package engine

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

// HTTPClient talks to the external media engine over HTTP. Timeouts are
// deliberately short: these are control-plane calls (dispatch, poll,
// cancel), never held open while FFmpeg runs.
type HTTPClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewHTTPClient(baseURL string, token string, timeout time.Duration) *HTTPClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

type engineCreateRequest struct {
	Type    string             `json:"type"`
	Source  engineSource       `json:"source"`
	Options engineCreateOption `json:"options"`
}

type engineSource struct {
	ObjectKey string `json:"objectKey"`
}

type engineCreateOption struct {
	GenerateThumbnail bool `json:"generateThumbnail"`
	GenerateHLS       bool `json:"generateHLS"`
}

type engineCreateResponse struct {
	JobID string `json:"jobId"`
}

type engineStatusResponse struct {
	JobID    string             `json:"jobId"`
	Status   string             `json:"status"`
	Progress int32              `json:"progress"`
	Output   engineOutputFields `json:"output"`
	Error    engineErrorFields  `json:"error"`
}

type engineOutputFields struct {
	ManifestURL  string `json:"manifestUrl"`
	ThumbnailURL string `json:"thumbnailUrl"`
	Duration     int64  `json:"durationSeconds"`
	Width        int32  `json:"width"`
	Height       int32  `json:"height"`
}

type engineErrorFields struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (c *HTTPClient) CreateJob(
	ctx context.Context,
	request CreateJobRequest,
) (CreateJobResponse, error) {
	payload := engineCreateRequest{
		Type:   request.Type,
		Source: engineSource{ObjectKey: request.Source.ObjectKey},
		Options: engineCreateOption{
			GenerateThumbnail: request.Options.GenerateThumbnail,
			GenerateHLS:       request.Options.GenerateHLS,
		},
	}

	var parsed engineCreateResponse

	if err := c.do(ctx, http.MethodPost, "/v1/jobs", payload, &parsed); err != nil {
		return CreateJobResponse{}, err
	}

	if parsed.JobID == "" {
		return CreateJobResponse{}, fmt.Errorf("engine: empty job id in response")
	}

	return CreateJobResponse{JobID: parsed.JobID}, nil
}

func (c *HTTPClient) GetJob(
	ctx context.Context,
	engineJobID string,
) (EngineJobStatus, error) {
	if engineJobID == "" {
		return EngineJobStatus{}, fmt.Errorf("engine: job id is required")
	}

	var parsed engineStatusResponse

	if err := c.do(
		ctx,
		http.MethodGet,
		"/v1/jobs/"+engineJobID,
		nil,
		&parsed,
	); err != nil {
		return EngineJobStatus{}, err
	}

	return EngineJobStatus{
		JobID:    parsed.JobID,
		Status:   parsed.Status,
		Progress: parsed.Progress,
		Output: EngineJobOutput{
			ManifestURL:  parsed.Output.ManifestURL,
			ThumbnailURL: parsed.Output.ThumbnailURL,
			Duration:     parsed.Output.Duration,
			Width:        parsed.Output.Width,
			Height:       parsed.Output.Height,
		},
		Error: EngineJobError{
			Code:    parsed.Error.Code,
			Message: parsed.Error.Message,
		},
	}, nil
}

func (c *HTTPClient) CancelJob(
	ctx context.Context,
	engineJobID string,
) error {
	if engineJobID == "" {
		return fmt.Errorf("engine: job id is required")
	}

	return c.do(
		ctx,
		http.MethodPost,
		"/v1/jobs/"+engineJobID+"/cancel",
		nil,
		nil,
	)
}

// do performs one engine call. Non-2xx responses and undecodable bodies
// become errors; the internal token never leaves the Authorization
// header and never appears in errors or logs.
func (c *HTTPClient) do(
	ctx context.Context,
	method string,
	path string,
	payload any,
	out any,
) error {
	var body io.Reader

	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("engine: encode request: %w", err)
		}

		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("engine: build request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.token)

	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("engine: request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("engine: unexpected status %d", response.StatusCode)
	}

	if out == nil {
		_, _ = io.Copy(io.Discard, response.Body)

		return nil
	}

	decoder := json.NewDecoder(response.Body)

	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("engine: decode response: %w", err)
	}

	return nil
}
