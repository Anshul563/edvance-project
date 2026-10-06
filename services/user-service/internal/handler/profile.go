package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/user-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/user-service/internal/model"
	"github.com/Anshul563/edvance-project/services/user-service/internal/service"
)

type profileService interface {
	GetMyProfile(ctx context.Context, userID uuid.UUID) (*model.UserProfile, error)
	GetProfileByUsername(ctx context.Context, username string) (*model.UserProfile, error)
	UpdateMyProfile(ctx context.Context, userID uuid.UUID, input service.UpdateProfileInput) (*model.UserProfile, error)
	UpdateUsername(ctx context.Context, userID uuid.UUID, username string) (*model.UserProfile, error)
	CheckUsername(ctx context.Context, username string) (*service.CheckUsernameOutput, error)
}

type ProfileHandler struct {
	profiles profileService
}

func NewProfileHandler(profiles profileService) *ProfileHandler {
	return &ProfileHandler{
		profiles: profiles,
	}
}

type myProfileResponse struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	Bio         *string   `json:"bio,omitempty"`
	AvatarURL   *string   `json:"avatarUrl,omitempty"`
	CoverURL    *string   `json:"coverUrl,omitempty"`
	WebsiteURL  *string   `json:"websiteUrl,omitempty"`
	Location    *string   `json:"location,omitempty"`
	CountryCode *string   `json:"countryCode,omitempty"`
	Timezone    *string   `json:"timezone,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type publicProfileResponse struct {
	Username    string  `json:"username"`
	DisplayName string  `json:"displayName"`
	Bio         *string `json:"bio,omitempty"`
	AvatarURL   *string `json:"avatarUrl,omitempty"`
	CoverURL    *string `json:"coverUrl,omitempty"`
	WebsiteURL  *string `json:"websiteUrl,omitempty"`
}

type extendedPublicProfileResponse struct {
	publicProfileResponse
	Location    *string   `json:"location,omitempty"`
	CountryCode *string   `json:"countryCode,omitempty"`
	Timezone    *string   `json:"timezone,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Me returns the caller's own full profile. Identity comes only from the
// JWT; any userId in the request is never read.
func (h *ProfileHandler) Me(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	profile, err := h.profiles.GetMyProfile(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toMyProfileResponse(profile))
}

type updateProfileRequest struct {
	Username    *string `json:"username"`
	DisplayName *string `json:"displayName"`
	Bio         *string `json:"bio"`
	AvatarURL   *string `json:"avatarUrl"`
	CoverURL    *string `json:"coverUrl"`
	WebsiteURL  *string `json:"websiteUrl"`
	Location    *string `json:"location"`
	CountryCode *string `json:"countryCode"`
	Timezone    *string `json:"timezone"`
}

// UpdateMe applies a partial profile update. Username changes are
// rejected here with guidance toward PATCH /users/me/username.
func (h *ProfileHandler) UpdateMe(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	var request updateProfileRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid request body",
			},
		)
		return
	}

	if request.Username != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "use PATCH /users/me/username to change username",
			},
		)
		return
	}

	profile, err := h.profiles.UpdateMyProfile(
		r.Context(),
		userID,
		service.UpdateProfileInput{
			DisplayName: request.DisplayName,
			Bio:         request.Bio,
			AvatarURL:   request.AvatarURL,
			CoverURL:    request.CoverURL,
			WebsiteURL:  request.WebsiteURL,
			Location:    request.Location,
			CountryCode: request.CountryCode,
			Timezone:    request.Timezone,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toMyProfileResponse(profile))
}

type updateUsernameRequest struct {
	Username string `json:"username"`
}

// UpdateMyUsername changes the caller's username.
func (h *ProfileHandler) UpdateMyUsername(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	var request updateUsernameRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid request body",
			},
		)
		return
	}

	if request.Username == "" {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "username is required",
			},
		)
		return
	}

	profile, err := h.profiles.UpdateUsername(
		r.Context(),
		userID,
		request.Username,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toMyProfileResponse(profile))
}

type checkUsernameResponse struct {
	Username  string `json:"username"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// CheckUsername is public: it reports whether a username can be claimed.
func (h *ProfileHandler) CheckUsername(
	w http.ResponseWriter,
	r *http.Request,
) {
	output, err := h.profiles.CheckUsername(
		r.Context(),
		chi.URLParam(r, "username"),
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		checkUsernameResponse{
			Username:  output.Username,
			Available: output.Available,
			Reason:    output.Reason,
		},
	)
}

// ByUsername returns the public profile card for a username.
func (h *ProfileHandler) ByUsername(
	w http.ResponseWriter,
	r *http.Request,
) {
	profile, err := h.profiles.GetProfileByUsername(
		r.Context(),
		chi.URLParam(r, "username"),
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		publicProfileResponse{
			Username:    profile.Username,
			DisplayName: profile.DisplayName,
			Bio:         profile.Bio,
			AvatarURL:   profile.AvatarURL,
			CoverURL:    profile.CoverURL,
			WebsiteURL:  profile.WebsiteURL,
		},
	)
}

// PublicProfile returns the extended public profile for a username.
func (h *ProfileHandler) PublicProfile(
	w http.ResponseWriter,
	r *http.Request,
) {
	profile, err := h.profiles.GetProfileByUsername(
		r.Context(),
		chi.URLParam(r, "username"),
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		extendedPublicProfileResponse{
			publicProfileResponse: publicProfileResponse{
				Username:    profile.Username,
				DisplayName: profile.DisplayName,
				Bio:         profile.Bio,
				AvatarURL:   profile.AvatarURL,
				CoverURL:    profile.CoverURL,
				WebsiteURL:  profile.WebsiteURL,
			},
			Location:    profile.Location,
			CountryCode: profile.CountryCode,
			Timezone:    profile.Timezone,
			CreatedAt:   profile.CreatedAt,
		},
	)
}

func toMyProfileResponse(profile *model.UserProfile) myProfileResponse {
	return myProfileResponse{
		ID:          profile.ID.String(),
		UserID:      profile.UserID.String(),
		Username:    profile.Username,
		DisplayName: profile.DisplayName,
		Bio:         profile.Bio,
		AvatarURL:   profile.AvatarURL,
		CoverURL:    profile.CoverURL,
		WebsiteURL:  profile.WebsiteURL,
		Location:    profile.Location,
		CountryCode: profile.CountryCode,
		Timezone:    profile.Timezone,
		CreatedAt:   profile.CreatedAt,
		UpdatedAt:   profile.UpdatedAt,
	}
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "unauthorized",
	})
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	data any,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}
