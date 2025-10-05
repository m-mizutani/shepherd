package usecase

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/google/go-github/v75/github"
	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
)

//go:embed prompts/package_detection_system.md
var systemPrompt string

//go:embed prompts/package_detection_user.md
var userPromptTemplate string

type packageDetector struct {
	llmClient    gollem.LLMClient
	githubClient *github.Client
	userTemplate *template.Template
}

// NewPackageDetector creates a new PackageDetectorUseCase instance
func NewPackageDetector(
	llmClient gollem.LLMClient,
	githubClient *github.Client,
) (interfaces.PackageDetectorUseCase, error) {
	// Parse user prompt template
	tmpl, err := template.New("user").Parse(userPromptTemplate)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to parse user prompt template")
	}

	return &packageDetector{
		llmClient:    llmClient,
		githubClient: githubClient,
		userTemplate: tmpl,
	}, nil
}

// DetectPackageUpdate processes a pull_request opened event
func (uc *packageDetector) DetectPackageUpdate(ctx context.Context, event *model.WebhookEvent) error {
	logger := ctxlog.From(ctx)

	// Parse GitHub event payload
	var prEvent github.PullRequestEvent
	if err := json.Unmarshal(event.RawPayload, &prEvent); err != nil {
		return goerr.Wrap(err, "failed to unmarshal PR event")
	}

	// Extract PR information
	prInfo := &model.PRInfo{
		Owner:  prEvent.GetRepo().GetOwner().GetLogin(),
		Repo:   prEvent.GetRepo().GetName(),
		Number: prEvent.GetPullRequest().GetNumber(),
		Title:  prEvent.GetPullRequest().GetTitle(),
		Body:   prEvent.GetPullRequest().GetBody(),
	}

	logger.Info("Analyzing PR for package updates",
		"owner", prInfo.Owner,
		"repo", prInfo.Repo,
		"number", prInfo.Number,
	)

	// Detect package updates from PR info using LLM
	detection, err := uc.DetectFromPRInfo(ctx, prInfo)
	if err != nil {
		return goerr.Wrap(err, "failed to detect package updates from PR")
	}

	logger.Info("Package update detection completed",
		"is_package_update", detection.IsPackageUpdate,
		"language", detection.Language,
		"package_count", len(detection.Packages),
	)

	// Post comment if it's a package update
	if detection.IsPackageUpdate {
		if err := uc.postComment(ctx, detection, prInfo); err != nil {
			logger.Error("Failed to post comment", "error", err)
			return goerr.Wrap(err, "failed to post comment")
		}
	}

	return nil
}

// DetectFromPRInfo detects package updates from PR information using LLM
func (uc *packageDetector) DetectFromPRInfo(ctx context.Context, prInfo *model.PRInfo) (*model.PackageUpdateDetection, error) {
	logger := ctxlog.From(ctx)

	// Format user prompt using template
	var buf bytes.Buffer
	if err := uc.userTemplate.Execute(&buf, map[string]string{
		"Title": prInfo.Title,
		"Body":  prInfo.Body,
	}); err != nil {
		return nil, goerr.Wrap(err, "failed to execute user prompt template")
	}
	userPrompt := buf.String()

	logger.Debug("Calling LLM for package detection", "prompt_length", len(userPrompt))

	// Create session and generate content
	session, err := uc.llmClient.NewSession(ctx,
		gollem.WithSessionContentType(gollem.ContentTypeJSON),
		gollem.WithSessionSystemPrompt(systemPrompt),
	)
	if err != nil {
		logger.Error("Failed to create LLM session", "error", err)
		return nil, goerr.Wrap(err, "failed to create LLM session")
	}

	resp, err := session.GenerateContent(ctx, gollem.Text(userPrompt))
	if err != nil {
		logger.Error("Failed to generate LLM content", "error", err)
		return nil, goerr.Wrap(err, "failed to generate LLM content")
	}

	// Parse JSON response
	var detection model.PackageUpdateDetection
	if len(resp.Texts) == 0 {
		return nil, goerr.New("no response from LLM")
	}
	if err := json.Unmarshal([]byte(resp.Texts[0]), &detection); err != nil {
		logger.Error("Failed to parse LLM response", "error", err, "response", resp.Texts[0])
		return nil, goerr.Wrap(err, "failed to parse LLM response", goerr.V("response", resp.Texts[0]))
	}

	return &detection, nil
}

// postComment posts a comment to the PR with detection results
func (uc *packageDetector) postComment(ctx context.Context, detection *model.PackageUpdateDetection, prInfo *model.PRInfo) error {
	logger := ctxlog.From(ctx)

	comment := formatComment(detection)

	logger.Info("Posting detection result to PR",
		"owner", prInfo.Owner,
		"repo", prInfo.Repo,
		"number", prInfo.Number,
	)

	_, _, err := uc.githubClient.Issues.CreateComment(ctx, prInfo.Owner, prInfo.Repo, prInfo.Number, &github.IssueComment{
		Body: github.String(comment),
	})
	if err != nil {
		return goerr.Wrap(err, "failed to create comment")
	}

	logger.Info("Successfully posted comment to PR")

	return nil
}

// formatComment formats the detection result as a markdown comment
func formatComment(detection *model.PackageUpdateDetection) string {
	var sb strings.Builder

	sb.WriteString("## 📦 Package Update Detection\n\n")
	sb.WriteString("This pull request appears to be a **package update**.\n\n")
	sb.WriteString(fmt.Sprintf("**Language**: %s\n\n", detection.Language))

	if len(detection.Packages) > 0 {
		sb.WriteString("**Packages**:\n")
		for _, pkg := range detection.Packages {
			sb.WriteString(fmt.Sprintf("- `%s`: %s → %s\n", pkg.Name, pkg.FromVersion, pkg.ToVersion))
		}
	}

	sb.WriteString("\n---\n")
	sb.WriteString("🤖 Detected by Shepherd\n")

	return sb.String()
}
