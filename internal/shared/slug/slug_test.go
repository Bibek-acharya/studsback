package slug

import (
	"strings"
	"testing"
)

// GenerateUnique must terminate and return a slug that differs from its input.
//
// It used to loop forever for any base slug in [79, 80] bytes. The suffix was
// appended (base + "-12345" = 85 bytes), then the whole string was truncated to
// 80 bytes, which cut the suffix off and left the trailing "-", and TrimRight
// then removed that "-". The candidate collapsed back to exactly `base`, so
// exists(base) stayed true and the loop re-queried the same row forever. That
// is the wall of identical
//
//	SELECT count(*) FROM news WHERE slug = '...'
//
// queries in the SLOW SQL log: base was 79 bytes, one byte below the limit.
func TestGenerateUnique_TerminatesForLongBaseSlug(t *testing.T) {
	// 79 bytes: one below the 80-byte cap, so the old suffix-then-truncate
	// round trip landed exactly on the boundary that erased the suffix.
	base := Generate("edu-tu-iost-publishes-exam-application-form-notice-for-post-graduate-first-year")
	if len(base) != 79 {
		t.Fatalf("fixture assumption broken: base is %d bytes, want 79 (%q)", len(base), base)
	}

	// The row from the log is already taken. Model a couple more collisions so
	// the fix has to walk forward, and cap the probes so a regression shows up
	// as a failure instead of a hung test binary.
	taken := map[string]bool{
		base: true,
		base + "-1": true,
	}
	queries := 0
	exists := func(candidate string) bool {
		queries++
		if queries > 20 {
			t.Fatalf("GenerateUnique probed %d candidates without converging; it is not making progress", queries)
		}
		return taken[candidate]
	}

	got := GenerateUnique("edu-tu-iost-publishes-exam-application-form-notice-for-post-graduate-first-year", exists)

	if got == base {
		t.Errorf("GenerateUnique returned the colliding base %q; it must return a distinct slug", got)
	}
	if len(got) > 80 {
		t.Errorf("slug is %d bytes, must be <= 80: %q", len(got), got)
	}
	if strings.HasSuffix(got, "-") {
		t.Errorf("slug must not end in a bare dash: %q", got)
	}
}

// Every byte length in the collision-prone window must produce a distinct,
// in-budget slug. The old bug was specific to bases near the cap, so pin the
// whole window rather than one fixture.
func TestGenerateUnique_DistinctForEveryBaseLength(t *testing.T) {
	for n := 1; n <= 80; n++ {
		title := strings.Repeat("a", n)

		exists := func(candidate string) bool { return candidate == Generate(title) }
		got := GenerateUnique(title, exists)

		if got == Generate(title) {
			t.Errorf("base length %d: GenerateUnique returned the colliding base", n)
		}
		if len(got) > 80 {
			t.Errorf("base length %d: slug is %d bytes, must be <= 80", n, len(got))
		}
	}
}

// The common path must not regress into extra probes: a free title is used
// as-is after a single existence check.
func TestGenerateUnique_UnusedTitleIsReturnedAsIs(t *testing.T) {
	probes := 0
	got := GenerateUnique("NIST College", func(string) bool {
		probes++
		return false
	})

	if got != "nist-college" {
		t.Errorf("got %q, want %q", got, "nist-college")
	}
	if probes != 1 {
		t.Errorf("probed %d times, want exactly 1", probes)
	}
}
