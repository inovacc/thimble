# Bug Tracker

## Open Bugs

_No known bugs yet._

## Bug Template

### Bug Title
- **Severity:** Critical / High / Medium / Low
- **Status:** Open / In Progress / Fixed
- **Reproducible:** Always / Sometimes / Rare
- **Description:** What happens
- **Expected:** What should happen
- **Steps to Reproduce:**
  1. Step 1
  2. Step 2
- **Workaround:** Temporary solution if any

## Fixed Bugs

| Bug | Severity | Fix | Date |
|-----|----------|-----|------|
| `client.test.exe` zombie processes on Windows — `os.Executable()` returns test binary, `StartOnDemand` spawns detached orphans | High | Injectable `startOnDemandFunc` (93616db) | 2026-03-16 |
| Data race in `client.Reset()` — concurrent goroutines writing to `once`/`instance`/`initError` without synchronization | Medium | Added `sync.Mutex` to `Reset()` (93616db) | 2026-03-16 |
