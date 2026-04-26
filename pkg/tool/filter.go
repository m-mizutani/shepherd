package tool

import "github.com/m-mizutani/gollem"

// FilterByName narrows a tool slice down to entries whose Spec().Name appears
// in names. Used by triage subtasks to hand each child agent only the tools
// the planner whitelisted for that subtask, while still drawing from the
// workspace-scoped Catalog set.
//
// Order is preserved relative to tools (not names). Names that do not match
// any tool are silently ignored — enforcing exact membership is the caller's
// responsibility, since some allowed_tools entries may belong to providers
// disabled in the workspace.
func FilterByName(tools []gollem.Tool, names []string) []gollem.Tool {
	if len(tools) == 0 || len(names) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(names))
	for _, n := range names {
		allowed[n] = struct{}{}
	}
	out := make([]gollem.Tool, 0, len(tools))
	for _, t := range tools {
		if _, ok := allowed[t.Spec().Name]; ok {
			out = append(out, t)
		}
	}
	return out
}
