package interfaces

import (
	"context"

	"github.com/google/go-github/v75/github"
)

// GitHubClient defines operations for interacting with GitHub API
type GitHubClient interface {
	// DownloadZipball downloads the source code zipball for a specific commit
	DownloadZipball(ctx context.Context, owner, repo, ref string) ([]byte, error)

	// CreateComment creates a comment on a pull request or issue
	CreateComment(ctx context.Context, owner, repo string, number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error)
}