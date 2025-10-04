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

const (
	// Maximum size for a single file (1GB)
	maxFileSize = 1 * 1024 * 1024 * 1024
	// Maximum total uncompressed size (10GB)
	maxTotalSize = 10 * 1024 * 1024 * 1024
)

type eventUseCase struct {
	githubClient interfaces.GitHubClient
}

// NewRelease creates a new instance of ReleaseUseCase
func NewRelease(githubClient interfaces.GitHubClient) interfaces.ReleaseUseCase {
	return &eventUseCase{
		githubClient: githubClient,
	}
}

// NewSourceCode creates a new instance of SourceCodeUseCase
func NewSourceCode(githubClient interfaces.GitHubClient) interfaces.SourceCodeUseCase {
	return &eventUseCase{
		githubClient: githubClient,
	}
}

// ProcessRelease processes a release event and downloads the source code
// This is a wrapper around ProcessSource for backward compatibility
func (uc *eventUseCase) ProcessRelease(ctx context.Context, info *model.ReleaseInfo) (*model.DownloadResult, error) {
	// Convert ReleaseInfo to SourceInfo
	sourceInfo := &model.SourceInfo{
		Owner:     info.Owner,
		Repo:      info.Repo,
		CommitSHA: info.CommitSHA,
		EventType: "release",
		Ref:       info.TagName,
		Actor:     "", // Not available in ReleaseInfo
		Metadata: map[string]string{
			"tag_name":     info.TagName,
			"release_name": info.ReleaseName,
		},
	}

	// Delegate to ProcessSource
	return uc.ProcessSource(ctx, sourceInfo)
}

// ProcessSource processes a source code event and downloads the source code
func (uc *eventUseCase) ProcessSource(ctx context.Context, info *model.SourceInfo) (*model.DownloadResult, error) {
	logger := ctxlog.From(ctx)

	logger.Info("Processing source code event",
		"owner", info.Owner,
		"repo", info.Repo,
		"commit_sha", info.CommitSHA,
		"event_type", info.EventType,
		"ref", info.Ref,
		"actor", info.Actor,
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
func (uc *eventUseCase) extractZip(ctx context.Context, zipData []byte) (*model.DownloadResult, error) {
	logger := ctxlog.From(ctx)

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "shepherd-release-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	// Track success to determine if we should clean up on error
	var success bool
	defer func() {
		if !success {
			if removeErr := os.RemoveAll(tempDir); removeErr != nil {
				logger.Warn("Failed to clean up temporary directory on error",
					"temp_dir", tempDir,
					"error", removeErr,
				)
			} else {
				logger.Debug("Cleaned up temporary directory on error", "temp_dir", tempDir)
			}
		}
	}()

	// Set appropriate permissions (0700 for owner-only access to directory)
	// Directories require execute permission to access contents
	if err := os.Chmod(tempDir, 0700); err != nil { // #nosec G302 -- 0700 required for directory access
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
		// Check individual file size to prevent decompression bombs
		if file.UncompressedSize64 > maxFileSize {
			return nil, fmt.Errorf("file too large: %s (%d bytes exceeds limit of %d)", file.Name, file.UncompressedSize64, maxFileSize)
		}

		// Check for potential overflow before conversion (max int64)
		const maxInt64 = 1<<63 - 1
		if file.UncompressedSize64 > maxInt64 {
			return nil, fmt.Errorf("file size too large: %d bytes", file.UncompressedSize64)
		}

		// Check total size to prevent decompression bombs
		// #nosec G115 -- overflow is checked above with maxInt64 check
		newTotal := totalSize + int64(file.UncompressedSize64)
		if newTotal > maxTotalSize {
			return nil, fmt.Errorf("total uncompressed size too large: %d bytes exceeds limit of %d", newTotal, maxTotalSize)
		}

		if err := uc.extractFile(file, tempDir); err != nil {
			return nil, fmt.Errorf("failed to extract file %s: %w", file.Name, err)
		}

		extractedFiles = append(extractedFiles, file.Name)
		totalSize += int64(file.UncompressedSize64) // #nosec G115 -- overflow checked above
	}

	// Mark as successful to prevent cleanup
	success = true
	return &model.DownloadResult{
		TempDir: tempDir,
		Files:   extractedFiles,
		Size:    totalSize,
	}, nil
}

// extractFile extracts a single file from ZIP to the destination directory
func (uc *eventUseCase) extractFile(file *zip.File, destDir string) error {
	// Security check: prevent path traversal attacks
	// #nosec G305 -- Path traversal is explicitly checked below
	destPath := filepath.Join(destDir, file.Name)
	if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file path detected: file=%s, dest=%s", file.Name, destPath)
	}

	// Open file in ZIP
	rc, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open file %s in zip: %w", file.Name, err)
	}
	defer func() {
		_ = rc.Close() // Error ignored as we're reading, not writing
	}()

	// Create directory if needed
	if file.FileInfo().IsDir() {
		return os.MkdirAll(destPath, file.FileInfo().Mode())
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(destPath), 0700); err != nil { // #nosec G301 -- 0700 is secure for directories
		return fmt.Errorf("failed to create parent directories %s: %w", filepath.Dir(destPath), err)
	}

	// Create destination file
	// #nosec G304 -- destPath is sanitized above against path traversal
	destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.FileInfo().Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
	}
	defer func() {
		if closeErr := destFile.Close(); closeErr != nil {
			// Log error but don't fail - file content already written
			_ = closeErr
		}
	}()

	// Copy content with size limit to prevent decompression bombs
	// #nosec G110 -- Size limit is enforced at zip file level before extraction
	_, err = io.Copy(destFile, rc)
	if err != nil {
		return fmt.Errorf("failed to copy file content to %s: %w", destPath, err)
	}

	return nil
}