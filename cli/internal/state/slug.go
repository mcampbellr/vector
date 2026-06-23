package state

import (
	"regexp"
	"strings"
)

const maxSlugLen = 64

var (
	nonSlugChars  = regexp.MustCompile(`[^a-z0-9]+`)
	leadTrailDash = regexp.MustCompile(`^-+|-+$`)
)

// Slug converts arbitrary text into a kebab-case identifier suitable for a spec
// id and an OpenSpec change name. Non-alphanumeric runs collapse to a single
// dash; the result is lowercased and truncated to maxSlugLen at a dash boundary.
// Returns "" for input with no usable characters.
func Slug(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	dashed := nonSlugChars.ReplaceAllString(lower, "-")
	slug := leadTrailDash.ReplaceAllString(dashed, "")
	if len(slug) <= maxSlugLen {
		return slug
	}
	slug = slug[:maxSlugLen]
	return leadTrailDash.ReplaceAllString(slug, "")
}
