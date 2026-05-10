# sprinqua

Orbit Go app.

- **Settings** — `orbit.project.json` at the repo root (device address, build paths).
- **SDK** — `./orbit-os-sdk-go` at **v26.0.1**. Refresh from the extension with **Add Orbit SDK** if you replace it.
- **SDK API reference (Go, API 26+)** — https://www.orbit-os.org/sdk/api-reference.html

- **Launcher icon (fixed path)** — `cmd/sprinqua/orb/icon.svg`: must stay `cmd/<your app>/orb/icon.svg` (bundled default or your SVG).
- **TLS** — dev files in `cmd/certs/grpc/` (not committed). Replace with real certs for production; use the extension’s certificate help if needed.
- **Go** — the extension runs `go work sync` and `go mod tidy` at the end of project creation (needs `go` on your PATH, e.g. WSL or native). If the editor still shows red squiggles in `go.mod` for dependencies that only belong to `./orbit-os-sdk-go`, try **Developer: Reload Window** or run `go work sync` and `go mod tidy` in the project root and in `./orbit-os-sdk-go` yourself.
- **Build & deploy** — Orbit side bar, or **Orbit: Build ORB** and **Orbit: Deploy / Deploy to device**. If Go complains about the version, use **Orbit: Set go** to align `go.mod` / `go.work` with your machine.

**From the project name:** module `sprinqua` · on-device id `sdk.dev.sprinqua` · app folder `cmd/sprinqua`
