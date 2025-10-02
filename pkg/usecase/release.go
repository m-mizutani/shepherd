package usecase

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
)

type releaseUseCase struct {
	githubClient interfaces.GitHubClient
}

// NewRelease creates a new instance of ReleaseUseCase
func NewRelease(githubClient interfaces.GitHubClient) interfaces.ReleaseUseCase {
	return &releaseUseCase{
		githubClient: githubClient,
	}
}

// ProcessRelease processes a release event and downloads the source code
func (uc *releaseUseCase) ProcessRelease(ctx context.Context, info *model.ReleaseInfo) (*model.DownloadResult, error) {
	logger := ctxlog.From(ctx)

	logger.Info("Processing release event",
		"owner", info.Owner,
		"repo", info.Repo,
		"commit_sha", info.CommitSHA,
		"tag_name", info.TagName,
		"release_name", info.ReleaseName,
	)

	// Download ZIP from GitHub
	zipData, err := uc.githubClient.DownloadZipball(ctx, info.Owner, info.Repo, info.CommitSHA)
	if err != nil {
		logger.Error("Failed to download zipball",
			"error", err,
			"owner", info.Owner,
			"repo", info.Repo,
			"commit_sha", info.CommitSHA,
		)
		return nil, fmt.Errorf("failed to download zipball for %s/%s@%s: %w", info.Owner, info.Repo, info.CommitSHA, err)
	}

	logger.Info("Downloaded zipball",
		"size_bytes", len(zipData),
		"owner", info.Owner,
		"repo", info.Repo,
	)

	// Extract ZIP to temporary directory
	result, err := uc.extractZip(ctx, zipData)
	if err != nil {
		logger.Error("Failed to extract zip",
			"error", err,
			"owner", info.Owner,
			"repo", info.Repo,
		)
		return nil, fmt.Errorf("failed to extract zip for %s/%s: %w", info.Owner, info.Repo, err)
	}

	logger.Info("Extracted zipball to temporary directory",
		"temp_dir", result.TempDir,
		"file_count", len(result.Files),
		"total_size_bytes", result.Size,
		"owner", info.Owner,
		"repo", info.Repo,
	)

	return result, nil
}

// extractZip extracts ZIP data to a temporary directory
func (uc *releaseUseCase) extractZip(ctx context.Context, zipData []byte) (*model.DownloadResult, error) {
	logger := ctxlog.From(ctx)

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "shepherd-release-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	// Set appropriate permissions
	if err := os.Chmod(tempDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to set directory permissions for %s: %w", tempDir, err)
	}

	logger.Debug("Created temporary directory", "temp_dir", tempDir)

	// Create ZIP reader
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create zip reader: %w", err)
	}

	var extractedFiles []string
	var totalSize int64

	// Extract each file
	for _, file := range zipReader.File {
		if err := uc.extractFile(file, tempDir); err != nil {
			return nil, fmt.Errorf("failed to extract file %s: %w", file.Name, err)
		}

		extractedFiles = append(extractedFiles, file.Name)
		totalSize += int64(file.UncompressedSize64)
	}

	return &model.DownloadResult{
		TempDir: tempDir,
		Files:   extractedFiles,
		Size:    totalSize,
	}, nil
}

// extractFile extracts a single file from ZIP to the destination directory
func (uc *releaseUseCase) extractFile(file *zip.File, destDir string) error {
	// Security check: prevent path traversal attacks
	destPath := filepath.Join(destDir, file.Name)
	if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file path detected: file=%s, dest=%s", file.Name, destPath)
	}

	// Open file in ZIP
	rc, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open file %s in zip: %w", file.Name, err)
	}
	defer rc.Close()

	// Create directory if needed
	if file.FileInfo().IsDir() {
		return os.MkdirAll(destPath, file.FileInfo().Mode())
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directories %s: %w", filepath.Dir(destPath), err)
	}

	// Create destination file
	destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.FileInfo().Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
	}
	defer destFile.Close()

	// Copy content
	_, err = io.Copy(destFile, rc)
	if err != nil {
		return fmt.Errorf("failed to copy file content to %s: %w", destPath, err)
	}

	return nil
}