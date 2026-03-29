package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inovacc/thimble/internal/model"
	"github.com/inovacc/thimble/internal/plugin"
	"github.com/inovacc/thimble/internal/session"
)

// setupTestDB creates a temporary session DB with seed data.
func setupTestDB(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "session.db")

	sdb, err := session.New(dbPath)
	if err != nil {
		t.Fatalf("create session db: %v", err)
	}

	defer sdb.Close()

	sid := "test-session-001"

	if err := sdb.EnsureSession(sid, "/tmp/project"); err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	events := []model.SessionEvent{
		{Type: "tool_call", Category: "tool", Priority: 2, Data: `{"tool":"ctx_search"}`},
		{Type: "tool_call", Category: "tool", Priority: 2, Data: `{"tool":"ctx_search"}`},
		{Type: "tool_call", Category: "tool", Priority: 2, Data: `{"tool":"ctx_execute"}`},
		{Type: "error", Category: "system", Priority: 3, Data: `{"error":"timeout"}`},
		{Type: "guidance", Category: "advisory", Priority: 1, Data: `{"msg":"use slog"}`},
	}

	for i, e := range events {
		// Modify data slightly to avoid dedup.
		e.Data = e.Data[:len(e.Data)-1] + `,"i":` + string(rune('0'+i)) + `}`

		if err := sdb.InsertEvent(sid, e, "PostToolUse"); err != nil {
			t.Fatalf("insert event %d: %v", i, err)
		}
	}

	return dbPath
}

func TestHandleDashboard(t *testing.T) {
	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Fatalf("expected text/html, got %q", ct)
	}

	body := rec.Body.String()
	if len(body) < 100 {
		t.Fatal("dashboard body too short")
	}
}

func TestHandleStats(t *testing.T) {
	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp statsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.SessionID == "" {
		t.Fatal("expected session_id")
	}

	if resp.TotalEvents != 5 {
		t.Fatalf("expected 5 events, got %d", resp.TotalEvents)
	}

	if resp.ErrorCount != 1 {
		t.Fatalf("expected 1 error, got %d", resp.ErrorCount)
	}

	if resp.EventsByType == nil {
		t.Fatal("expected events_by_type")
	}
}

func TestHandleEvents(t *testing.T) {
	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	t.Run("all events", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp eventsResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if len(resp.Events) != 5 {
			t.Fatalf("expected 5 events, got %d", len(resp.Events))
		}
	})

	t.Run("filtered by type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/events?type=error", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp eventsResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if len(resp.Events) != 1 {
			t.Fatalf("expected 1 error event, got %d", len(resp.Events))
		}
	})
}

func TestHandleTools(t *testing.T) {
	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp toolsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Tools) == 0 {
		t.Fatal("expected at least one tool")
	}
}

func TestStatsNoSession(t *testing.T) {
	// Create an empty DB with no sessions.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "session.db")

	sdb, err := session.New(dbPath)
	if err != nil {
		t.Fatalf("create session db: %v", err)
	}

	sdb.Close()

	srv := New(dbPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestStatsBadDB(t *testing.T) {
	// Point to a path that doesn't exist as a valid SQLite DB.
	badPath := filepath.Join(os.TempDir(), "thimble_test_nonexistent_"+t.Name(), "bad.db")

	srv := New(badPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestHandleMarketplacePage(t *testing.T) {
	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/marketplace", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Fatalf("expected text/html, got %q", ct)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Plugin Marketplace") {
		t.Fatal("marketplace page should contain 'Plugin Marketplace'")
	}
}

func TestHandleAPIMarketplace(t *testing.T) {
	// Save and restore the original fetch function.
	origFetch := FetchRegistryFunc

	defer func() { FetchRegistryFunc = origFetch }()

	FetchRegistryFunc = func() (*plugin.RegistryIndex, error) {
		return &plugin.RegistryIndex{
			Plugins: []plugin.RegistryEntry{
				{Name: "docker", Description: "Docker tools", Version: "1.0.0", Author: "test-author"},
				{Name: "k8s", Description: "Kubernetes tools", Version: "2.1.0", Author: "k8s-team"},
			},
		}, nil
	}

	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/marketplace", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp marketplaceResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(resp.Plugins))
	}

	if resp.Plugins[0].Name != "docker" {
		t.Fatalf("expected first plugin 'docker', got %q", resp.Plugins[0].Name)
	}

	if resp.Plugins[1].Author != "k8s-team" {
		t.Fatalf("expected author 'k8s-team', got %q", resp.Plugins[1].Author)
	}
}

func TestHandleAPIMarketplaceCached(t *testing.T) {
	origFetch := FetchRegistryFunc

	defer func() { FetchRegistryFunc = origFetch }()

	callCount := 0
	FetchRegistryFunc = func() (*plugin.RegistryIndex, error) {
		callCount++

		return &plugin.RegistryIndex{
			Plugins: []plugin.RegistryEntry{
				{Name: "test-plugin", Description: "Test", Version: "1.0.0"},
			},
		}, nil
	}

	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	// First call should fetch.
	req := httptest.NewRequest(http.MethodGet, "/api/marketplace", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Second call should use cache.
	req = httptest.NewRequest(http.MethodGet, "/api/marketplace", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if callCount != 1 {
		t.Fatalf("expected 1 fetch call (cached), got %d", callCount)
	}
}

func TestHandleAPIMarketplaceFetchError(t *testing.T) {
	origFetch := FetchRegistryFunc

	defer func() { FetchRegistryFunc = origFetch }()

	FetchRegistryFunc = func() (*plugin.RegistryIndex, error) {
		return nil, fmt.Errorf("network error")
	}

	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/marketplace", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandleInstalledPlugins(t *testing.T) {
	origLoad := LoadInstalledPluginsFunc

	defer func() { LoadInstalledPluginsFunc = origLoad }()

	LoadInstalledPluginsFunc = func() ([]plugin.PluginDef, error) {
		return []plugin.PluginDef{
			{Name: "docker", Version: "1.0.0"},
			{Name: "k8s", Version: "2.1.0"},
		}, nil
	}

	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/installed", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp installedResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Plugins) != 2 {
		t.Fatalf("expected 2 installed plugins, got %d", len(resp.Plugins))
	}
}

func TestHandleInstalledPluginsEmpty(t *testing.T) {
	origLoad := LoadInstalledPluginsFunc

	defer func() { LoadInstalledPluginsFunc = origLoad }()

	LoadInstalledPluginsFunc = func() ([]plugin.PluginDef, error) {
		return nil, nil
	}

	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/installed", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp installedResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Plugins) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(resp.Plugins))
	}
}

func TestHandleInstallPlugin(t *testing.T) {
	origInstall := InstallPluginFunc

	defer func() { InstallPluginFunc = origInstall }()

	InstallPluginFunc = func(source string) (*plugin.PluginDef, error) {
		return &plugin.PluginDef{Name: source, Version: "1.0.0"}, nil
	}

	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	body := strings.NewReader(`{"name": "docker"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["name"] != "docker" {
		t.Fatalf("expected name 'docker', got %q", resp["name"])
	}

	if resp["status"] != "installed" {
		t.Fatalf("expected status 'installed', got %q", resp["status"])
	}
}

func TestHandleInstallPluginMethodNotAllowed(t *testing.T) {
	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/install", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleInstallPluginEmptyName(t *testing.T) {
	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	body := strings.NewReader(`{"name": ""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleInstallPluginBadJSON(t *testing.T) {
	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleInstallPluginError(t *testing.T) {
	origInstall := InstallPluginFunc

	defer func() { InstallPluginFunc = origInstall }()

	InstallPluginFunc = func(source string) (*plugin.PluginDef, error) {
		return nil, fmt.Errorf("download failed")
	}

	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	body := strings.NewReader(`{"name": "bad-plugin"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandleTimeline(t *testing.T) {
	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	t.Run("default returns newest first", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/timeline", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp timelineResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if len(resp.Events) != 5 {
			t.Fatalf("expected 5 events, got %d", len(resp.Events))
		}

		// Newest first: IDs should be descending.
		for i := 1; i < len(resp.Events); i++ {
			if resp.Events[i].ID > resp.Events[i-1].ID {
				t.Fatalf("events not in newest-first order: id %d after %d", resp.Events[i].ID, resp.Events[i-1].ID)
			}
		}

		if resp.HasMore {
			t.Fatal("should not have more events with only 5 in DB")
		}
	})

	t.Run("with limit and offset", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/timeline?limit=2&offset=0", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp timelineResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if len(resp.Events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(resp.Events))
		}

		if !resp.HasMore {
			t.Fatal("expected has_more=true with limit=2 and 5 events")
		}
	})

	t.Run("data preview is truncated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/timeline", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		var resp timelineResponse

		_ = json.NewDecoder(rec.Body).Decode(&resp)

		for _, ev := range resp.Events {
			if len(ev.DataPreview) > 203 { // 200 + "..."
				t.Fatalf("data_preview too long: %d chars", len(ev.DataPreview))
			}
		}
	})

	t.Run("has type and category", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/timeline", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		var resp timelineResponse

		_ = json.NewDecoder(rec.Body).Decode(&resp)

		for _, ev := range resp.Events {
			if ev.Type == "" {
				t.Fatal("expected type to be set")
			}

			if ev.Category == "" {
				t.Fatal("expected category to be set")
			}

			if ev.CreatedAt == "" {
				t.Fatal("expected created_at to be set")
			}
		}
	})
}

func TestHandleTimelineNoSession(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "session.db")

	sdb, err := session.New(dbPath)
	if err != nil {
		t.Fatalf("create session db: %v", err)
	}

	sdb.Close()

	srv := New(dbPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/timeline", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleEventByID(t *testing.T) {
	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	t.Run("valid event", func(t *testing.T) {
		// First get an event ID from the timeline.
		req := httptest.NewRequest(http.MethodGet, "/api/timeline?limit=1", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		var tlResp timelineResponse
		if err := json.NewDecoder(rec.Body).Decode(&tlResp); err != nil {
			t.Fatalf("decode timeline: %v", err)
		}

		if len(tlResp.Events) == 0 {
			t.Fatal("expected at least one timeline event")
		}

		eventID := tlResp.Events[0].ID

		// Fetch full event.
		req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/event/%d", eventID), nil)
		rec = httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp eventDetailResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if resp.Event.ID != eventID {
			t.Fatalf("expected event ID %d, got %d", eventID, resp.Event.ID)
		}

		if resp.Event.Data == "" {
			t.Fatal("expected full event data")
		}

		if resp.Event.Type == "" {
			t.Fatal("expected event type")
		}
	})

	t.Run("missing id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/event/", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/event/abc", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/event/99999", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})
}

func TestHandleSessionDiff(t *testing.T) {
	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	// Get two event IDs from the timeline.
	req := httptest.NewRequest(http.MethodGet, "/api/timeline?limit=5", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	var tlResp timelineResponse
	if err := json.NewDecoder(rec.Body).Decode(&tlResp); err != nil {
		t.Fatalf("decode timeline: %v", err)
	}

	if len(tlResp.Events) < 2 {
		t.Fatal("need at least 2 events for diff test")
	}

	fromID := tlResp.Events[len(tlResp.Events)-1].ID // oldest
	toID := tlResp.Events[0].ID                       // newest

	t.Run("valid diff", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/session/diff?from=%d&to=%d", fromID, toID), nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp sessionDiffResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if resp.FromID != fromID {
			t.Fatalf("expected from_id %d, got %d", fromID, resp.FromID)
		}

		if resp.ToID != toID {
			t.Fatalf("expected to_id %d, got %d", toID, resp.ToID)
		}

		if resp.Added == nil || resp.Removed == nil || resp.Changed == nil {
			t.Fatal("expected non-nil diff slices")
		}
	})

	t.Run("missing params", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/session/diff", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("missing to param", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/session/diff?from=%d", fromID), nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("invalid from id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/session/diff?from=abc&to=1", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("invalid to id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/session/diff?from=1&to=abc", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("from event not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/session/diff?from=99999&to=%d", toID), nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("to event not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/session/diff?from=%d&to=99999", fromID), nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})
}

func TestComputeJSONDiff(t *testing.T) {
	t.Run("added removed changed", func(t *testing.T) {
		from := `{"tool":"ctx_search","count":1,"mode":"fast"}`
		to := `{"tool":"ctx_execute","count":1,"lang":"go"}`

		diff, err := computeJSONDiff(from, to)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// "tool" changed, "mode" removed, "lang" added, "count" unchanged.
		if len(diff.Changed) != 1 || diff.Changed[0].Field != "tool" {
			t.Fatalf("expected 1 changed field 'tool', got %+v", diff.Changed)
		}

		if len(diff.Removed) != 1 || diff.Removed[0] != "mode" {
			t.Fatalf("expected 1 removed field 'mode', got %v", diff.Removed)
		}

		if len(diff.Added) != 1 || diff.Added[0] != "lang" {
			t.Fatalf("expected 1 added field 'lang', got %v", diff.Added)
		}
	})

	t.Run("identical data", func(t *testing.T) {
		data := `{"tool":"ctx_search","count":1}`

		diff, err := computeJSONDiff(data, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(diff.Added) != 0 || len(diff.Removed) != 0 || len(diff.Changed) != 0 {
			t.Fatalf("expected no changes for identical data, got added=%v removed=%v changed=%v",
				diff.Added, diff.Removed, diff.Changed)
		}
	})

	t.Run("invalid from json", func(t *testing.T) {
		_, err := computeJSONDiff("not json", `{"a":1}`)
		if err == nil {
			t.Fatal("expected error for invalid from JSON")
		}
	})

	t.Run("invalid to json", func(t *testing.T) {
		_, err := computeJSONDiff(`{"a":1}`, "not json")
		if err == nil {
			t.Fatal("expected error for invalid to JSON")
		}
	})
}

func TestTruncateData(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "short", 200, "short"},
		{"exact length", strings.Repeat("a", 200), 200, strings.Repeat("a", 200)},
		{"over length", strings.Repeat("a", 201), 200, strings.Repeat("a", 200) + "..."},
		{"empty string", "", 200, ""},
		{"empty string zero max", "", 0, ""},
		{"maxLen zero with content", "hello", 0, "..."},
		{"maxLen one", "hello", 1, "h..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateData(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateData(%d chars, %d) = %q, want %q",
					len(tt.input), tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestHandleStatsZeroEvents(t *testing.T) {
	// Create a DB with a session but no events — tests the division-by-zero guard.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "session.db")

	sdb, err := session.New(dbPath)
	if err != nil {
		t.Fatalf("create session db: %v", err)
	}

	sid := "empty-session-001"
	if err := sdb.EnsureSession(sid, "/tmp/empty"); err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	sdb.Close()

	srv := New(dbPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp statsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.TotalEvents != 0 {
		t.Fatalf("expected 0 events, got %d", resp.TotalEvents)
	}

	if resp.ErrorRate != 0 {
		t.Fatalf("expected 0 error_rate, got %f", resp.ErrorRate)
	}

	if resp.ErrorCount != 0 {
		t.Fatalf("expected 0 error_count, got %d", resp.ErrorCount)
	}
}

func TestHandleTimelineEdgeCaseParams(t *testing.T) {
	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	t.Run("invalid limit falls back to default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/timeline?limit=abc", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp timelineResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		// Default limit=50, we have 5 events, so all should be returned.
		if len(resp.Events) != 5 {
			t.Fatalf("expected 5 events with default limit, got %d", len(resp.Events))
		}
	})

	t.Run("negative limit falls back to default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/timeline?limit=-5", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp timelineResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if len(resp.Events) != 5 {
			t.Fatalf("expected 5 events with default limit, got %d", len(resp.Events))
		}
	})

	t.Run("limit exceeding max clamps to default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/timeline?limit=999", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp timelineResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		// limit=999 > 200 so ignored, falls back to default 50.
		if len(resp.Events) != 5 {
			t.Fatalf("expected 5 events, got %d", len(resp.Events))
		}
	})

	t.Run("invalid offset falls back to zero", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/timeline?offset=xyz", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp timelineResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if len(resp.Events) != 5 {
			t.Fatalf("expected 5 events with offset=0, got %d", len(resp.Events))
		}
	})

	t.Run("negative offset falls back to zero", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/timeline?offset=-3", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp timelineResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if len(resp.Events) != 5 {
			t.Fatalf("expected 5 events with offset=0, got %d", len(resp.Events))
		}
	})

	t.Run("offset beyond total returns empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/timeline?offset=100", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp timelineResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if len(resp.Events) != 0 {
			t.Fatalf("expected 0 events with large offset, got %d", len(resp.Events))
		}

		if resp.HasMore {
			t.Fatal("expected has_more=false with offset beyond total")
		}
	})
}

func TestHandleInstalledPluginsError(t *testing.T) {
	origLoad := LoadInstalledPluginsFunc

	defer func() { LoadInstalledPluginsFunc = origLoad }()

	LoadInstalledPluginsFunc = func() ([]plugin.PluginDef, error) {
		return nil, fmt.Errorf("disk error")
	}

	dbPath := setupTestDB(t)
	srv := New(dbPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/installed", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "0s"},
		{"seconds only", 45 * time.Second, "45s"},
		{"minutes and seconds", 3*time.Minute + 15*time.Second, "3m15s"},
		{"hours minutes seconds", 2*time.Hour + 5*time.Minute + 30*time.Second, "2h5m30s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.d)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestAddr(t *testing.T) {
	srv := New("", 8080)
	if got := srv.Addr(); got != ":8080" {
		t.Fatalf("Addr() = %q, want %q", got, ":8080")
	}
}

func TestHandleToolsBadDB(t *testing.T) {
	badPath := filepath.Join(os.TempDir(), "thimble_test_nonexistent_"+t.Name(), "bad.db")
	srv := New(badPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestHandleToolsNoSession(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "session.db")

	sdb, err := session.New(dbPath)
	if err != nil {
		t.Fatalf("create session db: %v", err)
	}

	sdb.Close()

	srv := New(dbPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleEventsBadDB(t *testing.T) {
	badPath := filepath.Join(os.TempDir(), "thimble_test_nonexistent_"+t.Name(), "bad.db")
	srv := New(badPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestHandleEventsNoSession(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "session.db")

	sdb, err := session.New(dbPath)
	if err != nil {
		t.Fatalf("create session db: %v", err)
	}

	sdb.Close()

	srv := New(dbPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleSessionDiffBadDB(t *testing.T) {
	badPath := filepath.Join(os.TempDir(), "thimble_test_nonexistent_"+t.Name(), "bad.db")
	srv := New(badPath, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/session/diff?from=1&to=2", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestJSONEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b any
		want bool
	}{
		{"equal strings", "hello", "hello", true},
		{"different strings", "hello", "world", false},
		{"equal numbers", 42.0, 42.0, true},
		{"different numbers", 1.0, 2.0, false},
		{"nil values", nil, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jsonEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("jsonEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
