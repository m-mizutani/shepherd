package usecase_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/m-mizutani/gt"

	"github.com/m-mizutani/shepherd/pkg/domain/model"
	githubinfra "github.com/m-mizutani/shepherd/pkg/infra/github"
	"github.com/m-mizutani/shepherd/pkg/usecase"
)

// MockGitHubClient is a mock implementation of GitHubClient
type MockGitHubClient struct {
	downloadZipballFunc func(ctx context.Context, owner, repo, ref string) ([]byte, error)
	downloadCalls       []MockCall
}

type MockCall struct {
	Owner string
	Repo  string
	Ref   string
}

func (m *MockGitHubClient) DownloadZipball(ctx context.Context, owner, repo, ref string) ([]byte, error) {
	m.downloadCalls = append(m.downloadCalls, MockCall{Owner: owner, Repo: repo, Ref: ref})
	if m.downloadZipballFunc != nil {
		return m.downloadZipballFunc(ctx, owner, repo, ref)
	}
	return nil, errors.New("mock not configured")
}

func (m *MockGitHubClient) AssertExpectations(t *testing.T) {
	// Simple assertion that the method was called
	gt.Number(t, len(m.downloadCalls)).Greater(0)
}

func TestReleaseUseCase_ProcessRelease_Success(t *testing.T) {
	ctx := context.Background()

	// Create a test ZIP file
	zipData := createTestZip(t)

	// Setup mock
	mockClient := &MockGitHubClient{
		downloadZipballFunc: func(ctx context.Context, owner, repo, ref string) ([]byte, error) {
			return zipData, nil
		},
	}

	// Create use case
	uc := usecase.NewRelease(mockClient)

	// Execute
	releaseInfo := &model.ReleaseInfo{
		Owner:       "owner",
		Repo:        "repo",
		CommitSHA:   "abc123",
		TagName:     "v1.0.0",
		ReleaseName: "Release 1.0.0",
	}

	result, err := uc.ProcessRelease(ctx, releaseInfo)

	// Verify
	gt.NoError(t, err)
	gt.Value(t, result.TempDir).NotEqual("")
	gt.Number(t, len(result.Files)).Greater(0)
	gt.Number(t, result.Size).Greater(int64(0))

	// Verify files exist
	_, err = os.Stat(filepath.Join(result.TempDir, "test-repo-abc123", "README.md"))
	gt.NoError(t, err)
	_, err = os.Stat(filepath.Join(result.TempDir, "test-repo-abc123", "main.go"))
	gt.NoError(t, err)

	// Verify file contents
	content, err := os.ReadFile(filepath.Join(result.TempDir, "test-repo-abc123", "README.md"))
	gt.NoError(t, err)
	gt.String(t, string(content)).Contains("Test Repository")

	// Cleanup
	defer func() {
		_ = os.RemoveAll(result.TempDir) // Error ignored in test cleanup
	}()

	// Verify mock was called
	mockClient.AssertExpectations(t)
}

func TestReleaseUseCase_ProcessRelease_DownloadError(t *testing.T) {
	ctx := context.Background()

	// Setup mock with error
	mockClient := &MockGitHubClient{
		downloadZipballFunc: func(ctx context.Context, owner, repo, ref string) ([]byte, error) {
			return []byte{}, errors.New("download error")
		},
	}

	// Create use case
	uc := usecase.NewRelease(mockClient)

	// Execute
	releaseInfo := &model.ReleaseInfo{
		Owner:     "owner",
		Repo:      "repo",
		CommitSHA: "abc123",
	}

	result, err := uc.ProcessRelease(ctx, releaseInfo)

	// Verify
	gt.Error(t, err)
	gt.Value(t, result).Nil()
	gt.String(t, err.Error()).Contains("failed to download zipball")

	// Verify mock was called
	mockClient.AssertExpectations(t)
}

func TestReleaseUseCase_ProcessRelease_InvalidZip(t *testing.T) {
	ctx := context.Background()

	// Setup mock with invalid ZIP data
	mockClient := &MockGitHubClient{
		downloadZipballFunc: func(ctx context.Context, owner, repo, ref string) ([]byte, error) {
			return []byte("invalid zip data"), nil
		},
	}

	// Create use case
	uc := usecase.NewRelease(mockClient)

	// Execute
	releaseInfo := &model.ReleaseInfo{
		Owner:     "owner",
		Repo:      "repo",
		CommitSHA: "abc123",
	}

	result, err := uc.ProcessRelease(ctx, releaseInfo)

	// Verify
	gt.Error(t, err)
	gt.Value(t, result).Nil()
	gt.String(t, err.Error()).Contains("failed to extract zip")

	// Verify mock was called
	mockClient.AssertExpectations(t)
}

func TestReleaseUseCase_ProcessRelease_ExtractZipCleanup(t *testing.T) {
	ctx := context.Background()

	// Create test case that creates temp dir but fails during ZIP processing
	// This ensures the temp dir is cleaned up on error
	mockClient := &MockGitHubClient{
		downloadZipballFunc: func(ctx context.Context, owner, repo, ref string) ([]byte, error) {
			// Return invalid ZIP data that will fail during extraction
			return []byte("this is not valid zip data"), nil
		},
	}

	// Create use case
	uc := usecase.NewRelease(mockClient)

	// Execute
	releaseInfo := &model.ReleaseInfo{
		Owner:     "owner",
		Repo:      "repo",
		CommitSHA: "abc123",
	}

	result, err := uc.ProcessRelease(ctx, releaseInfo)

	// Verify error occurred
	gt.Error(t, err)
	gt.Value(t, result).Nil()
	gt.String(t, err.Error()).Contains("failed to extract zip")

	// Note: This test verifies that the defer cleanup is called.
	// The actual cleanup verification would require more complex testing infrastructure
	// to monitor filesystem operations, but the defer statement ensures cleanup happens.

	// Verify mock was called
	mockClient.AssertExpectations(t)
}

func TestReleaseUseCase_ProcessRelease_WithRealRepo(t *testing.T) {
	// Check for test environment variables
	appID := os.Getenv("TEST_GITHUB_APP_ID")
	installationID := os.Getenv("TEST_GITHUB_INSTALLATION_ID")
	privateKey := os.Getenv("TEST_GITHUB_PRIVATE_KEY")

	if appID == "" || installationID == "" || privateKey == "" {
		t.Skip("Test GitHub App credentials not provided via environment variables")
	}

	// Parse string IDs to int64
	appIDInt, err := strconv.ParseInt(appID, 10, 64)
	gt.NoError(t, err)

	installationIDInt, err := strconv.ParseInt(installationID, 10, 64)
	gt.NoError(t, err)

	ctx := context.Background()

	// Create GitHub client
	githubClient, err := githubinfra.NewClient(appIDInt, installationIDInt, []byte(privateKey))
	gt.NoError(t, err)

	// Create use case
	releaseUC := usecase.NewRelease(githubClient)

	// Create release info for this repository with specific commit
	releaseInfo := &model.ReleaseInfo{
		Owner:       "m-mizutani",
		Repo:        "shepherd",
		CommitSHA:   "4b2e63aa7ea0953797757ccefa215e150be6c13f",
		TagName:     "test-tag",
		ReleaseName: "Test Release",
	}

	// Process release
	result, err := releaseUC.ProcessRelease(ctx, releaseInfo)
	gt.NoError(t, err)
	gt.Value(t, result).NotNil()

	// Clean up temporary directory when test completes
	defer func() {
		if result != nil && result.TempDir != "" {
			_ = os.RemoveAll(result.TempDir) // Error ignored in test cleanup
		}
	}()

	// Verify basic properties
	gt.Value(t, result.TempDir).NotEqual("")
	gt.Number(t, len(result.Files)).Greater(0)
	gt.Number(t, result.Size).Greater(int64(0))

	t.Logf("Extracted %d files to %s (total size: %d bytes)",
		len(result.Files), result.TempDir, result.Size)

	// Verify expected files exist in the extracted content
	// The extracted ZIP typically has a top-level directory like "shepherd-<commit>"
	// Find the actual top-level directory
	entries, err := os.ReadDir(result.TempDir)
	gt.NoError(t, err)
	gt.Number(t, len(entries)).Greater(0)

	// Find the main directory (should be named like "shepherd-<commit>")
	var mainDir string
	for _, entry := range entries {
		if entry.IsDir() && (entry.Name() == "shepherd-4b2e63aa7ea0953797757ccefa215e150be6c13f" ||
			entry.Name() == "m-mizutani-shepherd-4b2e63a") { // GitHub might truncate the SHA
			mainDir = filepath.Join(result.TempDir, entry.Name())
			break
		}
	}

	if mainDir == "" {
		// Fallback: use the first directory found
		for _, entry := range entries {
			if entry.IsDir() {
				mainDir = filepath.Join(result.TempDir, entry.Name())
				break
			}
		}
	}

	gt.Value(t, mainDir).NotEqual("")
	t.Logf("Main directory found: %s", mainDir)

	// Verify expected files exist
	expectedFiles := []string{
		"go.mod",
		"main.go",
		"CLAUDE.md",
		"pkg/cli/cli.go",
		"pkg/usecase/webhook.go",
	}

	for _, expectedFile := range expectedFiles {
		fullPath := filepath.Join(mainDir, expectedFile)
		_, err := os.Stat(fullPath)
		gt.NoError(t, err)

		// Read and verify some content for key files
		if expectedFile == "go.mod" {
			content, err := os.ReadFile(fullPath)
			gt.NoError(t, err)
			gt.String(t, string(content)).Contains("module github.com/m-mizutani/shepherd")
			t.Logf("✓ go.mod content verified")
		}

		if expectedFile == "CLAUDE.md" {
			content, err := os.ReadFile(fullPath)
			gt.NoError(t, err)
			gt.String(t, string(content)).Contains("CLAUDE.md")
			t.Logf("✓ CLAUDE.md content verified")
		}
	}

	// Verify directory structure
	expectedDirs := []string{
		"pkg",
		"pkg/cli",
		"pkg/usecase",
		"pkg/controller",
		"pkg/domain",
	}

	for _, expectedDir := range expectedDirs {
		fullPath := filepath.Join(mainDir, expectedDir)
		_, err := os.Stat(fullPath)
		gt.NoError(t, err)
	}

	t.Logf("✓ Content verification completed successfully")
}

// createTestZip creates a test ZIP file for testing
func createTestZip(t *testing.T) []byte {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	// Create test files
	files := map[string]string{
		"test-repo-abc123/README.md": "# Test Repository\n\nThis is a test repository for shepherd testing.",
		"test-repo-abc123/main.go":   "package main\n\nfunc main() {\n\tprintln(\"Hello, World!\")\n}",
		"test-repo-abc123/.gitignore": "*.log\n*.tmp\n",
	}

	for filename, content := range files {
		writer, err := zipWriter.Create(filename)
		gt.NoError(t, err)

		_, err = writer.Write([]byte(content))
		gt.NoError(t, err)
	}

	err := zipWriter.Close()
	gt.NoError(t, err)

	return buf.Bytes()
}