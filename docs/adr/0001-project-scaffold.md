# ADR-0001: Project Scaffold and Tooling Choices

## Status
Accepted

## Context
Setting up a new Go project requires choosing a standard structure, tooling, and conventions that will guide development.

## Decision
- **Structure:** Hexagonal/Clean Architecture (cmd/, internal/, pkg/)
- **CLI Framework:** Cobra via omni scaffold
- **Task Runner:** Taskfile (over Makefile) for cross-platform support
- **Linting:** golangci-lint v2 with curated ruleset
- **Releases:** GoReleaser for automated cross-platform builds
- **Module Path:** github.com/inovacc/thimble

## Consequences

### Positive
- Consistent project structure across all projects
- Cross-platform build and task support
- Automated release pipeline from day one
- Strict code quality from the start

### Negative
- Requires installing external tools (golangci-lint, goreleaser, task)
- Initial setup overhead for small/throwaway projects
