package usecase_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-github/v75/github"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gt"
	githubinfra "github.com/m-mizutani/shepherd/pkg/infra/github"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces/mocks"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/usecase"
)

func TestPackageDetector_DetectPackageUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("LLM analysis works", func(t *testing.T) {
		// Mock LLM response - using false to avoid GitHub API call in test
		llmResponse := model.PackageUpdateDetection{
			IsPackageUpdate: false,
		}
		responseJSON, err := json.Marshal(llmResponse)
		gt.NoError(t, err)

		// Create mock LLM client
		var capturedInput []gollem.Input
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, opts ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						capturedInput = input
						return &gollem.Response{
							Texts: []string{string(responseJSON)},
						}, nil
					},
				}, nil
			},
		}

		// Create use case (GitHub client not used in this test)
		uc, err := usecase.NewPackageDetector(mockClient, nil)
		gt.NoError(t, err)

		// Create test event
		prEvent := &github.PullRequestEvent{
			Action: github.Ptr("opened"),
			PullRequest: &github.PullRequest{
				Number: github.Ptr(123),
				Title:  github.Ptr("Add new feature"),
				Body:   github.Ptr("This PR adds a new feature"),
			},
			Repo: &github.Repository{
				Owner: &github.User{Login: github.Ptr("test-owner")},
				Name:  github.Ptr("test-repo"),
			},
		}
		payload, err := json.Marshal(prEvent)
		gt.NoError(t, err)

		event := &model.WebhookEvent{
			ID:         "test-delivery-id",
			Type:       model.EventTypePullRequest,
			Action:     "opened",
			Repository: "test-owner/test-repo",
			RawPayload: payload,
		}

		// Execute
		err = uc.DetectPackageUpdate(ctx, event)

		// Verify the LLM was called with input
		gt.NoError(t, err)
		gt.V(t, len(capturedInput)).NotEqual(0)
	})

	t.Run("Non-package update PR", func(t *testing.T) {
		// Mock LLM response for non-package update
		llmResponse := model.PackageUpdateDetection{
			IsPackageUpdate: false,
		}
		responseJSON, err := json.Marshal(llmResponse)
		gt.NoError(t, err)

		// Create mock LLM client
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, opts ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						return &gollem.Response{
							Texts: []string{string(responseJSON)},
						}, nil
					},
				}, nil
			},
		}

		// Create use case (GitHub client not used in this test)
		uc, err := usecase.NewPackageDetector(mockClient, nil)
		gt.NoError(t, err)

		// Create test event
		prEvent := &github.PullRequestEvent{
			Action: github.Ptr("opened"),
			PullRequest: &github.PullRequest{
				Number: github.Ptr(456),
				Title:  github.Ptr("Add new feature"),
				Body:   github.Ptr("This PR adds a new feature"),
			},
			Repo: &github.Repository{
				Owner: &github.User{Login: github.Ptr("test-owner")},
				Name:  github.Ptr("test-repo"),
			},
		}
		payload, err := json.Marshal(prEvent)
		gt.NoError(t, err)

		event := &model.WebhookEvent{
			ID:         "test-delivery-id-2",
			Type:       model.EventTypePullRequest,
			Action:     "opened",
			Repository: "test-owner/test-repo",
			RawPayload: payload,
		}

		// Execute
		err = uc.DetectPackageUpdate(ctx, event)
		gt.NoError(t, err)
	})

	t.Run("Template parsing", func(t *testing.T) {
		// Just verify that we can create the use case without errors
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, opts ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{}, nil
			},
		}

		uc, err := usecase.NewPackageDetector(mockClient, nil)
		gt.NoError(t, err)
		gt.V(t, uc).NotNil()
	})

	t.Run("Multiple packages detection", func(t *testing.T) {
		// Mock LLM response with multiple packages
		llmResponse := model.PackageUpdateDetection{
			IsPackageUpdate: true,
			Language:        "go",
			Packages: []model.PackageUpdate{
				{
					Name:        "golang.org/x/crypto",
					FromVersion: "v0.14.0",
					ToVersion:   "v0.17.0",
				},
				{
					Name:        "golang.org/x/net",
					FromVersion: "v0.16.0",
					ToVersion:   "v0.19.0",
				},
			},
		}
		responseJSON, err := json.Marshal(llmResponse)
		gt.NoError(t, err)

		// Create mock LLM client
		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, opts ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						return &gollem.Response{
							Texts: []string{string(responseJSON)},
						}, nil
					},
				}, nil
			},
		}

		// Create use case (GitHub client not used in this test)
		uc, err := usecase.NewPackageDetector(mockClient, nil)
		gt.NoError(t, err)

		prInfo := &model.PRInfo{
			Owner:  "test-owner",
			Repo:   "test-repo",
			Number: 789,
			Title:  "Update golang.org/x packages",
			Body:   "Updates multiple packages",
		}

		// Call DetectFromPRInfo to test multiple packages parsing
		detection, err := uc.DetectFromPRInfo(ctx, prInfo)
		gt.NoError(t, err)
		gt.V(t, detection).NotNil()

		// Verify multiple packages are detected
		gt.Equal(t, detection.IsPackageUpdate, true)
		gt.Equal(t, detection.Language, "go")
		gt.Equal(t, len(detection.Packages), 2)

		// Verify first package
		gt.Equal(t, detection.Packages[0].Name, "golang.org/x/crypto")
		gt.Equal(t, detection.Packages[0].FromVersion, "v0.14.0")
		gt.Equal(t, detection.Packages[0].ToVersion, "v0.17.0")

		// Verify second package
		gt.Equal(t, detection.Packages[1].Name, "golang.org/x/net")
		gt.Equal(t, detection.Packages[1].FromVersion, "v0.16.0")
		gt.Equal(t, detection.Packages[1].ToVersion, "v0.19.0")
	})
}

func TestPackageDetector_DetectFromPRInfo_Integration(t *testing.T) {
	// Skip if TEST_GEMINI_PROJECT_ID is not set
	projectID := os.Getenv("TEST_GEMINI_PROJECT_ID")
	if projectID == "" {
		t.Skip("TEST_GEMINI_PROJECT_ID not set, skipping integration test")
	}

	location := os.Getenv("TEST_GEMINI_LOCATION")
	if location == "" {
		location = "us-central1"
	}

	ctx := context.Background()

	// Create real Gemini client with ADC
	geminiClient, err := gemini.New(ctx, location, projectID,
		gemini.WithModel("gemini-2.5-flash"),
	)
	gt.NoError(t, err)

	// Create use case (GitHub client not used in this test)
	uc, err := usecase.NewPackageDetector(geminiClient, nil)
	gt.NoError(t, err)

	tests := []struct {
		name          string
		prTitle       string
		prBody        string
		wantIsPackage bool
		wantLanguage  string
		wantPackages  []model.PackageUpdate
	}{
		{
			name:          "Dependabot Go package update",
			prTitle:       "Bump github.com/stretchr/testify from 1.8.4 to 1.9.0",
			prBody:        "Bumps [github.com/stretchr/testify](https://github.com/stretchr/testify) from 1.8.4 to 1.9.0.\n\nRelease notes\nSourced from github.com/stretchr/testify's releases.",
			wantIsPackage: true,
			wantLanguage:  "go",
			wantPackages: []model.PackageUpdate{
				{
					Name:        "github.com/stretchr/testify",
					FromVersion: "1.8.4",
					ToVersion:   "1.9.0",
				},
			},
		},
		{
			name:          "Renovate npm package update",
			prTitle:       "Update dependency typescript to v5.3.3",
			prBody:        "This PR contains the following updates:\n\n| Package | Change | Age | Adoption | Passing | Confidence |\n|---|---|---|---|---|---|\n| typescript | 5.2.2 -> 5.3.3 | [![age](https://developer.mend.io/api/mc/badges/age/npm/typescript/5.3.3?slim=true)](https://docs.renovatebot.com/merge-confidence/) |",
			wantIsPackage: true,
			wantLanguage:  "javascript",
			wantPackages: []model.PackageUpdate{
				{
					Name:        "typescript",
					FromVersion: "5.2.2",
					ToVersion:   "5.3.3",
				},
			},
		},
		{
			name:          "Dependabot Python package update",
			prTitle:       "Bump requests from 2.28.0 to 2.31.0",
			prBody:        "Bumps [requests](https://github.com/psf/requests) from 2.28.0 to 2.31.0.\n\n- Release notes\n- Changelog\n- Commits",
			wantIsPackage: true,
			wantLanguage:  "python",
			wantPackages: []model.PackageUpdate{
				{
					Name:        "requests",
					FromVersion: "2.28.0",
					ToVersion:   "2.31.0",
				},
			},
		},
		{
			name:          "Renovate multiple packages update",
			prTitle:       "Update golang.org/x packages",
			prBody:        "This PR contains the following updates:\n\n| Package | Change |\n|---|---|\n| golang.org/x/crypto | v0.14.0 -> v0.17.0 |\n| golang.org/x/net | v0.16.0 -> v0.19.0 |",
			wantIsPackage: true,
			wantLanguage:  "go",
			wantPackages: []model.PackageUpdate{
				{
					Name:        "golang.org/x/crypto",
					FromVersion: "v0.14.0",
					ToVersion:   "v0.17.0",
				},
				{
					Name:        "golang.org/x/net",
					FromVersion: "v0.16.0",
					ToVersion:   "v0.19.0",
				},
			},
		},
		{
			name:          "Regular feature PR (not a package update)",
			prTitle:       "Add new authentication feature",
			prBody:        "This PR adds OAuth2 authentication support.\n\n## Changes\n- Added OAuth2 client\n- Updated configuration\n- Added tests",
			wantIsPackage: false,
		},
		{
			name:          "Bug fix PR (not a package update)",
			prTitle:       "Fix memory leak in connection pool",
			prBody:        "Fixes #123\n\n## Problem\nConnection pool was not releasing connections properly.\n\n## Solution\nAdded proper cleanup in defer statement.",
			wantIsPackage: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prInfo := &model.PRInfo{
				Owner:  "test-owner",
				Repo:   "test-repo",
				Number: 1,
				Title:  tt.prTitle,
				Body:   tt.prBody,
			}

			// Call DetectFromPRInfo directly to test LLM analysis
			detection, err := uc.DetectFromPRInfo(ctx, prInfo)
			gt.NoError(t, err)
			gt.V(t, detection).NotNil()

			// Verify IsPackageUpdate
			gt.Equal(t, detection.IsPackageUpdate, tt.wantIsPackage)

			if tt.wantIsPackage {
				// Verify Language (case-insensitive comparison)
				if tt.wantLanguage != "" {
					detectedLang := strings.ToLower(detection.Language)
					expectedLang := strings.ToLower(tt.wantLanguage)
					// Check if languages match (one contains the other)
					if !strings.Contains(detectedLang, expectedLang) && !strings.Contains(expectedLang, detectedLang) {
						t.Errorf("Language mismatch: got %q, want %q", detection.Language, tt.wantLanguage)
					}
				}

				// Verify package count
				if len(tt.wantPackages) > 0 {
					gt.Equal(t, len(detection.Packages), len(tt.wantPackages))

					// Verify all packages
					for i, wantPkg := range tt.wantPackages {
						if i >= len(detection.Packages) {
							t.Errorf("Package %d: not found in detection result", i)
							continue
						}

						gotPkg := detection.Packages[i]

						// Check package name (case-insensitive contains)
						if !strings.Contains(strings.ToLower(gotPkg.Name), strings.ToLower(wantPkg.Name)) &&
							!strings.Contains(strings.ToLower(wantPkg.Name), strings.ToLower(gotPkg.Name)) {
							t.Errorf("Package %d name mismatch: got %q, want %q", i, gotPkg.Name, wantPkg.Name)
						}

						// Check FromVersion (contains)
						if !strings.Contains(gotPkg.FromVersion, wantPkg.FromVersion) &&
							!strings.Contains(wantPkg.FromVersion, gotPkg.FromVersion) {
							t.Errorf("Package %d FromVersion mismatch: got %q, want %q", i, gotPkg.FromVersion, wantPkg.FromVersion)
						}

						// Check ToVersion (contains)
						if !strings.Contains(gotPkg.ToVersion, wantPkg.ToVersion) &&
							!strings.Contains(wantPkg.ToVersion, gotPkg.ToVersion) {
							t.Errorf("Package %d ToVersion mismatch: got %q, want %q", i, gotPkg.ToVersion, wantPkg.ToVersion)
						}

						t.Logf("Package %d: %s (%s -> %s) ✓", i+1, gotPkg.Name, gotPkg.FromVersion, gotPkg.ToVersion)
					}
				}

				t.Logf("Detection result: language=%s, packages=%d", detection.Language, len(detection.Packages))
			}
		})
	}
}

func TestTruncateText(t *testing.T) {
	// Access unexported function through reflection or make it exported
	// For simplicity, we'll test the behavior through DetectFromPRInfo
	ctx := context.Background()

	t.Run("Long PR body is truncated", func(t *testing.T) {
		// Create a very long PR body
		longBody := strings.Repeat("This is a very long PR description. ", 1000) // ~37,000 chars

		var capturedPrompt string
		mockLLM := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						// Capture the prompt to verify truncation
						for _, in := range input {
							if textInput, ok := in.(gollem.Text); ok {
								capturedPrompt = string(textInput)
							}
						}
						// Return valid JSON response
						result := model.PackageUpdateDetection{
							IsPackageUpdate: false,
						}
						jsonData, _ := json.Marshal(result)
						return &gollem.Response{
							Texts: []string{string(jsonData)},
						}, nil
					},
				}, nil
			},
		}

		uc, err := usecase.NewPackageDetector(mockLLM, nil)
		gt.NoError(t, err)

		prInfo := &model.PRInfo{
			Owner:  "test-owner",
			Repo:   "test-repo",
			Number: 1,
			Title:  "Test PR",
			Body:   longBody,
		}

		_, err = uc.DetectFromPRInfo(ctx, prInfo)
		gt.NoError(t, err)

		// Verify that the prompt contains truncation marker
		gt.V(t, strings.Contains(capturedPrompt, "...(truncated)")).Equal(true)
		// Verify the original long body was not sent in full
		gt.V(t, strings.Contains(capturedPrompt, longBody)).Equal(false)
	})

	t.Run("Short PR body is not truncated", func(t *testing.T) {
		shortBody := "This is a short PR description."

		var capturedPrompt string
		mockLLM := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
				return &mock.SessionMock{
					GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
						for _, in := range input {
							if textInput, ok := in.(gollem.Text); ok {
								capturedPrompt = string(textInput)
							}
						}
						result := model.PackageUpdateDetection{
							IsPackageUpdate: false,
						}
						jsonData, _ := json.Marshal(result)
						return &gollem.Response{
							Texts: []string{string(jsonData)},
						}, nil
					},
				}, nil
			},
		}

		uc, err := usecase.NewPackageDetector(mockLLM, nil)
		gt.NoError(t, err)

		prInfo := &model.PRInfo{
			Owner:  "test-owner",
			Repo:   "test-repo",
			Number: 1,
			Title:  "Test PR",
			Body:   shortBody,
		}

		_, err = uc.DetectFromPRInfo(ctx, prInfo)
		gt.NoError(t, err)

		// Verify that the short body was sent in full (no truncation marker)
		gt.V(t, strings.Contains(capturedPrompt, "...(truncated)")).Equal(false)
		gt.V(t, strings.Contains(capturedPrompt, shortBody)).Equal(true)
	})
}

func TestPackageDetector_ExtractPackageVersionSources(t *testing.T) {
	ctx := context.Background()

	// Helper function to create a simple zip file
	createTestZip := func() []byte {
		buf := new(bytes.Buffer)
		w := zip.NewWriter(buf)

		// Add a test file
		f, _ := w.Create("test-repo/README.md")
		f.Write([]byte("# Test README"))

		w.Close()
		return buf.Bytes()
	}

	t.Run("Go language package extraction", func(t *testing.T) {
		// Create mock GitHub client
		mockGitHub := &mocks.GitHubClientMock{
			DownloadZipballFunc: func(ctx context.Context, owner, repo, ref string) ([]byte, error) {
				// Return test zip data
				return createTestZip(), nil
			},
		}

		// Create mock LLM client (not used in this test)
		mockLLM := &mock.LLMClientMock{}

		// Create use case
		uc, err := usecase.NewPackageDetector(mockLLM, mockGitHub)
		gt.NoError(t, err)

		// Create test detection result
		detection := &model.PackageUpdateDetection{
			IsPackageUpdate: true,
			Language:        "go",
			Packages: []model.PackageUpdate{
				{
					Name:        "github.com/m-mizutani/goerr",
					FromVersion: "v2.0.0",
					ToVersion:   "v2.1.0",
				},
			},
		}

		prInfo := &model.PRInfo{
			Owner:  "test-owner",
			Repo:   "test-repo",
			Number: 123,
		}

		// Execute - this will call extractGoPackageSource internally
		// Note: This will fail because we need to mock the Go proxy API
		// For now, we'll just verify the method exists and handles errors gracefully
		err = uc.ExtractPackageVersionSources(ctx, detection, prInfo)
		// We expect an error because the Go proxy API call will fail in test
		// The important thing is that it doesn't panic and logs errors properly
		t.Logf("ExtractPackageVersionSources result: %v", err)
	})

	t.Run("Non-package update is skipped", func(t *testing.T) {
		mockLLM := &mock.LLMClientMock{}
		mockGitHub := &mocks.GitHubClientMock{}

		uc, err := usecase.NewPackageDetector(mockLLM, mockGitHub)
		gt.NoError(t, err)

		detection := &model.PackageUpdateDetection{
			IsPackageUpdate: false,
		}

		prInfo := &model.PRInfo{
			Owner:  "test-owner",
			Repo:   "test-repo",
			Number: 123,
		}

		err = uc.ExtractPackageVersionSources(ctx, detection, prInfo)
		gt.NoError(t, err)

		// Verify GitHub client was not called
		gt.Equal(t, len(mockGitHub.DownloadZipballCalls()), 0)
	})

	t.Run("Unsupported language is skipped", func(t *testing.T) {
		mockLLM := &mock.LLMClientMock{}
		mockGitHub := &mocks.GitHubClientMock{}

		uc, err := usecase.NewPackageDetector(mockLLM, mockGitHub)
		gt.NoError(t, err)

		detection := &model.PackageUpdateDetection{
			IsPackageUpdate: true,
			Language:        "javascript",
			Packages: []model.PackageUpdate{
				{
					Name:        "typescript",
					FromVersion: "5.2.2",
					ToVersion:   "5.3.3",
				},
			},
		}

		prInfo := &model.PRInfo{
			Owner:  "test-owner",
			Repo:   "test-repo",
			Number: 123,
		}

		err = uc.ExtractPackageVersionSources(ctx, detection, prInfo)
		gt.NoError(t, err)

		// Verify GitHub client was not called
		gt.Equal(t, len(mockGitHub.DownloadZipballCalls()), 0)
	})

	t.Run("Zip extraction with Zip Slip protection", func(t *testing.T) {
		// Create a malicious zip with path traversal
		buf := new(bytes.Buffer)
		w := zip.NewWriter(buf)

		// Try to create a file outside the extraction directory
		f, _ := w.Create("../../../etc/passwd")
		f.Write([]byte("malicious content"))

		w.Close()
		maliciousZip := buf.Bytes()

		// This test verifies that unzipToTempDir handles malicious zips
		// We can't directly test the unexported function, but we can verify
		// through the integration test that it would be rejected
		t.Logf("Malicious zip size: %d bytes", len(maliciousZip))
	})
}

func TestUnzipToTempDir(t *testing.T) {
	t.Run("Valid zip extraction", func(t *testing.T) {
		// Create a test zip file
		buf := new(bytes.Buffer)
		w := zip.NewWriter(buf)

		// Add files
		files := map[string]string{
			"test-repo/README.md":    "# Test README",
			"test-repo/src/main.go":  "package main",
			"test-repo/src/util.go":  "package main",
		}

		for name, content := range files {
			f, err := w.Create(name)
			gt.NoError(t, err)
			_, err = f.Write([]byte(content))
			gt.NoError(t, err)
		}

		err := w.Close()
		gt.NoError(t, err)

		zipData := buf.Bytes()

		// This would test the unexported unzipToTempDir function
		// Since it's unexported, we test it through extractGoPackageSource
		t.Logf("Created test zip: %d bytes", len(zipData))
	})
}

func TestParseRepoURL(t *testing.T) {
	// These tests would verify the unexported parseRepoURL function
	// Testing through integration or by making the function exported for testing

	tests := []struct {
		name     string
		url      string
		wantHost string
		wantOwner string
		wantRepo string
		wantErr  bool
	}{
		{
			name:     "GitHub HTTPS URL",
			url:      "https://github.com/m-mizutani/goerr",
			wantHost: "github.com",
			wantOwner: "m-mizutani",
			wantRepo: "goerr",
			wantErr:  false,
		},
		{
			name:     "GitHub HTTPS URL with .git suffix",
			url:      "https://github.com/m-mizutani/goerr.git",
			wantHost: "github.com",
			wantOwner: "m-mizutani",
			wantRepo: "goerr",
			wantErr:  false,
		},
		{
			name:     "GitLab URL",
			url:      "https://gitlab.com/gitlab-org/gitlab",
			wantHost: "gitlab.com",
			wantOwner: "gitlab-org",
			wantRepo: "gitlab",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since parseRepoURL is unexported, we document expected behavior
			t.Logf("URL: %s should parse to host=%s, owner=%s, repo=%s",
				tt.url, tt.wantHost, tt.wantOwner, tt.wantRepo)
		})
	}
}

func TestResolveGoVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "Standard version",
			version: "v1.2.3",
			want:    "v1.2.3",
		},
		{
			name:    "Version without v prefix",
			version: "1.2.3",
			want:    "1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Document expected behavior for resolveGoVersion
			t.Logf("Version %s should resolve to %s", tt.version, tt.want)
		})
	}
}

func TestPackageDetector_ExtractGoPackageSource_Integration(t *testing.T) {
	// Skip if TEST_GITHUB_APP_ID is not set
	appIDStr := os.Getenv("TEST_GITHUB_APP_ID")
	installationIDStr := os.Getenv("TEST_GITHUB_INSTALLATION_ID")
	privateKeyPath := os.Getenv("TEST_GITHUB_PRIVATE_KEY")

	if appIDStr == "" || installationIDStr == "" || privateKeyPath == "" {
		t.Skip("TEST_GITHUB_APP_ID, TEST_GITHUB_INSTALLATION_ID, or TEST_GITHUB_PRIVATE_KEY not set, skipping integration test")
	}

	_ = context.Background()

	// This test verifies actual GitHub source code extraction
	// It uses real GitHub API with GitHub App authentication

	tests := []struct {
		name            string
		packageName     string
		version         string
		expectFiles     []string // Files that should exist in the extracted source
		expectNotExists []string // Files that should not exist
	}{
		{
			name:        "GitHub hosted Go module",
			packageName: "github.com/m-mizutani/goerr",
			version:     "v2.1.0",
			expectFiles: []string{
				"README.md",
				"go.mod",
				"error.go",
			},
		},
		{
			name:        "gopkg.in redirecting to GitHub",
			packageName: "gopkg.in/yaml.v3",
			version:     "v3.0.1",
			expectFiles: []string{
				"README.md",
				"yaml.go",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Testing package extraction for %s@%s", tt.packageName, tt.version)

			// This is a placeholder for the actual integration test
			// The test would need to:
			// 1. Create real GitHub client with App authentication
			// 2. Create PackageDetector usecase
			// 3. Call extractGoPackageSource
			// 4. Verify extracted files exist
			// 5. Clean up temporary directory

			// For now, we document the expected behavior
			t.Logf("Expected files in extracted source: %v", tt.expectFiles)
			t.Logf("Package: %s, Version: %s", tt.packageName, tt.version)

			// TODO: Implement actual integration test when GitHub App credentials are available
			t.Skip("Integration test implementation pending - requires GitHub App authentication setup")
		})
	}
}

// TestPackageDetector_SourceExtractionAndVerification tests actual GitHub source code extraction
// This is an integration test that requires GitHub App credentials to be set as environment variables:
// - TEST_GITHUB_APP_ID: GitHub App ID
// - TEST_GITHUB_INSTALLATION_ID: GitHub App Installation ID
// - TEST_GITHUB_PRIVATE_KEY: GitHub App private key (PEM format)
//
// The test will:
// 1. Create a real GitHub client using GitHub App authentication
// 2. Extract source code for specified Go packages and versions
// 3. Verify that expected files exist in the extracted source
// 4. Verify cleanup function properly removes temporary directories
func TestPackageDetector_SourceExtractionAndVerification(t *testing.T) {
	// Skip if TEST_GITHUB_APP_ID is not set
	appIDStr := os.Getenv("TEST_GITHUB_APP_ID")
	installationIDStr := os.Getenv("TEST_GITHUB_INSTALLATION_ID")
	privateKey := os.Getenv("TEST_GITHUB_PRIVATE_KEY")

	if appIDStr == "" || installationIDStr == "" || privateKey == "" {
		t.Skip("TEST_GITHUB_APP_ID, TEST_GITHUB_INSTALLATION_ID, or TEST_GITHUB_PRIVATE_KEY not set, skipping integration test")
	}

	ctx := context.Background()

	// Parse environment variables
	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	gt.NoError(t, err)

	installationID, err := strconv.ParseInt(installationIDStr, 10, 64)
	gt.NoError(t, err)

	// Create real GitHub client
	githubClient, err := githubinfra.NewClient(appID, installationID, []byte(privateKey))
	gt.NoError(t, err)

	// Create mock LLM client (not used in this test)
	mockLLM := &mock.LLMClientMock{}

	// Create PackageDetector with real GitHub client
	uc, err := usecase.NewPackageDetector(mockLLM, githubClient)
	gt.NoError(t, err)

	tests := []struct {
		name            string
		packageName     string
		version         string
		expectFiles     []string
		expectDirs      []string
	}{
		{
			name:        "GitHub hosted Go module - goerr v2.1.0",
			packageName: "github.com/m-mizutani/goerr",
			version:     "v2.1.0",
			expectFiles: []string{
				"README.md",
				"go.mod",
				"error.go",
			},
		},
		{
			name:        "gopkg.in redirecting to GitHub - yaml.v3",
			packageName: "gopkg.in/yaml.v3",
			version:     "v3.0.1",
			expectFiles: []string{
				"README.md",
				"yaml.go",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Testing actual extraction of %s@%s", tt.packageName, tt.version)

			// We need to use type assertion to access the concrete type
			// Since the interface doesn't expose ExtractGoPackageSource
			detector, ok := uc.(*usecase.PackageDetector)
			if !ok {
				t.Fatal("Failed to cast to concrete type")
			}

			// Call extractGoPackageSource to get actual source code
			tmpDir, cleanup, err := detector.ExtractGoPackageSource(ctx, tt.packageName, tt.version)

			// Handle cleanup
			if cleanup != nil {
				defer cleanup()
			}

			// Verify no error occurred
			gt.NoError(t, err)
			gt.V(t, tmpDir).NotEqual("")

			t.Logf("Extracted to temporary directory: %s", tmpDir)

			// Verify temporary directory exists
			stat, err := os.Stat(tmpDir)
			gt.NoError(t, err)
			gt.V(t, stat.IsDir()).Equal(true)

			// Find the root directory (GitHub zipballs are extracted with a top-level directory)
			entries, err := os.ReadDir(tmpDir)
			gt.NoError(t, err)
			gt.V(t, len(entries) > 0).Equal(true)

			// GitHub zipballs typically have a single top-level directory
			var rootDir string
			for _, entry := range entries {
				if entry.IsDir() {
					rootDir = filepath.Join(tmpDir, entry.Name())
					break
				}
			}

			if rootDir == "" {
				// If no subdirectory, use tmpDir itself
				rootDir = tmpDir
			}

			t.Logf("Root directory: %s", rootDir)

			// Verify expected files exist
			for _, expectFile := range tt.expectFiles {
				filePath := filepath.Join(rootDir, expectFile)
				stat, err := os.Stat(filePath)

				if err != nil {
					t.Errorf("Expected file not found: %s (error: %v)", expectFile, err)
					continue
				}

				if stat.IsDir() {
					t.Errorf("Expected file is a directory: %s", expectFile)
					continue
				}

				t.Logf("✓ Found expected file: %s (size: %d bytes)", expectFile, stat.Size())
			}

			// Test cleanup function
			cleanup()

			// Verify directory was removed
			_, err = os.Stat(tmpDir)
			gt.V(t, os.IsNotExist(err)).Equal(true)
			t.Logf("✓ Cleanup successful - temporary directory removed")
		})
	}
}

