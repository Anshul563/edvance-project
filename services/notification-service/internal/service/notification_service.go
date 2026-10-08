package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/ntype"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/repository"
)

var (
	ErrInvalidType          = errors.New("invalid notification type")
	ErrInvalidChannel       = errors.New("invalid channel")
	ErrInvalidPriority      = errors.New("invalid priority")
	ErrEmailRequired        = errors.New("email address required for email channel")
	ErrNotificationNotFound = errors.New("notification not found")
)

// NotificationStore is the persistence contract for notifications.
// *repository.NotificationRepository satisfies it.
type NotificationStore interface {
	CreateWithDeliveries(
		ctx context.Context,
		notification *model.Notification,
		channels []model.Channel,
	) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Notification, error)
	FindByEventID(ctx context.Context, eventID string) (*model.Notification, error)
	ListByUser(
		ctx context.Context,
		userID uuid.UUID,
		unreadOnly bool,
		limit int,
		offset int,
	) ([]*model.Notification, error)
	CountByUser(
		ctx context.Context,
		userID uuid.UUID,
		unreadOnly bool,
	) (int64, error)
	MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Notification, error)
	MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error)
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
}

// PreferenceLookup reads preference rows (nil when never customized).
// *repository.PreferenceRepository satisfies it.
type PreferenceLookup interface {
	GetPreferences(ctx context.Context, userID uuid.UUID) (*model.Preference, error)
}

// TemplateLookup reads active templates (nil when unconfigured).
// *repository.TemplateRepository satisfies it.
type TemplateLookup interface {
	GetTemplate(
		ctx context.Context,
		notificationType string,
		channel model.Channel,
	) (*model.Template, error)
}

// KnownTypes is the registry of supported notification types. Unknown
// types are rejected rather than stored: the type list is a contract,
// not free text.
var KnownTypes = map[string]bool{
	ntype.UserEmailVerified: true,
	ntype.AuthPasswordReset: true,
	ntype.AuthSecurityAlert: true,

	ntype.CoursePublished: true,
	ntype.CourseUpdated:   true,

	ntype.LearningEnrolled:        true,
	ntype.LearningLessonCompleted: true,
	ntype.LearningCourseCompleted: true,

	ntype.PaymentCreated:  true,
	ntype.PaymentCaptured: true,
	ntype.PaymentFailed:   true,
	ntype.PaymentRefunded: true,

	ntype.OrderPaid:   true,
	ntype.OrderFailed: true,

	ntype.VideoProcessingCompleted: true,
	ntype.VideoProcessingFailed:    true,
}

type CreateNotificationRequest struct {
	UserID   uuid.UUID
	Type     string
	Title    string
	Body     string
	Data     map[string]string
	Priority model.Priority
	EventID  string
	Channels []model.Channel
	// Email is the recipient for the email channel. Originating
	// services supply it (they own user contact data); this service
	// never looks it up elsewhere.
	Email string
}

type CreatedNotification struct {
	Notification *model.Notification
	Duplicate    bool
	Email        *EmailContent
}

// EmailContent is the rendered email handed to the delivery layer.
type EmailContent struct {
	To      string
	Subject string
	Body    string
}

// NotificationService owns notification creation, preference-governed
// channel selection, and template rendering. Sending itself happens in
// DeliveryService right after the transaction commits.
type NotificationService struct {
	notifications NotificationStore
	preferences   PreferenceLookup
	templates     TemplateLookup
	delivery      *DeliveryService
}

func NewNotificationService(
	notifications NotificationStore,
	preferences PreferenceLookup,
	templates TemplateLookup,
	delivery *DeliveryService,
) (*NotificationService, error) {
	if notifications == nil || preferences == nil || templates == nil {
		return nil, errors.New("notification dependencies are required")
	}

	if delivery == nil {
		return nil, errors.New("delivery service is required")
	}

	return &NotificationService{
		notifications: notifications,
		preferences:   preferences,
		templates:     templates,
		delivery:      delivery,
	}, nil
}

// Create validates, dedupes by event id, resolves channels through
// preferences, renders templates, and persists notification + delivery
// rows atomically. Email sends fire synchronously afterwards (the dev
// implementation); the retry schedule and ClaimDue polling keep the
// design worker-ready without changing this contract.
func (s *NotificationService) Create(
	ctx context.Context,
	request CreateNotificationRequest,
) (*CreatedNotification, error) {
	if request.UserID == uuid.Nil {
		return nil, errors.New("user id is required")
	}

	if !KnownTypes[request.Type] {
		return nil, ErrInvalidType
	}

	if strings.TrimSpace(request.Title) == "" ||
		strings.TrimSpace(request.Body) == "" {
		return nil, errors.New("title and body are required")
	}

	priority := request.Priority
	if priority == "" {
		priority = model.PriorityNormal
	}

	if !validPriority(priority) {
		return nil, ErrInvalidPriority
	}

	channels := request.Channels
	if len(channels) == 0 {
		channels = []model.Channel{model.ChannelInApp}
	}

	for _, channel := range channels {
		if channel != model.ChannelInApp && channel != model.ChannelEmail {
			return nil, ErrInvalidChannel
		}
	}

	if request.EventID != "" {
		existing, err := s.notifications.FindByEventID(ctx, request.EventID)
		if err == nil {
			return &CreatedNotification{Notification: existing, Duplicate: true}, nil
		}

		if !errors.Is(err, repository.ErrNotificationNotFound) {
			return nil, fmt.Errorf("check event id: %w", err)
		}
	}

	allowed, err := s.resolveChannels(ctx, request)
	if err != nil {
		return nil, err
	}

	title, body, email, err := s.render(ctx, request, allowed)
	if err != nil {
		return nil, err
	}

	data, err := marshalData(request.Data)
	if err != nil {
		return nil, err
	}

	notification := &model.Notification{
		UserID:   request.UserID,
		Type:     request.Type,
		Title:    title,
		Body:     body,
		Data:     data,
		Priority: priority,
	}

	if request.EventID != "" {
		notification.EventID = &request.EventID
	}

	if err := s.notifications.CreateWithDeliveries(ctx, notification, allowed); err != nil {
		if errors.Is(err, repository.ErrDuplicateEvent) {
			// Lost the idempotency race: adopt the winner's row.
			existing, findErr := s.notifications.FindByEventID(ctx, request.EventID)
			if findErr != nil {
				return nil, fmt.Errorf("read duplicate notification: %w", findErr)
			}

			return &CreatedNotification{Notification: existing, Duplicate: true}, nil
		}

		return nil, fmt.Errorf("create notification: %w", err)
	}

	if err := s.delivery.DispatchNew(ctx, notification, email); err != nil {
		return nil, fmt.Errorf("dispatch deliveries: %w", err)
	}

	return &CreatedNotification{Notification: notification, Email: email}, nil
}

// resolveChannels filters requested channels through preferences.
// Security types bypass every toggle; everything else honors the
// channel master switch plus its category flag.
func (s *NotificationService) resolveChannels(
	ctx context.Context,
	request CreateNotificationRequest,
) ([]model.Channel, error) {
	if ntype.IsSecurity(request.Type) {
		return dedupeChannels(request.Channels), nil
	}

	preferences, err := s.preferences.GetPreferences(ctx, request.UserID)
	if err != nil {
		return nil, fmt.Errorf("load preferences: %w", err)
	}

	if preferences == nil {
		preferences = model.Defaults(request.UserID)
	}

	allowed := make([]model.Channel, 0, len(request.Channels))

	for _, channel := range dedupeChannels(request.Channels) {
		switch channel {
		case model.ChannelInApp:
			if !preferences.InAppEnabled {
				continue
			}

		case model.ChannelEmail:
			if !preferences.EmailEnabled {
				continue
			}

			if strings.TrimSpace(request.Email) == "" {
				return nil, ErrEmailRequired
			}

		default:
			continue
		}

		if !categoryAllowed(preferences, request.Type) {
			continue
		}

		allowed = append(allowed, channel)
	}

	return allowed, nil
}

// render applies active DB templates (subject/title/body) over the
// request content, falling back field-by-field when unconfigured.
func (s *NotificationService) render(
	ctx context.Context,
	request CreateNotificationRequest,
	allowed []model.Channel,
) (string, string, *EmailContent, error) {
	title := request.Title
	body := request.Body
	var email *EmailContent

	data := request.Data
	if data == nil {
		data = map[string]string{}
	}

	for _, channel := range allowed {
		if channel != model.ChannelEmail {
			continue
		}

		template, err := s.templates.GetTemplate(ctx, request.Type, channel)
		if err != nil {
			return "", "", nil, fmt.Errorf("load template: %w", err)
		}

		subject := request.Title
		rendered := request.Body

		if template != nil {
			if template.SubjectTemplate != nil && *template.SubjectTemplate != "" {
				subject, err = RenderTemplate(*template.SubjectTemplate, data)
				if err != nil {
					return "", "", nil, fmt.Errorf("render subject: %w", err)
				}
			} else if template.TitleTemplate != nil && *template.TitleTemplate != "" {
				subject, err = RenderTemplate(*template.TitleTemplate, data)
				if err != nil {
					return "", "", nil, fmt.Errorf("render title: %w", err)
				}
			}

			if template.BodyTemplate != "" {
				rendered, err = RenderTemplate(template.BodyTemplate, data)
				if err != nil {
					return "", "", nil, fmt.Errorf("render body: %w", err)
				}
			}
		}

		email = &EmailContent{
			To:      request.Email,
			Subject: subject,
			Body:    rendered,
		}
	}

	return title, body, email, nil
}

func categoryAllowed(preferences *model.Preference, notificationType string) bool {
	switch {
	case strings.HasPrefix(notificationType, "payment."),
		strings.HasPrefix(notificationType, "order."):
		return preferences.PaymentEnabled

	case strings.HasPrefix(notificationType, "learning."):
		return preferences.LearningEnabled

	case strings.HasPrefix(notificationType, "course."):
		return preferences.CourseUpdatesEnabled

	case strings.HasPrefix(notificationType, "user."),
		strings.HasPrefix(notificationType, "auth."):
		return true

	default:
		return true
	}
}

func dedupeChannels(channels []model.Channel) []model.Channel {
	seen := map[model.Channel]bool{}
	out := make([]model.Channel, 0, len(channels))

	for _, channel := range channels {
		if !seen[channel] {
			seen[channel] = true
			out = append(out, channel)
		}
	}

	return out
}

func validPriority(priority model.Priority) bool {
	return priority == model.PriorityLow ||
		priority == model.PriorityNormal ||
		priority == model.PriorityHigh ||
		priority == model.PriorityCritical
}

func marshalData(data map[string]string) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("encode data: %w", err)
	}

	return string(encoded), nil
}

// NotificationPage is a paginated notification slice.
type NotificationPage struct {
	Items      []*model.Notification
	Total      int64
	Page       int
	Limit      int
	TotalPages int64
}

// List returns one user's notifications, newest first, optionally
// unread-only.
func (s *NotificationService) List(
	ctx context.Context,
	userID uuid.UUID,
	unreadOnly bool,
	page int,
	limit int,
) (*NotificationPage, error) {
	if page < 1 {
		page = 1
	}

	if limit < 1 {
		limit = 20
	}

	if limit > 50 {
		limit = 50
	}

	items, err := s.notifications.ListByUser(
		ctx,
		userID,
		unreadOnly,
		limit,
		(page-1)*limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	total, err := s.notifications.CountByUser(ctx, userID, unreadOnly)
	if err != nil {
		return nil, fmt.Errorf("count notifications: %w", err)
	}

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	return &NotificationPage{
		Items:      items,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// UnreadCount returns one user's unread total via an efficient count.
func (s *NotificationService) UnreadCount(
	ctx context.Context,
	userID uuid.UUID,
) (int64, error) {
	total, err := s.notifications.CountByUser(ctx, userID, true)
	if err != nil {
		return 0, fmt.Errorf("count unread: %w", err)
	}

	return total, nil
}

// MarkRead marks one owned notification read. Unknown and foreign ids
// read identically as not found: no oracle.
func (s *NotificationService) MarkRead(
	ctx context.Context,
	userID uuid.UUID,
	id uuid.UUID,
) (*model.Notification, error) {
	notification, err := s.notifications.MarkRead(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotificationNotFound) {
			return nil, ErrNotificationNotFound
		}

		return nil, fmt.Errorf("mark read: %w", err)
	}

	return notification, nil
}

// MarkAllRead marks every unread row of one user. Scoping lives in the
// repository predicate: no cross-user updates possible.
func (s *NotificationService) MarkAllRead(
	ctx context.Context,
	userID uuid.UUID,
) (int64, error) {
	count, err := s.notifications.MarkAllRead(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("mark all read: %w", err)
	}

	return count, nil
}

// DeleteNotification removes one owned notification (deliveries
// cascade). Unknown and foreign ids read as not found.
func (s *NotificationService) DeleteNotification(
	ctx context.Context,
	userID uuid.UUID,
	id uuid.UUID,
) error {
	if err := s.notifications.Delete(ctx, id, userID); err != nil {
		if errors.Is(err, repository.ErrNotificationNotFound) {
			return ErrNotificationNotFound
		}

		return fmt.Errorf("delete notification: %w", err)
	}

	return nil
}
