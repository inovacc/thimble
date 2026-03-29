// Package web provides an embedded HTTP dashboard for session insights.
package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/inovacc/thimble/internal/model"
	"github.com/inovacc/thimble/internal/plugin"
	"github.com/inovacc/thimble/internal/session"
)

//go:embed dashboard.html marketplace.html
var dashboardFS embed.FS

// registryCache holds a cached copy of the plugin registry.
type registryCache struct {
	mu        sync.Mutex
	index     *plugin.RegistryIndex
	fetchedAt time.Time
}

const registryCacheTTL = 5 * time.Minute

// FetchRegistryFunc is the function used to fetch the plugin registry.
// It can be replaced in tests.
var FetchRegistryFunc = plugin.FetchRegistry

// InstallPluginFunc is the function used to install a plugin.
// It can be replaced in tests.
var InstallPluginFunc = plugin.Install

// LoadInstalledPluginsFunc loads installed plugins from the plugin directory.
// It can be replaced in tests.
var LoadInstalledPluginsFunc = func() ([]plugin.PluginDef, error) {
	return plugin.LoadPlugins(plugin.PluginDir())
}

// Server serves the web dashboard and JSON API.
type Server struct {
	mux    *http.ServeMux
	dbPath string
	port   int
	cache  registryCache
}

// New creates a new web Server.
func New(dbPath string, port int) *Server {
	s := &Server{
		mux:    http.NewServeMux(),
		dbPath: dbPath,
		port:   port,
	}

	s.mux.HandleFunc("/", s.handleDashboard)
	s.mux.HandleFunc("/api/stats", s.handleStats)
	s.mux.HandleFunc("/api/events", s.handleEvents)
	s.mux.HandleFunc("/api/tools", s.handleTools)
	s.mux.HandleFunc("/api/timeline", s.handleTimeline)
	s.mux.HandleFunc("/api/event/", s.handleEventByID)
	s.mux.HandleFunc("/marketplace", s.handleMarketplace)
	s.mux.HandleFunc("/api/marketplace", s.handleAPIMarketplace)
	s.mux.HandleFunc("/api/plugins/installed", s.handleInstalledPlugins)
	s.mux.HandleFunc("/api/plugins/install", s.handleInstallPlugin)
	s.mux.HandleFunc("/api/session/diff", s.handleSessionDiff)

	return s
}

// Handler returns the underlying http.Handler (useful for testing).
func (s *Server) Handler() http.Handler {
	return s.mux
}

// Addr returns the listen address string.
func (s *Server) Addr() string {
	return fmt.Sprintf(":%d", s.port)
}

// Start starts the HTTP server (blocks until error or shutdown).
func (s *Server) Start() error {
	srv := &http.Server{
		Addr:              s.Addr(),
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return srv.ListenAndServe()
}

func (s *Server) openDB() (*session.SessionDB, error) {
	return session.New(s.dbPath)
}

func (s *Server) handleDashboard(w http.ResponseWriter, _ *http.Request) {
	data, err := dashboardFS.ReadFile("dashboard.html")
	if err != nil {
		http.Error(w, "dashboard not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// statsResponse is the JSON shape for /api/stats.
type statsResponse struct {
	SessionID    string         `json:"session_id"`
	DurationSec  float64        `json:"duration_sec"`
	DurationText string         `json:"duration"`
	TotalEvents  int            `json:"total_events"`
	EventsByType map[string]int `json:"events_by_type"`
	ErrorCount   int            `json:"error_count"`
	ErrorRate    float64        `json:"error_rate"`
}

func (s *Server) handleStats(w http.ResponseWriter, _ *http.Request) {
	sdb, err := s.openDB()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer sdb.Close()

	sid, err := latestSession(sdb)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no sessions found"})
		return
	}

	duration, _ := sdb.SessionDuration(sid)
	eventsByType, _ := sdb.EventsByType(sid)
	totalEvents, _ := sdb.GetEventCount(sid)
	errorCount, _ := sdb.ErrorCount(sid)

	var errorRate float64
	if totalEvents > 0 {
		errorRate = float64(errorCount) / float64(totalEvents) * 100
	}

	resp := statsResponse{
		SessionID:    sid,
		DurationSec:  duration.Seconds(),
		DurationText: formatDuration(duration),
		TotalEvents:  totalEvents,
		EventsByType: eventsByType,
		ErrorCount:   errorCount,
		ErrorRate:    errorRate,
	}

	writeJSON(w, http.StatusOK, resp)
}

// eventsResponse is the JSON shape for /api/events.
type eventsResponse struct {
	Events []model.StoredEvent `json:"events"`
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	sdb, err := s.openDB()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer sdb.Close()

	sid, err := latestSession(sdb)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no sessions found"})
		return
	}

	var opts *session.GetEventsOpts

	if typ := r.URL.Query().Get("type"); typ != "" {
		opts = &session.GetEventsOpts{Type: typ, Limit: 200}
	} else {
		opts = &session.GetEventsOpts{Limit: 200}
	}

	events, err := sdb.GetEvents(sid, opts)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if events == nil {
		events = []model.StoredEvent{}
	}

	writeJSON(w, http.StatusOK, eventsResponse{Events: events})
}

// toolsResponse is the JSON shape for /api/tools.
type toolsResponse struct {
	Tools []model.ToolCount `json:"tools"`
}

func (s *Server) handleTools(w http.ResponseWriter, _ *http.Request) {
	sdb, err := s.openDB()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer sdb.Close()

	sid, err := latestSession(sdb)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no sessions found"})
		return
	}

	tools, err := sdb.TopTools(sid, 10)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if tools == nil {
		tools = []model.ToolCount{}
	}

	writeJSON(w, http.StatusOK, toolsResponse{Tools: tools})
}

// timelineEvent is a lightweight event for the timeline view.
type timelineEvent struct {
	ID          int64  `json:"id"`
	Type        string `json:"type"`
	Category    string `json:"category"`
	DataPreview string `json:"data_preview"`
	CreatedAt   string `json:"created_at"`
}

// timelineResponse is the JSON shape for /api/timeline.
type timelineResponse struct {
	Events  []timelineEvent `json:"events"`
	HasMore bool            `json:"has_more"`
}

func truncateData(data string, maxLen int) string {
	if len(data) <= maxLen {
		return data
	}

	return data[:maxLen] + "..."
}

func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	sdb, err := s.openDB()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer sdb.Close()

	sid, err := latestSession(sdb)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no sessions found"})
		return
	}

	limit := 50

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	offset := 0

	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Fetch limit+1 to detect if there are more events.
	events, err := sdb.GetEvents(sid, &session.GetEventsOpts{Limit: limit + offset + 1})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Reverse to newest-first.
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}

	hasMore := len(events) > offset+limit

	// Apply offset.
	if offset > 0 {
		if offset >= len(events) {
			events = nil
		} else {
			events = events[offset:]
		}
	}

	// Apply limit.
	if len(events) > limit {
		events = events[:limit]
	}

	result := make([]timelineEvent, 0, len(events))
	for _, e := range events {
		result = append(result, timelineEvent{
			ID:          e.ID,
			Type:        e.Type,
			Category:    e.Category,
			DataPreview: truncateData(e.Data, 200),
			CreatedAt:   e.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, timelineResponse{Events: result, HasMore: hasMore})
}

// eventDetailResponse is the JSON shape for /api/event/{id}.
type eventDetailResponse struct {
	Event model.StoredEvent `json:"event"`
}

func (s *Server) handleEventByID(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path: /api/event/{id}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/event/")
	if idStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing event id"})
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid event id"})
		return
	}

	sdb, err := s.openDB()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer sdb.Close()

	var e model.StoredEvent

	err = sdb.DB().QueryRow(
		`SELECT id, session_id, type, category, priority, data, source_hook, created_at, data_hash
		 FROM session_events WHERE id = ?`, id,
	).Scan(&e.ID, &e.SessionID, &e.Type, &e.Category, &e.Priority,
		&e.Data, &e.SourceHook, &e.CreatedAt, &e.DataHash)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "event not found"})
		return
	}

	writeJSON(w, http.StatusOK, eventDetailResponse{Event: e})
}

func latestSession(sdb *session.SessionDB) (string, error) {
	ids, err := sdb.ListSessionIDs()
	if err != nil {
		return "", err
	}

	if len(ids) == 0 {
		return "", fmt.Errorf("no sessions")
	}

	return ids[0], nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	if err := enc.Encode(v); err != nil {
		slog.Error("writeJSON: encode failed", "error", err)
	}
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	sec := int(d.Seconds()) % 60

	switch {
	case h > 0:
		return fmt.Sprintf("%dh%dm%ds", h, m, sec)
	case m > 0:
		return fmt.Sprintf("%dm%ds", m, sec)
	default:
		return fmt.Sprintf("%ds", sec)
	}
}

// --- Marketplace handlers ---

func (s *Server) handleMarketplace(w http.ResponseWriter, _ *http.Request) {
	data, err := dashboardFS.ReadFile("marketplace.html")
	if err != nil {
		http.Error(w, "marketplace page not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// marketplacePlugin is a flattened plugin entry for the marketplace API.
type marketplacePlugin struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Author      string   `json:"author,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
}

// marketplaceResponse is the JSON shape for /api/marketplace.
type marketplaceResponse struct {
	Plugins []marketplacePlugin `json:"plugins"`
}

func (s *Server) fetchRegistryCached() (*plugin.RegistryIndex, error) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()

	if s.cache.index != nil && time.Since(s.cache.fetchedAt) < registryCacheTTL {
		return s.cache.index, nil
	}

	idx, err := FetchRegistryFunc()
	if err != nil {
		return nil, err
	}

	s.cache.index = idx
	s.cache.fetchedAt = time.Now()

	return idx, nil
}

func (s *Server) handleAPIMarketplace(w http.ResponseWriter, _ *http.Request) {
	idx, err := s.fetchRegistryCached()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	plugins := make([]marketplacePlugin, 0, len(idx.Plugins))
	for _, entry := range idx.Plugins {
		plugins = append(plugins, marketplacePlugin{
			Name:        entry.Name,
			Description: entry.Description,
			Version:     entry.Version,
			Author:      entry.Author,
		})
	}

	writeJSON(w, http.StatusOK, marketplaceResponse{Plugins: plugins})
}

// installedResponse is the JSON shape for /api/plugins/installed.
type installedResponse struct {
	Plugins []installedPlugin `json:"plugins"`
}

type installedPlugin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (s *Server) handleInstalledPlugins(w http.ResponseWriter, _ *http.Request) {
	installed, err := LoadInstalledPluginsFunc()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	result := make([]installedPlugin, 0, len(installed))
	for _, p := range installed {
		result = append(result, installedPlugin{Name: p.Name, Version: p.Version})
	}

	writeJSON(w, http.StatusOK, installedResponse{Plugins: result})
}

// installRequest is the JSON body for POST /api/plugins/install.
type installRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleInstallPlugin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req installRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	p, err := InstallPluginFunc(req.Name)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"name":    p.Name,
		"version": p.Version,
		"status":  "installed",
	})
}

// --- Session diff handler ---

// diffChange represents a single field change between two events.
type diffChange struct {
	Field string `json:"field"`
	From  any    `json:"from"`
	To    any    `json:"to"`
}

// sessionDiffResponse is the JSON shape for /api/session/diff.
type sessionDiffResponse struct {
	FromID  int64        `json:"from_id"`
	ToID    int64        `json:"to_id"`
	Added   []string     `json:"added"`
	Removed []string     `json:"removed"`
	Changed []diffChange `json:"changed"`
}

func (s *Server) handleSessionDiff(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr == "" || toStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "both 'from' and 'to' query params are required"})
		return
	}

	fromID, err := strconv.ParseInt(fromStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid 'from' event id"})
		return
	}

	toID, err := strconv.ParseInt(toStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid 'to' event id"})
		return
	}

	sdb, err := s.openDB()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer sdb.Close()

	fromEvent, err := queryEventByID(sdb, fromID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("from event %d not found", fromID)})
		return
	}

	toEvent, err := queryEventByID(sdb, toID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("to event %d not found", toID)})
		return
	}

	diff, err := computeJSONDiff(fromEvent.Data, toEvent.Data)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("diff failed: %v", err)})
		return
	}

	diff.FromID = fromID
	diff.ToID = toID

	writeJSON(w, http.StatusOK, diff)
}

// queryEventByID fetches a single StoredEvent by its ID.
func queryEventByID(sdb *session.SessionDB, id int64) (*model.StoredEvent, error) {
	var e model.StoredEvent

	err := sdb.DB().QueryRow(
		`SELECT id, session_id, type, category, priority, data, source_hook, created_at, data_hash
		 FROM session_events WHERE id = ?`, id,
	).Scan(&e.ID, &e.SessionID, &e.Type, &e.Category, &e.Priority,
		&e.Data, &e.SourceHook, &e.CreatedAt, &e.DataHash)
	if err != nil {
		return nil, err
	}

	return &e, nil
}

// computeJSONDiff compares two JSON data strings and returns a structured diff.
func computeJSONDiff(fromData, toData string) (*sessionDiffResponse, error) {
	var fromMap, toMap map[string]any

	if err := json.Unmarshal([]byte(fromData), &fromMap); err != nil {
		return nil, fmt.Errorf("parse from data: %w", err)
	}

	if err := json.Unmarshal([]byte(toData), &toMap); err != nil {
		return nil, fmt.Errorf("parse to data: %w", err)
	}

	resp := &sessionDiffResponse{
		Added:   []string{},
		Removed: []string{},
		Changed: []diffChange{},
	}

	// Find removed and changed keys.
	for k, fromVal := range fromMap {
		toVal, exists := toMap[k]
		if !exists {
			resp.Removed = append(resp.Removed, k)
			continue
		}

		if !jsonEqual(fromVal, toVal) {
			resp.Changed = append(resp.Changed, diffChange{
				Field: k,
				From:  fromVal,
				To:    toVal,
			})
		}
	}

	// Find added keys.
	for k := range toMap {
		if _, exists := fromMap[k]; !exists {
			resp.Added = append(resp.Added, k)
		}
	}

	return resp, nil
}

// jsonEqual compares two values by their JSON representation.
func jsonEqual(a, b any) bool {
	aj, err1 := json.Marshal(a)
	bj, err2 := json.Marshal(b)

	if err1 != nil || err2 != nil {
		return false
	}

	return string(aj) == string(bj)
}
