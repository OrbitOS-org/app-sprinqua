# Multi-zone watering Programs (sequential zones, single start time)

## Context

Confirmed by reading the code: `config.Schedule` (`internal/config/config.go:29-38`) is a flat, single-zone record — `ZoneID` + `StartTime` + `DurMins`. The scheduler (`internal/scheduler/scheduler.go`) fires each `Schedule` independently when the clock hits its own `StartTime`. There is no concept of a multi-zone "program" with one start time and zones running back-to-back — exactly what the user observed. Changing one zone's duration never affects another zone's `StartTime`, because they're unrelated records.

Interestingly the UI/i18n already speaks of "Programs" (`sched_title` = "Programs", `sched_form_new` = "New program") even though the data model is single-zone. This change makes the data model match the language already used in the product: a Program has one `StartTime` + ordered list of `(Zone, Duration)` steps that run sequentially, computed automatically — never hand-edited per zone.

No versioning/migration system exists in `internal/config`. Old `config.json` files (flat `zone_id`/`dur_mins` per schedule) must keep loading correctly after this change — handled via a custom `UnmarshalJSON` migration, no explicit schema version needed.

## Data model — `internal/config/config.go`

- Add:
  ```go
  type ProgramZone struct {
      ZoneID  int `json:"zone_id"`
      DurMins int `json:"dur_mins"`
  }
  ```
- Change `Schedule` (lines 29-38): remove scalar `ZoneID`/`DurMins`, add `Zones []ProgramZone`. Keep `ID`, `Name`, `Days`, `StartTime`, `Enabled`, `SmartWatering` unchanged.
- Add `func (s Schedule) TotalMins() int` summing `DurMins` across `Zones` — replaces the `sched.DurMins <= 0` guard used in `scheduler.go` (tick + `NextRunFor`) and in `handlers.go` chart range/skip logic.
- Add a custom `UnmarshalJSON` on `Schedule` that decodes into a superset struct (current fields + legacy `zone_id`/`dur_mins`), and if the new `zones` array is empty but legacy `zone_id` is non-zero, synthesizes `Zones = []ProgramZone{{ZoneID: zone_id, DurMins: dur_mins}}`. This makes old `config.json` files upgrade silently in memory; the next `Save()` persists the new shape. Verified against the real `cmd/sprinqua/config.json` (currently `"schedules": null`, so no risk there, but field deployments may have populated schedules).

## Scheduler — `internal/scheduler/scheduler.go`

- `tick()` (line 97-99): change guard to `sched.TotalMins() <= 0`.
- `runSchedule()` (lines 118-179): keep the weather-skip gate (rain/frost) before anything starts, but it must record a skip for **every** zone in the program (loop `sched.Zones`, call `hist.Skip(z.ZoneID, history.SkipRain/SkipFrost)` per zone) so per-zone history stays accurate.
- Compute the smart-watering multiplier (`adjustment.Calc`) once per program run (it's weather-based, not zone-specific — confirmed via `internal/adjustment/adjustment.go:20`), then apply it to each zone step's `DurMins` individually.
- Replace the single TurnOn/sleep/TurnOff block with a sequential loop over `sched.Zones`: for each step, if adjusted `dur <= 0` call `hist.Skip(z.ZoneID, history.Schedule)` and continue to the next zone (don't abort the whole program); otherwise `eng.TurnOn`, `hist.Start`, `time.Sleep`, `eng.TurnOff`, `hist.Stop`, then proceed to the next step. This preserves today's "back-to-back" behavior but now it's computed, not hand-typed.
- `NextRunFor()` (line 183): change `sched.DurMins <= 0` to `sched.TotalMins() <= 0`. Logic is otherwise unaffected (only cares about `Days`/`StartTime`).

## Web layer — `internal/web/handlers.go`

- `parseScheduleForm` (lines 1346-1374): parse repeated `zone_id` / `dur_mins` form fields (same pattern already used for `days` — `r.Form["zone_id"]`, `r.Form["dur_mins"]`, paired by position/index since HTML forms preserve row submission order) into `[]config.ProgramZone`, skipping rows with empty/invalid zone.
- Add a "zones" validation alongside the existing "days" validation in `handleScheduleCreate`/`handleScheduleUpdate`/`handleScheduleNew`/`handleScheduleEdit`: redirect with `?err=zones` (mirrors `?err=days`) when the parsed program has zero zone steps. Add `ZonesError bool` to `scheduleFormData`.
- `scheduleView` (lines 711-717): replace `ZoneName`/`Color`/(implicit `DurMins` from embedded `Schedule`) with a `ZoneSteps []zoneStepView{Name, DurMins, Color}` (new small struct) and `TotalMins int`, built by resolving each `ProgramZone.ZoneID` through `zmap`/`zoneColor`.
- `buildSchedulePage` (lines 729-916): in the weekly-chart raw-bar loop (lines 776-808), iterate `sc.Zones` with a running `offset` (cumulative sum of previous steps' `DurMins` within the same schedule) to emit one `rawBar` per zone-step instead of one per schedule (`sm := scheduleStartMin + offset`, `em := sm + step.DurMins`). The existing lane-packing/adaptive-range code (Steps 2-4, lines 810-899) is reused unchanged — it already handles multiple bars per day.

## Templates

- `schedule_form.html`: replace the single "Zone" `<select>` + "Duration" `<input>` (lines 43-53, 120-131) with a repeatable "zones in this program" list:
  - Server-rendered rows (one per existing `Schedule.Zones[i]`, or a single default row when new) inside a container `#zone-steps`, each row = zone `<select name="zone_id">` + duration `<input name="dur_mins">` + remove (×) button + ▲▼ reorder buttons (reordering = swapping the row with its DOM sibling, since submission order = DOM order).
  - A hidden `<template>` element for the JS "+ Add zone" button to clone.
  - Inline `zones-error` message (mirrors the existing `days-error` pattern at lines 59-63, 245-261) plus client-side submit guard requiring ≥1 row.
  - Keep Name/Days/Time/Enabled/SmartWatering sections unchanged.
- `schedule_list.html`: card body (lines 19-28) currently shows `ZoneName` + single `DurMins`; replace with a sequential chip row, e.g. `Zone A · 5min → Zone B · 10min → Zone C · 8min`, and show `TotalMins` where `DurMins` is shown today (line 27).

## i18n (all 6 locales: en, pt, es, fr, de, it)

- Update `sched_form_zone` copy from "Zone" to a section label like "Zones in this program".
- Add `sched_form_add_zone` ("+ Add zone"), `sched_zones_error` ("Add at least one zone.").

## Verification

1. `go build ./...` after each layer (config → scheduler → handlers → templates).
2. Add a small regression test `internal/config/config_test.go` covering: (a) unmarshaling a legacy single-zone JSON schedule synthesizes one `ProgramZone`; (b) unmarshaling a new multi-zone JSON round-trips correctly. This is the one truly non-obvious/risky path (silent migration) worth pinning down even though the repo has no existing test suite.
3. Manually run the app (`/run` skill or `go run ./cmd/sprinqua`), open `/schedule`, create a program with 2-3 zones with different durations, confirm: rows reorder via ▲▼, the weekly chart shows back-to-back bars (not overlapping), changing zone 1's duration shifts zone 2's bar without touching any stored "start time" for zone 2, and the list card shows the sequential chips + correct total duration.
4. Confirm the existing `cmd/sprinqua/config.json` (currently `schedules: null`) still loads and the app starts cleanly.
