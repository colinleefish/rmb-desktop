## Settings & onboarding

- Localized save/restart messages; guided restart flow with step-by-step progress.
- API key field shows masked value with last two characters visible.
- Models tab adds connection test (same as onboarding).
- Settings page shows running version and commit at bottom-left.

## Tray & daemon

- Tray reads daemon address from config (fixes Starting… after port change).
- Tray auto-restarts rmbd when the service goes down.
- `POST /api/v1/system/restart` for in-app service restart.

## Other

- Launch at login applies with Save; embedding changes require confirmation.
- Config test endpoints fall back to saved API keys when fields are left blank.
