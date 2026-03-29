---
name: doctor
description: Run thimble diagnostic checks. Use when the user says "ctx doctor", "thimble doctor", or wants to troubleshoot thimble issues.
---

# Doctor — Diagnostics

Call the `ctx_doctor` MCP tool to run diagnostic checks on the thimble installation.

This verifies:
- Content store database integrity
- Session database state
- Runtime configuration
- Hook registration

Report the results clearly and suggest fixes for any issues found.
