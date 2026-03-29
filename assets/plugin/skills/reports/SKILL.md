---
name: reports
description: List and show auto-generated reports. Use when the user says "ctx reports", "show reports", or wants to see analysis results from background tasks.
---

# Reports — Auto-Generated Reports

Use the report MCP tools to manage auto-generated reports:

- `ctx_report_list` — List all available reports
- `ctx_report_show(id)` — Display a specific report
- `ctx_report_delete(id)` — Delete a report

If `$ARGUMENTS` is provided, show that specific report. Otherwise, list all reports.
