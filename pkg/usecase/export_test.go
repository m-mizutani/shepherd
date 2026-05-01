package usecase

import "github.com/m-mizutani/gollem"

// SetTicketUseCaseLLMForTest swaps the LLM client embedded in a TicketUseCase.
// The seam exists so lifecycle tests can sequence multiple distinct
// conclusion-generation runs against the same usecase instance without
// re-running the constructor (which would reset the in-memory repo too).
func SetTicketUseCaseLLMForTest(uc *TicketUseCase, llm gollem.LLMClient) {
	uc.llm = llm
}
