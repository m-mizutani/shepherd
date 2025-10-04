package github_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/go-github/v75/github"
	"github.com/m-mizutani/gt"

	githubcontroller "github.com/m-mizutani/shepherd/pkg/controller/github"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
)

// MockReleaseUseCase is a mock implementation of ReleaseUseCase
type MockReleaseUseCase struct {
	processReleaseFunc func(ctx context.Context, releaseInfo *model.ReleaseInfo) (*model.DownloadResult, error)
	processCalls       []MockReleaseCall
}

type MockReleaseCall struct {
	ReleaseInfo *model.ReleaseInfo
}

func (m *MockReleaseUseCase) ProcessRelease(ctx context.Context, releaseInfo *model.ReleaseInfo) (*model.DownloadResult, error) {
	m.processCalls = append(m.processCalls, MockReleaseCall{ReleaseInfo: releaseInfo})
	if m.processReleaseFunc != nil {
		return m.processReleaseFunc(ctx, releaseInfo)
	}
	return nil, errors.New("mock not configured")
}

// MockSourceCodeUseCase is a mock implementation of SourceCodeUseCase
type MockSourceCodeUseCase struct {
	processSourceFunc func(ctx context.Context, sourceInfo *model.SourceInfo) (*model.DownloadResult, error)
	processCalls      []MockSourceCall
}

type MockSourceCall struct {
	SourceInfo *model.SourceInfo
}

func (m *MockSourceCodeUseCase) ProcessSource(ctx context.Context, sourceInfo *model.SourceInfo) (*model.DownloadResult, error) {
	m.processCalls = append(m.processCalls, MockSourceCall{SourceInfo: sourceInfo})
	if m.processSourceFunc != nil {
		return m.processSourceFunc(ctx, sourceInfo)
	}
	return nil, errors.New("mock not configured")
}

func TestEventProcessor_ProcessReleaseEvent_CleanupTempDir(t *testing.T) {
	ctx := context.Background()

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "shepherd-test-*")
	gt.NoError(t, err)

	// Ensure temp dir exists before test
	_, err = os.Stat(tempDir)
	gt.NoError(t, err)

	// Setup mock use case
	mockUC := &MockReleaseUseCase{
		processReleaseFunc: func(ctx context.Context, releaseInfo *model.ReleaseInfo) (*model.DownloadResult, error) {
			return &model.DownloadResult{
				TempDir: tempDir,
				Files:   []string{"test.txt"},
				Size:    100,
			}, nil
		},
	}

	// Create event processor
	processor := githubcontroller.NewEventProcessor(mockUC, nil)

	// Create test release event
	action := "released"
	owner := "test-owner"
	repo := "test-repo"
	tagName := "v1.0.0"
	releaseName := "Test Release"
	commitSHA := "abc123"

	releaseEvent := &github.ReleaseEvent{
		Action: &action,
		Repo: &github.Repository{
			Owner: &github.User{Login: &owner},
			Name:  &repo,
		},
		Release: &github.RepositoryRelease{
			TagName:         &tagName,
			Name:            &releaseName,
			TargetCommitish: &commitSHA,
		},
	}

	// Process the event
	err = processor.ProcessEvent(ctx, "release", releaseEvent)
	gt.NoError(t, err)

	// Verify that the temporary directory has been cleaned up
	_, err = os.Stat(tempDir)
	gt.Value(t, os.IsNotExist(err)).Equal(true) // Directory should not exist after cleanup

	// Verify mock was called
	gt.Number(t, len(mockUC.processCalls)).Equal(1)
	gt.Value(t, mockUC.processCalls[0].ReleaseInfo.Owner).Equal("test-owner")
	gt.Value(t, mockUC.processCalls[0].ReleaseInfo.Repo).Equal("test-repo")
	gt.Value(t, mockUC.processCalls[0].ReleaseInfo.CommitSHA).Equal("abc123")
}

func TestEventProcessor_ProcessReleaseEvent_Error(t *testing.T) {
	ctx := context.Background()

	// Setup mock use case that returns error
	mockUC := &MockReleaseUseCase{
		processReleaseFunc: func(ctx context.Context, releaseInfo *model.ReleaseInfo) (*model.DownloadResult, error) {
			return nil, errors.New("processing failed")
		},
	}

	// Create event processor
	processor := githubcontroller.NewEventProcessor(mockUC, nil)

	// Create test release event
	action := "released"
	owner := "test-owner"
	repo := "test-repo"
	tagName := "v1.0.0"
	releaseName := "Test Release"
	commitSHA := "abc123"

	releaseEvent := &github.ReleaseEvent{
		Action: &action,
		Repo: &github.Repository{
			Owner: &github.User{Login: &owner},
			Name:  &repo,
		},
		Release: &github.RepositoryRelease{
			TagName:         &tagName,
			Name:            &releaseName,
			TargetCommitish: &commitSHA,
		},
	}

	// Process the event - should return error
	err := processor.ProcessEvent(ctx, "release", releaseEvent)
	gt.Error(t, err)
	gt.String(t, err.Error()).Contains("processing failed")

	// Verify mock was called
	gt.Number(t, len(mockUC.processCalls)).Equal(1)
}

func TestEventProcessor_ProcessEvent_UnsupportedEventType(t *testing.T) {
	ctx := context.Background()

	// Setup mock use case
	mockUC := &MockReleaseUseCase{}

	// Create event processor
	processor := githubcontroller.NewEventProcessor(mockUC, nil)

	// Process unsupported event type
	err := processor.ProcessEvent(ctx, "issues", nil)
	gt.NoError(t, err)

	// Verify mock was not called
	gt.Number(t, len(mockUC.processCalls)).Equal(0)
}

func TestEventProcessor_ProcessPushEvent(t *testing.T) {
	ctx := context.Background()

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "shepherd-test-*")
	gt.NoError(t, err)

	// Setup mock use case
	mockSourceUC := &MockSourceCodeUseCase{
		processSourceFunc: func(ctx context.Context, sourceInfo *model.SourceInfo) (*model.DownloadResult, error) {
			return &model.DownloadResult{
				TempDir: tempDir,
				Files:   []string{"test.txt"},
				Size:    100,
			}, nil
		},
	}

	// Create event processor
	processor := githubcontroller.NewEventProcessor(nil, mockSourceUC)

	// Create test push event
	owner := "test-owner"
	repo := "test-repo"
	ref := "refs/heads/main"
	commitSHA := "abc123"
	pusher := "test-user"
	before := "def456"

	pushEvent := &github.PushEvent{
		Ref: &ref,
		Before: &before,
		Repo: &github.PushEventRepository{
			Owner: &github.User{Login: &owner},
			Name:  &repo,
		},
		HeadCommit: &github.HeadCommit{
			ID: &commitSHA,
		},
		Pusher: &github.CommitAuthor{
			Name: &pusher,
		},
	}

	// Process the event
	err = processor.ProcessEvent(ctx, "push", pushEvent)
	gt.NoError(t, err)

	// Verify that the temporary directory has been cleaned up
	_, err = os.Stat(tempDir)
	gt.Value(t, os.IsNotExist(err)).Equal(true)

	// Verify mock was called with correct parameters
	gt.Number(t, len(mockSourceUC.processCalls)).Equal(1)
	gt.Value(t, mockSourceUC.processCalls[0].SourceInfo.Owner).Equal("test-owner")
	gt.Value(t, mockSourceUC.processCalls[0].SourceInfo.Repo).Equal("test-repo")
	gt.Value(t, mockSourceUC.processCalls[0].SourceInfo.CommitSHA).Equal("abc123")
	gt.Value(t, mockSourceUC.processCalls[0].SourceInfo.EventType).Equal("push")
	gt.Value(t, mockSourceUC.processCalls[0].SourceInfo.Ref).Equal("refs/heads/main")
	gt.Value(t, mockSourceUC.processCalls[0].SourceInfo.Actor).Equal("test-user")
}

func TestEventProcessor_ExtractPushInfo_MissingFields(t *testing.T) {
	ctx := context.Background()

	// Setup mock use case that should not be called
	mockSourceUC := &MockSourceCodeUseCase{}

	// Create event processor
	processor := githubcontroller.NewEventProcessor(nil, mockSourceUC)

	// Create push event with missing repository
	pushEvent := &github.PushEvent{
		Repo: nil,
	}

	// Process the event - should return error
	err := processor.ProcessEvent(ctx, "push", pushEvent)
	gt.Error(t, err)

	// Verify mock was not called
	gt.Number(t, len(mockSourceUC.processCalls)).Equal(0)
}

func TestEventProcessor_ExtractPushInfo_NilHeadCommit(t *testing.T) {
	ctx := context.Background()

	// Setup mock use case that should not be called
	mockSourceUC := &MockSourceCodeUseCase{}

	// Create event processor
	processor := githubcontroller.NewEventProcessor(nil, mockSourceUC)

	// Create push event with nil head_commit (branch/tag deletion scenario)
	owner := "test-owner"
	repo := "test-repo"
	ref := "refs/heads/deleted-branch"

	pushEvent := &github.PushEvent{
		Ref: &ref,
		Repo: &github.PushEventRepository{
			Owner: &github.User{Login: &owner},
			Name:  &repo,
		},
		HeadCommit: nil, // Nil when branch/tag is deleted
	}

	// Process the event - should return error
	err := processor.ProcessEvent(ctx, "push", pushEvent)
	gt.Error(t, err)
	gt.String(t, err.Error()).Contains("missing head commit")

	// Verify mock was not called
	gt.Number(t, len(mockSourceUC.processCalls)).Equal(0)
}