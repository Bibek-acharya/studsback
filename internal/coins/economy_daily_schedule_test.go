package coins

import (
	"testing"
	"time"
)

// The schedule, which is a pure function of two inputs and is therefore testable
// without a database or a clock. It is tested because its failure mode is invisible:
// a rollup that fires on the wrong schedule does not error, does not log, and stores a
// plausible number for the wrong day.

func TestTheRollupAlwaysFiresAtTheNextHourNotToday(t *testing.T) {
	// Called at 03:00 with a 02:00 hour — the hour has ALREADY passed today, so the
	// only safe answer is tomorrow. Returning today's 02:00 would be in the past, and a
	// timer set on a past instant fires IMMEDIATELY, which would roll up a day that is
	// still accruing — the exact failure the "always tomorrow" rule exists to prevent.
	from := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	got := nextRollupAt(from, 2)
	want := time.Date(2026, 10, 3, 2, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("next rollup from %s at hour 2 = %s, want %s — it must never be in the past",
			from.Format(time.RFC3339), got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
	if !got.After(from) {
		t.Error("the next rollup is not after now; the timer would fire immediately")
	}

	// Called at 01:00 with a 02:00 hour — today is still available.
	from = time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	got = nextRollupAt(from, 2)
	want = time.Date(2026, 10, 2, 2, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("next rollup from %s = %s, want %s — today's hour is still ahead", from.Format(time.RFC3339),
			got.Format(time.RFC3339), want.Format(time.RFC3339))
	}

	// EXACTLY on the hour. "Not before" means the boundary belongs to the NEXT run, so
	// two loops hitting the same instant cannot both roll up the same day.
	from = time.Date(2026, 10, 2, 2, 0, 0, 0, time.UTC)
	got = nextRollupAt(from, 2)
	want = time.Date(2026, 10, 3, 2, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("next rollup from exactly %s = %s, want %s — the boundary belongs to the following run",
			from.Format(time.RFC3339), got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

// The schedule must be ANCHORED TO MIDNIGHT UTC, not offset from process start. This is
// the difference between a rollup that lands at the same time every day and one that
// moves by however long the container took to boot — and it matters more here than for
// the reconciler, because every figure this job writes is indexed BY DAY.
func TestTheScheduleIsAnchoredToMidnightNotToProcessStart(t *testing.T) {
	// Two instances started at wildly different times must agree on the next run.
	morning := time.Date(2026, 10, 2, 6, 15, 0, 0, time.UTC)
	evening := time.Date(2026, 10, 2, 23, 45, 30, 0, time.UTC)
	if nextRollupAt(morning, 2) != nextRollupAt(evening, 2) {
		t.Errorf("the next run differs by start time: %s vs %s — the schedule must be anchored to midnight",
			nextRollupAt(morning, 2), nextRollupAt(evening, 2))
	}
	// And repeatedly waiting must not drift. This is what would happen if the
	// implementation added an interval to `from` rather than recomputing from midnight.
	from := morning
	for i := 0; i < 5; i++ {
		from = nextRollupAt(from, 2)
		if from.Hour() != 2 || from.Minute() != 0 || from.Second() != 0 {
			t.Fatalf("run %d scheduled at %s, want 02:00:00 exactly — the schedule drifted",
				i, from.Format(time.RFC3339))
		}
	}
}

// A LOCAL time must be normalised, not reinterpreted. A process whose clock is set to
// Kathmandu local time must still fire on the UTC hour the config named — the day
// BOUNDARY is UTC throughout this package and the schedule has to agree with it.
func TestTheScheduleNormalisesALocalClockToUTC(t *testing.T) {
	kathmandu := time.FixedZone("NPT", 5*60*45+15*60)
	// 2026-10-02 03:00 NPT is 2026-10-01 21:15 UTC.
	local := time.Date(2026, 10, 2, 3, 0, 0, 0, kathmandu)
	got := nextRollupAt(local, 2)
	if got.Location() != time.UTC {
		t.Errorf("the next run is in %s, want UTC", got.Location())
	}
	want := time.Date(2026, 10, 2, 2, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("next rollup from %s = %s, want %s", local.Format(time.RFC3339),
			got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

// A misconfigured hour must not produce a schedule that is not on a day boundary. The
// config loader already range-checks, but a caller constructing this directly should
// not be able to schedule the economy's only rollup at hour 25.
func TestAnOutOfRangeHourFallsBackToMidnightRatherThanDrifting(t *testing.T) {
	from := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	for _, hour := range []int{24, 25, 99, -1, -100} {
		got := nextRollupAt(from, hour)
		if !got.After(from) {
			t.Errorf("hour %d scheduled the next run at %s, which is not after %s",
				hour, got.Format(time.RFC3339), from.Format(time.RFC3339))
		}
		if !got.Equal(truncateToUTCDay(got)) {
			t.Errorf("hour %d scheduled the run at %s, which is not on a day boundary",
				hour, got.Format(time.RFC3339))
		}
	}
}

// The rollup covers YESTERDAY and THE DAY BEFORE, and never today. Two days rather
// than one because a deployment that skipped a run — or a process that was down across
// midnight — would otherwise leave a permanent hole, and a hole in the middle of a
// trend is invisible as a hole: it appears only as a step change nobody can explain.
//
// today is excluded because its row is still accruing. Storing a partial day would put
// a wrong number in the one row an operator is most likely to be looking at.
func TestTheRollupCoversYesterdayAndTheDayBeforeButNotToday(t *testing.T) {
	rolled := &recordingSweeper{}
	rolled.nowFunc = func() time.Time { return time.Date(2026, 10, 2, 2, 5, 0, 0, time.UTC) }

	rollupRecentDays(rolled)

	want := []string{"2026-10-01", "2026-09-30"}
	if len(rolled.days) != len(want) {
		t.Fatalf("rolled up %d days (%v), want %d", len(rolled.days), rolled.days, len(want))
	}
	for i, day := range rolled.days {
		if got := truncateToUTCDay(day).Format("2006-01-02"); got != want[i] {
			t.Errorf("day %d = %s, want %s", i, got, want[i])
		}
	}
	for _, day := range rolled.days {
		if truncateToUTCDay(day).Equal(truncateToUTCDay(rolled.nowUTC())) {
			t.Error("the rollup covered today, which is still accruing")
		}
	}
}

// A rollup that fails on one day must still attempt the next, and must not be fatal.
// A single bad day must not stop the series from advancing — a dashboard frozen on the
// day before the failure looks like a dead economy, and the log line is the only clue.
func TestOneFailedDayDoesNotStopTheNext(t *testing.T) {
	rolled := &recordingSweeper{failOn: "2026-10-01"}
	rolled.nowFunc = func() time.Time { return time.Date(2026, 10, 2, 2, 5, 0, 0, time.UTC) }

	rollupRecentDays(rolled)

	// BOTH days were attempted, in order, despite the first failing. The double
	// records ATTEMPTS, which is the property that matters: a loop that returned on
	// error would attempt one.
	if len(rolled.days) != 2 {
		t.Fatalf("attempted %d days (%v), want 2 — the failure stopped the loop",
			len(rolled.days), rolled.days)
	}
	want := []string{"2026-10-01", "2026-09-30"}
	for i, day := range rolled.days {
		if got := truncateToUTCDay(day).Format("2006-01-02"); got != want[i] {
			t.Errorf("attempt %d = %s, want %s", i, got, want[i])
		}
	}
}

// A nil sweeper must be a no-op. StartEconomyRollup is called from main.go where the
// argument is a constructor call, and a nil there should not panic a background job
// 23 hours before anyone notices.
func TestTheRollupOnANilSweeperIsANoOp(t *testing.T) {
	rollupRecentDays(nil)
}
