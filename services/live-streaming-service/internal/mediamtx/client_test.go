package mediamtx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// MediaMTX v1.21 /v3/paths/list item shape: publisher is `source`.
const v121PathsList = `{
  "itemCount": 2,
  "pageCount": 1,
  "items": [
    {
      "name": "ls_active",
      "confName": "all_others",
      "ready": true,
      "online": true,
      "source": {"type": "rtmpConn", "id": "src-active"},
      "tracks": ["H264", "MPEG-4 Audio"]
    },
    {
      "name": "ls_idle",
      "confName": "all_others",
      "ready": false,
      "online": false,
      "tracks": []
    }
  ]
}`

func TestListActivePathsUsesSourceField(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(v121PathsList))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 2*time.Second)
	paths, err := client.ListActivePaths(context.Background())
	if err != nil {
		t.Fatalf("list paths: %v", err)
	}

	if gotMethod != http.MethodGet || gotPath != "/v3/paths/list" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 active path, got %d (%v)", len(paths), paths)
	}
	if _, ok := paths["ls_active"]; !ok {
		t.Fatalf("expected ls_active, got %v", paths)
	}
}

func TestKickUsesRtmpConnsEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 2*time.Second)
	err := client.KickRTMPConnections(context.Background(), PathInfo{
		Name:   "ls_active",
		Source: &PathSource{ID: "src-active", Type: "rtmpConn"},
	})
	if err != nil {
		t.Fatalf("kick: %v", err)
	}
	if gotPath != "/v3/rtmp/conns/kick/src-active" {
		t.Fatalf("unexpected kick path: %s", gotPath)
	}

	// No source -> no request, no error.
	if err := client.KickRTMPConnections(context.Background(), PathInfo{Name: "ls_idle"}); err != nil {
		t.Fatalf("kick without source: %v", err)
	}
}
