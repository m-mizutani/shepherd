package github

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v75/github"
	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
)

// EventProcessor processes GitHub webhook events
type EventProcessor struct {
	releaseUC    interfaces.ReleaseUseCase
	sourceCodeUC interfaces.SourceCodeUseCase
}

// NewEventProcessor creates a new GitHub event processor
func NewEventProcessor(releaseUC interfaces.ReleaseUseCase, sourceCodeUC interfaces.SourceCodeUseCase) *EventProcessor {
	return &EventProcessor{
		releaseUC:    releaseUC,
		sourceCodeUC: sourceCodeUC,
	}
}

// ProcessEvent processes a GitHub webhook event
func (p *EventProcessor) ProcessEvent(ctx context.Context, eventType string, payload interface{}) error {
	logger := ctxlog.From(ctx)

	switch eventType {
	case "release":
		return p.processReleaseEvent(ctx, payload)
	case "push":
		return p.processPushEvent(ctx, payload)
	default:
		logger.Info("Ignoring unsupported event type", "event_type", eventType)
		return nil
	}
}

// processReleaseEvent processes a GitHub release event
func (p *EventProcessor) processReleaseEvent(ctx context.Context, payload interface{}) error {
	logger := ctxlog.From(ctx)

	releaseEvent, ok := payload.(*github.ReleaseEvent)
	if !ok {
		logger.Warn("Invalid release event payload")
		return nil
	}

	// Only process "released" action
	if releaseEvent.Action == nil || *releaseEvent.Action != "released" {
		logger.Info("Ignoring release event with non-released action",
			"action", releaseEvent.GetAction(),
		)
		return nil
	}

	// Extract release information
	releaseInfo, err := p.extractReleaseInfo(releaseEvent)
	if err != nil {
		logger.Error("Failed to extract release info", "error", err)
		return err
	}

	logger.Info("Processing release event",
		"owner", releaseInfo.Owner,
		"repo", releaseInfo.Repo,
		"tag", releaseInfo.TagName,
		"commit_sha", releaseInfo.CommitSHA,
	)

	// Process release through use case
	result, err := p.releaseUC.ProcessRelease(ctx, releaseInfo)
	if err != nil {
		logger.Error("Failed to process release", "error", err,
			"owner", releaseInfo.Owner,
			"repo", releaseInfo.Repo,
		)
		return err
	}

	// Clean up temporary directory after processing
	defer func() {
		if result != nil && result.TempDir != "" {
			if removeErr := os.RemoveAll(result.TempDir); removeErr != nil {
				logger.Warn("Failed to clean up temporary directory",
					"temp_dir", result.TempDir,
					"error", removeErr,
				)
			} else {
				logger.Debug("Cleaned up temporary directory", "temp_dir", result.TempDir)
			}
		}
	}()

	logger.Info("Successfully processed release",
		"owner", releaseInfo.Owner,
		"repo", releaseInfo.Repo,
		"temp_dir", result.TempDir,
		"file_count", len(result.Files),
		"total_size", result.Size,
	)

	return nil
}

// extractReleaseInfo extracts release information from a GitHub release event
func (p *EventProcessor) extractReleaseInfo(event *github.ReleaseEvent) (*model.ReleaseInfo, error) {
	if event.GetRepo() == nil {
		return nil, fmt.Errorf("missing repository information in release event")
	}

	if event.GetRelease() == nil {
		return nil, fmt.Errorf("missing release information in release event")
	}

	// Use Get*() helper methods for concise and nil-safe field access
	owner := event.GetRepo().GetOwner().GetLogin()
	repo := event.GetRepo().GetName()
	tagName := event.GetRelease().GetTagName()
	releaseName := event.GetRelease().GetName()
	commitSHA := event.GetRelease().GetTargetCommitish()

	if owner == "" || repo == "" || commitSHA == "" {
		return nil, fmt.Errorf("missing required fields: owner=%s, repo=%s, commit_sha=%s", owner, repo, commitSHA)
	}

	return &model.ReleaseInfo{
		Owner:       owner,
		Repo:        repo,
		CommitSHA:   commitSHA,
		TagName:     tagName,
		ReleaseName: releaseName,
	}, nil
}

// processPushEvent processes a GitHub push event
func (p *EventProcessor) processPushEvent(ctx context.Context, payload interface{}) error {
	logger := ctxlog.From(ctx)

	pushEvent, ok := payload.(*github.PushEvent)
	if !ok {
		logger.Warn("Invalid push event payload")
		return nil
	}

	// Extract push information
	sourceInfo, err := p.extractPushInfo(pushEvent)
	if err != nil {
		logger.Error("Failed to extract push info", "error", err)
		return err
	}

	logger.Info("Processing push event",
		"owner", sourceInfo.Owner,
		"repo", sourceInfo.Repo,
		"ref", sourceInfo.Ref,
		"commit_sha", sourceInfo.CommitSHA,
	)

	// Process push through use case
	result, err := p.sourceCodeUC.ProcessSource(ctx, sourceInfo)
	if err != nil {
		logger.Error("Failed to process push", "error", err,
			"owner", sourceInfo.Owner,
			"repo", sourceInfo.Repo,
		)
		return err
	}

	// Clean up temporary directory after processing
	defer func() {
		if result != nil && result.TempDir != "" {
			if removeErr := os.RemoveAll(result.TempDir); removeErr != nil {
				logger.Warn("Failed to clean up temporary directory",
					"temp_dir", result.TempDir,
					"error", removeErr,
				)
			} else {
				logger.Debug("Cleaned up temporary directory", "temp_dir", result.TempDir)
			}
		}
	}()

	logger.Info("Successfully processed push",
		"owner", sourceInfo.Owner,
		"repo", sourceInfo.Repo,
		"temp_dir", result.TempDir,
		"file_count", len(result.Files),
		"total_size", result.Size,
	)

	return nil
}

// extractPushInfo extracts source information from a GitHub push event
func (p *EventProcessor) extractPushInfo(event *github.PushEvent) (*model.SourceInfo, error) {
	if event.GetRepo() == nil {
		return nil, fmt.Errorf("missing repository information in push event")
	}

	// Use Get*() helper methods for concise and nil-safe field access
	owner := event.GetRepo().GetOwner().GetLogin()
	repo := event.GetRepo().GetName()
	ref := event.GetRef()
	commitSHA := event.GetHeadCommit().GetID()
	pusher := event.GetPusher().GetName()

	if owner == "" || repo == "" || commitSHA == "" {
		return nil, fmt.Errorf("missing required fields: owner=%s, repo=%s, commit_sha=%s", owner, repo, commitSHA)
	}

	metadata := make(map[string]string)
	if event.Before != nil {
		metadata["before"] = *event.Before
	}
	if event.Created != nil {
		metadata["created"] = fmt.Sprintf("%v", *event.Created)
	}
	if event.Deleted != nil {
		metadata["deleted"] = fmt.Sprintf("%v", *event.Deleted)
	}

	return &model.SourceInfo{
		Owner:     owner,
		Repo:      repo,
		CommitSHA: commitSHA,
		EventType: "push",
		Ref:       ref,
		Actor:     pusher,
		Metadata:  metadata,
	}, nil
}