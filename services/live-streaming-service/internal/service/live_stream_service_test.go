package service

import (
	"context"
	"testing"
	"time"
)

func TestCreateSession(t *testing.T) {
	service := NewLiveStreamService(newMemoryStore(), newMockProvider())

	session, err := service.CreateSession(context.Background(), CreateSessionInput{
		HostID: "user-42",
		Title:  "Product launch stream",
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	if session.ID == "" {
		t.Fatal("expected generated session id")
	}
	if session.Provider != "mock" {
		t.Fatalf("expected mock provider, got %q", session.Provider)
	}
	if session.Status != SessionStatusReady {
		t.Fatalf("expected status %q, got %q", SessionStatusReady, session.Status)
	}
}

func TestCreateSessionRequiresTitle(t *testing.T) {
	service := NewLiveStreamService(newMemoryStore(), newMockProvider())

	_, err := service.CreateSession(context.Background(), CreateSessionInput{
		HostID: "user-42",
	})
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestListSessions(t *testing.T) {
	service := NewLiveStreamService(newMemoryStore(), newMockProvider())

	_, err := service.CreateSession(context.Background(), CreateSessionInput{HostID: "user-1", Title: "First"})
	if err != nil {
		t.Fatalf("first session: %v", err)
	}
	_, err = service.CreateSession(context.Background(), CreateSessionInput{HostID: "user-2", Title: "Second"})
	if err != nil {
		t.Fatalf("second session: %v", err)
	}

	sessions, err := service.ListSessions(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session for user-1, got %d", len(sessions))
	}
	if sessions[0].Title != "First" {
		t.Fatalf("expected title First, got %q", sessions[0].Title)
	}
}

func TestStartAndStopSession(t *testing.T) {
	service := NewLiveStreamService(newMemoryStore(), newMockProvider())

	session, err := service.CreateSession(context.Background(), CreateSessionInput{HostID: "user-1", Title: "T"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := service.StartSession(context.Background(), session.ID); err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if err := service.StopSession(context.Background(), session.ID); err != nil {
		t.Fatalf("StopSession: %v", err)
	}

	updated, err := service.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if updated.Status != SessionStatusEnded {
		t.Fatalf("expected ended status, got %q", updated.Status)
	}
	if updated.UpdatedAt.Before(time.Now().Add(-time.Minute)) {
		t.Fatal("expected updated timestamp to be recent")
	}
}
