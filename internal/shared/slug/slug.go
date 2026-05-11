package slug

import (
	"fmt"
	"regexp"
	"strings"
	"time"
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
	if len(s) > 80 {
		s = s[:80]
	}
	s = strings.TrimRight(s, "-")
	if s == "" {
		s = "untitled"
	}
	return s
}

func GenerateUnique(title string, exists func(string) bool) string {
	base := Generate(title)
	slug := base
	for i := 1; exists(slug); i++ {
		suffix := fmt.Sprintf("%d", time.Now().UnixNano()%100000)
		slug = fmt.Sprintf("%s-%s", base, suffix)
		if len(slug) > 80 {
			slug = slug[:80]
		}
		slug = strings.TrimRight(slug, "-")
	}
	return slug
}
