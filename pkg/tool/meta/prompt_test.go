package meta_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	tmeta "github.com/m-mizutani/shepherd/pkg/tool/meta"
)

func TestFactory_Prompt(t *testing.T) {
	f := tmeta.New(nil, nil)
	got, err := f.Prompt(context.Background(), types.WorkspaceID(""))
	gt.NoError(t, err)
	gt.S(t, got).Contains("current_time")
	gt.S(t, got).Contains("workspace_describe")
}
