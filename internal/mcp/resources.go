package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/thimble/internal/analysis"
	"github.com/inovacc/thimble/internal/model"
	"github.com/inovacc/thimble/internal/plugin"
	"github.com/inovacc/thimble/internal/session"
)

// registerResources adds MCP resource providers to the server.
func (b *Bridge) registerResources() {
	b.server.AddResource(&mcpsdk.Resource{
		URI:         "thimble://session/events",
		Name:        "Session Events",
		Description: "Recent session events captured by hooks during this Claude Code session.",
		MIMEType:    "application/json",
	}, b.handleSessionEventsResource)

	b.server.AddResource(&mcpsdk.Resource{
		URI:         "thimble://session/snapshot",
		Name:        "Resume Snapshot",
		Description: "Current resume snapshot for the active session, used to restore context after interruptions.",
		MIMEType:    "application/json",
	}, b.handleSessionSnapshotResource)

	b.server.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "thimble://analysis/{path}",
		Name:        "File Analysis",
		Description: "Code analysis results for a file path — symbols, references, imports, and language detection.",
		MIMEType:    "application/json",
	}, b.handleAnalysisResource)

	b.server.AddResource(&mcpsdk.Resource{
		URI:         "thimble://plugins",
		Name:        "Installed Plugins",
		Description: "List of installed thimble plugins with metadata (name, version, scope, tools).",
		MIMEType:    "application/json",
	}, b.handlePluginsResource)
}

// handleSessionEventsResource returns recent session events as JSON.
func (b *Bridge) handleSessionEventsResource(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	if b.session == nil {
		return &mcpsdk.ReadResourceResult{
			Contents: []*mcpsdk.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     "[]",
			}},
		}, nil
	}

	sessionID := b.sessionID
	if sessionID == "" {
		sessionID = "default"
	}

	events, err := b.session.GetEvents(sessionID, &session.GetEventsOpts{Limit: 100})
	if err != nil {
		return nil, fmt.Errorf("get session events: %w", err)
	}

	if events == nil {
		events = []model.StoredEvent{}
	}

	data, err := json.Marshal(events)
	if err != nil {
		return nil, fmt.Errorf("marshal events: %w", err)
	}

	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		}},
	}, nil
}

// handleSessionSnapshotResource returns the current resume snapshot.
func (b *Bridge) handleSessionSnapshotResource(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	if b.session == nil {
		return &mcpsdk.ReadResourceResult{
			Contents: []*mcpsdk.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     "{}",
			}},
		}, nil
	}

	sessionID := b.sessionID
	if sessionID == "" {
		sessionID = "default"
	}

	resume, err := b.session.GetResume(sessionID)
	if err != nil {
		return nil, fmt.Errorf("get resume snapshot: %w", err)
	}

	if resume == nil {
		return &mcpsdk.ReadResourceResult{
			Contents: []*mcpsdk.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     "{}",
			}},
		}, nil
	}

	data, err := json.Marshal(resume)
	if err != nil {
		return nil, fmt.Errorf("marshal resume: %w", err)
	}

	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		}},
	}, nil
}

// handleAnalysisResource returns analysis results for a file path.
func (b *Bridge) handleAnalysisResource(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	// Extract path from URI: thimble://analysis/{path}
	uri := req.Params.URI

	const prefix = "thimble://analysis/"

	if !strings.HasPrefix(uri, prefix) {
		return nil, mcpsdk.ResourceNotFoundError(uri)
	}

	filePath := strings.TrimPrefix(uri, prefix)
	if filePath == "" {
		return nil, mcpsdk.ResourceNotFoundError(uri)
	}

	// Resolve relative paths against the project directory.
	if !filepath.IsAbs(filePath) && b.projectDir != "" {
		filePath = filepath.Join(b.projectDir, filePath)
	}

	analyzer := analysis.NewAnalyzer(b.projectDir)

	fileResult, err := analyzer.AnalyzeFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("analyze file %s: %w", filePath, err)
	}

	data, err := json.Marshal(fileResult)
	if err != nil {
		return nil, fmt.Errorf("marshal analysis: %w", err)
	}

	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		}},
	}, nil
}

// handlePluginsResource returns a list of installed plugins with metadata.
func (b *Bridge) handlePluginsResource(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	plugins, err := plugin.LoadAllScopes(b.projectDir)
	if err != nil {
		return nil, fmt.Errorf("load plugins: %w", err)
	}

	type pluginInfo struct {
		Name        string   `json:"name"`
		Version     string   `json:"version"`
		Description string   `json:"description,omitempty"`
		Scope       string   `json:"scope"`
		License     string   `json:"license,omitempty"`
		Keywords    []string `json:"keywords,omitempty"`
		ToolCount   int      `json:"tool_count"`
		ToolNames   []string `json:"tool_names"`
	}

	infos := make([]pluginInfo, 0, len(plugins))

	for _, sp := range plugins {
		toolNames := make([]string, 0, len(sp.Tools))
		for _, t := range sp.Tools {
			toolNames = append(toolNames, t.Name)
		}

		infos = append(infos, pluginInfo{
			Name:        sp.Name,
			Version:     sp.Version,
			Description: sp.Description,
			Scope:       string(sp.Scope),
			License:     sp.License,
			Keywords:    sp.Keywords,
			ToolCount:   len(sp.Tools),
			ToolNames:   toolNames,
		})
	}

	data, err := json.Marshal(infos)
	if err != nil {
		return nil, fmt.Errorf("marshal plugins: %w", err)
	}

	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		}},
	}, nil
}
