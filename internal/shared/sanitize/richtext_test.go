package sanitize

import (
	"strings"
	"testing"
)

func TestRichTextKeepsEditorFormatting(t *testing.T) {
	input := `<h2>Physics</h2><p><strong>Kinematics</strong> and <em>energy</em></p>` +
		`<ul><li>one</li><li>two</li></ul>` +
		`<ol start="3"><li>step</li></ol>` +
		`<p><a href="https://example.com/x" target="_blank">link</a></p>` +
		`<table><thead><tr><th>Year</th></tr></thead><tbody><tr><td>2081</td></tr></tbody></table>`

	out := RichText(input)

	for _, want := range []string{
		"<h2>Physics</h2>",
		"<strong>Kinematics</strong>",
		"<em>energy</em>",
		"<ul>", "<li>one</li>",
		`start="3"`,
		`href="https://example.com/x"`,
		"<table>", "<th>Year</th>", "<td>2081</td>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRichTextStripsScriptsAndHandlers(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		absent []string
	}{
		{
			name:   "script element",
			input:  `<p>ok</p><script>alert(1)</script>`,
			absent: []string{"<script", "alert(1)"},
		},
		{
			name:   "event handler attribute",
			input:  `<p onclick="steal()">ok</p>`,
			absent: []string{"onclick", "steal()"},
		},
		{
			name:   "image onerror",
			input:  `<img src="x.png" onerror="alert(1)">`,
			absent: []string{"onerror", "alert(1)"},
		},
		{
			name:   "javascript url",
			input:  `<a href="javascript:alert(1)">click</a>`,
			absent: []string{"javascript:"},
		},
		{
			name:   "data url",
			input:  `<a href="data:text/html;base64,PHNjcmlwdD4=">click</a>`,
			absent: []string{"data:text/html"},
		},
		{
			name:   "iframe and object",
			input:  `<iframe src="https://evil.test"></iframe><object data="x"></object>`,
			absent: []string{"<iframe", "<object"},
		},
		{
			name:   "style element",
			input:  `<style>body{display:none}</style><p>ok</p>`,
			absent: []string{"<style", "display:none"},
		},
		{
			name:   "inline style attribute",
			input:  `<p style="position:fixed;top:0">ok</p>`,
			absent: []string{"position:fixed", "style="},
		},
		{
			name:   "svg onload",
			input:  `<svg onload="alert(1)"></svg>`,
			absent: []string{"<svg", "onload"},
		},
		{
			name:   "form and input",
			input:  `<form action="https://evil.test"><input name="pw" type="password"></form>`,
			absent: []string{"<form", "<input"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := RichText(tc.input)
			for _, absent := range tc.absent {
				if strings.Contains(out, absent) {
					t.Errorf("output still contains %q:\n%s", absent, out)
				}
			}
		})
	}
}

func TestRichTextAddsNoopenerToBlankLinks(t *testing.T) {
	out := RichText(`<a href="https://example.com" target="_blank">x</a>`)
	if !strings.Contains(out, "noopener") {
		t.Errorf("expected rel=noopener on fully qualified blank link, got:\n%s", out)
	}
}

func TestRichTextTrimsAndCaps(t *testing.T) {
	if got := RichText("   "); got != "" {
		t.Errorf("blank input = %q, want empty", got)
	}
	if got := RichText(""); got != "" {
		t.Errorf("empty input = %q, want empty", got)
	}

	long := "<p>" + strings.Repeat("a", MaxRichTextLength*2) + "</p>"
	out := RichText(long)
	if len(out) > MaxRichTextLength {
		t.Errorf("length = %d, want <= %d", len(out), MaxRichTextLength)
	}
}

func TestRichTextLeavesPlainTextAlone(t *testing.T) {
	if got := RichText("Kinematics notes"); got != "Kinematics notes" {
		t.Errorf("plain text = %q", got)
	}
}

// The shared policy must keep its historical behavior: the tightened rich-text
// policy is additive and may not change existing modules.
func TestSharedPolicyUnchanged(t *testing.T) {
	out := HTML(`<p style="color:red">styled</p>`)
	if !strings.Contains(out, "style") {
		t.Errorf("shared policy should still keep style attributes, got:\n%s", out)
	}
}
