package course

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGetCourse(t *testing.T) {
	courseID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/courses/"+courseID.String() {
			t.Errorf("unexpected path %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"course": {
				"id": "` + courseID.String() + `",
				"creatorId": "` + uuid.NewString() + `",
				"title": "Go",
				"slug": "go",
				"status": "published",
				"visibility": "public",
				"priceCents": 99900,
				"currency": "INR"
			}
		}`))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 5*time.Second)

	course, err := client.GetCourse(context.Background(), courseID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if !course.Saleable() || course.PriceCents != 99900 {
		t.Fatalf("unexpected course: %+v", course)
	}
}

func TestClientFailures(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewHTTPClient(server.URL, 5*time.Second)

		if _, err := client.GetCourse(
			context.Background(),
			uuid.New(),
		); !errors.Is(err, ErrCourseNotFound) {
			t.Fatalf("expected not-found, got %v", err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewHTTPClient(server.URL, 5*time.Second)

		if _, err := client.GetCourse(
			context.Background(),
			uuid.New(),
		); !errors.Is(err, ErrCourseUnavailable) {
			t.Fatalf("expected unavailable, got %v", err)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{broken`))
		}))
		defer server.Close()

		client := NewHTTPClient(server.URL, 5*time.Second)

		if _, err := client.GetCourse(
			context.Background(),
			uuid.New(),
		); !errors.Is(err, ErrCourseUnavailable) {
			t.Fatalf("expected unavailable, got %v", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(300 * time.Millisecond)
		}))
		defer server.Close()

		client := NewHTTPClient(server.URL, 50*time.Millisecond)

		if _, err := client.GetCourse(
			context.Background(),
			uuid.New(),
		); !errors.Is(err, ErrCourseUnavailable) {
			t.Fatalf("expected unavailable, got %v", err)
		}
	})
}
