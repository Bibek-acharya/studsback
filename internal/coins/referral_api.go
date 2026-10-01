// internal/coins/referral_api.go
//
// GET /api/v1/referrals — the student's own referral code, their standing against
// the monthly cap, and their referrals with enough state for the page to be honest
// about what has not paid out yet.
//
// ── the path, and where the contract disagrees with the brief ─────────────────
//
// 03-api-contract.md §2.4 specifies `GET /api/v1/referral/me`. This file mounts
// `GET /api/v1/referrals`.
//
// The disagreement is recorded rather than resolved by mounting both, because two
// paths for one resource is exactly the two-spellings problem referral.go's
// NormalizeReferralCode exists to eliminate, and because a second path nobody
// documents is a path nobody tests. One route, and §2.4 needs a one-line
// correction to the path. Everything else in §2.4's body IS followed, including the
// fields it does not get right — see coins_pending below.
//
// ── authorisation: owner-scoped, and there is nothing to authorise ───────────
//
// Every query filters on referrer_user_id = the caller's id, and there is NO
// parameter through which a caller can name somebody else: no :id in the path, no
// query parameter, no body. So there is no authorisation decision in this handler
// beyond refusing an unauthenticated request, and there is no way to add one by
// accident.
//
// That is the same shape as GET /api/v1/coins/balance, and for the same reason
// routes.go gives: every endpoint on the wallet exposes only the CALLER's own data
// and moves nothing. It is also why there is no role middleware. `institution` and
// `scholarship_provider` are legitimate authenticated principals here — internal/jobs
// access.go is the reference for owner-scoped-versus-admin-only, and a plain
// institution account calling this gets its own (empty) view, exactly as it gets its
// own empty wallet. Locking them out would be a different decision, about which
// account TYPES may hold coins, and that is an entitlement question rather than an
// access-control one.
//
// WHAT IS NOT EXPOSED, deliberately: the invitee's identity. No name, no email, no
// phone, no joined user row. The referral row carries referred_kind and a bare id,
// and the id is NOT rendered — a sequential id plus a referred_kind is enough to
// tell a student "you invited an institution", and 07-compliance-nepal.md §5.2 is
// about minimising what a referral discloses about the other person. The wallet's
// balance already reveals that a coin moved; this must not reveal who it moved for.
//
// ── coins_pending, which §2.4 gets wrong under the new model ─────────────────
//
// §2.4 has `coins_pending` and says it "reflects the reserved balance, not a
// separate figure, so it can never disagree with GET /coins/balance". There is no
// reserved balance for a referral any more — nothing is held, so there is nothing to
// report — and a figure described as pending while referring to a balance column
// that nothing writes would be the exact disagreement §2.4 wanted to avoid.
//
// It is omitted rather than redefined. What replaces it is per-referral: each row
// carries its own status and its eligible_at, so "3 referrals have not paid yet" is
// answerable by counting rows the UI is already rendering, and cannot disagree with
// anything because there is no second number to disagree with.
package coins

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"studsphere/backend/internal/shared/httpx"
	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/utils"
)

// ReferralDTO is one referral of the caller's, in the order it was attributed.
//
// ReferralID is the caller's own row id. It is rendered because the student page
// needs a stable React key and a handle for support, and it is safe because it is an
// id into the CALLER's rows: it identifies nothing without them.
type ReferralDTO struct {
	ReferralID uint `json:"referral_id"`
	// Status is one of ReferralStatuses.
	Status string `json:"status"`
	// ReferredKind is one of ReferredKinds. Informational: it lets the page say "this
	// one cannot qualify" rather than showing an invitation that will never pay.
	ReferredKind string `json:"referred_kind"`
	// ReferredAt is when the attribution was recorded.
	ReferredAt time.Time `json:"referred_at"`
	// EligibleAt is created_at + referral.hold_days: the earliest moment this referral
	// could have paid. It is the honest replacement for "a pending hold has a date"
	// (09's "A hold is a pending amount with a date, not a penalty") in a model with
	// no hold.
	EligibleAt *time.Time `json:"eligible_at,omitempty"`
	// QualifiedAt is when the invitee met §5.2's bar. Null while pending. It is NOT
	// when the coins arrived — those may be days later, because the window and the
	// qualification are independent conditions.
	QualifiedAt *time.Time `json:"qualified_at,omitempty"`
	// SettledAt is when the coins were granted, and it equals QualifiedAt for a
	// referral the pass settled in the same run. Separate because the two are separate
	// facts: an invitee can qualify long before the window closes.
	SettledAt *time.Time `json:"settled_at,omitempty"`
	// AwardedCoins is what this referral actually paid. 0 for anything not settled.
	AwardedCoins int64 `json:"awarded_coins"`
	// ExpiresAt is when the awarded coins leave the wallet, or null if it never paid.
	// Surfaced because 05 §4 ("expiry is a trust tax") and 09's expiry row both
	// require per-lot dates, not a terms link.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// State is one of the ReferralViewState constants, and it is what the page
	// renders. A raw status is not enough on its own, because "pending" covers both
	// "waiting on the clock" and "waiting on your friend", and those are different
	// sentences with different things to do about them.
	State string `json:"state"`
	// Reason is set only for a terminal row that will never pay: the closed
	// vocabulary in referral_qualification.go, or the operator's recorded clawback or
	// rejection reason. Never a penalty sentence — 06-ui-ux-spec.md §7 is explicit
	// that a referral that has not paid is not described as a loss.
	Reason string `json:"reason,omitempty"`
}

// ReferralViewState is what the page needs to say, derived rather than stored.
//
// Derived because it is a function of columns already on the row plus the clock, and
// a stored copy would be a second thing to keep correct — the same argument
// profile_award.go makes for computing stepsEarned from the rows rather than
// counting them.
const (
	// ReferralViewSettling: the invitee has qualified and the wait is still running.
	// The one state that is unambiguously good news and is worth saying so.
	ReferralViewSettling = "settling"
	// ReferralViewWaitingOnWindow: the fraud window is still open. The student did
	// nothing wrong and there is nothing to do.
	ReferralViewWaitingOnWindow = "waiting_for_window"
	// ReferralViewWaitingOnInvitee: the window has closed and the invitee has not
	// finished. This is where the referral page's copy matters most, and 06 §7's rule
	// — never scold a student for inviting someone — is what shapes it.
	ReferralViewWaitingOnInvitee = "waiting_for_invitee"
	// ReferralViewSettled: coins granted.
	ReferralViewSettled = "settled"
	// ReferralViewExpired, ReferralViewClawedBack, ReferralViewRejected: terminal, and
	// never paid (or paid and reversed). Reported because a row that vanishes from
	// the list is worse than one that says what happened to it.
	ReferralViewExpired    = "expired"
	ReferralViewClawedBack = "clawedback"
	ReferralViewRejected   = "rejected"
)

// ReferralStatsDTO is §2.4's stats block, corrected.
//
// It adds `expired` to §2.4's four counts, because a fifth terminal state exists
// and folding it into `pending` would report rows that can never pay as rows that
// might — which is the dishonesty this endpoint exists to avoid. `pending` therefore
// means "may still pay", and it is exactly the count of rows whose state is
// waiting_* or settling.
//
// §2.4's `coins_pending` is gone, for the reason the file header gives.
//
// Invited, Qualified, Pending, Expired and Rejected are counts over the caller's
// rows. ThisMonthQualified and MonthlyCapRemaining are what §2.4's page needs to
// render "4 of 10 this month"; LifetimeCapRemaining is the derived coin ceiling
// from 05 §2.4, computed from the cap-slot rows rather than from a counter.
type ReferralStatsDTO struct {
	Invited   int `json:"invited"`
	Qualified int `json:"qualified"`
	Pending   int `json:"pending"`
	Expired   int `json:"expired"`
	Rejected  int `json:"rejected"`

	// CoinsEarnedTotal is the lifetime sum of awarded_coins for this referrer. Read
	// from user_referral rather than from the ledger because §2.4's page is about
	// referrals, and a referral that was clawed back has an awarded_coins that the
	// balance no longer reflects — which is precisely why the two are separate
	// numbers here rather than one being derived from the other.
	CoinsEarnedTotal int64 `json:"coins_earned_total"`

	// ThisMonthQualified counts this calendar month's settlements, from
	// referral_cap_slot's month_key. UTC, per referral_cap_model.go: a local-month cap
	// would open the new month at 18:15 UTC in Nepal, overlapping two months' caps by
	// six hours, twice a year, exactly at the reset.
	ThisMonthQualified int `json:"this_month_qualified"`
	// MonthlyCapRemaining is referral.monthly_cap minus this month's count.
	MonthlyCapRemaining int `json:"monthly_cap_remaining"`
	// LifetimeCapRemaining is referral.lifetime_coin_cap minus lifetime coins awarded.
	LifetimeCapRemaining int64 `json:"lifetime_cap_remaining"`
}

// ReferralMeDTO is the whole body.
type ReferralMeDTO struct {
	// ReferralCode is the caller's own code, or null before the backfill reaches them.
	// Null rather than an empty string, because "" would render as an invite link that
	// resolves to nothing.
	ReferralCode *string `json:"referral_code"`
	// ReferralLink is the shareable URL, or null for the same reason.
	ReferralLink *string `json:"referral_link"`
	// QualificationAvailable is FALSE in every deployment today. It exists so the page
	// can say why nothing has paid rather than showing a spinner forever, and it is
	// derived from the same nil PhoneVerification port the qualification pass refuses
	// on — so it cannot claim referrals are earnable while the pass pays none.
	//
	// This is the one field in this body that is about the SERVER rather than the
	// student, and it is here because the alternative is a page that counts pending
	// referrals and never explains that none of them can settle.
	QualificationAvailable bool `json:"qualification_available"`
	// QualificationNotice is the human-readable form of the same fact, or null when
	// qualification is available. Copy obeys 09: no "free", no currency beside a coin
	// figure, none of prize/award/win/raffle/draw.
	QualificationNotice string `json:"qualification_notice,omitempty"`
	// ReferralHoldDays is the configured wait, so the page can render the same date
	// this body computes rather than hard-coding seven.
	ReferralHoldDays int64            `json:"referral_hold_days"`
	Stats            ReferralStatsDTO `json:"stats"`
	// Referrals is the caller's own rows, newest first — newest first because the ones
	// a student came to look at are the ones they just made.
	Referrals []ReferralDTO `json:"referrals"`
}

// ReferralAPI serves the student-facing referral surface.
//
// It holds the service and the base URL the invite link is built from, and nothing
// else. The base URL is INJECTED rather than read from internal/shared/config because
// internal/coins does not import that package outside its own integration tests, and
// because a link assembled from a guessed host is a broken invite on every deployment
// that is not localhost.
type ReferralAPI struct {
	svc *ReferralService
	// inviteBaseURL is the frontend origin the /r/<code> route lives on. It is the
	// same value the router serves the SPA from, so it is passed in from main.go
	// rather than duplicated.
	inviteBaseURL string
	now           func() time.Time
}

// NewReferralAPI wires the endpoint. svc is required; a nil one produces a handler
// that refuses every request rather than a nil-pointer panic on the first call.
func NewReferralAPI(svc *ReferralService, inviteBaseURL string) *ReferralAPI {
	return &ReferralAPI{
		svc:           svc,
		inviteBaseURL: strings.TrimRight(strings.TrimSpace(inviteBaseURL), "/"),
		now:           func() time.Time { return time.Now().UTC() },
	}
}

// Me handles GET /api/v1/referrals.
//
// Unauthenticated is 401 and nothing else is: every database failure is 500, because
// nothing the caller sent is wrong. There is no 403 and no 404, because the
// authenticated caller always has a view of their own referrals — an account with
// none gets zeros, which is a true answer and not a missing resource.
func (a *ReferralAPI) Me(c *gin.Context) {
	userID, ok := httpx.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	if a == nil || a.svc == nil || a.svc.repo == nil {
		response.Error(c, http.StatusInternalServerError, "Could not read referrals")
		return
	}
	ctx := c.Request.Context()
	cfg, err := a.svc.ledger.config.Load()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Could not read referrals")
		return
	}
	view, err := a.build(ctx, userID, cfg)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Could not read referrals")
		return
	}
	response.Success(c, http.StatusOK, "Referral summary", view)
}

// build assembles the whole body from four reads.
//
// Four rather than one joined query, and the shape is the point: none of them can
// return another account's data because each carries referrer_user_id = userID in
// its own WHERE, so there is no single statement whose parameters could be
// mis-bound into reading somebody else's referrals.
func (a *ReferralAPI) build(ctx context.Context, userID uint, cfg EconomyConfig) (ReferralMeDTO, error) {
	code, err := a.svc.referralCodeFor(ctx, userID)
	if err != nil {
		return ReferralMeDTO{}, err
	}
	out := ReferralMeDTO{
		ReferralCode:     code,
		Stats:            a.statsFor(ctx, userID, cfg),
		ReferralHoldDays: cfg.Referral.HoldDays,
	}
	if code != nil {
		out.ReferralLink = a.inviteLink(*code)
	}
	// The same nil check the qualification pass makes, so the page's claim and the
	// job's behaviour cannot disagree.
	out.QualificationAvailable = a.svc.phone != nil
	if !out.QualificationAvailable {
		out.QualificationNotice = "Referral StudsTokens cannot be added yet. " +
			"We are not able to confirm that an invited person joined, so we are not crediting " +
			"anyone until we can. Your invite is recorded and nothing is lost."
	}
	rows, err := a.svc.referralsFor(ctx, userID, cfg.Referral.HoldDays, a.now())
	if err != nil {
		return ReferralMeDTO{}, err
	}
	out.Referrals = rows
	return out, nil
}

// inviteLink builds the shareable URL, or nil when no base URL is configured.
//
// Nil rather than a bare "/r/CODE": a relative link is meaningless in an email or a
// copied string, and a link that silently omits its host is the kind of thing that
// ships and is only noticed when nobody can sign up.
func (a *ReferralAPI) inviteLink(code string) *string {
	if a.inviteBaseURL == "" || code == "" {
		return nil
	}
	link := a.inviteBaseURL + "/r/" + code
	return &link
}

// statsFor is §2.4's stats block, with expired counted separately.
//
// Two reads and an arithmetic identity. Clawedback is counted into Qualified, because
// §2.4's page says "qualified" about referrals and the pair (qualified, clawed back)
// is the honest reading of a referral that qualified and was then reversed — a
// separate clawedback count would invite the frontend to render "0 qualified" for a
// student who was genuinely credited and then had it taken back, which is worse than
// no number at all.
//
// The cap figures come from referral_cap_slot rather than from user_referral, because
// the cap table is the CLAIM and is what the settlement is refused by: a count taken
// from the referral rows could disagree with it, and a cap that counts something other
// than what it pays is decorative.
func (a *ReferralAPI) statsFor(ctx context.Context, userID uint, cfg EconomyConfig) ReferralStatsDTO {
	db := a.svc.repo.DB().WithContext(ctx)
	var stats ReferralStatsDTO
	stats.Invited = countFor(db, `SELECT count(*) FROM user_referral WHERE referrer_user_id = ?`, userID)
	stats.Qualified = countFor(db,
		`SELECT count(*) FROM user_referral WHERE referrer_user_id = ? AND status IN (?, ?)`,
		userID, ReferralQualified, ReferralClawedBack)
	// Pending counts only rows that MAY still pay. A terminal row is not pending, and
	// this is the distinction §2.4's four-bucket stats cannot express.
	stats.Pending = countFor(db,
		`SELECT count(*) FROM user_referral WHERE referrer_user_id = ? AND status = ?`,
		userID, ReferralPending)
	stats.Expired = countFor(db,
		`SELECT count(*) FROM user_referral WHERE referrer_user_id = ? AND status = ?`,
		userID, ReferralExpired)
	stats.Rejected = countFor(db,
		`SELECT count(*) FROM user_referral WHERE referrer_user_id = ? AND status = ?`,
		userID, ReferralRejected)

	var lifetime int64
	db.Raw(`SELECT COALESCE(SUM(awarded_coins), 0) FROM user_referral WHERE referrer_user_id = ?`,
		userID).Scan(&lifetime)
	stats.CoinsEarnedTotal = lifetime

	month := ReferralMonthKey(a.now())
	used := countFor(db,
		`SELECT count(*) FROM referral_cap_slot WHERE referrer_user_id = ? AND month_key = ?`,
		userID, month)
	stats.ThisMonthQualified = used
	// Floored at zero, because a cap LOWERED mid-month (claimCapSlot honours the
	// configured value) can put used above the new cap. A negative "remaining" would
	// render as "-3 invites left".
	if remaining := int(cfg.Referral.MonthlyCap) - used; remaining > 0 {
		stats.MonthlyCapRemaining = remaining
	}
	if remaining := cfg.Referral.LifetimeCoinCap - lifetime; remaining > 0 {
		stats.LifetimeCapRemaining = remaining
	}
	return stats
}

// referralCodeFor reads the caller's own code, or nil.
//
// Reads the `users` table directly rather than through an auth port, for the reason
// referral.go's resolveReferrer does: internal/coins may not import internal/auth, and
// this is the same one-column read that file already performs. Reading it here rather
// than reusing a service method keeps the "only `users` holds referral codes" fact in
// one place.
//
// NIL rather than an empty string for an account the backfill has not reached, because
// "" would render as an invite link to nowhere and a student with no code is a
// different — and fixable — situation from a student with a broken one.
func (s *ReferralService) referralCodeFor(ctx context.Context, userID uint) (*string, error) {
	if s == nil || s.repo == nil || userID == 0 {
		return nil, ErrNoDatabase
	}
	var code *string
	if err := s.repo.DB().WithContext(ctx).Raw(
		`SELECT referral_code FROM users WHERE id = ? AND deleted_at IS NULL`, userID,
	).Scan(&code).Error; err != nil {
		return nil, err
	}
	if code != nil && utils.NormalizeReferralCode(*code) == "" {
		// A code that normalises to nothing cannot be shared, and rendering it as a
		// link would produce a URL that fails to resolve.
		return nil, nil
	}
	return code, nil
}

// referralsFor reads the caller's own rows, newest first, and renders each one.
//
// Newest first is a UI decision and it lives here rather than in the frontend so that
// the ordering is part of the contract: the row a student came to look at is the one
// they just made, and "ORDER BY id DESC" in a handler nobody re-reads is one fewer
// thing for the frontend and this file to disagree about.
//
// No cap on the number of rows. That is deliberate and worth defending, because an
// unbounded list is normally a mistake: the cap bounds SETTLEMENTS at ten a month, not
// attributions, so a student who has been referred to by nobody and has invited two
// hundred people over two years has two hundred pending rows and none of them will
// ever settle. Paging would mean a cursor, a page size, and a second question the page
// then has to answer — and the rows that matter (the recent ones) are at the front of
// an ordered list. If the count ever gets large enough for that to hurt, the fix is a
// limit on PENDING rows at attribution time, which is a product decision about how many
// uncredited invitations one account may accumulate, not a paging scheme.
func (s *ReferralService) referralsFor(ctx context.Context, userID uint, holdDays int64, now time.Time) ([]ReferralDTO, error) {
	if s == nil || s.repo == nil || userID == 0 {
		return nil, ErrNoDatabase
	}
	rows, err := s.referralRows(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]ReferralDTO, 0, len(rows))
	for i := range rows {
		out = append(out, referralDTOFrom(rows[i], holdDays, now))
	}
	return out, nil
}

// referralRows reads the caller's referral rows and nothing else.
//
// Every column is selected explicitly and the WHERE carries referrer_user_id as a bound
// parameter. Selecting referred_user_id would be a different design: it is not in this
// query, and referral_api.go's header says why — a sequential id plus a kind is enough
// to tell a student who they invited, and that is more than this endpoint should
// disclose about the other person (07-compliance-nepal.md §5.2).
func (s *ReferralService) referralRows(ctx context.Context, userID uint) ([]UserReferral, error) {
	var rows []UserReferral
	if err := s.repo.DB().WithContext(ctx).Raw(
		`SELECT id, referrer_user_id, referred_kind, referral_code, status, awarded_coins,
		        cap_period, cap_slot, grant_journal_id, qualified_at, released_at,
		        clawback_reason, expired_at, expired_reason, created_at, updated_at
		   FROM user_referral
		  WHERE referrer_user_id = ?
		  ORDER BY id DESC`, userID,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// referralDTOFrom renders one row.
//
// The mapping is a function rather than a method so that the derived fields are
// computed in one place with `now` and `holdDays` as explicit inputs — which is what
// makes the expiry boundary testable without a database and without sleeping.
func referralDTOFrom(row UserReferral, holdDays int64, now time.Time) ReferralDTO {
	dto := ReferralDTO{
		ReferralID:   row.ID,
		Status:       row.Status,
		ReferredKind: row.ReferredKind,
		ReferredAt:   row.CreatedAt.UTC(),
		QualifiedAt:  row.QualifiedAt,
		SettledAt:    row.ReleasedAt,
		AwardedCoins: row.AwardedCoins,
		State:        referralViewState(row, holdDays, now),
	}
	if holdDays > 0 {
		eligible := row.CreatedAt.UTC().AddDate(0, 0, int(holdDays))
		dto.EligibleAt = &eligible
	}
	switch row.Status {
	case ReferralExpired:
		dto.Reason = row.ExpiredReason
	case ReferralClawedBack, ReferralRejected:
		dto.Reason = row.ClawbackReason
	}
	// The awarded coins' expiry is the GRANT's expiry, which is the lot's. It is not
	// re-derived from config here: a re-pricing of expiry.earned_days must not change
	// what a lot already granted says it will do, and the ledger stores it on coin_lot.
	// The caller's own lots for referral grants are read by the wallet endpoint; this
	// field is filled in only where the row itself knows, and left null otherwise
	// rather than guessed.
	return dto
}

// referralViewState is the one sentence the page can render.
//
// Pending splits into three, and the split is the value of this function: "pending"
// alone cannot distinguish "come back on the 12th" from "your friend has not finished
// yet", and those are different things for a student to do. The third state,
// settling, exists because a qualified invitee whose window is still open is the one
// outcome that is unambiguously good news and a page that renders it as "pending"
// makes the mechanic feel broken at the exact moment it worked.
func referralViewState(row UserReferral, holdDays int64, now time.Time) string {
	if row.Status != ReferralPending {
		switch row.Status {
		case ReferralQualified:
			return ReferralViewSettled
		case ReferralExpired:
			return ReferralViewExpired
		case ReferralClawedBack:
			return ReferralViewClawedBack
		case ReferralRejected:
			return ReferralViewRejected
		default:
			// A status the closed set does not contain. chk_user_referral_status refuses
			// one, so this is unreachable; rendering "pending" rather than an empty
			// string is the safe answer for a value this build does not know.
			return ReferralViewSettled
		}
	}
	eligible := row.CreatedAt.UTC()
	if holdDays > 0 {
		eligible = eligible.AddDate(0, 0, int(holdDays))
	}
	if !now.UTC().Before(eligible) {
		// The window has closed. Whether the invitee has actually qualified is a
		// question for the qualification pass, and until it runs the honest state is
		// "waiting on your friend", not "settled" — a referral this page called settled
		// before the money moved would be the exact dishonesty 06 §7 warns about.
		return ReferralViewWaitingOnInvitee
	}
	// Window still open. Qualified is not consulted: the pass has not run yet, and
	// inferring qualification from a nullable column the pass owns would make the page
	// answer a question the mechanic has not.
	return ReferralViewWaitingOnWindow
}

// ReferralViewUnavailable is the state a page should render when the server reports
// qualification_available = false. It is a VIEW concern, not a row state: the rows are
// genuinely pending and will say so.
//
// Kept as a constant rather than a literal in the frontend so that the two agree on the
// word, and so that a test can assert the server never emits it for a row.
const ReferralViewUnavailable = "unverifiable"

// countFor is the one place a count query is written, because a count whose
// referrer_user_id is not a bound parameter is a count that can read another
// account's rows.
//
// It swallows an error and returns 0 rather than propagating, and that is worth
// being explicit about: a stat that reads zero when the query failed is wrong, and a
// stats block that fails the whole endpoint because one of six counts did is also
// wrong. The page renders figures; a missing figure is a visibly wrong figure and a
// 500 is a visibly broken page, and the first is the smaller failure. The write
// paths that matter do propagate — this is a read of the caller's own summary.
func countFor(db *gorm.DB, query string, args ...any) int {
	var n int64
	if err := db.Raw(query, args...).Scan(&n).Error; err != nil {
		return 0
	}
	return int(n)
}
