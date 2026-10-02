// internal/coins/support_view.go
//
// 04 §6 (Phase 4): GET /admin/coins/users/:id — the support view.
//
// ── what this endpoint is for ─────────────────────────────────────────────────
//
// One question, asked constantly, and unanswerable without this:
//	"why does this student have 30 coins?"
//
// The wallet shows a balance. The balance is a SUM over postings, so it is
// arithmetic with no visible operands, and a student who sees 30 rather than the
// 80 they earned has no way to reconcile it and neither does support. This
// endpoint is the answer: every journal, every posting, every lot, and the
// arithmetic that connects them.
//
// It is described in the plan as "the practical payoff of the ledger design", and
// that is the honest reading — a double-entry ledger earns its cost the first time
// somebody has to explain a balance, not the first time the balance is correct.
//
// ── the arithmetic, stated so a support engineer can check it by hand ─────────
//
//	available(user)  = SUM over the user's bucket accounts of (posted - reserved)
//	lots(user)       = every CoinLot, with granted / consumed / expired
//	journals(user)    = every journal WHERE scope = 'user' AND owner_user_id = ?
//	postings(journal) = the signed legs
//
// and the two that must reconcile, each of which is a real invariant rather than a
// nicety:
//
//	posted_balance(account) == SUM(posting.amount for that account)
//	lots: SUM(consumed) == the debited side of that user's SPEND journals
//
// A row where either fails is a genuine ledger defect, so this endpoint reports the
// per-user figures rather than only the headline: a support engineer should be able
// to see the discrepancy, not be handed a number that hides it.
//
// ── this is a support tool, so it discloses a lot — and that is correct ────────
//
// It names every award, every source resource, every journal. That is the point:
// the alternative is an operator guessing. It is behind the same platform-admin
// gate as coin PRICING (routes.go's own comment), because an operator who can
// rewrite prices and an operator who can read any student's financial history are
// the same trust boundary. It is NOT the wallet's own data model — the student's
// own transactions endpoint stays scoped to them.

package coins

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// SupportView is one student's complete coin history.
type SupportView struct {
	UserID        uint                 `json:"user_id"`
	GeneratedAt   time.Time            `json:"generated_at"`
	Accounts      []SupportAccountView `json:"accounts"`
	TotalPosted   int64                `json:"total_posted"`
	TotalReserved int64                `json:"total_reserved"`
	// TotalAvailable is the number the wallet shows. It is computed HERE rather
	// than read from a cached field, and the note on SupportAccountView explains
	// why that matters.
	TotalAvailable int64                `json:"total_available"`
	Lots           []SupportLotView     `json:"lots"`
	Journals       []SupportJournalView `json:"journals"`
	// Discrepancies is non-empty when this student's rows do not reconcile. It is
	// the most valuable field on the endpoint: an empty slice is the normal case,
	// and its presence means there is a real bug to look at rather than a student
	// to explain.
	Discrepancies []string `json:"discrepancies,omitempty"`
}

// SupportAccountView is one bucket account with its cached and computed balance.
type SupportAccountView struct {
	AccountID uint   `json:"account_id"`
	Bucket    string `json:"bucket"`
	Kind      string `json:"kind"`
	// Closed is a support hold: nothing may move through a closed account (423 in
	// 09). Surfaced because "their coins are frozen" is a question support is asked
	// and currently has no way to answer.
	Closed bool `json:"closed"`
	// CachedPosted and CachedReserved are coin_account_balance, the projection the
	// wallet renders. ComputedPosted and ComputedReserved are SUM(posting.amount).
	//
	// FOUR numbers rather than two, and that is the whole design of this type: the
	// wallet shows CachedPosted, so when a support engineer sees CachedPosted !=
	// ComputedPosted they have found the bug rather than being told the answer.
	// A two-field version of this endpoint would make drift invisible by
	// construction.
	CachedPosted     int64 `json:"cached_posted"`
	CachedReserved   int64 `json:"cached_reserved"`
	ComputedPosted   int64 `json:"computed_posted"`
	ComputedReserved int64 `json:"computed_reserved"`
	Drifted          bool  `json:"drifted"`
}

// SupportLotView is one lot: a grant with its own expiry.
type SupportLotView struct {
	ID        uint       `json:"id"`
	JournalID string     `json:"journal_id"`
	Bucket    string     `json:"bucket"`
	Granted   int64      `json:"granted"`
	Consumed  int64      `json:"consumed"`
	Remaining int64      `json:"remaining"`
	ExpiresAt *time.Time `json:"expires_at"`
	// Expired is computed against the requested instant, so support can answer "did
	// these lapse?" without doing arithmetic. A lot whose ExpiresAt has passed but
	// that has NOT been swept yet is the case worth seeing: the coins are already
	// unspendable and the balance is still counting them.
	Expired   bool      `json:"expired"`
	CreatedAt time.Time `json:"created_at"`
}

// SupportJournalView is one journal with its legs.
type SupportJournalView struct {
	ID             string    `json:"id"`
	EntryType      string    `json:"entry_type"`
	State          string    `json:"state"`
	ReasonCode     string    `json:"reason_code"`
	RefType        string    `json:"ref_type,omitempty"`
	RefID          *uint64   `json:"ref_id,omitempty"`
	IdempotencyKey string    `json:"idempotency_key"`
	EffectiveAt    time.Time `json:"effective_at"`
	CreatedBy      string    `json:"created_by"`
	// Amount is the signed net the journal moved for THIS user, which is not the
	// journal's total (a grant is two legs summing to zero across two accounts).
	// Support asks "how much did this event give them", and the answer for a grant
	// is positive and for a spend is negative.
	Amount   int64                `json:"amount"`
	Metadata map[string]any       `json:"metadata,omitempty"`
	Postings []SupportPostingView `json:"postings"`
}

// SupportPostingView is one signed leg.
type SupportPostingView struct {
	Seq       int16 `json:"seq"`
	AccountID uint  `json:"account_id"`
	Amount    int64 `json:"amount"`
	LotID     *uint `json:"lot_id,omitempty"`
}

// ErrSupportUserRequired is a request for user 0.
var ErrSupportUserRequired = errors.New("a support view needs a user id")

// SupportQuery bounds one support read. The limits exist because this endpoint can
// be pointed at a user with thousands of journals, and an unbounded read on an
// admin console is how an operator's browser hangs.
type SupportQuery struct {
	UserID uint
	// Limit is the maximum number of JOURNALS returned. 0 means SupportDefaultLimit.
	Limit int
	// Now is the instant `expired` is computed against. Injectable so a test can
	// pin the boundary rather than sleeping, and so a support engineer looking at
	// "why did these lapse yesterday" sees it as expired.
	Now time.Time
}

// SupportDefaultLimit bounds the journal list.
const SupportDefaultLimit = 200

// SupportView assembles one student's complete coin history.
//
// Read-only by construction: it takes no Ledger and cannot write, because the only
// reason an operator should ever call this is to understand something. Changing a
// balance is `POST /admin/coins/adjust`, and keeping the two apart is what stops
// "let me just fix it" from becoming a mutation with no journal.
func (l *Ledger) SupportView(ctx context.Context, q SupportQuery) (*SupportView, error) {
	if l == nil || l.repo == nil {
		return nil, ErrNoDatabase
	}
	if q.UserID == 0 {
		return nil, ErrSupportUserRequired
	}
	if q.Limit <= 0 {
		q.Limit = SupportDefaultLimit
	}
	if q.Now.IsZero() {
		q.Now = l.now().UTC()
	}

	view := &SupportView{UserID: q.UserID, GeneratedAt: q.Now, Accounts: []SupportAccountView{},
		Lots: []SupportLotView{}, Journals: []SupportJournalView{}}

	accounts, err := l.repo.SupportAccountsForUser(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("read accounts for user %d: %w", q.UserID, err)
	}
	for _, account := range accounts {
		view.TotalPosted += account.CachedPosted
		view.TotalReserved += account.CachedReserved
		view.TotalAvailable += account.CachedPosted - account.CachedReserved
		if account.Drifted {
			view.Discrepancies = append(view.Discrepancies,
				fmt.Sprintf("account %d (%s): cached posted %d but postings sum to %d",
					account.AccountID, account.Bucket, account.CachedPosted, account.ComputedPosted))
		}
		view.Accounts = append(view.Accounts, account)
	}

	lots, err := l.repo.SupportLotsForUser(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("read lots for user %d: %w", q.UserID, err)
	}
	for _, lot := range lots {
		remaining := lot.Granted - lot.Consumed
		entry := SupportLotView{
			ID: lot.ID, JournalID: lot.JournalID, Bucket: lot.Bucket,
			Granted: lot.Granted, Consumed: lot.Consumed, Remaining: remaining,
			ExpiresAt: lot.ExpiresAt, CreatedAt: lot.CreatedAt,
		}
		// Expired, NOT swept, and still holding coins: the three things a support
		// engineer most needs to see together, because the wallet shows the balance
		// including these while openLotQuery already refuses to spend them.
		entry.Expired = lot.ExpiresAt != nil && !lot.ExpiresAt.After(q.Now) && remaining > 0
		view.Lots = append(view.Lots, entry)
	}

	journals, err := l.repo.SupportJournalsForUser(ctx, q.UserID, q.Limit)
	if err != nil {
		return nil, fmt.Errorf("read journals for user %d: %w", q.UserID, err)
	}
	for _, journal := range journals {
		view.Journals = append(view.Journals, journal)
	}
	return view, nil
}
