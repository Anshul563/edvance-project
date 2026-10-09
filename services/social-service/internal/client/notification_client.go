package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrNotificationUnavailable = errors.New("notification service unavailable")

type NotificationClient interface {
	EmitFollowed(ctx context.Context, followerID, followingID uuid.UUID) error
	EmitCommentCreated(ctx context.Context, actorID, targetUserID uuid.UUID, contentRef string) error
	EmitCommentReplied(ctx context.Context, actorID, targetUserID uuid.UUID, contentRef string) error
}

type HTTPNotificationClient struct {
	baseURL    string
	serviceKey string
	http       *http.Client
}

func NewHTTPNotificationClient(baseURL, serviceKey string, timeout time.Duration) *NotificationClient {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	client := &HTTPNotificationClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		serviceKey: serviceKey,
		http: &http.Client{
			Timeout: timeout,
		},
	}

	var iface NotificationClient = client
	return &iface
}

type eventRequest struct {
	EventID string            `json:"eventId"`
	Type    string            `json:"type"`
	UserID  string            `json:"userId"`
	Data    map[string]string `json:"data"`
}

func (c *HTTPNotificationClient) emitEvent(
	ctx context.Context,
	evtType string,
	targetUserID uuid.UUID,
	data map[string]string,
) error {
	if c.serviceKey == "" {
		return ErrNotificationUnavailable
	}

	payload := eventRequest{
		EventID: fmt.Sprintf("%s-%s-%s-%d", evtType, targetUserID.String(), strings.Join(dataValues(data), ":"), time.Now().UnixNano()),
		Type:    evtType,
		UserID:  targetUserID.String(),
		Data:    data,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: encode", ErrNotificationUnavailable)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/internal/v1/notifications/events",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("%w: build", ErrNotificationUnavailable)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.serviceKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotificationUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("%w: status %d", ErrNotificationUnavailable, resp.StatusCode)
}

func dataValues(m map[string]string) []string {
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func (c *HTTPNotificationClient) EmitFollowed(ctx context.Context, followerID, followingID uuid.UUID) error {
	return c.emitEvent(ctx, "social.followed", followingID, map[string]string{
		"followerId":  followerID.String(),
		"followingId": followingID.String(),
	})
}

func (c *HTTPNotificationClient) EmitCommentCreated(ctx context.Context, actorID, targetUserID uuid.UUID, contentRef string) error {
	return c.emitEvent(ctx, "social.comment_created", targetUserID, map[string]string{
		"actorId":     actorID.String(),
		"contentRef":  contentRef,
		"targetUserId": targetUserID.String(),
	})
}

func (c *HTTPNotificationClient) EmitCommentReplied(ctx context.Context, actorID, targetUserID uuid.UUID, contentRef string) error {
	return c.emitEvent(ctx, "social.comment_replied", targetUserID, map[string]string{
		"actorId":     actorID.String(),
		"contentRef":  contentRef,
		"targetUserId": targetUserID.String(),
	})
}
