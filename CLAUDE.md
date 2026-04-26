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

- **Write commit messages in English only. No exceptions, even when conversing in another language.**
- Write concise, single-line commit messages following Semantic Commit format: `<type>: <subject>`
- Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `ci`, `style`, `perf`
- Up to 2-3 lines are acceptable when a single line cannot adequately convey the change
- Examples: `feat: add Slack OAuth callback endpoint`, `fix: resolve nil pointer in ticket handler`
- Do NOT include `Co-Authored-By` trailers

## Pull Request Descriptions

- **Write PR titles and bodies in English only. No exceptions, even when conversing in another language.**
- PR titles MUST follow Semantic naming: `<type>: <subject>` (same convention as commit messages)
- Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `ci`, `style`, `perf`
- Examples: `feat: add Slack OAuth callback endpoint`, `fix: resolve nil pointer in ticket handler`
- Keep titles short (under 70 characters) and use the body for details
- Include a Summary section and a Test plan section

## Firestore Storage (CRITICAL)

- Never use `firestore` struct tags — they are a bug magnet. Use Go's default field names (PascalCase) as Firestore keys.
- Never create wrapper types or conversion functions (e.g., `ticketToMap` / `mapToTicket`) for Firestore serialization. Store domain models directly via `ref.Set(ctx, model)` and retrieve via `doc.DataTo(&model)`.
- Domain model structs in `pkg/domain/model/` are the single source of truth for both application logic and persistence schema.
- **Do not design schemas or queries that require composite indexes.** Composite indexes have to be provisioned out-of-band (Firebase console / Terraform), break dev-mode bring-up, and are easy to forget when the schema evolves. Prefer queries that work against Firestore's automatic single-field indexes — typically a single `Where` *or* a single `OrderBy`, not both on different fields. When you need a filter + ordering combination, fetch the parent collection (which is already workspace-scoped and bounded), then filter and sort in Go. If a workload ever genuinely outgrows this, surface it explicitly rather than silently adding a composite-index requirement.
- Multi-writer paths must be atomic. Avoid read-modify-write on a Firestore document; use `ref.Set` with `firestore.Merge([]string{"Field", "subkey"}, ...)` for per-field updates, or `RunTransaction` when multiple documents are involved.

## Tech Stack

- Backend: Go (chi/v5, goerr/v2, urfave/cli/v3)
- Frontend: TypeScript + React (Vite, Tailwind CSS, shadcn/ui)
- API: OpenAPI-first (oapi-codegen for Go, openapi-typescript + openapi-fetch for TS)
- DB: Firestore (with in-memory implementation for development)
- Auth: Slack OIDC (with --no-authn dev mode)
- Error tracking: Sentry (getsentry/sentry-go)
- Frontend is embedded into Go binary via go:embed

## Internationalization (CRITICAL)

Whenever you add, modify, or move user-facing string literals, you MUST route them through the i18n layer instead of hardcoding text in the source. This applies to both backend and frontend.

**Backend (Go):**
- Slack notifications and any other end-user copy must be emitted via `i18n.From(ctx).T(key, "name", value, ...)` from `pkg/utils/i18n/`.
- Add the key to `pkg/utils/i18n/keys.go` and provide translations for **every** supported language (`en.go`, `ja.go`). Missing-key parity is enforced by tests in `pkg/utils/i18n/i18n_test.go`.
- The active language is decided once at startup from `--lang` / `SHEPHERD_LANG`; do not invent per-request language switches.
- Out of scope (keep English): `slog` log lines, internal `goerr` messages, HTTP error response bodies — these are operator-facing, not end-user-facing.

**Frontend (TypeScript / React):**
- All user-visible strings must come from `useTranslation().t("key", { ...params })`. Never hardcode English (or Japanese) text in JSX, `placeholder`, `aria-label`, `title`, toast/error messages, etc.
- Add the key to `frontend/src/i18n/keys.ts` and supply translations in **both** `en.ts` and `ja.ts`. The `Messages` type forces parity at compile time — a missing entry is a `tsc` error.
- Use `{name}` placeholders for interpolation (e.g., `"Page {current} of {total}"`); never build sentences via string concatenation, since word order differs between languages.
- Strings that are intentionally untranslatable (URLs, code identifiers, brand names like `Shepherd`, Slack channel IDs) may stay as literals.

If you find a hardcoded user-facing string while editing nearby code, take the opportunity to convert it — leaving them mixed defeats the purpose of the i18n layer.

## Project Structure

- `pkg/cli/` — CLI commands (serve, migrate, validate)
- `pkg/controller/http/` — HTTP handlers, middleware, routing
- `pkg/usecase/` — Business logic
- `pkg/domain/` — Domain models, types, interfaces
- `pkg/repository/` — Data access (firestore, memory)
- `pkg/utils/errutil/` — Error handling (slog + Sentry)
- `pkg/utils/logging/` — Structured logging (slog)
- `frontend/` — React SPA (embedded via static.go)
