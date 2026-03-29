---
name: doctor
description: Run thimble diagnostic checks. Use when the user says "ctx doctor", "thimble doctor", or wants to troubleshoot thimble issues.
---

# Doctor — Diagnostics

Call the `ctx_doctor` MCP tool to run diagnostic checks on the thimble installation.

This verifies:
- gRPC daemon connectivity and health
- Content store database integrity
- Session database state
- Port availability (50351-50355)
- Server info file consistency

Report the results clearly and suggest fixes for any issues found.
