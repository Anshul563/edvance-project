package handler

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Anshul563/edvance-project/services/content-service/internal/service"
)

func TestMapServiceError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"not found", service.ErrNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"unauthorized", service.ErrUnauthorized, http.StatusUnauthorized, "UNAUTHORIZED"},
		{"forbidden", service.ErrForbidden, http.StatusForbidden, "FORBIDDEN"},
		{"no creator profile", service.ErrNoCreatorProfile, http.StatusForbidden, "CREATOR_PROFILE_REQUIRED"},
		{"duplicate", service.ErrDuplicate, http.StatusConflict, "ALREADY_EXISTS"},
		{"invalid status", service.ErrInvalidStatus, http.StatusConflict, "INVALID_STATUS"},
		{"not publishable", service.ErrNotPublishable, http.StatusConflict, "NOT_PUBLISHABLE"},
		{"invalid visibility", service.ErrInvalidVisibility, http.StatusBadRequest, "INVALID_VISIBILITY"},
		{"invalid input", service.ErrInvalidInput, http.StatusBadRequest, "INVALID_INPUT"},
		{"creator unavailable", service.ErrCreatorUnavailable, http.StatusInternalServerError, "INTERNAL"},
		{"wrapped not found", fmt.Errorf("load tags: %w", service.ErrNotFound), http.StatusNotFound, "NOT_FOUND"},
		{"unknown", errors.New("boom"), http.StatusInternalServerError, "INTERNAL"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := mapServiceError(tc.err)

			if status != tc.wantStatus {
				t.Fatalf("expected %d, got %d", tc.wantStatus, status)
			}

			if body.Code != tc.wantCode {
				t.Fatalf("expected %s, got %s", tc.wantCode, body.Code)
			}

			if body.Message == "" {
				t.Fatal("message must never be empty")
			}
		})
	}
}
