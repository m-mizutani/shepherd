package interfaces

//go:generate moq -out mocks/usecase_mock.go -pkg mocks . WebhookUseCase

import (
	"context"

	"github.com/m-mizutani/shepherd/pkg/domain/model"
)

// WebhookUseCase defines the interface for webhook event processing
type WebhookUseCase interface {
	// ProcessEvent processes a webhook event
	ProcessEvent(ctx context.Context, event *model.WebhookEvent) error
}
