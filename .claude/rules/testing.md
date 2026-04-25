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
