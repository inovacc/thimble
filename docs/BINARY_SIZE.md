# Binary Size Analysis

Measured on Go 1.26.1 / Windows amd64 (2026-03-20). Updated for v3.0.0
(dropped `golangci-lint/v2` and `cli/cli/v2` Go imports).

## Build Variants

| Build | Size | Flags |
|-------|------|-------|
| Standard | ~56 MB | (none) |
| Stripped | 28 MB | `-ldflags="-s -w"` |
| Stripped + trimpath | 28 MB | `-ldflags="-s -w" -trimpath` |

Stripping debug info (`-s -w`) saves ~50%. Trimpath has negligible
size impact but removes local filesystem paths from the binary.

132 transitive modules (down from 709 in v2.6.0).

## Top Contributors (by symbol size)

| Module / Category | Size | Notes |
|---|---|---|
| `runtime` (incl. pclntab, type info) | 35.7 MB | Go runtime overhead; scales with total code |
| `crypto` (incl. FIPS 140 module) | 33.3 MB | 32.6 MB from `crypto/internal/fips140` alone |
| ~~`github.com/cli/cli/v2`~~ | ~~2.3 MB~~ | Removed — gh CLI now invoked as subprocess |
| `modernc.org/sqlite` | 2.0 MB | Pure-Go SQLite (expected, no CGO alternative) |
| `golang.org/x/tools` | 1.4 MB | Used by golangci-lint and analysis |
| `github.com/yuin/goldmark-emoji` | 1.0 MB | Emoji shortname lookup table (1 MB static data) |
| `google.golang.org/protobuf` | 0.9 MB | gRPC serialization |
| `github.com/alecthomas/chroma/v2` | 0.9 MB | Syntax highlighting (lexer + style data) |
| `github.com/google/go-github/v82` | 0.9 MB | GitHub REST API client |
| `github.com/github/github-mcp-server` | 0.8 MB | GitHub MCP tool bridge |
| ~~`github.com/golangci/golangci-lint/v2`~~ | ~~0.8 MB~~ | Removed — now subprocess |
| ~~`github.com/ccojocar/zxcvbn-go`~~ | ~~0.7 MB~~ | Removed (was golangci-lint transitive dep) |
| `github.com/inovacc/thimble` | 0.8 MB | Thimble's own code |

Total transitive dependencies: 132 modules (down from 709 pre-v3.0.0).

## Key Findings

### 1. FIPS 140 Crypto Module (32.6 MB)

Go 1.24+ includes a FIPS 140-3 compliant crypto module by default. This adds a
~32 MB static memory reservation (`crypto/internal/fips140/drbg.memory` alone is
33 MB). There is currently no `GOEXPERIMENT=nofips` option to disable it in
Go 1.26.

**Impact:** This is the single largest contributor. It is a Go runtime cost that
affects all Go 1.24+ binaries, not specific to thimble.

### 2. golangci-lint (subprocess)

`internal/server/services_lint.go` invokes `golangci-lint` as an external
subprocess. The Go dependency on `golangci-lint/v2/pkg/commands` has been
removed, eliminating ~300 transitive modules and ~4 MB of binary bloat.

**Status:** Resolved. No build tag needed — requires `golangci-lint` on PATH.

### 3. gh CLI (subprocess)

`internal/server/services_gh.go` invokes `gh` as an external subprocess.
The Go dependency on `cli/cli/v2` has been removed, eliminating ~3 MB of
binary bloat and significant transitive dependencies.

**Status:** Resolved. No build tag needed — requires `gh` on PATH.

### 4. goldmark-emoji (1 MB)

The emoji shortname table is a 1 MB static blob. If emoji rendering is not
critical, this could be replaced with a smaller subset or lazy-loaded.

## Optimizations Already Applied

- **GoReleaser** (`.goreleaser.yaml`): `-s -w` ldflags + `-trimpath`
- **Taskfile** (`Taskfile.yml`): `task build` and `task install` now use
  `-ldflags="-s -w" -trimpath` by default

## External CLI Dependencies

Both the gh CLI and golangci-lint are now invoked as external subprocesses.
No build tags are needed — both services degrade gracefully if the binary
is not found on PATH.

## Future Optimization Opportunities

| Optimization | Est. Savings | Effort |
|---|---|---|
| UPX compression (`upx --best`) | ~40-50% of stripped | Low (CI step) |
| Remove goldmark-emoji or use subset | ~1 MB | Low |
| Wait for Go to offer FIPS opt-out | ~32 MB | None (upstream) |

UPX would compress the 28 MB stripped binary to roughly 12-15 MB but adds
startup decompression time (~100ms). Suitable for distribution but not
recommended for daemon mode where startup latency matters.
