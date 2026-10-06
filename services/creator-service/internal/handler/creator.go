package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/creator-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/service"
)

type creatorService interface {
	OnboardCreator(ctx context.Context, userID uuid.UUID, input service.OnboardInput) (*service.CreatorProfile, error)
	GetMyCreator(ctx context.Context, userID uuid.UUID) (*service.CreatorProfile, error)
	GetCreatorByHandle(ctx context.Context, handle string) (*service.CreatorProfile, error)
	UpdateCreator(ctx context.Context, userID uuid.UUID, input service.UpdateCreatorInput) (*service.CreatorProfile, error)
}

type CreatorHandler struct {
	creators creatorService
}

func NewCreatorHandler(creators creatorService) *CreatorHandler {
	return &CreatorHandler{
		creators: creators,
	}
}

type channelResponse struct {
	ID          string  `json:"id"`
	Handle      string  `json:"handle"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	BannerURL   *string `json:"bannerUrl,omitempty"`
	AvatarURL   *string `json:"avatarUrl,omitempty"`
}

type myCreatorResponse struct {
	ID          string          `json:"id"`
	UserID      string          `json:"userId"`
	Status      string          `json:"status"`
	DisplayName string          `json:"displayName"`
	Headline    *string         `json:"headline,omitempty"`
	Bio         *string         `json:"bio,omitempty"`
	WebsiteURL  *string         `json:"websiteUrl,omitempty"`
	CoverURL    *string         `json:"coverUrl,omitempty"`
	Channel     channelResponse `json:"channel"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type publicCreatorResponse struct {
	Handle      string  `json:"handle"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	AvatarURL   *string `json:"avatarUrl,omitempty"`
	BannerURL   *string `json:"bannerUrl,omitempty"`
	Status      string  `json:"status"`
}

type onboardRequest struct {
	ChannelName string `json:"channelName"`
	Handle      string `json:"handle"`
	Description string `json:"description"`
}

// Onboard creates the caller's creator and channel atomically. Identity
// comes only from the JWT; any userId in the request is never read.
func (h *CreatorHandler) Onboard(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	var request onboardRequest

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

	profile, err := h.creators.OnboardCreator(
		r.Context(),
		userID,
		service.OnboardInput{
			ChannelName: request.ChannelName,
			Handle:      request.Handle,
			Description: request.Description,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toMyCreatorResponse(profile))
}

// Me returns the caller's creator and channel.
func (h *CreatorHandler) Me(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	profile, err := h.creators.GetMyCreator(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toMyCreatorResponse(profile))
}

type updateCreatorRequest struct {
	DisplayName *string `json:"displayName"`
	Headline    *string `json:"headline"`
	Bio         *string `json:"bio"`
	WebsiteURL  *string `json:"websiteUrl"`
	CoverURL    *string `json:"coverUrl"`
	ChannelName *string `json:"channelName"`
	Description *string `json:"description"`
	AvatarURL   *string `json:"avatarUrl"`
	BannerURL   *string `json:"bannerUrl"`
}

// UpdateMe applies a partial update. Creator ID, user ID, handle, status,
// and timestamps are never client-writable.
func (h *CreatorHandler) UpdateMe(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	var request updateCreatorRequest

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

	profile, err := h.creators.UpdateCreator(
		r.Context(),
		userID,
		service.UpdateCreatorInput{
			DisplayName: request.DisplayName,
			Headline:    request.Headline,
			Bio:         request.Bio,
			WebsiteURL:  request.WebsiteURL,
			CoverURL:    request.CoverURL,
			ChannelName: request.ChannelName,
			Description: request.Description,
			AvatarURL:   request.AvatarURL,
			BannerURL:   request.BannerURL,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toMyCreatorResponse(profile))
}

// ByHandle returns the public creator profile. Suspended/disabled
// creators read as not found.
func (h *CreatorHandler) ByHandle(
	w http.ResponseWriter,
	r *http.Request,
) {
	profile, err := h.creators.GetCreatorByHandle(
		r.Context(),
		chi.URLParam(r, "handle"),
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		publicCreatorResponse{
			Handle:      profile.Channel.Handle,
			Name:        profile.Channel.Name,
			Description: profile.Channel.Description,
			AvatarURL:   profile.Channel.AvatarURL,
			BannerURL:   profile.Channel.BannerURL,
			Status:      string(profile.Creator.Status),
		},
	)
}

func toMyCreatorResponse(profile *service.CreatorProfile) myCreatorResponse {
	return myCreatorResponse{
		ID:          profile.Creator.ID.String(),
		UserID:      profile.Creator.UserID.String(),
		Status:      string(profile.Creator.Status),
		DisplayName: profile.Creator.DisplayName,
		Headline:    profile.Creator.Headline,
		Bio:         profile.Creator.Bio,
		WebsiteURL:  profile.Creator.WebsiteURL,
		CoverURL:    profile.Creator.CoverURL,
		Channel: channelResponse{
			ID:          profile.Channel.ID.String(),
			Handle:      profile.Channel.Handle,
			Name:        profile.Channel.Name,
			Description: profile.Channel.Description,
			BannerURL:   profile.Channel.BannerURL,
			AvatarURL:   profile.Channel.AvatarURL,
		},
		CreatedAt: profile.Creator.CreatedAt,
		UpdatedAt: profile.Creator.UpdatedAt,
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
