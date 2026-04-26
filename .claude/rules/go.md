---
paths:
  - "**/*.go"
---

# Go conventions

## Visibility

- **Default to unexported.** An identifier may only be capitalised when it is *actually* used from another package in non-test code. "A test uses it" is not a reason to export — that is what `export_test.go` is for.
- Before adding a capitalised name, grep for non-test callers (`grep -rn "pkg\." --include='*.go' | grep -v _test.go`). If none exist, lowercase it.
- For test-only access to internal identifiers, place a `package <pkg>` (NOT `<pkg>_test`) file named `export_test.go` and re-export under a `*ForTest` alias / variable / constant. The `ForTest` suffix is required so the seam is obvious at the call site. Example:

  ```go
  // notion/export_test.go
  package notion

  var BuildToolsForTest = buildTools
  func SetTokenForTest(f *Factory, t string) { f.token = t }
  ```

- Do NOT add capitalised names with comments like `// exported for testing` directly in production files. Move them into `export_test.go` instead — the compiler then enforces that the seam never reaches the production binary.
- Helper / setup files that exist only to support tests must end in `_test.go` so they never compile into the production binary.

## Other Go house-keeping

- Use `goerr/v2` (`github.com/m-mizutani/goerr/v2`) for wrapping errors with context: `goerr.Wrap(err, "load ticket", goerr.V("ticket_id", id))`. Do not use `fmt.Errorf("%w", ...)` for new error sites — the codebase has standardised on goerr.
- Logging goes through `pkg/utils/logging`; error reporting through `pkg/utils/errutil` (these are also documented in CLAUDE.md but matter for every Go file you touch).
- Background goroutines launch via `pkg/utils/async.Dispatch` / `RunParallel`, never raw `go func(){...}()` — those helpers wrap panic recovery and Sentry routing.
