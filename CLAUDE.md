# Shepherd - Development Guide

## Build & Run

```bash
task build          # Full build (generate + frontend + go build)
task dev            # Run both Go server and frontend dev server
task dev:go         # Run Go server only (NoAuthn + memory)
task dev:frontend   # Run frontend dev server only
task generate       # Run all code generation (Go + TypeScript)
task generate:go    # Generate Go server code from OpenAPI spec
task generate:ts    # Generate TypeScript types from OpenAPI spec
task test           # Run Go unit and integration tests
task test:e2e       # Run Playwright E2E tests
```

## Error Handling (CRITICAL)

All errors MUST go through `errutil.Handle(ctx, err)` or `errutil.HandleHTTP(ctx, w, err, statusCode)` in `pkg/utils/errutil/`. These functions handle both slog logging and Sentry reporting in one call.

**Rules:**
- Never use `slog.Error()` directly for error logging — always use `errutil.Handle`
- Never silently discard errors (`_ = someFunc()`) unless the error is genuinely ignorable (e.g., closing a response body)
- HTTP handlers must use `errutil.HandleHTTP` to log + respond in one step
- Background goroutines must `recover()` panics and route them through `errutil.Handle`
- Wrap errors with context using `goerr.Wrap(err, "message")` before passing to Handle

## Logging (CRITICAL)

Never call `slog.Info()`, `slog.Error()`, `slog.Debug()`, `slog.Warn()` or other global slog logger functions directly. Always obtain a logger via `logging.From(ctx)` or `logging.Default()` from `pkg/utils/logging/`.
- Attribute constructors (`slog.String()`, `slog.Any()`, `slog.Int64()`, etc.) are fine — use them as-is
- Use `logging.ErrAttr(err)` for error attributes

## Documentation Rules

- When adding or modifying Slack-related features (events, OAuth, bot behavior, webhook endpoints, CLI flags, etc.), you MUST also update `docs/slack.md` to reflect the changes.
- Any new external integration should have a corresponding setup guide in `docs/`.

## Git Commit Messages

- Write concise, single-line commit messages following Semantic Commit format: `<type>: <subject>`
- Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `ci`, `style`, `perf`
- Up to 2-3 lines are acceptable when a single line cannot adequately convey the change
- Examples: `feat: add Slack OAuth callback endpoint`, `fix: resolve nil pointer in ticket handler`
- Do NOT include `Co-Authored-By` trailers

## Tech Stack

- Backend: Go (chi/v5, goerr/v2, urfave/cli/v3)
- Frontend: TypeScript + React (Vite, Tailwind CSS, shadcn/ui)
- API: OpenAPI-first (oapi-codegen for Go, openapi-typescript + openapi-fetch for TS)
- DB: Firestore (with in-memory implementation for development)
- Auth: Slack OIDC (with --no-authn dev mode)
- Error tracking: Sentry (getsentry/sentry-go)
- Frontend is embedded into Go binary via go:embed

## Project Structure

- `pkg/cli/` — CLI commands (serve, migrate, validate)
- `pkg/controller/http/` — HTTP handlers, middleware, routing
- `pkg/usecase/` — Business logic
- `pkg/domain/` — Domain models, types, interfaces
- `pkg/repository/` — Data access (firestore, memory)
- `pkg/utils/errutil/` — Error handling (slog + Sentry)
- `pkg/utils/logging/` — Structured logging (slog)
- `frontend/` — React SPA (embedded via static.go)
