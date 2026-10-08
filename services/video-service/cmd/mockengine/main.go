package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// mockengine is the development stand-in for the external Go media
// engine. The real engine owns FFmpeg; this stub accepts jobs and, after
// a short simulated processing delay, posts a fully-formed success
// callback to the video service. It produces NO bytes: it exists purely
// to exercise the orchestration path (dispatch -> callback -> ready).
//
// The stub honors two payload knobs used by tests:
//
//	payload["mock:fail"]     = true      -> post a failed callback
//	payload["mock:retriable"]= false     -> mark a failure non-retriable
//	payload["mock:delay"]    = "100..2s" -> simulate processing duration
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	err := run(logger)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("mockengine stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	port := os.Getenv("MOCKENGINE_PORT")
	if port == "" {
		port = "9090"
	}

	// jobAuth is what the video service uses to authenticate job
	// submissions (MEDIA_ENGINE_TOKEN). callbackKey is what callbacks to
	// the video service carry in X-Internal-Key (INTERNAL_SERVICE_TOKEN).
	// In local dev both may be set to the same dev value.
	jobAuth := os.Getenv("MEDIA_ENGINE_TOKEN")
	callbackKey := os.Getenv("INTERNAL_SERVICE_TOKEN")

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/jobs", func(w http.ResponseWriter, r *http.Request) {
		handleJob(w, r, jobAuth, callbackKey, logger)
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "mockengine"})
	})

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)

	go func() {
		logger.Info("mockengine listening", "port", port)

		errCh <- server.ListenAndServe()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err

	case <-sig:
		logger.Info("shutdown signal received")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return server.Shutdown(ctx)
}

type jobRequest struct {
	JobID            string         `json:"jobId"`
	MediaAssetID     string         `json:"mediaAssetId"`
	JobType          string         `json:"jobType"`
	SourceStorageKey string         `json:"sourceStorageKey"`
	OutputPrefix     string         `json:"outputPrefix"`
	CallbackURL      string         `json:"callbackUrl"`
	Payload          map[string]any `json:"payload,omitempty"`
}

func handleJob(
	w http.ResponseWriter,
	r *http.Request,
	expectedToken string,
	callbackKey string,
	logger *slog.Logger,
) {
	if expectedToken != "" && r.Header.Get("Authorization") != "Bearer "+expectedToken {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})

		return
	}

	if r.Body == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "empty body"})

		return
	}

	request := jobRequest{}

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad job body"})

		return
	}

	logger.Info(
		"job accepted",
		"job_id", request.JobID,
		"asset_id", request.MediaAssetID,
		"job_type", request.JobType,
		"callback_url", request.CallbackURL,
	)

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})

	go simulateProcessing(request, callbackKey, logger)
}

func simulateProcessing(request jobRequest, callbackToken string, logger *slog.Logger) {
	delay := parseDelay(request.Payload["mock:delay"])

	time.Sleep(delay)

	callback := map[string]any{
		"jobId":        request.JobID,
		"mediaAssetId": request.MediaAssetID,
		"status":       "succeeded",
	}

	if fail, _ := request.Payload["mock:fail"].(bool); fail {
		retriable, _ := request.Payload["mock:retriable"].(bool)

		callback["status"] = "failed"
		callback["error"] = map[string]any{
			"code":      "mock_transcode_error",
			"message":   "simulated engine failure",
			"retriable": retriable,
		}
	} else {
		callback["metadata"] = map[string]any{
			"durationSeconds": 60,
			"width":           1280,
			"height":          720,
			"frameRate":       30,
			"codec":           "h264",
			"bitrate":         2500000,
			"container":       "mp4",
		}
		callback["variants"] = []map[string]any{
			{
				"quality":    "720p",
				"width":      1280,
				"height":     720,
				"bitrate":    2500000,
				"codec":      "h264",
				"container":  "mp4",
				"storageKey": request.OutputPrefix + "variant-720p.mp4",
			},
			{
				"quality":    "480p",
				"width":      854,
				"height":     480,
				"bitrate":    1250000,
				"codec":      "h264",
				"container":  "mp4",
				"storageKey": request.OutputPrefix + "variant-480p.mp4",
			},
		}
		callback["thumbnails"] = []map[string]any{
			{
				"storageKey":       request.OutputPrefix + "thumb-0.jpg",
				"width":            640,
				"height":           360,
				"timestampSeconds": 5.0,
				"isPrimary":        true,
			},
		}
		callback["captions"] = []map[string]any{
			{
				"language":   "en",
				"label":      "English",
				"format":     "vtt",
				"storageKey": request.OutputPrefix + "caption-en.vtt",
				"isDefault":  true,
			},
		}
	}

	body, err := json.Marshal(callback)
	if err != nil {
		logger.Error("marshal callback failed", "error", err)

		return
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		request.CallbackURL,
		bytes.NewReader(body),
	)
	if err != nil {
		logger.Error("build callback failed", "error", err)

		return
	}

	req.Header.Set("Content-Type", "application/json")

	if callbackToken != "" {
		req.Header.Set("X-Internal-Key", callbackToken)
	}

	client := &http.Client{Timeout: 15 * time.Second}

	response, err := client.Do(req)
	if err != nil {
		logger.Warn(
			"callback delivery failed (will be redelivered by the engine in production)",
			"job_id", request.JobID,
			"error", err,
		)

		return
	}
	defer response.Body.Close()

	logger.Info(
		"callback delivered",
		"job_id", request.JobID,
		"status", response.StatusCode,
		"callback_status", callback["status"],
	)
}

func parseDelay(value any) time.Duration {
	raw, ok := value.(string)
	if !ok || raw == "" {
		return 300 * time.Millisecond
	}

	if milliseconds, err := strconv.Atoi(raw); err == nil {
		return time.Duration(milliseconds) * time.Millisecond
	}

	return 300 * time.Millisecond
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(payload)
}
