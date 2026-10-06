package engine

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testClient(server *httptest.Server) *HTTPClient {
	return NewHTTPClient(server.URL, "test-token", 5*time.Second)
}

func TestCreateJob(t *testing.T) {
	var gotAuth string
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/jobs" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}

		gotAuth = r.Header.Get("Authorization")

		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jobId":"eng-123"}`))
	}))
	defer server.Close()

	resp, err := testClient(server).CreateJob(context.Background(), CreateJobRequest{
		Type:   "video_transcode",
		Source: JobSource{ObjectKey: "videos/a/source.mp4"},
		Options: JobOptions{
			GenerateThumbnail: true,
			GenerateHLS:       true,
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if resp.JobID != "eng-123" {
		t.Fatalf("expected eng-123, got %q", resp.JobID)
	}

	if gotAuth != "Bearer test-token" {
		t.Fatalf("expected bearer auth, got %q", gotAuth)
	}

	for _, want := range []string{`"type":"video_transcode"`, `"objectKey":"videos/a/source.mp4"`, `"generateHLS":true`} {
		if !strings.Contains(gotBody, want) {
			t.Fatalf("expected %s in %s", want, gotBody)
		}
	}
}

func TestGetJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/jobs/eng-123" || r.Method != http.MethodGet {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"jobId": "eng-123",
			"status": "processing",
			"progress": 42,
			"output": {"manifestUrl": "", "thumbnailUrl": "", "durationSeconds": 0, "width": 0, "height": 0},
			"error": {"code": "", "message": ""}
		}`))
	}))
	defer server.Close()

	status, err := testClient(server).GetJob(context.Background(), "eng-123")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if status.Status != "processing" || status.Progress != 42 {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestCancelJob(t *testing.T) {
	var called bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/jobs/eng-123/cancel" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}

		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := testClient(server).CancelJob(context.Background(), "eng-123"); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	if !called {
		t.Fatal("expected cancel endpoint to be hit")
	}
}

func TestClientFailures(t *testing.T) {
	t.Run("unavailable", func(t *testing.T) {
		client := NewHTTPClient("http://127.0.0.1:1", "tok", time.Second)

		if _, err := client.CreateJob(context.Background(), CreateJobRequest{}); err == nil {
			t.Fatal("expected connection error")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(300 * time.Millisecond)
		}))
		defer server.Close()

		client := NewHTTPClient(server.URL, "tok", 50*time.Millisecond)

		if _, err := client.GetJob(context.Background(), "x"); err == nil {
			t.Fatal("expected timeout error")
		}
	})

	t.Run("non-2xx", func(t *testing.T) {
		for _, code := range []int{400, 500} {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))

			err := testClient(server).CancelJob(context.Background(), "x")
			server.Close()

			if err == nil {
				t.Fatalf("expected error for %d", code)
			}

			if strings.Contains(err.Error(), "test-token") {
				t.Fatal("error must never contain the internal token")
			}
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{not json`))
		}))
		defer server.Close()

		if _, err := testClient(server).GetJob(context.Background(), "x"); err == nil {
			t.Fatal("expected decode error")
		}
	})

	t.Run("empty job id", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()

		if _, err := testClient(server).CreateJob(
			context.Background(),
			CreateJobRequest{},
		); err == nil {
			t.Fatal("expected empty job id error")
		}
	})
}
