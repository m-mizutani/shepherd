package interfaces

import "context"

// GitHubClient defines operations for interacting with GitHub API
type GitHubClient interface {
	// DownloadZipball downloads the source code zipball for a specific commit
	DownloadZipball(ctx context.Context, owner, repo, ref string) ([]byte, error)
}