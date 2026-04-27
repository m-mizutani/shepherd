package repository_test

import (
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

// freshWS returns a workspace id unique across test executions.
// Note: the codebase elsewhere truncates UUID v7 to the first 8 hex chars,
// but those chars are the upper 32 bits of a millisecond timestamp and only
// change every ~65 seconds — they collide when the same test runs twice in
// quick succession (e.g. `go test -count=2`). Use the full UUID here.
func freshWS(prefix string) types.WorkspaceID {
	return types.WorkspaceID(prefix + "-" + uuid.Must(uuid.NewV7()).String())
}

func TestPromptAppendAssignsMonotonicVersions(t *testing.T) {
	runTest(t, "PromptAppendAssignsMonotonicVersions", func(t *testing.T, repo interfaces.Repository) {
		ws := freshWS("test-prompt-mono")
		c := ctx(t)

		v1 := gt.R1(repo.Prompt().Append(c, ws, model.PromptIDTriage, &model.PromptVersion{
			Version:   1,
			Content:   "v1",
			UpdatedAt: time.Now().Truncate(time.Millisecond),
			UpdatedBy: "alice",
		})).NoError(t)
		gt.Equal(t, v1.Version, 1)

		v2 := gt.R1(repo.Prompt().Append(c, ws, model.PromptIDTriage, &model.PromptVersion{
			Version:   2,
			Content:   "v2",
			UpdatedAt: time.Now().Truncate(time.Millisecond),
			UpdatedBy: "alice",
		})).NoError(t)
		gt.Equal(t, v2.Version, 2)

		cur := gt.R1(repo.Prompt().GetCurrent(c, ws, model.PromptIDTriage)).NoError(t)
		gt.Equal(t, cur.Version, 2)
		gt.Equal(t, cur.Content, "v2")
	})
}

// TestPromptAppendDetectsConflictOnSameVersion is the repository-level
// counterpart of the WebUI scenario "two users edit, the second saver loses".
// Both writers try to claim the same Version; the second must be rejected
// AND the loser's content must not leak into the persisted state — verified
// across GetCurrent / GetVersion / List so we catch any partial-write or
// silent-overwrite regression.
func TestPromptAppendDetectsConflictOnSameVersion(t *testing.T) {
	runTest(t, "PromptAppendDetectsConflictOnSameVersion", func(t *testing.T, repo interfaces.Repository) {
		ws := freshWS("test-prompt-same-ver")
		c := ctx(t)

		// Winner.
		winner := gt.R1(repo.Prompt().Append(c, ws, model.PromptIDTriage, &model.PromptVersion{
			Version: 1, Content: "first", UpdatedBy: "alice",
		})).NoError(t)
		gt.Equal(t, winner.Version, 1)
		gt.Equal(t, winner.Content, "first")

		// Loser races on the same Version. Must be rejected.
		_, err := repo.Prompt().Append(c, ws, model.PromptIDTriage, &model.PromptVersion{
			Version: 1, Content: "racer", UpdatedBy: "bob",
		})
		gt.Error(t, err)
		gt.True(t, errors.Is(err, interfaces.ErrPromptVersionConflict))

		// Persisted state: only the winner's data exists.
		cur := gt.R1(repo.Prompt().GetCurrent(c, ws, model.PromptIDTriage)).NoError(t)
		gt.Equal(t, cur.Version, 1)
		gt.Equal(t, cur.Content, "first")
		gt.Equal(t, cur.UpdatedBy, "alice")

		v1 := gt.R1(repo.Prompt().GetVersion(c, ws, model.PromptIDTriage, 1)).NoError(t)
		gt.Equal(t, v1.Content, "first")
		gt.Equal(t, v1.UpdatedBy, "alice")

		list := gt.R1(repo.Prompt().List(c, ws, model.PromptIDTriage)).NoError(t)
		gt.Equal(t, len(list), 1)
		gt.Equal(t, list[0].Content, "first")
	})
}

// TestPromptAppendSequentialStaleVersionRejected mirrors the WebUI scenario
// directly: a writer reads current, a second writer commits, then the first
// writer (still holding the stale current snapshot) tries to write the
// version it had originally planned. Must be rejected — and again, the
// rejected content must not appear anywhere in persisted state.
func TestPromptAppendSequentialStaleVersionRejected(t *testing.T) {
	runTest(t, "PromptAppendSequentialStaleVersionRejected", func(t *testing.T, repo interfaces.Repository) {
		ws := freshWS("test-prompt-stale")
		c := ctx(t)

		// Both A and B "see" current=0 here (no override yet); both intend to
		// write Version=1.

		// A wins.
		gt.R1(repo.Prompt().Append(c, ws, model.PromptIDTriage, &model.PromptVersion{
			Version: 1, Content: "A wins", UpdatedBy: "alice",
		})).NoError(t)

		// B, still convinced no override exists, tries Version=1 — stale.
		_, err := repo.Prompt().Append(c, ws, model.PromptIDTriage, &model.PromptVersion{
			Version: 1, Content: "B loses", UpdatedBy: "bob",
		})
		gt.Error(t, err)
		gt.True(t, errors.Is(err, interfaces.ErrPromptVersionConflict))

		// Even though B then refreshes and tries Version=2 (which is the new
		// current+1), the loser content from the conflicting attempt must NOT
		// have been persisted as v1 nor leaked into v2's payload.
		v2 := gt.R1(repo.Prompt().Append(c, ws, model.PromptIDTriage, &model.PromptVersion{
			Version: 2, Content: "B retries", UpdatedBy: "bob",
		})).NoError(t)
		gt.Equal(t, v2.Version, 2)
		gt.Equal(t, v2.Content, "B retries")

		v1 := gt.R1(repo.Prompt().GetVersion(c, ws, model.PromptIDTriage, 1)).NoError(t)
		gt.Equal(t, v1.Content, "A wins")
		gt.Equal(t, v1.UpdatedBy, "alice")

		list := gt.R1(repo.Prompt().List(c, ws, model.PromptIDTriage)).NoError(t)
		gt.Equal(t, len(list), 2)
		// "B loses" must not appear in any persisted version.
		for _, v := range list {
			if v.Content == "B loses" {
				t.Fatalf("rejected content leaked into v%d", v.Version)
			}
		}
	})
}

// PromptAppendDetectsSkipAhead is the regression test for the previously-
// broken Firestore path: a caller that submits a Version higher than
// current+1 must be rejected as a conflict, NOT silently create a
// version-number gap.
func TestPromptAppendDetectsSkipAhead(t *testing.T) {
	runTest(t, "PromptAppendDetectsSkipAhead", func(t *testing.T, repo interfaces.Repository) {
		ws := freshWS("test-prompt-skip")
		c := ctx(t)

		gt.R1(repo.Prompt().Append(c, ws, model.PromptIDTriage, &model.PromptVersion{Version: 1, Content: "v1"})).NoError(t)

		_, err := repo.Prompt().Append(c, ws, model.PromptIDTriage, &model.PromptVersion{Version: 11, Content: "leap"})
		gt.Error(t, err)
		gt.True(t, errors.Is(err, interfaces.ErrPromptVersionConflict))

		// And no leap-version was persisted.
		list := gt.R1(repo.Prompt().List(c, ws, model.PromptIDTriage)).NoError(t)
		gt.Equal(t, len(list), 1)
		gt.Equal(t, list[0].Version, 1)
	})
}

func TestPromptAppendInitialNonOneRejected(t *testing.T) {
	runTest(t, "PromptAppendInitialNonOneRejected", func(t *testing.T, repo interfaces.Repository) {
		ws := freshWS("test-prompt-init")
		c := ctx(t)

		// No versions exist yet; only Version=1 is valid for the first append.
		_, err := repo.Prompt().Append(c, ws, model.PromptIDTriage, &model.PromptVersion{Version: 5, Content: "x"})
		gt.Error(t, err)
		gt.True(t, errors.Is(err, interfaces.ErrPromptVersionConflict))

		list := gt.R1(repo.Prompt().List(c, ws, model.PromptIDTriage)).NoError(t)
		gt.Equal(t, len(list), 0)
	})
}

func TestPromptListAscending(t *testing.T) {
	runTest(t, "PromptListAscending", func(t *testing.T, repo interfaces.Repository) {
		ws := freshWS("test-prompt-list")
		c := ctx(t)

		for i, content := range []string{"a", "b", "c"} {
			gt.R1(repo.Prompt().Append(c, ws, model.PromptIDTriage, &model.PromptVersion{
				Version:   i + 1,
				Content:   content,
				UpdatedAt: time.Now().Truncate(time.Millisecond),
			})).NoError(t)
		}

		list := gt.R1(repo.Prompt().List(c, ws, model.PromptIDTriage)).NoError(t)
		gt.Equal(t, len(list), 3)
		for i, v := range list {
			gt.Equal(t, v.Version, i+1)
		}
	})
}

func TestPromptGetCurrentEmpty(t *testing.T) {
	runTest(t, "PromptGetCurrentEmpty", func(t *testing.T, repo interfaces.Repository) {
		ws := freshWS("test-prompt-empty")
		got, err := repo.Prompt().GetCurrent(ctx(t), ws, model.PromptIDTriage)
		gt.NoError(t, err)
		gt.Nil(t, got)
	})
}

func TestPromptGetVersionNotFound(t *testing.T) {
	runTest(t, "PromptGetVersionNotFound", func(t *testing.T, repo interfaces.Repository) {
		ws := freshWS("test-prompt-getv")
		c := ctx(t)

		gt.R1(repo.Prompt().Append(c, ws, model.PromptIDTriage, &model.PromptVersion{Version: 1, Content: "v1"})).NoError(t)

		_, err := repo.Prompt().GetVersion(c, ws, model.PromptIDTriage, 99)
		gt.Error(t, err)
		gt.True(t, errors.Is(err, interfaces.ErrPromptVersionNotFound))
	})
}

func TestPromptIsolatedAcrossWorkspaceAndID(t *testing.T) {
	runTest(t, "PromptIsolatedAcrossWorkspaceAndID", func(t *testing.T, repo interfaces.Repository) {
		c := ctx(t)
		wsA := freshWS("test-prompt-iso-a")
		wsB := freshWS("test-prompt-iso-b")

		gt.R1(repo.Prompt().Append(c, wsA, model.PromptIDTriage, &model.PromptVersion{Version: 1, Content: "a-triage"})).NoError(t)
		gt.R1(repo.Prompt().Append(c, wsB, model.PromptIDTriage, &model.PromptVersion{Version: 1, Content: "b-triage"})).NoError(t)
		gt.R1(repo.Prompt().Append(c, wsA, model.PromptID("future"), &model.PromptVersion{Version: 1, Content: "a-future"})).NoError(t)

		a := gt.R1(repo.Prompt().GetCurrent(c, wsA, model.PromptIDTriage)).NoError(t)
		gt.Equal(t, a.Content, "a-triage")
		gt.Equal(t, a.Version, 1)

		b := gt.R1(repo.Prompt().GetCurrent(c, wsB, model.PromptIDTriage)).NoError(t)
		gt.Equal(t, b.Content, "b-triage")
		gt.Equal(t, b.Version, 1)
	})
}

// Concurrent writers must end up with exactly one success and N-1 conflicts,
// and the surviving version must carry exactly one writer's content (no
// torn / merged state). Firestore's RunTransaction will retry transparently
// on contention, so even when many writers race the strict invariant is
// "one wins per (ws, id), losers do not leak into persisted state".
func TestPromptAppendConcurrent(t *testing.T) {
	runTest(t, "PromptAppendConcurrent", func(t *testing.T, repo interfaces.Repository) {
		ws := freshWS("test-prompt-concurrent")
		c := ctx(t)

		const writers = 6
		results := make([]error, writers)
		contents := make([]string, writers)
		for i := range writers {
			contents[i] = "racer-" + strconv.Itoa(i)
		}
		var wg sync.WaitGroup
		for i := range writers {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, err := repo.Prompt().Append(c, ws, model.PromptIDTriage, &model.PromptVersion{
					Version:   1,
					Content:   contents[i],
					UpdatedAt: time.Now().Truncate(time.Millisecond),
				})
				results[i] = err
			}(i)
		}
		wg.Wait()

		successes := 0
		conflicts := 0
		for _, err := range results {
			switch {
			case err == nil:
				successes++
			case errors.Is(err, interfaces.ErrPromptVersionConflict):
				conflicts++
			default:
				t.Fatalf("unexpected error: %v", err)
			}
		}
		gt.Equal(t, successes, 1)
		gt.Equal(t, conflicts, writers-1)

		list := gt.R1(repo.Prompt().List(c, ws, model.PromptIDTriage)).NoError(t)
		gt.Equal(t, len(list), 1)

		// The single survivor must be exactly one of the writers' content,
		// not a merge / partial / blank.
		matched := false
		for _, c := range contents {
			if list[0].Content == c {
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("surviving content %q does not match any writer's input", list[0].Content)
		}
	})
}
