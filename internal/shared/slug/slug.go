package slug

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reNonAlpha   = regexp.MustCompile(`[^a-z0-9\s-]`)
	reMultiSpace = regexp.MustCompile(`\s+`)
	reMultiDash  = regexp.MustCompile(`-+`)
)

func Generate(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = reNonAlpha.ReplaceAllString(s, "")
	s = reMultiSpace.ReplaceAllString(s, "-")
	s = reMultiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > maxSlugLen {
		s = s[:maxSlugLen]
	}
	s = strings.TrimRight(s, "-")
	if s == "" {
		s = "untitled"
	}
	return s
}

// maxSlugLen is the byte budget for a stored slug. Several callers rely on it
// (the news/event/college URLs read it back as a path segment), so the budget
// and the uniquifier both have to respect it.
const maxSlugLen = 80

// GenerateUnique returns a slug for title that no existing row already uses.
//
// The uniquifier is appended to a stem that has been shortened to leave exactly
// enough room for it. Appending first and truncating afterwards does not work:
// for a base within one suffix-length of the cap, the truncation removes the
// suffix, TrimRight removes the now-trailing dash, and the candidate collapses
// back onto the base — so exists() keeps reporting a collision on the very same
// string and the caller loops forever re-issuing the same lookup.
func GenerateUnique(title string, exists func(string) bool) string {
	base := Generate(title)
	if !exists(base) {
		return base
	}

	for i := 1; ; i++ {
		suffix := fmt.Sprintf("-%d", i)
		stem := base
		if len(stem)+len(suffix) > maxSlugLen {
			// Cut the stem, never the suffix, and drop any dash the cut
			// leaves dangling at the seam.
			stem = strings.TrimRight(stem[:maxSlugLen-len(suffix)], "-")
		}
		candidate := stem + suffix
		if !exists(candidate) {
			return candidate
		}
	}
}
