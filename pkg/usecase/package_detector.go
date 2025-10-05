package usecase

import (
	"archive/zip"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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

// PackageDetector implements PackageDetectorUseCase
type PackageDetector struct {
	llmClient    gollem.LLMClient
	githubClient interfaces.GitHubClient
	userTemplate *template.Template
}

type packageDetector = PackageDetector

// NewPackageDetector creates a new PackageDetectorUseCase instance
func NewPackageDetector(
	llmClient gollem.LLMClient,
	githubClient interfaces.GitHubClient,
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

		// Extract package source code for version comparison
		if err := uc.ExtractPackageVersionSources(ctx, detection, prInfo); err != nil {
			logger.Error("Failed to extract package version sources", "error", err)
			// Don't return error - this is an enhancement feature, not critical
		}
	}

	return nil
}

// truncateText truncates text to a maximum length
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "...(truncated)"
}

// DetectFromPRInfo detects package updates from PR information using LLM
func (uc *packageDetector) DetectFromPRInfo(ctx context.Context, prInfo *model.PRInfo) (*model.PackageUpdateDetection, error) {
	logger := ctxlog.From(ctx)

	// Truncate PR body to prevent excessive LLM costs
	const maxBodyLength = 5000
	truncatedBody := truncateText(prInfo.Body, maxBodyLength)

	// Format user prompt using template
	var buf bytes.Buffer
	if err := uc.userTemplate.Execute(&buf, map[string]string{
		"Title": prInfo.Title,
		"Body":  truncatedBody,
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

	_, _, err := uc.githubClient.CreateComment(ctx, prInfo.Owner, prInfo.Repo, prInfo.Number, &github.IssueComment{
		Body: github.Ptr(comment),
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

// unzipToTempDir extracts zip data to a temporary directory
func unzipToTempDir(zipData []byte) (string, error) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "shepherd-package-*")
	if err != nil {
		return "", goerr.Wrap(err, "failed to create temporary directory")
	}

	// Create zip reader from bytes
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", goerr.Wrap(err, "failed to create zip reader")
	}

	// Extract all files
	for _, file := range zipReader.File {
		// Security check: prevent Zip Slip attack
		destPath := filepath.Join(tmpDir, file.Name)
		cleanPath := filepath.Clean(destPath)
		if !strings.HasPrefix(cleanPath, filepath.Clean(tmpDir)+string(os.PathSeparator)) {
			os.RemoveAll(tmpDir)
			return "", goerr.New("invalid file path in zip", goerr.V("path", file.Name))
		}

		if file.FileInfo().IsDir() {
			// Create directory
			if err := os.MkdirAll(destPath, file.Mode()); err != nil {
				os.RemoveAll(tmpDir)
				return "", goerr.Wrap(err, "failed to create directory", goerr.V("path", destPath))
			}
			continue
		}

		// Create parent directory if needed
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			os.RemoveAll(tmpDir)
			return "", goerr.Wrap(err, "failed to create parent directory", goerr.V("path", filepath.Dir(destPath)))
		}

		// Extract file
		if err := extractFile(file, destPath); err != nil {
			os.RemoveAll(tmpDir)
			return "", goerr.Wrap(err, "failed to extract file", goerr.V("path", destPath))
		}
	}

	return tmpDir, nil
}

// extractFile extracts a single file from zip archive
func extractFile(file *zip.File, destPath string) error {
	// Open file in zip
	rc, err := file.Open()
	if err != nil {
		return goerr.Wrap(err, "failed to open file in zip")
	}
	defer rc.Close()

	// Create destination file
	dest, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return goerr.Wrap(err, "failed to create destination file")
	}
	defer dest.Close()

	// Copy content
	if _, err := io.Copy(dest, rc); err != nil {
		return goerr.Wrap(err, "failed to copy file content")
	}

	return nil
}

// GoModuleInfo represents VCS repository information for a Go module
type GoModuleInfo struct {
	RepoURL string // Repository URL (e.g., "https://github.com/m-mizutani/goerr")
	Host    string // VCS host (e.g., "github.com")
	Owner   string // Repository owner (e.g., "m-mizutani")
	Repo    string // Repository name (e.g., "goerr")
}

// goProxyInfoResponse represents the response from Go proxy /@v/<version>.info endpoint
type goProxyInfoResponse struct {
	Version string `json:"Version"`
	Time    string `json:"Time"`
	Origin  *struct {
		VCS  string `json:"VCS"`
		URL  string `json:"URL"`
		Ref  string `json:"Ref"`
		Hash string `json:"Hash"`
	} `json:"Origin"`
}

// resolveGoModuleRepo resolves Go module path and version to VCS repository information
func resolveGoModuleRepo(ctx context.Context, modulePath, version string) (*GoModuleInfo, error) {
	// Construct Go proxy API URL
	proxyURL := fmt.Sprintf("https://proxy.golang.org/%s/@v/%s.info", modulePath, url.PathEscape(version))

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", proxyURL, nil)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create HTTP request", goerr.V("url", proxyURL))
	}

	// Execute request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to fetch Go proxy info", goerr.V("url", proxyURL))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, goerr.New("unexpected status code from Go proxy", goerr.V("status", resp.StatusCode), goerr.V("url", proxyURL))
	}

	// Parse JSON response
	var proxyInfo goProxyInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&proxyInfo); err != nil {
		return nil, goerr.Wrap(err, "failed to parse Go proxy response", goerr.V("url", proxyURL))
	}

	// Extract repository URL from Origin
	if proxyInfo.Origin == nil || proxyInfo.Origin.URL == "" {
		return nil, goerr.New("no origin URL in Go proxy response", goerr.V("module", modulePath), goerr.V("version", version))
	}

	// Parse repository URL
	moduleInfo, err := parseRepoURL(proxyInfo.Origin.URL)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to parse repository URL", goerr.V("url", proxyInfo.Origin.URL))
	}

	return moduleInfo, nil
}

// parseRepoURL parses repository URL to extract host, owner, and repo
func parseRepoURL(repoURL string) (*GoModuleInfo, error) {
	// Parse URL
	u, err := url.Parse(repoURL)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to parse URL")
	}

	// Extract path components
	// Expected format: https://github.com/owner/repo or https://github.com/owner/repo.git
	pathPattern := regexp.MustCompile(`^/([^/]+)/([^/]+?)(?:\.git)?$`)
	matches := pathPattern.FindStringSubmatch(u.Path)
	if len(matches) != 3 {
		return nil, goerr.New("invalid repository URL format", goerr.V("url", repoURL), goerr.V("path", u.Path))
	}

	return &GoModuleInfo{
		RepoURL: repoURL,
		Host:    u.Host,
		Owner:   matches[1],
		Repo:    matches[2],
	}, nil
}

// resolveGoVersion resolves Go version string to git ref
func resolveGoVersion(version string) string {
	// Currently returns the version as-is
	// Future: handle pseudo-versions, retracted versions, etc.
	return version
}

// ExtractGoPackageSource extracts Go package source code for a specific version (exposed for testing)
func (uc *packageDetector) ExtractGoPackageSource(ctx context.Context, packageName, version string) (tmpDir string, cleanup func(), err error) {
	return uc.extractGoPackageSource(ctx, packageName, version)
}

// extractGoPackageSource extracts Go package source code for a specific version
func (uc *packageDetector) extractGoPackageSource(ctx context.Context, packageName, version string) (tmpDir string, cleanup func(), err error) {
	logger := ctxlog.From(ctx)

	// Resolve module to repository information
	moduleInfo, err := resolveGoModuleRepo(ctx, packageName, version)
	if err != nil {
		return "", nil, goerr.Wrap(err, "failed to resolve Go module repository", goerr.V("package", packageName), goerr.V("version", version))
	}

	logger.Debug("Resolved Go module to repository",
		"package", packageName,
		"version", version,
		"host", moduleInfo.Host,
		"owner", moduleInfo.Owner,
		"repo", moduleInfo.Repo,
	)

	// Check if the repository is hosted on GitHub
	if moduleInfo.Host != "github.com" {
		return "", nil, goerr.New("unsupported VCS host (only GitHub is supported)", goerr.V("host", moduleInfo.Host), goerr.V("package", packageName))
	}

	// Resolve version to git ref
	ref := resolveGoVersion(version)

	// Download zipball from GitHub
	zipData, err := uc.githubClient.DownloadZipball(ctx, moduleInfo.Owner, moduleInfo.Repo, ref)
	if err != nil {
		return "", nil, goerr.Wrap(err, "failed to download zipball from GitHub", goerr.V("owner", moduleInfo.Owner), goerr.V("repo", moduleInfo.Repo), goerr.V("ref", ref))
	}

	logger.Debug("Downloaded zipball from GitHub",
		"owner", moduleInfo.Owner,
		"repo", moduleInfo.Repo,
		"ref", ref,
		"size", len(zipData),
	)

	// Extract zipball to temporary directory
	tmpDir, err = unzipToTempDir(zipData)
	if err != nil {
		return "", nil, goerr.Wrap(err, "failed to extract zipball to temporary directory")
	}

	logger.Info("Extracted Go package source to temporary directory",
		"package", packageName,
		"version", version,
		"tmpDir", tmpDir,
	)

	// Create cleanup function
	cleanup = func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			logger.Warn("Failed to remove temporary directory", "tmpDir", tmpDir, "error", err)
		} else {
			logger.Debug("Removed temporary directory", "tmpDir", tmpDir)
		}
	}

	return tmpDir, cleanup, nil
}

// ExtractPackageVersionSources extracts package source code for before and after versions
func (uc *packageDetector) ExtractPackageVersionSources(ctx context.Context, detection *model.PackageUpdateDetection, prInfo *model.PRInfo) error {
	logger := ctxlog.From(ctx)

	// Check if it's a package update
	if !detection.IsPackageUpdate {
		logger.Debug("Not a package update, skipping source extraction")
		return nil
	}

	// Language detection - only process Go language
	language := strings.ToLower(detection.Language)
	if language != "go" {
		logger.Info("Unsupported language for source extraction, skipping",
			"language", detection.Language,
		)
		return nil
	}

	logger.Info("Starting Go package source extraction",
		"language", detection.Language,
		"package_count", len(detection.Packages),
	)

	// Process each package
	for i, pkg := range detection.Packages {
		logger.Info("Processing package",
			"index", i+1,
			"total", len(detection.Packages),
			"package", pkg.Name,
			"from_version", pkg.FromVersion,
			"to_version", pkg.ToVersion,
		)

		// Extract FromVersion source
		fromDir, fromCleanup, err := uc.extractGoPackageSource(ctx, pkg.Name, pkg.FromVersion)
		if err != nil {
			logger.Error("Failed to extract source for FromVersion",
				"package", pkg.Name,
				"version", pkg.FromVersion,
				"error", err,
			)
			// Continue processing other packages
			continue
		}
		defer fromCleanup()

		// Extract ToVersion source
		toDir, toCleanup, err := uc.extractGoPackageSource(ctx, pkg.Name, pkg.ToVersion)
		if err != nil {
			logger.Error("Failed to extract source for ToVersion",
				"package", pkg.Name,
				"version", pkg.ToVersion,
				"error", err,
			)
			// Continue processing other packages
			continue
		}
		defer toCleanup()

		logger.Info("Successfully extracted package sources",
			"package", pkg.Name,
			"from_version", pkg.FromVersion,
			"from_dir", fromDir,
			"to_version", pkg.ToVersion,
			"to_dir", toDir,
		)

		// TODO: Future enhancement - analyze differences between fromDir and toDir
		// TODO: Future enhancement - generate summary using LLM
	}

	logger.Info("Completed Go package source extraction")

	return nil
}
