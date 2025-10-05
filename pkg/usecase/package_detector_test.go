package usecase_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/go-github/v75/github"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gt"
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
