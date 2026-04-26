---
paths:
  - "**/*_test.go"
---

# Testing

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

Place lifecycle tests next to the orchestrating usecase (e.g. `pkg/usecase/triage/lifecycle_test.go`) rather than in the per-method files, so they are easy to find and clearly distinct from unit tests.
