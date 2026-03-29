---
name: thimble
description: Context-aware code intelligence routing. Use when researching code, running commands, fetching URLs, or analyzing projects. Routes data-heavy operations through thimble's FTS5 knowledge base to protect the context window.
---

# Thimble — Context Window Protection

Raw tool output floods your context window. Use thimble MCP tools to keep raw data in the sandbox.

## Tool Selection Hierarchy

1. **GATHER**: `ctx_batch_execute(commands, queries)`
   - Primary tool for research. Runs all commands, auto-indexes, and searches.
   - ONE call replaces many individual steps.

2. **FOLLOW-UP**: `ctx_search(queries: ["q1", "q2", ...])`
   - Use for all follow-up questions. ONE call, many queries.

3. **PROCESSING**: `ctx_execute(language, code)` | `ctx_execute_file(path, language, code)`
   - Use for API calls, log analysis, and data processing.

4. **ANALYZE**: `ctx_analyze(path)`
   - Code analysis with symbol extraction (Go, Python, Rust, TypeScript, Protobuf).

5. **FETCH**: `ctx_fetch_and_index(url)`
   - Fetch URL, convert to markdown, index in knowledge base.

6. **DELEGATE**: `ctx_delegate(task)` | `ctx_delegate_status(id)` | `ctx_delegate_cancel(id)` | `ctx_delegate_list`
   - Background task execution (max 5 concurrent, 1MB output cap).

## Forbidden Actions

- DO NOT use Bash for commands producing >20 lines of output.
- DO NOT use Read for analysis (use `ctx_execute_file`). Read IS correct for files you intend to Edit.
- DO NOT use WebFetch (use `ctx_fetch_and_index` instead).
- Bash is ONLY for git/mkdir/rm/mv/navigation.

## Output Constraints

- Keep your final response under 500 words.
- Write artifacts (code, configs, PRDs) to FILES. NEVER return them as inline text.
- Return only: file path + 1-line description.
