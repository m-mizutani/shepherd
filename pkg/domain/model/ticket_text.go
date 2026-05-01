package model

import (
	"strings"
)

// CanonicalText returns the normalised text used as the input to the
// embedding model. Title, description, and conclusion are stitched together
// with paragraph breaks; surrounding whitespace is trimmed and runs of
// whitespace inside each section are collapsed to a single space so that
// trivial reformatting (extra blank lines, tab vs space) does not cause
// gratuitous re-embeds.
func CanonicalText(t *Ticket) string {
	if t == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	for _, s := range []string{t.Title, t.Description, t.Conclusion} {
		if normalised := normaliseWhitespace(s); normalised != "" {
			parts = append(parts, normalised)
		}
	}
	return strings.Join(parts, "\n\n")
}

func normaliseWhitespace(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '\v', '\f':
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		default:
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return b.String()
}
