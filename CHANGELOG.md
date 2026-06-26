# Changelog

Notable changes to Sprinqua, grouped by day. Each entry summarizes the
commits made that day; see `git log` for full diffs.

## 2026-06-22

> Current version: `0.2.2`.

### Smart Watering
- Split **Skip Protection** (skip on rain/frost) from **Adjustment Method**
  (duration scaling) — previously one toggle controlled both
- New `SkipEnabled` field on `SmartWateringConfig`; existing configs
  auto-migrate (key absent + `Enabled=true` → `SkipEnabled=true`)
- Renamed "Skip only" method to "No adjustment" (now first/default) across
  all 6 locales
- Settings: each adjustment method card expands inline when selected
  (progressive disclosure)
- New Skip Protection card with inline rain threshold + frost threshold
  fields
- Scheduler now requires `SkipEnabled` (not just `Enabled`) before applying
  rain/frost skip logic
- Zimmerman/ETo estimates for upcoming runs: `~×N` badge + adjusted total
  duration shown when yesterday's weather data is available, plain `~`
  otherwise; amber styling distinguishes estimates from the solid badge
  used by manual/monthly
- Home dashboard: next-program banner shows the Smart Watering badge and
  estimated total duration

### Schedule editor
- Per-zone soak time (pause between zones) and pulse duration
- Field order changed: Name + Active toggle → Start time → Days → Zones →
  Smart Watering; Active toggle moved to the header

### Reliability (SD card safety)
- `config.json` / `history.json`: atomic writes (temp file + rename) —
  prevents corruption on power loss mid-write
- Schedule create/update/toggle/delete now save asynchronously (background
  goroutine + mutex) — UI responds immediately instead of blocking on SD
  card write latency

### Backup & Restore
- New export/import endpoints for `config.json`, with validation on import
- Settings: zones list made collapsible; backup buttons aligned

### Hardware
- Added SB Components Zero Relay board (GPIO22 + GPIO5, `ActiveLow: false`)
- Supported board count 8+ → 9+, channel range 3–8 → 2–8 updated across all
  6 locales

### Setup wizard
- Board selector shows description + SKU below the dropdown on selection
  (both initial render and HTMX updates)

### Home / Zones page
- HA Managed banner added to the Zones page (already existed on Schedule)
- Winter Mode suppressed when HA Managed is active (both pages)

### Settings
- MQTT connection status indicator (connected/disconnected)
- Winter Mode toggle disabled and greyed out in passive (HA-managed) mode
- General mobile layout polish
