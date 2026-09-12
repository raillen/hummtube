# AGENTS.md — HummTube Web

## Mission
Build **HummTube Web** (formerly NanoTube Web) as an ultra-light, modern, performant desktop YouTube client for Linux and modest hardware.
Stack: **Wails v3 + Svelte + Tailwind CSS + Lucide Icons + Go + SQLite**.
Optimize for correctness, maintainability, responsive UX and measured resource usage.

## Canonical entrypoint
Read `ATLAS.md` first.
Then read only the domain documents relevant to the task.
Use `docs/09-decisions/DECISION_REGISTER.md` to distinguish decisions from proposals.

## Target
Reference hardware: Intel Celeron 1037U / Modern Quad-Core, ~2 GB RAM.
Primary platform: Linux (X11/Wayland), cross-platform compatible.
Backend: Go 1.22+ with Wails v3 Service Bridge.
Frontend: Svelte 5 / Svelte + Vite SPA + Tailwind CSS + Lucide Icons (`lucide-svelte`).
Storage: SQLite (`modernc.org/sqlite`) + FTS5 + Goose migrations.
Player: Custom Web Player (HTML5/HLS.js) backed by `PlaybackResolver` (yt-dlp explícito, POT Provider, Invidious fallback).
Account: OAuth2/PKCE + official YouTube Data API client.
Secrets: System keyring behind `SecretStore`.
Recommendations: Local/explainable (MMR + affinity + decay); no LLM runtime dependency.

## Non-negotiable architecture
1. **Separation of Concerns**: Keep Svelte UI completely independent of raw YouTube extraction APIs and yt-dlp details. Communication is through clean Wails v3 service bindings.
2. **Pure Domain Model**: Go domain models and repository interfaces stay decoupled from web frameworks and third-party libraries.
3. **Single Player Instance**: Only one active video playback stream/session at a time.
4. **Asynchronous Non-Blocking Execution**: Never block the Wails event loop or UI thread with network, disk, or heavy subprocess work. Use Go goroutines with `context.Context`, `errgroup` and `singleflight`.
5. **Declarative UI**: Component styling exclusively in Tailwind CSS with semantic tokens and theme variables. Icons exclusively via Lucide Svelte.
6. **Strict Security & Privacy**: No token logging, CSP enabled in Wails, SSRF guards on stream and Invidious resolution, encrypted local backups (AES-256-GCM).

## Concurrency
- Long-lived tasks take `context.Context`.
- Use `errgroup` for bounded fan-out.
- Use `singleflight` for duplicate idempotent work (e.g., thumbnail fetches).
- Return updates to frontend via Wails v3 events or async service method responses.

## Tooling
- Use Taskfile (`task dev`, `task test`, `task build`, `task verify`).
- Verification gate:
  - `go test ./...`
  - `go vet ./...`
  - `pnpm check` / `npm run check` (TypeScript / Svelte check)
  - `pnpm lint` / `npm run lint`

## Agent Roles
- **Architect**: Architecture, ADRs, contract design, technology boundaries.
- **Implementer (Frontend)**: Svelte components, Tailwind styling, Lucide icons, UI reactivity, TV mode.
- **Implementer (Backend)**: Go services, Wails v3 bridges, SQLite repositories, Sync, Auth.
- **Implementer (Player)**: HTML5/HLS player, PlaybackResolver, stream handling, Audio DSP.
- **Debugger**: Error investigation, logs, devtools, edge cases, regression fixes.
- **Tester**: Unit tests, integration tests, UI mocks, Playwright tests.
- **Reviewer**: Code review, bundle size, performance budget verification.
- **Security Reviewer**: CSP audit, SSRF guard, keyring security, crypto checks.
- **Documentation Maintainer**: Keeping `ATLAS.md`, `docs/`, `DECISION_REGISTER.md` consistent.
- **Release Verifier**: Multi-distro packaging, release checklist, verification gates.

## Change protocol
1. Read ATLAS and impacted docs.
2. State expected modules and files.
3. Implement smallest coherent slice.
4. Run relevant tests and checks.
5. Update canonical docs and decision register when behavior changes.
