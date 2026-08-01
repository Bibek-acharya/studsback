package sanitize

import (
	"github.com/microcosm-cc/bluemonday"
)

var policy *bluemonday.Policy

func init() {
	policy = bluemonday.NewPolicy()

	policy.AllowElements(
		"p", "br",
		"h1", "h2", "h3",
		"strong", "em", "u", "s",
		"ol", "ul", "li",
		"a", "img",
		"blockquote",
		"pre", "code",
		"table", "thead", "tbody", "tr", "th", "td",
		"span", "div",
	)

	policy.AllowAttrs("href", "target", "rel").OnElements("a")
	policy.AllowAttrs("src", "alt", "width", "height").OnElements("img")
	policy.AllowAttrs("class", "style").Globally()
	policy.AllowAttrs("start", "type").OnElements("ol")
	policy.AllowAttrs("type").OnElements("ul")

	policy.AllowStandardURLs()
	policy.AllowURLSchemes("https", "http", "mailto", "tel")
}

func HTML(input string) string {
	if input == "" {
		return ""
	}
	return policy.Sanitize(input)
}
