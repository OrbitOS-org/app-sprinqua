# Sprinqua

Smart irrigation controller for Raspberry Pi relay boards, built as an [Orbit OS](https://www.orbit-os.org) app.

**[sprinqua.com](https://www.sprinqua.com)**

Sprinqua runs directly on your Raspberry Pi and exposes a web UI accessible through the Orbit OS app hub. No cloud account, no external service — everything stays on your device.

---

## Features

- **Multi-zone control** — manual ON/OFF and timed pulse per zone
- **Scheduler** — weekly programs with a visual Gantt chart and time picker
- **Smart Watering** — skip or proportionally adjust watering based on weather, powered by [Open-Meteo](https://open-meteo.com) (no API key required):
  - **Skip only** — skip a run when forecast rain exceeds your threshold
  - **Manual / Monthly** — fixed or month-by-month percentage adjustment
  - **Zimmerman ET** — adapts duration to yesterday's temperature, humidity and rain
  - **Reference ETo** — FAO-56 evapotranspiration vs. an automatically calculated 12-month baseline
  - **Frost protection** — skip a run when the forecast minimum temperature drops below your threshold
- **Exclusive zone mode** — only one relay active at a time (configurable)
- **Per-zone configuration** — rename zones, change type, max duration and enable/disable, anytime from Settings
- **MQTT** — publish zone state and receive commands; compatible with Home Assistant auto-discovery
- **Activation history** — log of every run with an adaptive 24-hour timeline chart; skipped entries (rain/frost) are marked separately
- **i18n** — English, Portuguese, Spanish, French, German, Italian
- **No build step** — UI uses HTMX + Tailwind CSS CDN, rendered server-side with Go templates

---

## Supported relay boards

All boards connect directly to the Raspberry Pi GPIO header (40-pin).

| Board | SKU | Channels |
|---|---|---|
| Waveshare RPi 3-Channel Relay | 11638 | 3 |
| Seengreat 3-CH Relay HAT | 250509 | 3 |
| Keyestudio RPI 4-Channel Relay | KS0212 | 4 |
| Seengreat 4-CH Relay HAT | 220741 | 4 |
| BC Robotics 4-Channel Relay HAT | RAS-193 | 4 |
| Waveshare RPi Zero 6-ch Relay | 20863 | 6 |
| Waveshare RPi 8-Channel Relay | 15423 | 8 |
| Seengreat 8-CH Relay Board | 260115 | 8 |

---

## Building from source

Sprinqua is an [Orbit OS](https://www.orbit-os.org) app. Orbit OS is a lightweight edge computing platform that runs on top of your existing Linux installation — it does not replace your OS. Apps run inside the Gravity RT runtime and are managed through the device Launcher.

To build Sprinqua you need [Orbit Studio](https://www.orbit-os.org/getting_started.html) — the VS Code extension that provides project scaffolding, local development against a real device over TCP/mTLS, and one-click build and deploy. You also need Go installed on your machine.

```bash
git clone https://github.com/OrbitOS-org/app-sprinqua
cd sprinqua
go work sync
go mod tidy
```

Then use Orbit Studio to build (`Orbit: Build ORB`) and deploy (`Orbit: Deploy`) to your Raspberry Pi. Once deployed, Sprinqua appears in the device Launcher alongside other installed apps.

---

## License

GPL-3.0 — see [LICENSE](LICENSE).
