---
paths:
  - "**/*_test.go"
---

# Testing

## Test file naming

- Each `xxx_test.go` MUST pair with a `xxx.go` in the same directory. Functional / topical names like `lifecycle_test.go`, `e2e_test.go`, `integration_test.go` are not allowed — pick the source file whose public API the test exercises and name accordingly. If a test spans several source files, name it after the orchestrating type's file (e.g. tests for `usecase.go`'s end-to-end flow live in `usecase_test.go`, not in a separate `lifecycle_test.go`).
- Exceptions:
  - `export_test.go` — Go stdlib convention for exposing internals to the same-package test build.
  - `pkg/repository/*_test.go` — the `runTest` parity pattern tests an interface method against multiple backends from a single file; the file is named after the method/feature, not a single source file.
- Why: keeping the 1:1 mapping makes it trivial to find the test for a given source file (and vice versa) and prevents "kitchen-sink" test files that quietly accumulate.

## String literals in tests

- **Every string literal in `_test.go` MUST be English.** Test fixtures (JSON payloads, struct field values, prompts), `t.Fatalf` / `t.Errorf` messages, expected values in `gt.S(...).Equal(...)`, table-test names — all of it. No Japanese or other non-English text in test code.
- This applies even when the production code under test produces Japanese user-facing copy. The contract you assert against is the i18n key path or the structural shape, not the rendered Japanese sentence — the latter changes whenever a translator tweaks a word and silently breaks tests.
- Why: tests are the executable spec. When fixtures mix languages, grep stops finding callers, diffs become unreadable for non-Japanese contributors, and "why does this test pass / fail" investigations get slowed down by unnecessary translation overhead. Also, fixture text bleeding into production is a recurring bug class.
- If you genuinely need to verify Japanese output (e.g. an i18n smoke test that the `ja` translation file resolves), pull the expected string from the same translation source the production code reads, do not hardcode it inline.

## General

- Repository tests use the `runTest` helper in `pkg/repository/repository_test.go` which runs each test against both Memory and Firestore backends.
- Firestore tests are activated by setting `TEST_FIRESTORE_PROJECT_ID` (and optionally `TEST_FIRESTORE_DATABASE_ID`). When these env vars are absent, Firestore tests are skipped.
- Memory and Firestore must run the exact same test cases — no backend-specific test logic. The `runTest` pattern ensures feature parity between implementations.
- Every new repository interface method must have a corresponding test via `runTest` covering: normal CRUD, empty/not-found cases, and ordering guarantees where applicable.
- Use the `gt` library (`github.com/m-mizutani/gt`) for assertions, not `testify` or bare `if` checks.
- Usecase tests use `memory.New()` as the repository backend.

## Usecase-level tests

Usecase tests must verify *observable outcomes*, not just `err == nil`. A test that only asserts no error returned is not a test — it is a smoke check, and it will let regressions slip through. Every new public method on a usecase needs a real test on this bar:

- **Drive every external dependency through an interface** so tests can substitute fakes/mocks. If a usecase still embeds a concrete client (`*foo.Client`), introduce a small interface in the usecase package covering only the methods it actually calls (e.g. `SlackClient` in `pkg/usecase/slack.go`). Concrete production types satisfy the interface implicitly.
- **For LLM-touching code, use the gollem-provided mocks** (`github.com/m-mizutani/gollem/mock`: `LLMClientMock`, `SessionMock`). Set `NewSessionFunc` / `GenerateFunc` to control the conversation, and inspect the recorded `*Calls()` slices to assert the model was actually invoked with the expected input.
- **For Slack-touching code, use a hand-written fake** that records every method call (channel, thread_ts, body) into a slice, then assert the count, ordering, and exact field values. Do not assert "an error did not occur" alone.
- **Persistence is part of the contract**: when a usecase writes to the repo (creates a ticket, appends a comment, updates status), the test must read it back via the repository interface and assert the stored fields — including foreign keys (`SlackUserID`, `IsBot`, timestamps where deterministic). Cover deduplication paths too (same Slack TS twice → exactly one row).
- **Negative paths still need assertions**: when the code is supposed to no-op (LLM not configured, channel unmapped, ticket not found), the test must assert that the dependency was *not* called — e.g. `t.Fatalf` inside the mock's `NewSessionFunc`, or `len(fake.calls) == 0` afterwards. "Returns nil error" is not enough.
- **Prompt / output content matters**: when the usecase builds a string for an external system (LLM prompt, Slack reply), assert the rendered content contains the expected ticket context, sanitization happened (e.g. `<@U…>` mention tokens stripped), and the downstream call received the model's exact output, not a paraphrase.
- Keep the test file colocated with the usecase as `*_test.go` in the `_test` package, and share a small `setup` / rig helper (see `setupTicketUseCase`, `newSlackTestRig`) so individual tests stay focused on the behavior under verification.

## Lifecycle / end-to-end tests

Per-method usecase tests are necessary but not sufficient. They verify each entry point in isolation, but they cannot catch state-machine bugs where the output of one entry point is the input to another (e.g. "submit invalidates the form because the propose_ask history shape doesn't match what HandleSubmit expects"). For any feature that spans multiple entry points / events, also write at least one test that drives the *full lifecycle* through the public API in order, with no mid-flight reaching into internals to set up state.

Concretely, a lifecycle test should:

- **Start from the real entry point.** For Slack-driven flows, that is the `HandleNewMessage` / `HandleAppMention` / interactivity handler — not a hand-rolled `repo.Ticket().Create` followed by `executor.Run`. Setting up intermediate state by hand defeats the purpose: the bug is usually in *how* intermediate state gets written.
- **Walk every observable transition.** For triage: ticket creation → planner posts an Ask → reporter clicks Submit → planner resumes → planner posts Complete. Assert at every hop: agent history shape after each step, Slack call ordering, and the final ticket fields (`Triaged`, `AssigneeID`).
- **Drive `async.Dispatch` deterministically.** Background work uses the package-level WaitGroup; tests must `async.Wait()` after each entry point call before asserting on side effects. Do not rely on `time.Sleep`.
- **Sequence the LLM mock by call count.** Use an `atomic.Int32` (or a slice of canned `Response`s) so call N returns `propose_ask`, call N+1 returns `propose_complete`, and any extra call fails the test. This is what catches "the planner kept looping after submit" or "submit didn't trigger a new planner turn".
- **Assert on persisted state, not only Slack output.** After Submit, the agent history must contain a user-role message with the formatted answers; after Complete, the ticket row must have `Triaged=true` and the right assignee. Slack-only assertions miss persistence regressions.

A useful rule of thumb: if you can imagine a refactor that changes which internal helper does which step but preserves external behavior, the lifecycle test should still pass. If your tests would all break, they're locked too tightly to the current call shape.

Place lifecycle tests in the test file paired with the orchestrating source file (e.g. tests for `pkg/usecase/triage/usecase.go`'s end-to-end flow live in `usecase_test.go`). The "Test file naming" rule above forbids carved-out files like `lifecycle_test.go` — keep the lifecycle test alongside the per-method tests of the same orchestrating type, prefixed with `TestLifecycle_...` so it is still trivially greppable.

## Frontend E2E tests (Playwright)

The Playwright suite under `frontend/e2e/tests/` is the only contract that verifies the UI actually works end-to-end. Run it with `task test:e2e`.

When you add or change a frontend feature, extend the E2E suite **in the same change**:

- One spec file per feature area (`prompts.spec.ts`, `ticket-crud.spec.ts`, `workspace-settings.spec.ts`, …). Add tests inside the matching file; do not invent ad-hoc files for one-off fixes.
- Cover at minimum: the happy path through the UI, the primary error / edge state surfaced by the feature (validation, conflict, empty), and any persistence side effect (cross-check via the HTTP API in the same test using the Playwright `request` fixture).
- Tests run serially against a single memory-backed server (`workers: 1`). Do NOT assume a fresh server between tests in the same file: read current state through the API at the start of the test and assert deltas (e.g. `version === before.version + 1`), not absolute numbers. Tests whose assertions only hold on a pristine server must guard with `test.skip(...)`.
- Use stable role / text selectors (`getByRole`, `getByText`) anchored on i18n strings from `frontend/src/i18n/en.ts`. Avoid CSS-class selectors — the Tailwind class soup is not a contract.
- All string literals in `frontend/e2e/tests/*.spec.ts` MUST be English, for the same reasons described in "String literals in tests" above (greppability, contributor friction). When asserting against UI text, use the `en` translation as the source of truth — never hardcode the Japanese rendering.
- Run `task test:e2e` and confirm green before reporting the task done. "tsc passes" + "Go tests pass" is **not** sufficient evidence the feature works in a browser.
