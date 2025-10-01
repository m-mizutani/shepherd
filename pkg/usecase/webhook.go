package usecase

import (
	"context"

	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
)

type webhookUseCase struct{}

// NewWebhook creates a new instance of WebhookUseCase
func NewWebhook() *webhookUseCase {
	return &webhookUseCase{}
}

// ProcessEvent processes a webhook event
// Current implementation only logs the event
func (uc *webhookUseCase) ProcessEvent(ctx context.Context, event *model.WebhookEvent) error {
	logger := ctxlog.From(ctx)

	logger.Info("Processing webhook event",
		"id", event.ID,
		"type", event.Type,
		"action", event.Action,
		"repository", event.Repository,
		"sender", event.Sender,
		"supported", event.IsSupportedEvent(),
	)

	if !event.IsSupportedEvent() {
		logger.Warn("Unsupported event received",
			"type", event.Type,
			"action", event.Action,
		)
	}

	return nil
}
