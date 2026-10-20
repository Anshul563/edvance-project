package service

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/mediamtx"
	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/model"
)

// memoryStore is an in-memory StreamStore used by unit tests. Production
// uses the PostgreSQL repository; this exists only so lifecycle and
// authorization logic can be tested without a database.
type memoryStore struct {
	mu       sync.Mutex
	sessions map[uuid.UUID]*model.Stream
	byPath   map[string]uuid.UUID
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		sessions: make(map[uuid.UUID]*model.Stream),
		byPath:   make(map[string]uuid.UUID),
	}
}

func (s *memoryStore) Create(_ context.Context, stream *model.Stream) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[stream.ID] = stream.Clone()
	s.byPath[stream.PathName] = stream.ID

	return nil
}

func (s *memoryStore) GetByID(_ context.Context, id uuid.UUID) (*model.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stream, ok := s.sessions[id]
	if !ok {
		return nil, model.ErrStreamNotFound
	}

	return stream.Clone(), nil
}

func (s *memoryStore) GetByPathName(_ context.Context, pathName string) (*model.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.byPath[pathName]
	if !ok {
		return nil, model.ErrStreamNotFound
	}

	return s.sessions[id].Clone(), nil
}

func (s *memoryStore) ListByCreator(_ context.Context, creatorID uuid.UUID) ([]*model.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]*model.Stream, 0)
	for _, stream := range s.sessions {
		if stream.CreatorID == creatorID {
			out = append(out, stream.Clone())
		}
	}

	return out, nil
}

func (s *memoryStore) ListActive(_ context.Context) ([]*model.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]*model.Stream, 0)
	for _, stream := range s.sessions {
		if stream.Status == model.StatusLive {
			out = append(out, stream.Clone())
		}
	}

	return out, nil
}

func (s *memoryStore) ListLiveOrWaiting(_ context.Context, _ time.Time) ([]*model.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]*model.Stream, 0)
	for _, stream := range s.sessions {
		if stream.Status == model.StatusLive || stream.Status == model.StatusWaiting {
			out = append(out, stream.Clone())
		}
	}

	return out, nil
}

func (s *memoryStore) Update(_ context.Context, stream *model.Stream) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[stream.ID]; !ok {
		return model.ErrStreamNotFound
	}

	s.sessions[stream.ID] = stream.Clone()

	return nil
}

// fakeMedia is a MediaEngine test double.
type fakeMedia struct {
	mu    sync.Mutex
	paths map[string]mediamtxPath
	kicks []string
}

type mediamtxPath struct {
	publisherID string
}

func newFakeMedia() *fakeMedia {
	return &fakeMedia{paths: make(map[string]mediamtxPath)}
}

func (f *fakeMedia) setPublisher(pathName, connID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.paths[pathName] = mediamtxPath{publisherID: connID}
}

func (f *fakeMedia) removePublisher(pathName string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.paths, pathName)
}

func (f *fakeMedia) ListActivePaths(_ context.Context) (map[string]mediamtx.PathInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make(map[string]mediamtx.PathInfo, len(f.paths))
	for name, path := range f.paths {
		out[name] = mediamtx.PathInfo{
			Name:   name,
			Ready:  true,
			Online: true,
			Source: &mediamtx.PathSource{ID: path.publisherID, Type: "rtmpConn"},
		}
	}

	return out, nil
}

func (f *fakeMedia) KickRTMPConnections(_ context.Context, path mediamtx.PathInfo) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if path.Source != nil {
		f.kicks = append(f.kicks, path.Source.ID)
	}
	delete(f.paths, path.Name)

	return nil
}

func (f *fakeMedia) kickCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.kicks)
}
