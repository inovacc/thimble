---
name: analyze
description: Run code analysis on a file or directory. Extracts symbols, call graphs, and structure for Go, Python, Rust, TypeScript, and Protobuf. Use when the user asks to analyze, understand, or map out code structure.
---

# Code Analysis

Use the `ctx_analyze` MCP tool to analyze the target path. This extracts:
- Symbol definitions (functions, types, interfaces, structs)
- Call graphs and dependency relationships
- File structure and module organization

Supported languages: Go, Python, Rust, TypeScript, Protobuf.

After analysis, use `ctx_search` to query the indexed results for specific symbols or patterns.

If `$ARGUMENTS` is provided, analyze that path. Otherwise, analyze the current project root.
