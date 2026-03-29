package hooks

import "sync"

// guidanceTracker ensures that advisory messages are shown at most once per
// session per advisory type, preventing the same guidance from flooding the
// conversation context.
type guidanceTracker struct {
	seen sync.Map // key: sessionID + "\x00" + advisoryType
}

// Advisory types for guidance throttling.
const (
	AdvisoryUseCtxSearch        = "use_ctx_search"
	AdvisoryUseCtxExecuteFile   = "use_ctx_execute_file"
	AdvisoryUseCtxFetchAndIndex = "use_ctx_fetch_and_index"
	AdvisoryReadForAnalysis     = "read_for_analysis"
	AdvisoryGrepContextFlood    = "grep_context_flood"
	AdvisoryBashLargeOutput     = "bash_large_output"
)

func newGuidanceTracker() *guidanceTracker {
	return &guidanceTracker{}
}

// ShouldShow returns true only on the first call for a given session + advisory
// combination. Subsequent calls with the same pair return false.
func (g *guidanceTracker) ShouldShow(sessionID, advisory string) bool {
	key := sessionID + "\x00" + advisory
	_, loaded := g.seen.LoadOrStore(key, struct{}{})

	return !loaded
}

// Reset clears all tracked advisories (useful for testing).
func (g *guidanceTracker) Reset() {
	g.seen.Range(func(key, _ any) bool {
		g.seen.Delete(key)
		return true
	})
}
