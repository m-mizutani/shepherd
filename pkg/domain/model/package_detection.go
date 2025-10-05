package model

// PackageUpdateDetection represents the result of LLM-based package update detection
type PackageUpdateDetection struct {
	IsPackageUpdate bool            `json:"is_package_update"`
	Language        string          `json:"language"`
	Packages        []PackageUpdate `json:"packages"`
}

// PackageUpdate represents a single package update
type PackageUpdate struct {
	Name        string `json:"name"`
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
}

// PRInfo holds Pull Request information for detection
type PRInfo struct {
	Owner  string
	Repo   string
	Number int
	Title  string
	Body   string
}
