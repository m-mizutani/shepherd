package ticket_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	tticket "github.com/m-mizutani/shepherd/pkg/tool/ticket"
)

func TestFactory_Prompt(t *testing.T) {
	f := tticket.New(nil, nil)
	got, err := f.Prompt(context.Background(), types.WorkspaceID(""))
	gt.NoError(t, err)
	gt.S(t, got).Contains("ticket_search")
	gt.S(t, got).Contains("ticket_get")
	gt.S(t, got).Contains("ticket_get_comments")
	gt.S(t, got).Contains("ticket_get_history")
}
