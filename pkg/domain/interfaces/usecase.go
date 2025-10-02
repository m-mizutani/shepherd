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

// ReleaseUseCase defines operations for release event processing
type ReleaseUseCase interface {
	// ProcessRelease processes a release event and downloads the source code
	ProcessRelease(ctx context.Context, info *model.ReleaseInfo) (*model.DownloadResult, error)
}
