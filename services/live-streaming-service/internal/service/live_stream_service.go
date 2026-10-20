package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/mediamtx"
	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/model"
)

var (
	ErrStreamNotFound   = model.ErrStreamNotFound
	ErrForbidden        = errors.New("forbidden")
	ErrInvalidInput     = errors.New("invalid input")
	ErrUnauthorizedPub  = errors.New("publishing credentials rejected")
	ErrAlreadyEnded     = errors.New("stream already ended")
	ErrNothingToReconci = errors.New("media server not configured")
)

const (
	maxTitleLength       = 200
	maxDescriptionLength = 2000
	tokenBytes           = 32
	pathIDBytes          = 8
)

// StreamStore persists stream sessions. Implemented by the PostgreSQL
// repository in production and by an in-memory store in tests.
type StreamStore interface {
	Create(ctx context.Context, stream *model.Stream) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Stream, error)
	GetByPathName(ctx context.Context, pathName string) (*model.Stream, error)
	ListByCreator(ctx context.Context, creatorID uuid.UUID) ([]*model.Stream, error)
	ListActive(ctx context.Context) ([]*model.Stream, error)
	ListLiveOrWaiting(ctx context.Context, updatedAfter time.Time) ([]*model.Stream, error)
	Update(ctx context.Context, stream *model.Stream) error
}

// MediaEngine abstracts the MediaMTX control API.
type MediaEngine interface {
	ListActivePaths(ctx context.Context) (map[string]mediamtx.PathInfo, error)
	KickRTMPConnections(ctx context.Context, path mediamtx.PathInfo) error
}

// PublicAddresses are the externally visible ingest/playback addresses.
// They must be reachable from creator and student devices.
type PublicAddresses struct {
	RTMPURL    string
	HLSBaseURL string
	PathPrefix string
}

// CreateStreamInput is the creator-facing create request.
type CreateStreamInput struct {
	Title       string
	Description string
}

// StreamWithKey pairs a session with the raw publisher stream key. The
// raw key is only ever produced at creation or regeneration and is only
// ever returned to the authenticated owner.
type StreamWithKey struct {
	Stream    *model.Stream
	StreamKey string
}

// PlaybackInfo is the public, secret-free playback description handed to
// students.
type PlaybackInfo struct {
	ID          uuid.UUID    `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Status      model.Status `json:"status"`
	IsLive      bool         `json:"isLive"`
	PlaybackURL string       `json:"playbackUrl"`
	CreatedAt   time.Time    `json:"createdAt"`
	StartedAt   *time.Time   `json:"startedAt,omitempty"`
}

type LiveStreamService struct {
	store   StreamStore
	media   MediaEngine
	public  PublicAddresses
	now     func() time.Time
	logger  *slog.Logger
	kickTTL time.Duration
}

func NewLiveStreamService(
	store StreamStore,
	media MediaEngine,
	public PublicAddresses,
	logger *slog.Logger,
) *LiveStreamService {
	if logger == nil {
		logger = slog.Default()
	}
	if public.PathPrefix == "" {
		public.PathPrefix = "ls"
	}

	return &LiveStreamService{
		store:   store,
		media:   media,
		public:  public,
		now:     time.Now,
		logger:  logger,
		kickTTL: 5 * time.Second,
	}
}

// CreateSession creates a stream session, generates a fresh publisher
// token and returns the session together with the raw stream key. The
// session starts in status "waiting": it only becomes "live" once the
// media server reports a connected publisher.
func (s *LiveStreamService) CreateSession(
	ctx context.Context,
	creatorID uuid.UUID,
	input CreateStreamInput,
) (*StreamWithKey, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" || len(title) > maxTitleLength {
		return nil, fmt.Errorf("%w: title must be 1-%d characters", ErrInvalidInput, maxTitleLength)
	}

	description := strings.TrimSpace(input.Description)
	if len(description) > maxDescriptionLength {
		return nil, fmt.Errorf("%w: description must be at most %d characters", ErrInvalidInput, maxDescriptionLength)
	}

	pathName, err := s.generatePathName()
	if err != nil {
		return nil, fmt.Errorf("generate stream path: %w", err)
	}

	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate stream token: %w", err)
	}

	now := s.now().UTC()
	stream := &model.Stream{
		ID:            uuid.New(),
		CreatorID:     creatorID,
		Title:         title,
		Description:   description,
		Status:        model.StatusWaiting,
		PathName:      pathName,
		StreamKeyHash: hashToken(token),
		StreamKeyHint: hintToken(token),
		RTMPURL:       s.public.RTMPURL,
		PlaybackURL:   s.playbackURL(pathName),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.store.Create(ctx, stream); err != nil {
		return nil, fmt.Errorf("persist stream session: %w", err)
	}

	return &StreamWithKey{
		Stream:    stream.Clone(),
		StreamKey: streamKeyFor(pathName, token),
	}, nil
}

// GetOwnSession returns a session the creator owns.
func (s *LiveStreamService) GetOwnSession(
	ctx context.Context,
	creatorID uuid.UUID,
	id uuid.UUID,
) (*model.Stream, error) {
	stream, err := s.store.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, model.ErrStreamNotFound) {
			return nil, ErrStreamNotFound
		}
		return nil, err
	}
	if stream.CreatorID != creatorID {
		return nil, ErrForbidden
	}

	return stream.Clone(), nil
}

// ListCreatorSessions lists the creator's own sessions, newest first.
func (s *LiveStreamService) ListCreatorSessions(
	ctx context.Context,
	creatorID uuid.UUID,
) ([]*model.Stream, error) {
	streams, err := s.store.ListByCreator(ctx, creatorID)
	if err != nil {
		return nil, err
	}

	out := make([]*model.Stream, 0, len(streams))
	for _, stream := range streams {
		out = append(out, stream.Clone())
	}

	return out, nil
}

// ListActiveSessions lists sessions that are currently live. This is the
// student-facing discovery endpoint; it never exposes publisher secrets.
func (s *LiveStreamService) ListActiveSessions(ctx context.Context) ([]PlaybackInfo, error) {
	streams, err := s.store.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]PlaybackInfo, 0, len(streams))
	for _, stream := range streams {
		out = append(out, toPlaybackInfo(stream))
	}

	return out, nil
}

// GetPlayback returns public playback information for a session. The
// stream key is never included.
func (s *LiveStreamService) GetPlayback(ctx context.Context, id uuid.UUID) (*PlaybackInfo, error) {
	stream, err := s.store.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, model.ErrStreamNotFound) {
			return nil, ErrStreamNotFound
		}
		return nil, err
	}

	info := toPlaybackInfo(stream)
	return &info, nil
}

// EndSession terminates a session: publishing credentials are revoked
// (status "ended" makes the authorization hook reject OBS) and any
// connected publisher is kicked from the media server. Idempotent.
func (s *LiveStreamService) EndSession(
	ctx context.Context,
	creatorID uuid.UUID,
	id uuid.UUID,
) (*model.Stream, error) {
	stream, err := s.GetOwnSession(ctx, creatorID, id)
	if err != nil {
		return nil, err
	}
	if stream.Status == model.StatusEnded {
		return stream, nil
	}

	now := s.now().UTC()
	stream.Status = model.StatusEnded
	stream.EndedAt = &now
	stream.UpdatedAt = now

	if err := s.store.Update(ctx, stream); err != nil {
		return nil, fmt.Errorf("end stream session: %w", err)
	}

	s.kickPublisher(stream.PathName)

	return stream.Clone(), nil
}

// RegenerateStreamKey rotates the publisher token of a session. The path
// name (and therefore the OBS server URL and the playback URL) stays the
// same; only the token embedded in the stream key changes. Idempotent in
// the sense that each call produces a new key.
func (s *LiveStreamService) RegenerateStreamKey(
	ctx context.Context,
	creatorID uuid.UUID,
	id uuid.UUID,
) (*StreamWithKey, error) {
	stream, err := s.GetOwnSession(ctx, creatorID, id)
	if err != nil {
		return nil, err
	}
	if stream.Status == model.StatusEnded {
		return nil, ErrAlreadyEnded
	}

	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate stream token: %w", err)
	}

	stream.StreamKeyHash = hashToken(token)
	stream.StreamKeyHint = hintToken(token)
	stream.UpdatedAt = s.now().UTC()

	if err := s.store.Update(ctx, stream); err != nil {
		return nil, fmt.Errorf("update stream key: %w", err)
	}

	return &StreamWithKey{
		Stream:    stream.Clone(),
		StreamKey: streamKeyFor(stream.PathName, token),
	}, nil
}

// AuthorizePublish validates a MediaMTX publish authorization request.
// The stream key token (from the OBS stream key query parameter) must
// match the stored hash and the session must not have ended.
func (s *LiveStreamService) AuthorizePublish(
	ctx context.Context,
	pathName string,
	token string,
) error {
	pathName = strings.TrimSpace(pathName)
	if pathName == "" || strings.TrimSpace(token) == "" {
		return ErrUnauthorizedPub
	}

	stream, err := s.store.GetByPathName(ctx, pathName)
	if err != nil {
		if errors.Is(err, model.ErrStreamNotFound) {
			return ErrUnauthorizedPub
		}
		return err
	}

	if stream.Status == model.StatusEnded {
		return ErrUnauthorizedPub
	}

	if subtleEqual(stream.StreamKeyHash, hashToken(token)) {
		return nil
	}

	return ErrUnauthorizedPub
}

// MarkPublisherOnline is the MediaMTX runOnOnline hook: the publisher is
// connected and media is available. Idempotent for repeated events.
func (s *LiveStreamService) MarkPublisherOnline(ctx context.Context, pathName string) error {
	stream, err := s.store.GetByPathName(ctx, strings.TrimSpace(pathName))
	if err != nil {
		return err
	}
	if stream.Status == model.StatusEnded {
		// The session was ended between authorization and this event;
		// ignore the stale event.
		return nil
	}
	if stream.Status == model.StatusLive {
		return nil
	}

	now := s.now().UTC()
	stream.Status = model.StatusLive
	if stream.StartedAt == nil {
		started := now
		stream.StartedAt = &started
	}
	stream.UpdatedAt = now

	return s.store.Update(ctx, stream)
}

// MarkPublisherOffline is the MediaMTX runOnOffline hook: the publisher
// disconnected. The session returns to "waiting" so OBS can reconnect
// until the creator ends it. Idempotent for repeated events.
func (s *LiveStreamService) MarkPublisherOffline(ctx context.Context, pathName string) error {
	stream, err := s.store.GetByPathName(ctx, strings.TrimSpace(pathName))
	if err != nil {
		return err
	}
	if stream.Status != model.StatusLive {
		return nil
	}

	now := s.now().UTC()
	stream.Status = model.StatusWaiting
	stream.UpdatedAt = now

	return s.store.Update(ctx, stream)
}

// Reconcile syncs session status with the media server, covering missed
// hook events (process restarts, dropped hook deliveries).
func (s *LiveStreamService) Reconcile(ctx context.Context) error {
	if s.media == nil {
		return ErrNothingToReconci
	}

	paths, err := s.media.ListActivePaths(ctx)
	if err != nil {
		return fmt.Errorf("list media server paths: %w", err)
	}

	streams, err := s.store.ListLiveOrWaiting(ctx, s.now().Add(-24*time.Hour))
	if err != nil {
		return err
	}

	for _, stream := range streams {
		_, publishing := paths[stream.PathName]

		switch {
		case stream.Status == model.StatusLive && !publishing:
			if err := s.MarkPublisherOffline(ctx, stream.PathName); err != nil {
				return err
			}
			s.logger.Info("reconcile: publisher gone", "stream_id", stream.ID, "path", stream.PathName)
		case stream.Status == model.StatusWaiting && publishing:
			if err := s.MarkPublisherOnline(ctx, stream.PathName); err != nil {
				return err
			}
			s.logger.Info("reconcile: publisher present", "stream_id", stream.ID, "path", stream.PathName)
		}
	}

	return nil
}

// StartReconciler runs Reconcile on an interval until the context is
// cancelled. Failures are logged and retried on the next tick.
func (s *LiveStreamService) StartReconciler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcileCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if err := s.Reconcile(reconcileCtx); err != nil && !errors.Is(reconcileCtx.Err(), context.Canceled) {
				s.logger.Warn("reconcile failed", "error", err)
			}
			cancel()
		}
	}
}

func (s *LiveStreamService) kickPublisher(pathName string) {
	if s.media == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.kickTTL)
	defer cancel()

	paths, err := s.media.ListActivePaths(ctx)
	if err != nil {
		s.logger.Warn("kick publisher: list paths failed", "path", pathName, "error", err)
		return
	}

	path, ok := paths[pathName]
	if !ok {
		return
	}

	if err := s.media.KickRTMPConnections(ctx, path); err != nil {
		s.logger.Warn("kick publisher failed", "path", pathName, "error", err)
	}
}

func (s *LiveStreamService) playbackURL(pathName string) string {
	return strings.TrimRight(s.public.HLSBaseURL, "/") + "/" + pathName + "/index.m3u8"
}

func (s *LiveStreamService) generatePathName() (string, error) {
	suffix, err := randomHex(pathIDBytes)
	if err != nil {
		return "", err
	}
	return s.public.PathPrefix + "_" + suffix, nil
}

func toPlaybackInfo(stream *model.Stream) PlaybackInfo {
	return PlaybackInfo{
		ID:          stream.ID,
		Title:       stream.Title,
		Description: stream.Description,
		Status:      stream.Status,
		IsLive:      stream.Status == model.StatusLive,
		PlaybackURL: stream.PlaybackURL,
		CreatedAt:   stream.CreatedAt,
		StartedAt:   stream.StartedAt,
	}
}

// streamKeyFor formats the value the creator pastes into OBS: the public
// path plus the secret token as a query parameter. MediaMTX extracts the
// token for authorization and never includes it in the playback URL.
func streamKeyFor(pathName, token string) string {
	return pathName + "?token=" + url.QueryEscape(token)
}

func generateToken() (string, error) {
	return randomHex(tokenBytes)
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func hintToken(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:8]
}

func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
