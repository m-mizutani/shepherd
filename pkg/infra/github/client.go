package github

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v75/github"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
)

type client struct {
	githubClient *github.Client
}

// NewClient creates a new GitHub client with App authentication
func NewClient(appID, installationID int64, privateKey []byte) (interfaces.GitHubClient, error) {
	// Create GitHub App transport
	itr, err := ghinstallation.New(http.DefaultTransport, appID, installationID, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub App transport: %w", err)
	}

	// Create GitHub client
	githubClient := github.NewClient(&http.Client{Transport: itr})

	return &client{
		githubClient: githubClient,
	}, nil
}


// DownloadZipball downloads the source code zipball for a specific commit
func (c *client) DownloadZipball(ctx context.Context, owner, repo, ref string) ([]byte, error) {
	// Get download URL for zipball
	url, _, err := c.githubClient.Repositories.GetArchiveLink(ctx, owner, repo, github.Zipball, &github.RepositoryContentGetOptions{
		Ref: ref,
	}, 3) // Follow up to 3 redirects

	if err != nil {
		return nil, fmt.Errorf("failed to get zipball download URL for %s/%s@%s: %w", owner, repo, ref, err)
	}

	// Create HTTP request for download
	req, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request for %s: %w", url.String(), err)
	}

	// Use the same client transport for authentication
	httpClient := &http.Client{Transport: c.githubClient.Client().Transport}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download zipball from %s: %w", url.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d for %s", resp.StatusCode, url.String())
	}

	// Read the entire response
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}
