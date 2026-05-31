# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`local-sub2api-lite` is a **fork** of [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api), an AI API gateway, re-targeted from multi-tenant SaaS to a **single-user desktop app**. It proxies requests to Claude / OpenAI / Gemini / Antigravity upstreams and ships as a Wails desktop binary.

The key consequence of the fork: the same codebase runs in two modes, switched by `run_mode` in config.
- **`local`** (this fork's default): SQLite single file + in-process miniredis, one admin user, billing/quota/payments/registration **disabled**.
- **`saas`/`simple`** (upstream paths, still present): PostgreSQL + external Redis, multi-user.

When changing behavior, check `cfg.IsLocalMode()` / `cfg.IsSimpleLike()` / `cfg.UsesSQLite()` / `cfg.UsesEmbeddedRedis()` (in [backend/internal/config/local.go](backend/internal/config/local.go)) — a lot of SaaS logic is gated off in local mode, and SQLite cannot run some Postgres-only aggregate queries.

> The Go module path is still `github.com/Wei-Shaw/sub2api` (upstream, **not** renamed). Imports use that prefix.

## Repository layout

```
backend/   # Go server: the entire gateway + admin API + business logic
frontend/  # Vue 3 + TS admin UI (Vite, pnpm, Pinia, vue-i18n, Tailwind)
desktop/   # Wails v2 shell — embeds frontend/dist, boots the backend in-process
deploy/    # config.example.yaml
dist/      # build output (gitignored)
```

The desktop shell ([desktop/app.go](desktop/app.go), [desktop/main.go](desktop/main.go)) does **not** spawn the backend as a subprocess — it calls `serverapp.Initialize()` directly and runs the HTTP server in a goroutine, then points the WebView at `http://127.0.0.1:<port>`. `desktop/` has its own `go.mod` separate from `backend/go.mod`.

## Build & run

### Frontend (always build first — the backend embeds it)
```bash
cd frontend && pnpm install && pnpm run build   # outputs to backend/internal/web/dist (see vite.config.ts)
```

### Desktop binary (the shippable artifact)
```bash
./scripts/build-desktop.sh        # or build-desktop.ps1 on Windows PowerShell
# Debug build (DevTools + attached console + "(Debug)" title):
SUB2API_DESKTOP_DEBUG=1 ./scripts/build-desktop.sh
```
Output: `dist/local-sub2api-lite.exe` (~90–125 MB; embedded UI + WebView).

### Backend only (no desktop shell)
```bash
cp deploy/config.example.yaml /path/to/DATA_DIR/config.yaml
export DATA_DIR=/path/to/data
cd backend && go run -tags embed ./cmd/server
# or: cd backend && make build   (-> bin/server)
```

### Desktop hot reload (needs Wails CLI)
```bash
cd desktop && wails dev
```

## Build tags — these matter and are easy to get wrong

- **`embed`** — embeds `backend/internal/web/dist` into the binary via `//go:embed`. Without it, the server serves the frontend from disk instead ([backend/internal/web/embed_on.go](backend/internal/web/embed_on.go) vs `embed_off.go`). The backend **must** be built/run with `-tags embed` to serve the bundled UI.
- **`production` / `debug`** — Wails build modes (desktop only). Release strips symbols and uses `-H windowsgui` (no console); debug keeps both and auto-opens DevTools. Set via `desktop/build_release.go` / `build_debug.go` (flips `IsDebugBuild`).
- **DB driver** — the `sqlite` modernc driver is registered in [backend/internal/repository/ent_sqlite.go](backend/internal/repository/ent_sqlite.go). The actual driver used at runtime is chosen from config (`database.driver`), not a build tag.
- Desktop release builds combine all: `-tags "production,embed"`.

## Tests & lint

```bash
cd backend
go test ./...                       # everything
go test -tags=unit ./...            # unit only        (make test-unit)
go test -tags=integration ./...     # integration      (make test-integration; spins up testcontainers Postgres/Redis — needs Docker)
go test -tags=e2e -v -timeout=300s ./internal/integration/...   # e2e (make test-e2e-local)
golangci-lint run ./...             # lint (config: backend/.golangci.yml)

# Run a single test:
go test ./internal/service/ -run TestAccountService_Foo -v
go test -tags=unit ./internal/handler/ -run TestGateway -v   # add the matching build tag if the file is tagged
```
Test files use `//go:build unit | integration | e2e` tags — to run a specific tagged test you must pass the matching `-tags`. Files with no build tag run under plain `go test`.

Frontend:
```bash
cd frontend
pnpm run typecheck     # vue-tsc --noEmit
pnpm run lint          # eslint --fix
pnpm run test:run      # vitest
```

## Backend architecture

Clean-ish layered architecture wired by **google/wire**. The big picture (worth reading across files):

```
cmd/server/main.go        # entry: setup-wizard branch vs runMainServer; graceful shutdown
  └─ serverapp.Initialize()   # wire.Build aggregates every layer's ProviderSet
       config → repository → service → payment → middleware → handler → server
```

- **`internal/handler/`** — gin HTTP handlers. `Handlers` / `AdminHandlers` structs ([handler.go](backend/internal/handler/handler.go)) aggregate every handler. Gateway request handling (the proxy hot path, account failover loop, streaming) lives here: `gateway_handler*.go`, `openai_gateway_handler.go`, `failover_loop.go`.
- **`internal/service/`** — business logic (~hundreds of files). Account pooling/scheduling, OAuth token refresh, billing, usage aggregation, ops monitoring, channel monitors. Provider-specific gateway logic: `gateway_service.go`, `openai_gateway_service.go`, `antigravity_gateway_service.go`, `openai_account_scheduler.go`.
- **`internal/repository/`** — data access over **ent** + Redis caches. AES-encrypted credential storage, layered caches (api_key/billing/concurrency/email/dashboard).
- **`internal/server/`** — router assembly ([router.go](backend/internal/server/router.go)) and `routes/` (admin / auth / user / gateway / payment). `internal/server/middleware/` holds auth (JWT, admin, API-key), CORS, security headers, body limits, rate limiting.
- **`internal/config/`** — viper-based config + `run_mode` helpers.
- **`internal/localconfig/`** — desktop-only: resolves `DATA_DIR` (next to the exe), generates `config.yaml` + secrets on first run, port availability/auto-switch.
- **`internal/setup/`** — first-run setup wizard (HTTP + CLI via `--setup`).
- **`ent/`** — **generated** ent code. Schema definitions are in `ent/schema/*.go`; everything else under `ent/` is codegen output. Regenerate with `make generate`, never hand-edit generated files.

### Enforced dependency rule (golangci `depguard`)
[backend/.golangci.yml](backend/.golangci.yml) **fails the build** if these are violated:
- `internal/service/**` must **not** import `internal/repository`, `gorm`, or `go-redis` (a few `ops_*` files are explicitly exempted). Services depend on repository **interfaces**, not the concrete package.
- `internal/handler/**` must **not** import `internal/repository`.

Keep the layering: handler → service → repository(interface). Don't reach across.

### Gateway routing
`/v1/*` (Claude-compatible), `/v1beta/*` (Gemini-native), `/chat/completions`, `/responses`, `/antigravity/*` are registered in [backend/internal/server/routes/gateway.go](backend/internal/server/routes/gateway.go). Several endpoints **auto-route by the API key's group `platform`** (`getGroupPlatform`) — e.g. `/v1/messages` dispatches to the OpenAI handler when the group is OpenAI, else the Claude handler. Authentication is by API key (`sk-...`); the default key is auto-generated on first launch.

## Database migrations

Raw SQL migrations in [backend/migrations/](backend/migrations/) (`NNN_description.sql`), embedded via `//go:embed *.sql` and run automatically on startup by a custom runner. **Critical rules** (see [backend/migrations/README.md](backend/migrations/README.md)):
- Migrations are **immutable once applied** — verified by SHA256 checksum stored in `schema_migrations`. Editing an applied file causes a checksum-mismatch boot failure. To change something, add a **new** higher-numbered migration.
- Forward-only. No goose-style Up/Down in one file (the runner executes the whole file).
- Must be idempotent (`IF [NOT] EXISTS`).
- `*_notx.sql` suffix = executed outside a transaction (only for `CREATE/DROP INDEX CONCURRENTLY`, Postgres).

Migrations are written with Postgres syntax; the SQLite path tolerates/translates a subset — when adding migrations, be mindful both drivers must apply them in local-vs-saas modes.

## Conventions

- Mixed-language comments (Chinese + English) are normal in this codebase — match the surrounding file.
- `cmd/server/VERSION` is the source of truth for the version (embedded at build; overridable via `-ldflags -X main.Version`).
- **ent codegen**: after editing `ent/schema/*.go`, run `cd backend && make generate` (or `go generate ./ent`). Never hand-edit generated `ent/` files.
- **wire codegen**: the `//go:generate wire` directive lives in [backend/internal/serverapp/wire.go](backend/internal/serverapp/wire.go) (not `cmd/server`, so `make generate` does *not* cover it). After changing any `ProviderSet` or provider signature, regenerate with `cd backend && go generate ./internal/serverapp/` (or `go run github.com/google/wire/cmd/wire ./internal/serverapp`). Do not hand-edit `wire_gen.go`.
