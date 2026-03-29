package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/thimble/internal/model"
)

// ── Shared Store Input Types ──

type sharedIndexInput struct {
	Content     string `json:"content" jsonschema:"the content to index into the shared knowledge base"`
	Label       string `json:"label" jsonschema:"source label for later reference"`
	ContentType string `json:"content_type,omitempty" jsonschema:"content type: markdown or plain or json (default markdown)"`
}

type sharedSearchInput struct {
	Query   string   `json:"query,omitempty" jsonschema:"single search query (convenience alias for queries)"`
	Queries []string `json:"queries,omitempty" jsonschema:"search queries to run against the shared knowledge base"`
	Limit   int      `json:"limit,omitempty" jsonschema:"max results per query (default 5)"`
	Source  string   `json:"source,omitempty" jsonschema:"filter results to a specific source label"`
}

type sharedListInput struct{}

// registerSharedTools adds cross-session shared knowledge tools to the MCP server.
func (b *Bridge) registerSharedTools() {
	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_shared_index",
		Description: "Index content into the shared (cross-project) knowledge base. Content persists across all projects and sessions.",
	}, b.handleSharedIndex)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_shared_search",
		Description: "Search the shared (cross-project) knowledge base. Returns results from content indexed by any project.",
	}, b.handleSharedSearch)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_shared_list",
		Description: "List all content sources in the shared (cross-project) knowledge base.",
	}, b.handleSharedList)
}

func (b *Bridge) handleSharedIndex(ctx context.Context, _ *mcpsdk.CallToolRequest, input sharedIndexInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_shared_index")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_shared_index", false)

	shared, err := b.getSharedStore()
	if err != nil {
		return errorResult("shared store: " + err.Error()), struct{}{}, nil
	}

	ct := input.ContentType
	if ct == "" {
		ct = "markdown"
	}

	var (
		res model.IndexResult
		idxErr error
	)

	switch ct {
	case "plain":
		res, idxErr = shared.IndexPlainText(input.Content, input.Label, 20)
	case "json":
		res, idxErr = shared.IndexJSON(input.Content, input.Label)
	default: // "markdown"
		res, idxErr = shared.Index(input.Content, input.Label)
	}

	if idxErr != nil {
		return errorResult("shared index failed: " + idxErr.Error()), struct{}{}, nil
	}

	return textResult(fmt.Sprintf("Indexed into shared store as %q (%d chunks)", res.Label, res.TotalChunks)), struct{}{}, nil
}

func (b *Bridge) handleSharedSearch(ctx context.Context, _ *mcpsdk.CallToolRequest, input sharedSearchInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_shared_search")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_shared_search", true)

	shared, err := b.getSharedStore()
	if err != nil {
		return errorResult("shared store: " + err.Error()), struct{}{}, nil
	}

	// Normalize: if Query is set and Queries is empty, use Query.
	if input.Query != "" && len(input.Queries) == 0 {
		input.Queries = []string{input.Query}
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 5
	}

	var sb strings.Builder

	for i, q := range input.Queries {
		results, searchErr := shared.SearchWithFallback(q, limit, input.Source)
		if searchErr != nil {
			fmt.Fprintf(&sb, "## Query %d: %s\nError: %s\n\n", i+1, q, searchErr)
			continue
		}

		fmt.Fprintf(&sb, "## Query %d: %s (%d results)\n", i+1, q, len(results))

		for _, r := range results {
			fmt.Fprintf(&sb, "### %s [%s] (rank: %.2f, layer: %s)\n%s\n\n",
				r.Title, r.Source, r.Rank, r.MatchLayer, r.Content)
		}
	}

	output := sb.String()
	if output == "" {
		sources, srcErr := shared.ListSources()
		if srcErr == nil && len(sources) > 0 {
			var srcSb strings.Builder
			srcSb.WriteString("No results found in shared store.\n\n## Available Shared Sources\n")

			for _, src := range sources {
				fmt.Fprintf(&srcSb, "- **%s** (%d chunks)\n", src.Label, src.ChunkCount)
			}

			srcSb.WriteString("\nTry searching with different terms or specify a source filter.")
			output = srcSb.String()
		} else {
			output = "No results found in shared store."
		}
	}

	return textResult(output), struct{}{}, nil
}

func (b *Bridge) handleSharedList(ctx context.Context, _ *mcpsdk.CallToolRequest, _ sharedListInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_shared_list")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_shared_list", true)

	shared, err := b.getSharedStore()
	if err != nil {
		return errorResult("shared store: " + err.Error()), struct{}{}, nil
	}

	sources, err := shared.ListSources()
	if err != nil {
		return errorResult("list shared sources: " + err.Error()), struct{}{}, nil
	}

	if len(sources) == 0 {
		return textResult("Shared knowledge base is empty. Use ctx_shared_index to add cross-project content."), struct{}{}, nil
	}

	stats, _ := shared.GetStats()

	var sb strings.Builder
	fmt.Fprintf(&sb, "## Shared Knowledge Base\n\n")
	fmt.Fprintf(&sb, "**Sources:** %d | **Chunks:** %d | **Code chunks:** %d\n\n", stats.Sources, stats.Chunks, stats.CodeChunks)

	for _, src := range sources {
		fmt.Fprintf(&sb, "- **%s** (%d chunks)\n", src.Label, src.ChunkCount)
	}

	return textResult(sb.String()), struct{}{}, nil
}
