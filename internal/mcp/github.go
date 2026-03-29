package mcp

import (
	"context"
	"fmt"
	"net/url"
	"os"

	gogithub "github.com/google/go-github/v82/github"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"

	ghpkg "github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
)

// registerGitHubTools registers all GitHub MCP Server tools on the bridge's
// MCP server. Requires GITHUB_PERSONAL_ACCESS_TOKEN in the environment.
// If the token is absent, tools are silently skipped (GitHub features optional).
func (b *Bridge) registerGitHubTools() {
	token := os.Getenv("GITHUB_PERSONAL_ACCESS_TOKEN")
	if token == "" {
		b.logger.Info("GITHUB_PERSONAL_ACCESS_TOKEN not set, skipping GitHub tools")
		return
	}

	host := os.Getenv("GITHUB_HOST")
	if host == "" {
		host = "github.com"
	}

	ctx := context.Background()

	// Build GitHub API clients.
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(ctx, ts)

	restClient := gogithub.NewClient(httpClient)
	gqlClient := githubv4.NewClient(httpClient)

	// For GHE, override base URLs.
	if host != "github.com" {
		apiURL := fmt.Sprintf("https://%s/api/v3/", host)
		uploadURL := fmt.Sprintf("https://%s/api/uploads/", host)

		var err error

		restClient, err = gogithub.NewClient(httpClient).WithEnterpriseURLs(apiURL, uploadURL)
		if err != nil {
			b.logger.Error("failed to create GHE REST client", "error", err)
			return
		}

		gqlURL := fmt.Sprintf("https://%s/api/graphql", host)
		gqlClient = githubv4.NewEnterpriseClient(gqlURL, httpClient)
	}

	rawURL, _ := url.Parse("https://raw.githubusercontent.com")
	if host != "github.com" {
		rawURL, _ = url.Parse(fmt.Sprintf("https://%s/raw", host))
	}

	rawClient := raw.NewClient(restClient, rawURL)

	// Translation helper (passthrough — no i18n needed).
	t := translations.NullTranslationHelper

	// Build dependencies.
	deps := ghpkg.NewBaseDeps(
		restClient,
		gqlClient,
		rawClient,
		nil, // repoAccessCache — not needed without lockdown mode
		t,
		ghpkg.FeatureFlags{},
		0,   // contentWindowSize — use default
		nil, // featureChecker — none
	)

	// Build inventory with all tools.
	inv, err := ghpkg.NewInventory(t).
		WithToolsets(ghpkg.ResolvedEnabledToolsets(false, []string{"all"}, nil)).
		WithDeprecatedAliases(ghpkg.DeprecatedToolAliases).
		Build()
	if err != nil {
		b.logger.Error("failed to build GitHub tool inventory", "error", err)
		return
	}

	// Inject deps into context via middleware so tool handlers can retrieve them.
	b.server.AddReceivingMiddleware(ghpkg.InjectDepsMiddleware(deps))

	// Register all GitHub tools, resources, and prompts on thimble's MCP server.
	inv.RegisterAll(ctx, b.server, deps)

	b.logger.Info("registered GitHub MCP tools",
		"host", host,
		"tool_count", len(ghpkg.AllTools(t)),
	)
}
