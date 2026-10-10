package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type SessionStatus string

const (
	SessionStatusReady SessionStatus = "ready"
	SessionStatusLive  SessionStatus = "live"
	SessionStatusEnded SessionStatus = "ended"
)

type CreateSessionInput struct {
	HostID    string `json:"host_id"`
	Title     string `json:"title"`
	Desc      string `json:"description,omitempty"`
	Provider  string `json:"provider,omitempty"`
	IsPrivate bool   `json:"is_private,omitempty"`
}

type StreamSession struct {
	ID          string        `json:"id"`
	HostID      string        `json:"host_id"`
	Title       string        `json:"title"`
	Description string        `json:"description,omitempty"`
	Provider    string        `json:"provider"`
	Status      SessionStatus `json:"status"`
	StreamKey   string        `json:"stream_key"`
	RTMPURL     string        `json:"rtmp_url"`
	PlaybackURL string        `json:"playback_url"`
	IsPrivate   bool          `json:"is_private,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

type Provider interface {
	CreateStream(ctx context.Context, request CreateStreamRequest) (CreateStreamResponse, error)
	StartStream(ctx context.Context, sessionID string) error
	StopStream(ctx context.Context, sessionID string) error
}

type CreateStreamRequest struct {
	SessionID string
	HostID    string
	Title     string
	Provider  string
}

type CreateStreamResponse struct {
	Provider    string
	StreamKey   string
	RTMPURL     string
	PlaybackURL string
}

type SessionStore interface {
	Create(session *StreamSession) error
	Get(id string) (*StreamSession, error)
	ListByHost(hostID string) ([]*StreamSession, error)
	Update(session *StreamSession) error
}

type LiveStreamService struct {
	store    SessionStore
	provider Provider
}

func NewLiveStreamService(store SessionStore, provider Provider) *LiveStreamService {
	if store == nil {
		store = newMemoryStore()
	}
	if provider == nil {
		provider = newMockProvider()
	}
	return &LiveStreamService{store: store, provider: provider}
}

func (s *LiveStreamService) CreateSession(ctx context.Context, input CreateSessionInput) (*StreamSession, error) {
	if strings.TrimSpace(input.HostID) == "" {
		return nil, errors.New("host_id is required")
	}
	if strings.TrimSpace(input.Title) == "" {
		return nil, errors.New("title is required")
	}

	providerName := strings.TrimSpace(input.Provider)
	if providerName == "" {
		providerName = "mock"
	}

	sessionID := uuid.NewString()
	now := time.Now()
	response, err := s.provider.CreateStream(ctx, CreateStreamRequest{
		SessionID: sessionID,
		HostID:    input.HostID,
		Title:     input.Title,
		Provider:  providerName,
	})
	if err != nil {
		return nil, fmt.Errorf("create stream in provider %s: %w", providerName, err)
	}

	session := &StreamSession{
		ID:          sessionID,
		HostID:      input.HostID,
		Title:       strings.TrimSpace(input.Title),
		Description: strings.TrimSpace(input.Desc),
		Provider:    response.Provider,
		Status:      SessionStatusReady,
		StreamKey:   response.StreamKey,
		RTMPURL:     response.RTMPURL,
		PlaybackURL: response.PlaybackURL,
		IsPrivate:   input.IsPrivate,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if session.Provider == "" {
		session.Provider = providerName
	}
	if err := s.store.Create(session); err != nil {
		return nil, fmt.Errorf("persist stream session: %w", err)
	}
	return cloneSession(session), nil
}

func (s *LiveStreamService) ListSessions(ctx context.Context, hostID string) ([]*StreamSession, error) {
	if strings.TrimSpace(hostID) == "" {
		return nil, errors.New("host_id is required")
	}
	sessions, err := s.store.ListByHost(hostID)
	if err != nil {
		return nil, fmt.Errorf("list stream sessions: %w", err)
	}
	out := make([]*StreamSession, 0, len(sessions))
	for _, session := range sessions {
		out = append(out, cloneSession(session))
	}
	return out, nil
}

func (s *LiveStreamService) GetSession(ctx context.Context, id string) (*StreamSession, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("session_id is required")
	}
	session, err := s.store.Get(id)
	if err != nil {
		return nil, fmt.Errorf("load stream session: %w", err)
	}
	return cloneSession(session), nil
}

func (s *LiveStreamService) StartSession(ctx context.Context, id string) error {
	session, err := s.store.Get(id)
	if err != nil {
		return err
	}
	if session.Status == SessionStatusEnded {
		return errors.New("session already ended")
	}
	if err := s.provider.StartStream(ctx, id); err != nil {
		return fmt.Errorf("start stream for session %s: %w", id, err)
	}
	session.Status = SessionStatusLive
	session.UpdatedAt = time.Now()
	return s.store.Update(session)
}

func (s *LiveStreamService) StopSession(ctx context.Context, id string) error {
	session, err := s.store.Get(id)
	if err != nil {
		return err
	}
	if err := s.provider.StopStream(ctx, id); err != nil {
		return fmt.Errorf("stop stream for session %s: %w", id, err)
	}
	session.Status = SessionStatusEnded
	session.UpdatedAt = time.Now()
	return s.store.Update(session)
}

func cloneSession(in *StreamSession) *StreamSession {
	if in == nil {
		return nil
	}
	clone := *in
	return &clone
}

type memoryStore struct {
	mu       sync.Mutex
	sessions map[string]*StreamSession
}

func newMemoryStore() *memoryStore {
	return &memoryStore{sessions: make(map[string]*StreamSession)}
}

func (s *memoryStore) Create(session *StreamSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session == nil {
		return errors.New("session is required")
	}
	s.sessions[session.ID] = cloneSession(session)
	return nil
}

func (s *memoryStore) Get(id string) (*StreamSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session, ok := s.sessions[id]; ok {
		return cloneSession(session), nil
	}
	return nil, fmt.Errorf("session %s not found", id)
}

func (s *memoryStore) ListByHost(hostID string) ([]*StreamSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*StreamSession, 0)
	for _, session := range s.sessions {
		if session.HostID == hostID {
			out = append(out, cloneSession(session))
		}
	}
	return out, nil
}

func (s *memoryStore) Update(session *StreamSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session == nil {
		return errors.New("session is required")
	}
	if _, ok := s.sessions[session.ID]; !ok {
		return fmt.Errorf("session %s not found", session.ID)
	}
	s.sessions[session.ID] = cloneSession(session)
	return nil
}

type mockProvider struct{}

func newMockProvider() *mockProvider {
	return &mockProvider{}
}

func (p *mockProvider) CreateStream(ctx context.Context, request CreateStreamRequest) (CreateStreamResponse, error) {
	streamKey := fmt.Sprintf("live-%s", request.SessionID)
	return CreateStreamResponse{
		Provider:    "mock",
		StreamKey:   streamKey,
		RTMPURL:     fmt.Sprintf("rtmp://localhost/live/%s", request.SessionID),
		PlaybackURL: fmt.Sprintf("https://example.invalid/live/%s", request.SessionID),
	}, nil
}

func (p *mockProvider) StartStream(ctx context.Context, sessionID string) error {
	return nil
}

func (p *mockProvider) StopStream(ctx context.Context, sessionID string) error {
	return nil
}
