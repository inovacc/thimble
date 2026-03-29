---
name: remember
description: Index content into thimble's FTS5 knowledge base for later retrieval. Use when the user wants to save context, index a URL, or store information for cross-session recall.
---

# Remember — Index Content

Index content into the thimble knowledge base for later retrieval via `ctx_search`.

## Actions

If the user provides a **URL**, use `ctx_fetch_and_index` to fetch and index it.

If the user provides **text or code**, use `ctx_index` with the content.

If the user provides a **file path**, use `ctx_execute_file` to read and auto-index the output.

After indexing, confirm what was stored and remind the user they can retrieve it with `ctx_search`.
