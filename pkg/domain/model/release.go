package model

// ReleaseInfo represents information extracted from a release event
type ReleaseInfo struct {
	Owner       string // Repository owner
	Repo        string // Repository name
	CommitSHA   string // Commit SHA for the release
	TagName     string // Release tag name
	ReleaseName string // Release name
}