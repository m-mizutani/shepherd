// Package meta exposes ambient tools that describe the workspace itself or
// expose runtime information (current time) to the LLM.
package meta

import (
	"time"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
)

// Deps bundles dependencies for meta tools.
type Deps struct {
	Registry *model.WorkspaceRegistry
	Now      func() time.Time
}

// Tools returns every gollem.Tool exported from this package.
func Tools(d Deps) []gollem.Tool {
	now := d.Now
	if now == nil {
		now = time.Now
	}
	return []gollem.Tool{
		newWorkspaceDescribeTool(d.Registry),
		newCurrentTimeTool(now),
	}
}
