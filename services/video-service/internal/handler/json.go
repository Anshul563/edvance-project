package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// maxBodyBytes bounds JSON request bodies. Initiate/callback payloads
// are tiny; nothing user-controlled comes through here as bytes (those
// flow straight to the signed S3 URL).
const maxBodyBytes = 1 << 20

var errBodyTooLarge = errors.New("request body too large")

// readJSON decodes a single JSON object from the request body, bounding
// the read so a hostile body cannot exhaust memory.
func readJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	decoder := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode body: %w", err)
	}

	return nil
}

// writeJSON encodes any value as JSON. Failures here are unanswerable,
// so they are only logged by the caller.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(payload)
}
