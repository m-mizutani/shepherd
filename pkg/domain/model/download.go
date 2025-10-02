package model

// DownloadResult represents the result of a ZIP download and extraction
type DownloadResult struct {
	TempDir string   // Path to temporary directory
	Files   []string // List of extracted files
	Size    int64    // Total size in bytes
}