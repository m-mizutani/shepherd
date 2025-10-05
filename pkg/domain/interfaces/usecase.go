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

// SourceCodeUseCase defines operations for source code event processing
type SourceCodeUseCase interface {
	// ProcessSource processes a source code event and downloads the source code
	ProcessSource(ctx context.Context, info *model.SourceInfo) (*model.DownloadResult, error)
}

// PackageDetectorUseCase defines operations for package update detection
type PackageDetectorUseCase interface {
	// DetectPackageUpdate processes a pull_request opened event and detects package updates
	DetectPackageUpdate(ctx context.Context, event *model.WebhookEvent) error
	// DetectFromPRInfo analyzes PR information using LLM and returns the detection result
	DetectFromPRInfo(ctx context.Context, prInfo *model.PRInfo) (*model.PackageUpdateDetection, error)
	// ExtractPackageVersionSources extracts package source code for before and after versions
	ExtractPackageVersionSources(ctx context.Context, detection *model.PackageUpdateDetection, prInfo *model.PRInfo) error
}
