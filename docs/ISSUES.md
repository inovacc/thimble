# Known Issues

## Open Issues

### False "hook error" labels in Claude Code (upstream bug)

**Affected**: All hook events (PreToolUse, PostToolUse, SessionStart, UserPromptSubmit)

Claude Code displays "hook error" in the transcript for **every** hook execution, regardless of whether the hook succeeded. Hooks exit 0, return valid JSON on stdout, produce 0 bytes on stderr, and complete in <40ms — but the label still appears.

**Impact**:
- **Cosmetic**: Users see "PreToolUse:Read hook error" / "SessionStart:resume hook error" on every tool call
- **Functional**: The false error labels are injected into the model's context. With multiple hooks, the accumulated "error" signals cause the model to prematurely end turns, especially in plugin-heavy setups
- **Diagnosis**: If `<context_window_protection>` or `<session_knowledge>` blocks appear in the session, the hooks **are working** despite the label

**Upstream tracking**: [anthropics/claude-code#34713](https://github.com/anthropics/claude-code/issues/34713) (and related: #10936, #10463, #12671, #17088, #27886)

**Workaround**: None available — this is a Claude Code UI/runtime bug. Thimble hooks function correctly; ignore the "hook error" labels.

### ~~Hook settings pointing to stale test binary~~ [RESOLVED]

Fixed: `isTestBinary()` guard in `generateHookConfig()` and `generateMCPConfig()` detects `.test` / `.test.exe` binaries and refuses to persist them. See resolved issues table below.

## Resolved Issues

| Issue | Resolution | Date |
|-------|------------|------|
| `formatViaAdapter` drops server responses — server uses `"result"` key but CLI looks for `"additionalContext"`/`"context"`, causing empty stdout for SessionStart, PreCompact, PreToolUse advisories, and projectDir injection | Added type-switch on `"result"` key in `formatViaAdapter`: string → context, object → modify | 2026-03-18 |
| WebFetch, curl/wget, build tools hard-blocked — prevents legitimate fetches and builds | Converted from hard deny (`Blocked: true`) to soft advisory (`HookOutput.Result` context injection) | 2026-03-18 |
| `client.test.exe` zombie spawns on Windows — `StartOnDemand` called `os.Executable()` during tests, spawning detached test binaries | Removed entirely — no daemon, no `StartOnDemand` (ADR-0009) | 2026-03-23 |
| Data race in `client.Reset()` under concurrent calls | Removed entirely — no singleton client (ADR-0009) | 2026-03-23 |
| Hook commands fail silently when daemon not running | Removed entirely — hooks now run in-process (ADR-0009) | 2026-03-23 |
| Daemon idle-shuts down during long MCP sessions with no tool calls | Removed entirely — no daemon (ADR-0009) | 2026-03-23 |
| Hook settings pointing to stale test binary (`thimble.test.exe` from Go build cache) | `isTestBinary()` guard in `generateHookConfig()`/`generateMCPConfig()` detects `.test`/`.test.exe` binaries and refuses to register | 2026-03-27 |
