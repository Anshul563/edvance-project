package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// s3stub is the development stand-in for an S3-compatible object store
// (MinIO in production). It keeps objects in memory and speaks just
// enough S3 for the video service's storage client:
//
//	PUT /{bucket}/{key}    store body (presigned or plain)
//	HEAD /{bucket}/{key}   200 + Content-Length when present, else 404
//	DELETE /{bucket}/{key} 200 (idempotent)
//	GET /{bucket}/{key}    download body
//
// It does not verify SigV4 signatures: the service signs with the real
// AWS SDK, and this stub exists only to exercise upload/verify/cleanup.
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	err := run(logger)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("s3stub stopped", "error", err)

		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	port := os.Getenv("S3STUB_PORT")
	if port == "" {
		port = "9000"
	}

	store := &objectStore{objects: map[string][]byte{}}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleObject(w, r, store, logger)
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","service":"s3stub"}`))
	})

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)

	go func() {
		logger.Info("s3stub listening", "port", port)

		errCh <- server.ListenAndServe()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil

	case <-sig:
		logger.Info("shutdown signal received")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return server.Shutdown(ctx)
}

type objectStore struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

func (s *objectStore) get(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	body, ok := s.objects[key]

	return body, ok
}

func (s *objectStore) put(key string, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.objects[key] = body
}

func (s *objectStore) delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.objects, key)
}

func handleObject(
	w http.ResponseWriter,
	r *http.Request,
	store *objectStore,
	logger *slog.Logger,
) {
	bucket, key, ok := splitKey(r.URL.EscapedPath())
	if !ok {
		logger.Warn("bad object path", "path", r.URL.Path)

		http.Error(w, "bad request", http.StatusBadRequest)

		return
	}

	fqKey := bucket + "/" + key

	switch r.Method {
	case http.MethodPut:
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(2<<30)))
		if err != nil {
			logger.Warn("read body failed", "key", fqKey, "error", err)

			return
		}

		store.put(fqKey, body)

		sum := md5.Sum(body)
		w.Header().Set("ETag", `"`+hex.EncodeToString(sum[:])+`"`)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)

		logger.Info("object stored", "key", fqKey, "bytes", len(body))

	case http.MethodHead:
		body, present := store.get(fqKey)
		if !present {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		sum := md5.Sum(body)
		w.Header().Set("ETag", `"`+hex.EncodeToString(sum[:])+`"`)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		body, present := store.get(fqKey)
		if !present {
			http.NotFound(w, r)

			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		_, _ = w.Write(body)

	case http.MethodDelete:
		store.delete(fqKey)
		w.WriteHeader(http.StatusOK)

		logger.Info("object deleted", "key", fqKey)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// splitKey splits /{bucket}/{key...} into its parts, returning ok=false
// for malformed paths like "/", "/bucket" or the health route.
func splitKey(escapedPath string) (bucket, key string, ok bool) {
	trimmed := strings.TrimPrefix(escapedPath, "/")
	first, rest, found := strings.Cut(trimmed, "/")
	if !found || rest == "" || first == "health" {
		return "", "", false
	}

	unescaped, err := url.PathUnescape(rest)
	if err != nil {
		return "", "", false
	}

	return first, unescaped, true
}
