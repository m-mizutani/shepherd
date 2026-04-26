package triage

// Test seam re-exporting package-private symbols under *ForTest names so
// the *_test (external test) package can drive the planner state machine
// directly without us having to enlarge the production API surface.

import (
	"context"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

// TriagePlanSchemaForTest exposes the structured-output schema fed to the
// LLM via WithResponseSchema, so tests can assert it is valid.
func TriagePlanSchemaForTest() *gollem.Parameter { return triagePlanSchema() }

// DecodePlanFromJSONForTest exposes the JSON-to-TriagePlan decoder used on
// agent responses; tests use it to verify the round-trip from raw model
// output to a validated TriagePlan.
func DecodePlanFromJSONForTest(raw string) (*model.TriagePlan, error) {
	return decodePlanFromJSON(raw)
}

func PlanSessionIDForTest(ws types.WorkspaceID, id types.TicketID) string {
	return planSessionID(ws, id)
}

func SubtaskSessionIDForTest(ws types.WorkspaceID, id types.TicketID, sid types.SubtaskID) string {
	return subtaskSessionID(ws, id, sid)
}

func AppendUserMessageForTest(ctx context.Context, repo gollem.HistoryRepository, ws types.WorkspaceID, id types.TicketID, text string) error {
	return appendUserMessage(ctx, repo, ws, id, text)
}

func LoadLatestTriagePlanForTest(ctx context.Context, repo gollem.HistoryRepository, ws types.WorkspaceID, id types.TicketID) (*model.TriagePlan, error) {
	return loadLatestTriagePlan(ctx, repo, ws, id)
}

func IsWaitingUserSubmitForTest(ctx context.Context, repo gollem.HistoryRepository, ws types.WorkspaceID, id types.TicketID) (bool, error) {
	return isWaitingUserSubmit(ctx, repo, ws, id)
}

func CountPlannerTurnsForTest(ctx context.Context, repo gollem.HistoryRepository, ws types.WorkspaceID, id types.TicketID) (int, error) {
	return countPlannerTurns(ctx, repo, ws, id)
}

func HasPlanHistoryForTest(ctx context.Context, repo gollem.HistoryRepository, ws types.WorkspaceID, id types.TicketID) (bool, error) {
	return hasPlanHistory(ctx, repo, ws, id)
}

// RunForTest is the test-only entry point for PlanExecutor's planner loop.
func (e *PlanExecutor) RunForTest(ctx context.Context, ws types.WorkspaceID, id types.TicketID) error {
	return e.run(ctx, ws, id)
}
