# ANT-Sub2API-Local

Cross-platform **personal desktop** AI API gateway with an Anthropic-inspired UI (warm off-white + clay accent, serif headings). A fork of [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) for local single-user use, not multi-tenant SaaS.

> Repo directory and Go module path remain `local-sub2api-lite` / `github.com/Wei-Shaw/sub2api`; `ANT-Sub2API-Local` is the product/display name (window title, site name, build artifacts).

[中文](README_CN.md)

## vs. upstream Sub2API

| | Upstream Sub2API | This repo |
|---|------------------|-----------|
| Target | Multi-user SaaS / self-hosted | Single-machine desktop app |
| Database | PostgreSQL | SQLite (single file) |
| Cache | External Redis | Embedded in-process Redis |
| Users | Registration / many users | One default admin only |
| Payments / subscriptions | Supported | Disabled |
| Delivery | Docker / install scripts | Wails desktop binary |

Core features remain: gateway proxying, accounts/groups/channels, admin UI, ops monitoring. SaaS flows are off when `run_mode: local`.

## Features

- AI gateway compatible with Claude, OpenAI, Gemini, Antigravity, and more
- Admin UI for accounts, groups, channels, and settings
- Ops monitoring (some heavy aggregates are simplified on SQLite)
- Auto-generated default API Key on first start; copy from **Local settings** (`/admin/local`)

## Requirements

- Go 1.26+
- Node.js 18+, pnpm
- Windows / macOS / Linux (system WebView2 / WebKit, etc.)

## Build

```bash
cd frontend && pnpm install && pnpm run build

cd ..
./scripts/build-desktop.ps1   # Windows PowerShell
# or: ./scripts/build-desktop.sh

# Manual build (requires both production and embed tags)
cd desktop
go mod tidy
go build -tags "production,embed" -ldflags "-s -w -H windowsgui" -o ../dist/ANT-Sub2API-Local.exe .
```

Output: `dist/ANT-Sub2API-Local.exe` (~95MB, embedded UI + WebView).

For development with hot reload (requires [Wails CLI](https://wails.io/)):

```bash
cd desktop && wails dev
```

## First run

1. Launch the app. Data is stored under:
   - Next to the executable: `config.yaml`, `sub2api.db`, etc. (override with `DATA_DIR` if needed)
   - Linux: `~/.config/local-sub2api-lite`
2. Sign in to the admin UI as `admin@localhost` (random password on first launch if unset — see Local settings).
3. Copy the default API Key from **Local settings** and point your tools at `http://127.0.0.1:8080`.

**HTTP port**

- Admin UI → **Local settings** (`/admin/local`): set port, check availability, save, then restart the app
- Or edit `server.port` in `config.yaml` under the data directory
- If the configured port is busy at startup, the next available port is chosen and written to the config (you will see a dialog)

See [deploy/config.example.yaml](deploy/config.example.yaml).

## Backend only (no desktop shell)

```bash
cp deploy/config.example.yaml /path/to/DATA_DIR/config.yaml
export DATA_DIR=/path/to/data
cd backend && go run -tags embed ./cmd/server
```

## Layout

```
local-sub2api-lite/
├── backend/     # Go server
├── frontend/    # Vue admin UI
├── desktop/     # Wails entry
├── deploy/      # Sample config
└── dist/        # Build output (gitignored)
```

## License

This project is based on upstream code under [GNU LGPL v3.0](LICENSE). Forked code is distributed under the same license. See the upstream repository for disclaimers.

## Acknowledgements

- Upstream: [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api)
