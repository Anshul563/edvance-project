package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/service"
)

type preferenceService interface {
	Get(ctx context.Context, userID uuid.UUID) (*model.Preference, error)
	Update(
		ctx context.Context,
		userID uuid.UUID,
		input service.UpdatePreferencesInput,
	) (*model.Preference, error)
}

type PreferenceHandler struct {
	preferences preferenceService
}

func NewPreferenceHandler(
	preferences preferenceService,
) *PreferenceHandler {
	return &PreferenceHandler{
		preferences: preferences,
	}
}

type preferenceResponse struct {
	EmailEnabled         bool `json:"emailEnabled"`
	InAppEnabled         bool `json:"inAppEnabled"`
	MarketingEnabled     bool `json:"marketingEnabled"`
	CourseUpdatesEnabled bool `json:"courseUpdatesEnabled"`
	LearningEnabled      bool `json:"learningEnabled"`
	PaymentEnabled       bool `json:"paymentEnabled"`
	SecurityEnabled      bool `json:"securityEnabled"`
	CreatorEnabled       bool `json:"creatorEnabled"`
}

// Get returns the caller's effective preferences (defaults when never
// customized).
func (h *PreferenceHandler) Get(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	preference, err := h.preferences.Get(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toPreferenceResponse(preference))
}

type updatePreferenceRequest struct {
	EmailEnabled         *bool `json:"emailEnabled"`
	InAppEnabled         *bool `json:"inAppEnabled"`
	MarketingEnabled     *bool `json:"marketingEnabled"`
	CourseUpdatesEnabled *bool `json:"courseUpdatesEnabled"`
	LearningEnabled      *bool `json:"learningEnabled"`
	PaymentEnabled       *bool `json:"paymentEnabled"`
	SecurityEnabled      *bool `json:"securityEnabled"`
	CreatorEnabled       *bool `json:"creatorEnabled"`
}

// Update applies a partial preference update. securityEnabled is forced
// true regardless of input: security notifications are mandatory.
func (h *PreferenceHandler) Update(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	var request updatePreferenceRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "PREFERENCE_UPDATE_FAILED", "invalid request body")
		return
	}

	preference, err := h.preferences.Update(
		r.Context(),
		userID,
		service.UpdatePreferencesInput{
			EmailEnabled:         request.EmailEnabled,
			InAppEnabled:         request.InAppEnabled,
			MarketingEnabled:     request.MarketingEnabled,
			CourseUpdatesEnabled: request.CourseUpdatesEnabled,
			LearningEnabled:      request.LearningEnabled,
			PaymentEnabled:       request.PaymentEnabled,
			SecurityEnabled:      request.SecurityEnabled,
			CreatorEnabled:       request.CreatorEnabled,
		},
	)
	if err != nil {
		writeBadRequest(w, "PREFERENCE_UPDATE_FAILED", "invalid preferences")
		return
	}

	writeJSON(w, http.StatusOK, toPreferenceResponse(preference))
}

func toPreferenceResponse(preference *model.Preference) preferenceResponse {
	return preferenceResponse{
		EmailEnabled:         preference.EmailEnabled,
		InAppEnabled:         preference.InAppEnabled,
		MarketingEnabled:     preference.MarketingEnabled,
		CourseUpdatesEnabled: preference.CourseUpdatesEnabled,
		LearningEnabled:      preference.LearningEnabled,
		PaymentEnabled:       preference.PaymentEnabled,
		SecurityEnabled:      preference.SecurityEnabled,
		CreatorEnabled:       preference.CreatorEnabled,
	}
}
