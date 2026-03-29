---
name: cloud-setup
description: |
  Connect thimble to a cloud sync endpoint.
  Guides through API URL, token, and org ID configuration.
  Saves config to ~/.thimble/sync.json and tests the connection.
  Trigger: /thimble:cloud-setup
user-invocable: true
---

# Thimble Cloud Setup

Interactive onboarding flow to connect thimble to a cloud sync endpoint.

## Instructions

1. **Check existing config** — read `~/.thimble/sync.json` using ctx_execute:
   ```
   cat ~/.thimble/sync.json 2>/dev/null || echo "NOT_FOUND"
   ```
   - If the file exists and contains a non-empty `api_token`, inform the user that cloud sync is **already configured** and show the current `api_url` and `organization_id` (never reveal the token — show only the last 4 characters masked as `thm_****abcd`).
   - Ask if they want to **reconfigure** or **keep the current settings**. If they want to keep, stop here.

2. **Collect configuration** — ask the user for three values, one at a time:

   **a) API URL**
   - No default — the user must provide their endpoint URL.
   - Tell the user: *"Paste your cloud sync API URL."*

   **b) API Token**
   - Tell the user: *"Paste your API token from the sync dashboard."*
   - This field is **required** — do not proceed without it.
   - Validate: token should be at least 20 characters. If too short, warn and ask again.

   **c) Organization ID**
   - Tell the user: *"Paste your Organization ID from the dashboard."*
   - This field is **required** — do not proceed without it.

3. **Save config** — write the merged config to `~/.thimble/sync.json`:
   ```bash
   mkdir -p ~/.thimble
   cat > ~/.thimble/sync.json << 'JSONEOF'
   {
     "enabled": true,
     "api_url": "<API_URL>",
     "api_token": "<API_TOKEN>",
     "organization_id": "<ORG_ID>",
     "batch_size": 50,
     "flush_interval_ms": 30000
   }
   JSONEOF
   chmod 600 ~/.thimble/sync.json
   ```
   Replace `<API_URL>`, `<API_TOKEN>`, and `<ORG_ID>` with the collected values.

4. **Test the connection** — send a health check:
   ```bash
   curl -sf -o /dev/null -w "%{http_code}" \
     -H "Authorization: Bearer <API_TOKEN>" \
     "<API_URL>/api/health"
   ```
   - `200` = success
   - Any other code or failure = connection error

5. **Display results** as markdown:

   On **success**:
   ```
   ## thimble cloud setup
   - [x] Config saved to ~/.thimble/sync.json
   - [x] Connection test: PASS (200 OK)
   - [x] Organization: <ORG_ID>

   Cloud sync is now active. Run /thimble:cloud-status to check
   sync health at any time.
   ```

   On **failure**:
   ```
   ## thimble cloud setup
   - [x] Config saved to ~/.thimble/sync.json
   - [ ] Connection test: FAIL (<error details>)

   Config was saved but the connection test failed. Check that:
   1. Your API URL is reachable
   2. Your API token is valid and not expired
   3. Your network allows outbound HTTPS

   Run /thimble:cloud-setup again to reconfigure.
   ```

## Security Notes

- Never log or display the full API token. Always mask it.
- Set file permissions to 600 (owner read/write only).
- The token is sent only over HTTPS in the Authorization header.
