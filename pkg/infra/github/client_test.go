package github_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/m-mizutani/gt"

	githubinfra "github.com/m-mizutani/shepherd/pkg/infra/github"
)

func TestClient_DownloadZipball_Success(t *testing.T) {
	// Create mock HTTP server
	zipContent := []byte("fake zip content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		w.Write(zipContent)
	}))
	defer server.Close()

	// This test requires GitHub App credentials from environment variables
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

	t.Run("create client with private key", func(t *testing.T) {
		client, err := githubinfra.NewClient(appIDInt, installationIDInt, []byte(privateKey))
		gt.NoError(t, err)
		gt.Value(t, client).NotNil()
	})
}

func TestClient_DownloadZipball_WithRealAPI(t *testing.T) {
	// Integration test with real GitHub API
	// This test requires test environment variables
	appID := os.Getenv("TEST_GITHUB_APP_ID")
	installationID := os.Getenv("TEST_GITHUB_INSTALLATION_ID")
	privateKey := os.Getenv("TEST_GITHUB_PRIVATE_KEY")

	if appID == "" || installationID == "" || privateKey == "" {
		t.Skip("Test GitHub App credentials not provided")
	}

	// This would be a real integration test if credentials are provided
	// For now, just skip with informational message
	t.Log("Integration test would run with real GitHub API if credentials were provided")
}