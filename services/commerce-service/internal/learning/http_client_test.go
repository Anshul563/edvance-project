package learning

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestProvisionSuccess(t *testing.T) {
	var gotBody string
	var gotInternalKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/enrollments" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		gotInternalKey = r.Header.Get("X-Internal-Key")

		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewHTTPProvisioner(server.URL, "learning-internal-test-key", 5*time.Second)

	userID := uuid.New()
	courseID := uuid.New()

	if err := client.ProvisionEnrollment(context.Background(), userID, courseID, "purchase"); err != nil {
		t.Fatalf("provision: %v", err)
	}

	for _, want := range []string{userID.String(), courseID.String(), "purchase"} {
		if !strings.Contains(gotBody, want) {
			t.Fatalf("expected %s in %s", want, gotBody)
		}
	}
	if gotInternalKey != "learning-internal-test-key" {
		t.Fatalf("expected internal auth header, got %q", gotInternalKey)
	}
}

func TestProvisionFailures(t *testing.T) {
	t.Run("non-2xx", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewHTTPProvisioner(server.URL, "learning-internal-test-key", 5*time.Second)

		if err := client.ProvisionEnrollment(
			context.Background(),
			uuid.New(),
			uuid.New(),
			"purchase",
		); !errors.Is(err, ErrProvisionFailed) {
			t.Fatalf("expected provision failure, got %v", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(300 * time.Millisecond)
		}))
		defer server.Close()

		client := NewHTTPProvisioner(server.URL, "learning-internal-test-key", 50*time.Millisecond)

		if err := client.ProvisionEnrollment(
			context.Background(),
			uuid.New(),
			uuid.New(),
			"purchase",
		); !errors.Is(err, ErrProvisionFailed) {
			t.Fatalf("expected timeout failure, got %v", err)
		}
	})

	t.Run("unreachable", func(t *testing.T) {
		client := NewHTTPProvisioner("http://127.0.0.1:1", "learning-internal-test-key", time.Second)

		if err := client.ProvisionEnrollment(
			context.Background(),
			uuid.New(),
			uuid.New(),
			"purchase",
		); !errors.Is(err, ErrProvisionFailed) {
			t.Fatalf("expected failure, got %v", err)
		}
	})
}
