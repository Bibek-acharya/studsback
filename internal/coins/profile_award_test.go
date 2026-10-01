// Unit tests for the profile award's pure logic: the threshold arithmetic and the
// step-count derivation. No database, no ledger.
//
// These are the tests that pin the property the whole slice exists to get right —
// a completion percentage that goes DOWN must not re-earn — and they are unit
// tests because the property is a pure function of (percent, instalments). The
// database tests in profile_award_pg_test.go prove the same property end to end;
// this file proves the arithmetic itself, which is where an off-by-one would
// otherwise hide.
package coins

import "testing"

// TestProfileStepThresholds pins the ladder to the documented shape: five
// instalments divide 100 evenly, so the thresholds are exactly 20/40/60/80/100
// and the fifth is paid only at a genuinely complete profile.
func TestProfileStepThresholds(t *testing.T) {
	want := []int{20, 40, 60, 80, 100}
	for i, w := range want {
		step := i + 1
		if got := ProfileStepThresholdPercent(step, 5); got != w {
			t.Errorf("step %d threshold = %d, want %d", step, got, w)
		}
	}
}

// TestProfileStepThresholdsNonDivisor is the case the naive arithmetic gets
// wrong. With three instalments, 100/3 is 33, so `step * (100/instalments)` pays
// the last step at 99% — leaving a student with a 100% profile who has not
// earned the whole award. The ceil form puts it at 100.
func TestProfileStepThresholdsNonDivisor(t *testing.T) {
	cases := []struct {
		instalments int64
		want        []int
	}{
		{3, []int{34, 67, 100}},
		{6, []int{17, 34, 50, 67, 84, 100}},
		{7, []int{15, 29, 43, 58, 72, 86, 100}},
		{1, []int{100}},
		{4, []int{25, 50, 75, 100}},
	}
	for _, c := range cases {
		for i, w := range c.want {
			step := int64(i + 1)
			if got := ProfileStepThresholdPercent(int(step), c.instalments); got != w {
				t.Errorf("instalments=%d step %d threshold = %d, want %d",
					c.instalments, step, got, w)
			}
		}
	}
}

// TestProfileStepThresholdFinalStepIsAlwaysComplete is the invariant behind the
// test above, stated once: for ANY instalment count, the last step is paid only at
// 100%. If this ever fails, a student can hold a complete profile and still be
// owed coins, and no amount of configuration would make that consistent.
func TestProfileStepThresholdFinalStepIsAlwaysComplete(t *testing.T) {
	for instalments := int64(1); instalments <= 40; instalments++ {
		last := ProfileStepThresholdPercent(int(instalments), instalments)
		if last != 100 {
			t.Errorf("instalments=%d: last step threshold = %d, want 100", instalments, last)
		}
	}
}

// TestProfileStepThresholdsMonotonic guards the property stepsEarned relies on
// when it breaks out of its loop early: thresholds must strictly increase.
func TestProfileStepThresholdsMonotonic(t *testing.T) {
	for instalments := int64(1); instalments <= 40; instalments++ {
		prev := 0
		for step := 1; step <= int(instalments); step++ {
			got := ProfileStepThresholdPercent(step, instalments)
			if got <= prev {
				t.Errorf("instalments=%d step %d threshold %d is not above %d",
					instalments, step, got, prev)
			}
			prev = got
		}
	}
}

// TestStepsEarned is the threshold crossing table.
func TestStepsEarned(t *testing.T) {
	cases := []struct {
		percent int
		want    int
	}{
		{0, 0},
		{1, 0},
		{19, 0},
		{20, 1}, // exactly step 1
		{21, 1}, // just past it, still one step
		{39, 1},
		{40, 2}, // exactly step 2
		{59, 2},
		{60, 3},
		{79, 3},
		{80, 4},
		{99, 4},
		{100, 5}, // complete: all five
	}
	for _, c := range cases {
		if got := stepsEarned(c.percent, 5); got != c.want {
			t.Errorf("stepsEarned(%d%%, 5 instalments) = %d, want %d", c.percent, got, c.want)
		}
	}
}

// TestStepsEarnedNeverExceedsInstalments is the structural half of the 25-coin
// ceiling. stepsEarned cannot return more steps than the ladder has, so there is
// no code path that could even ask for a sixth instalment — the 5x5 product is
// bounded by the loop, not only by the unique constraint.
func TestStepsEarnedNeverExceedsInstalments(t *testing.T) {
	for instalments := int64(1); instalments <= 20; instalments++ {
		for percent := 0; percent <= 100; percent++ {
			if got := stepsEarned(percent, instalments); got > int(instalments) {
				t.Fatalf("stepsEarned(%d%%, %d) = %d, above the ladder length",
					percent, instalments, got)
			}
		}
	}
}

// TestStepsEarnedZeroInstalments is the misconfiguration guard. Zero instalments
// must yield zero steps rather than dividing by zero or looping forever.
func TestStepsEarnedZeroInstalments(t *testing.T) {
	for _, in := range []int64{0, -1} {
		if got := stepsEarned(100, in); got != 0 {
			t.Errorf("stepsEarned(100%%, %d) = %d, want 0", in, got)
		}
	}
}

// TestStepsEarnedIsMonotonic is what makes "award every unclaimed step up to
// target" correct: a higher completion can never earn FEWER steps than a lower
// one. Without this, a percentage that went backwards through a refactor would
// silently stop paying.
func TestStepsEarnedIsMonotonic(t *testing.T) {
	for _, instalments := range []int64{1, 3, 4, 5, 6, 7} {
		prev := 0
		for percent := 0; percent <= 100; percent++ {
			got := stepsEarned(percent, instalments)
			if got < prev {
				t.Errorf("instalments=%d: stepsEarned dropped from %d to %d at %d%%",
					instalments, prev, got, percent)
			}
			prev = got
		}
	}
}

// TestProfileStepAwardCode pins the wire format. The award_code is part of the
// contract with anything that reads reward_grant later (support tooling, the
// referral phase), so the exact string is asserted rather than assumed.
func TestProfileStepAwardCode(t *testing.T) {
	cases := map[int]string{
		1: "PROFILE_STEP:1",
		3: "PROFILE_STEP:3",
		5: "PROFILE_STEP:5",
	}
	for step, want := range cases {
		if got := ProfileStepAwardCode(step); got != want {
			t.Errorf("ProfileStepAwardCode(%d) = %q, want %q", step, got, want)
		}
	}
}

// TestProfileStepAwardCodesAreDistinct is the property the UNIQUE constraint
// relies on: two different steps must never produce the same award_code, or the
// ceiling would be lower than configured for reasons nobody could see.
func TestProfileStepAwardCodesAreDistinct(t *testing.T) {
	seen := map[string]int{}
	for step := 1; step <= 5; step++ {
		code := ProfileStepAwardCode(step)
		if prev, dup := seen[code]; dup {
			t.Fatalf("steps %d and %d share award_code %q", prev, step, code)
		}
		seen[code] = step
	}
}
