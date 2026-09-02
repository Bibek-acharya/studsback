package ai

import (
	"strings"
	"testing"
)

func TestValidateResponse(t *testing.T) {
	s := &Service{}
	ctx := []contextResult{{Type: "college", Title: "Kathmandu University"}}

	t.Run("valid json keeps cited answer", func(t *testing.T) {
		raw := `{"answer": "KU offers BE [College: Kathmandu University].", "sources": ["College: Kathmandu University"]}`
		got := s.validateResponse(raw, ctx)
		if !strings.Contains(got, "KU offers BE") {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("fenced json is extracted", func(t *testing.T) {
		raw := "```json\n{\"answer\": \"KU exists [College: Kathmandu University].\", \"sources\": [\"College: Kathmandu University\"]}\n```"
		got := s.validateResponse(raw, ctx)
		if !strings.Contains(got, "KU exists") {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("plain text reply is shown, not database fallback", func(t *testing.T) {
		raw := "The best colleges in Nepal include Kathmandu University [College: Kathmandu University]."
		got := s.validateResponse(raw, ctx)
		if strings.Contains(got, "don't see that") {
			t.Fatalf("parse failure must not return the database fallback, got %q", got)
		}
		if got != raw {
			t.Fatalf("raw reply should be returned verbatim, got %q", got)
		}
	})

	t.Run("paraphrased citation with whitespace mismatch is kept", func(t *testing.T) {
		raw := `{"answer": "Programs include BEc-LL.B [Course: Bachelor of Economics & Bachelor of Law(BEc-LL.B)].", "sources": ["Course: Bachelor of Economics & Bachelor of Law(BEc-LL.B)"]}`
		ctx := []contextResult{{Type: "course", Title: "Bachelor of Economics & Bachelor of Law (BEc-LL.B)"}}
		got := s.validateResponse(raw, ctx)
		if !strings.Contains(got, "Programs include") || strings.Contains(got, "don't see that") {
			t.Fatalf("whitespace-differing citation must validate, got %q", got)
		}
	})

	t.Run("unrelated citation is stripped and answer falls back", func(t *testing.T) {
		raw := `{"answer": "MIT is great [College: MIT].", "sources": ["College: MIT"]}`
		got := s.validateResponse(raw, ctx)
		if !strings.Contains(got, "don't see that") {
			t.Fatalf("unrelated source must trigger fallback, got %q", got)
		}
	})

	t.Run("empty reply falls back", func(t *testing.T) {
		got := s.validateResponse("", ctx)
		if !strings.Contains(got, "don't see that") {
			t.Fatalf("got %q", got)
		}
	})
}
