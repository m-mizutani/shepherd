// Package clamp provides numeric clamping helpers used by tool argument
// handling so each tool stays within sane size limits.
package clamp

// Limit returns req clamped to [1, max]. When req is zero or negative,
// defaultN is returned. defaultN must be <= max.
func Limit(req, defaultN, max int) int {
	if req <= 0 {
		return defaultN
	}
	if req > max {
		return max
	}
	return req
}
