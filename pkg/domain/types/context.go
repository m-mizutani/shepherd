package types

import "context"

type workspaceContextKey struct{}

// ContextWithWorkspace binds a WorkspaceID to the context. Tools executed in a
// scope where the active workspace is known should call this so downstream
// helpers can recover the ID via WorkspaceFromContext.
func ContextWithWorkspace(ctx context.Context, id WorkspaceID) context.Context {
	return context.WithValue(ctx, workspaceContextKey{}, id)
}

// WorkspaceFromContext returns the WorkspaceID previously bound by
// ContextWithWorkspace. The second return value is false when no workspace is
// bound.
func WorkspaceFromContext(ctx context.Context) (WorkspaceID, bool) {
	v, ok := ctx.Value(workspaceContextKey{}).(WorkspaceID)
	return v, ok
}
