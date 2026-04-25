// Package ticket exposes gollem.Tool implementations for reading tickets,
// their comments, and their status history out of the workspace repository.
// All tools require a workspace ID bound to the context via
// types.ContextWithWorkspace.
package ticket

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

// Deps bundles the repository tools depend on.
type Deps struct {
	Repo interfaces.Repository
}

// Tools returns every gollem.Tool exported from this package.
func Tools(d Deps) []gollem.Tool {
	return []gollem.Tool{
		newSearchTool(d.Repo),
		newGetTool(d.Repo),
		newGetCommentsTool(d.Repo),
		newGetHistoryTool(d.Repo),
	}
}

// workspaceFromCtx returns the active workspace ID, erroring out cleanly when
// the caller forgot to bind one. The error message is structured for the LLM.
func workspaceFromCtx(ctx context.Context) (types.WorkspaceID, error) {
	id, ok := types.WorkspaceFromContext(ctx)
	if !ok || id == "" {
		return "", goerr.New("no active workspace bound to context")
	}
	return id, nil
}

func ptrInt(n int) *int { return &n }

func stringArg(args map[string]any, key string, required bool) (string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		if required {
			return "", goerr.New("missing required argument", goerr.V("argument", key))
		}
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", goerr.New("argument is not a string", goerr.V("argument", key))
	}
	if required && s == "" {
		return "", goerr.New("argument is empty", goerr.V("argument", key))
	}
	return s, nil
}

func intArg(args map[string]any, key string) int {
	v, ok := args[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

func int64Arg(args map[string]any, key string) (int64, bool) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	}
	return 0, false
}

func stringSliceArg(args map[string]any, key string) []string {
	v, ok := args[key]
	if !ok || v == nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
