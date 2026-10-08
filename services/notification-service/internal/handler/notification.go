package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/service"
)

type notificationService interface {
	List(
		ctx context.Context,
		userID uuid.UUID,
		unreadOnly bool,
		page int,
		limit int,
	) (*service.NotificationPage, error)
	UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error)
	MarkRead(
		ctx context.Context,
		userID uuid.UUID,
		id uuid.UUID,
	) (*model.Notification, error)
	MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error)
	DeleteNotification(
		ctx context.Context,
		userID uuid.UUID,
		id uuid.UUID,
	) error
}

type NotificationHandler struct {
	notifications notificationService
}

func NewNotificationHandler(
	notifications notificationService,
) *NotificationHandler {
	return &NotificationHandler{
		notifications: notifications,
	}
}

type notificationResponse struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Priority  string     `json:"priority"`
	ReadAt    *time.Time `json:"readAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

type notificationListResponse struct {
	Items      []notificationResponse `json:"items"`
	Pagination paginationResponse     `json:"pagination"`
}

type paginationResponse struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

// List returns the caller's notifications, newest first. Only ever the
// caller's own rows: identity comes solely from the JWT.
func (h *NotificationHandler) List(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	query := r.URL.Query()

	page, err := strconv.Atoi(query.Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(query.Get("limit"))
	if err != nil || limit < 1 {
		limit = 20
	}

	unreadOnly := query.Get("unread") == "true"

	pageOut, err := h.notifications.List(
		r.Context(),
		userID,
		unreadOnly,
		page,
		limit,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	items := make([]notificationResponse, 0, len(pageOut.Items))

	for _, notification := range pageOut.Items {
		items = append(items, toNotificationResponse(notification))
	}

	writeJSON(
		w,
		http.StatusOK,
		notificationListResponse{
			Items: items,
			Pagination: paginationResponse{
				Page:       pageOut.Page,
				Limit:      pageOut.Limit,
				Total:      pageOut.Total,
				TotalPages: pageOut.TotalPages,
			},
		},
	)
}

// UnreadCount returns the caller's unread total via an efficient count.
func (h *NotificationHandler) UnreadCount(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	count, err := h.notifications.UnreadCount(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]int64{"count": count})
}

// MarkRead marks one owned notification read. Idempotent: already-read
// rows succeed; unknown and foreign ids read as not found.
func (h *NotificationHandler) MarkRead(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "notificationID"))
	if err != nil {
		writeBadRequest(w, "NOTIFICATION_NOT_FOUND", "invalid notification id")
		return
	}

	notification, err := h.notifications.MarkRead(r.Context(), userID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toNotificationResponse(notification))
}

// MarkAllRead marks every unread row of the caller. Scoping lives in
// the repository predicate: no cross-user updates possible.
func (h *NotificationHandler) MarkAllRead(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	count, err := h.notifications.MarkAllRead(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]int64{"markedRead": count})
}

// Delete removes one owned notification (deliveries cascade).
func (h *NotificationHandler) Delete(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "notificationID"))
	if err != nil {
		writeBadRequest(w, "NOTIFICATION_NOT_FOUND", "invalid notification id")
		return
	}

	if err := h.notifications.DeleteNotification(r.Context(), userID, id); err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func toNotificationResponse(notification *model.Notification) notificationResponse {
	return notificationResponse{
		ID:        notification.ID.String(),
		Type:      notification.Type,
		Title:     notification.Title,
		Body:      notification.Body,
		Priority:  string(notification.Priority),
		ReadAt:    notification.ReadAt,
		CreatedAt: notification.CreatedAt,
	}
}
