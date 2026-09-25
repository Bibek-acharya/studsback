package sanitize

import (
	"strings"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
)

// MaxRichTextLength caps the size of a stored rich-text body. Quill output
// grows quickly with embedded images, so descriptions are truncated to a
// generous but bounded length after sanitizing.
const MaxRichTextLength = 20000

// richTextPolicy is an ADDITIVE, tightened sibling of the shared `policy` in
// sanitize.go. It keeps normal editor (Quill/CKEditor) formatting working while
// dropping everything that can execute or exfiltrate:
//
//   - script/style/iframe/object/embed/form elements (allow-list),
//   - every on* event handler attribute (allow-list),
//   - javascript:, vbscript:, data: and unknown URL schemes
//     (AllowStandardURLs + an explicit scheme allow-list),
//   - the global `style` attribute the shared policy still allows,
//   - target=_blank without rel="noopener" (AddTargetBlankToFullyQualifiedLinks
//     rewrites fully-qualified links to rel="noopener noreferrer").
//
// The shared policy is intentionally left untouched so no other module changes
// behavior; use RichText for admin-authored rich text and HTML for everything
// that already relies on the legacy policy.
var richTextPolicy = newRichTextPolicy()

func newRichTextPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()

	p.AllowElements(
		// Block-level text formatting emitted by Quill and its common peers.
		"p", "br", "hr",
		"h1", "h2", "h3", "h4", "h5", "h6",
		"blockquote", "pre", "code",
		"ol", "ul", "li",
		"div", "span", "section", "figure", "figcaption",
		// Inline formatting.
		"strong", "b", "em", "i", "u", "s", "strike", "del", "ins", "mark",
		"sub", "sup", "small",
		// Links and media.
		"a", "img",
		// Quill's table module.
		"table", "caption", "colgroup", "col",
		"thead", "tbody", "tfoot", "tr", "th", "td",
	)

	// Presentational hooks only. `style` is deliberately NOT allowed here.
	p.AllowAttrs("class").OnElements(
		"p", "div", "span", "section", "figure", "figcaption",
		"h1", "h2", "h3", "h4", "h5", "h6",
		"blockquote", "pre", "code", "li",
		"ol", "ul", "strong", "b", "em", "i", "u", "s", "sub", "sup",
		"table", "caption", "thead", "tbody", "tfoot", "tr", "th", "td",
	)
	p.AllowAttrs("href", "title", "target", "rel").OnElements("a")
	p.AllowAttrs("src", "alt", "title", "width", "height").OnElements("img")
	p.AllowAttrs("start", "type").OnElements("ol")
	p.AllowAttrs("type").OnElements("ul")
	p.AllowAttrs("colspan", "rowspan", "scope", "align", "valign").OnElements("th", "td")
	p.AllowAttrs("align", "valign").OnElements("table", "thead", "tbody", "tfoot", "tr", "col", "colgroup", "caption")
	p.AllowAttrs("cite").OnElements("blockquote", "q")

	p.AllowStandardURLs()
	p.AllowRelativeURLs(true)
	p.AllowURLSchemes("https", "http", "mailto", "tel")
	p.RequireNoFollowOnLinks(true)
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)

	return p
}

// RichText sanitizes admin-authored rich text with the tightened policy and
// caps the result at MaxRichTextLength characters. It is safe to call with
// plain text: plain text passes through unchanged apart from trimming.
func RichText(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	out := strings.TrimSpace(richTextPolicy.Sanitize(trimmed))
	return truncateUTF8(out, MaxRichTextLength)
}

// TruncatePlainText caps a plain-text field at max bytes on a rune boundary.
func TruncatePlainText(input string, max int) string {
	trimmed := strings.TrimSpace(input)
	if max <= 0 || len(trimmed) <= max {
		return trimmed
	}
	return truncateUTF8(trimmed, max)
}

func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return strings.TrimSpace(cut)
}
