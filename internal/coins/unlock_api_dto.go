// internal/coins/unlock_api_dto.go
//
// The wire contract of 03-api-contract.md §2.1, §2.2 and §2.3, and the error
// envelope of §4. Every json tag in this file is a published field name; a
// rename here is a breaking change for the frontend, and the frontend is the
// only consumer of these bodies.
//
// ── what is deliberately NOT in here ─────────────────────────────────────────
//
// UnlockRequest has two fields and no third. It has no amount, no price, no
// coins, no quantity and no bucket. That absence is the whole of 02-architecture
// §12.1 ("Never trust a client-supplied amount… A client that can say what a
// coin is worth is an exploit") expressed as a struct shape rather than as a
// comment that a future edit could route around: a handler cannot read a price
// out of this request because there is nowhere to read it from.
//
// The price is resolved server-side by spendPrice, keyed on the resource class,
// and the test that keeps this honest is
// TestUnlockRequestCarriesNoAmountAndThePriceComesFromConfig.
//
// ── the error envelope, and why it is not response.Response ─────────────────
//
// The §intro says every response goes through response.Success /
// response.Error, and response.Error takes a string. But the §2.3 table gives
// every failure a named code (INVALID_IDEMPOTENCY_KEY, INSUFFICIENT_COINS,
// IDEMPOTENCY_KEY_REUSE, ALLOWANCE_EXPIRED, RESOURCE_NOT_FOUND) and the 402 body
// is explicitly "a designed object, not a string" carrying required, available,
// shortfall, expires_in_days and ways_to_earn.
//
// A string error field has nowhere to put a code or a payload, so the unlock
// endpoint answers with ErrorEnvelope: the same four top-level keys
// (success/message/data/error) with `error` promoted from a string to an object.
// success:false and `error` are both present in both shapes, so a client that
// only branches on the status code and the success flag is unaffected.
//
// The three read endpoints use response.Success / response.Error unchanged:
// they have no designed error object in the contract, and their only failures
// are a missing identity and a database error.
package coins

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// ── POST /api/v1/coins/unlock ────────────────────────────────────────────────

// UnlockRequest names the thing being unlocked and nothing else.
//
// ResourceType is one of study_resource | video | mock_test, validated by
// validateResourceType and refused with ErrInvalidResourceType otherwise.
// ResourceID must be non-zero: an unlock of resource 0 is an unlock of nothing.
type UnlockRequest struct {
	ResourceType string `json:"resource_type"`
	ResourceID   uint64 `json:"resource_id"`
}

// UnlockResponse is the 200 body of §2.3.
//
// Unlocked is always true in a 200: a response that reached this struct either
// created the entitlement or found it already there. AlreadyUnlocked
// distinguishes the two, and it is the flag that says nothing was charged.
type UnlockResponse struct {
	Unlocked        bool           `json:"unlocked"`
	AlreadyUnlocked bool           `json:"already_unlocked"`
	CoinsPaid       int64          `json:"coins_paid"`
	BalanceAfter    int64          `json:"balance_after"`
	SpentFrom       []SpentFromDTO `json:"spent_from"`
	UsedAllowance   bool           `json:"used_allowance"`
}

// SpentFromDTO is one lot a spend burned, FEFO order, as §2.3 renders it.
//
// SpentFrom is never null: an allowance unlock and an already-owned unlock both
// spend nothing, and "spent_from": [] is a fact about the request while
// "spent_from": null would be a question about the JSON encoder.
type SpentFromDTO struct {
	Bucket    string     `json:"bucket"`
	Coins     int64      `json:"coins"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// ── GET /api/v1/coins/balance ────────────────────────────────────────────────

// BalanceResponse is §2.1.
//
// TotalAvailable is the cached projection (SUM(posted_balance - reserved) over
// the caller's accounts) and the Buckets are the open lots. They are two
// different tables on purpose — the projection is what a spend is checked
// against, the lots are what a spend burns — and they can disagree by exactly
// the value of lots that have expired but not yet been swept, which is what the
// nightly reconciliation exists to notice.
type BalanceResponse struct {
	TotalAvailable int64           `json:"total_available"`
	TotalReserved  int64           `json:"total_reserved"`
	Buckets        []BucketDTO     `json:"buckets"`
	SpendOrder     []SpendOrderDTO `json:"spend_order"`
	Allowance      *AllowanceDTO   `json:"allowance"`
}

// BucketDTO is one bucket in the wallet. ExpiresAt is the SOONEST expiry among
// that bucket's open lots, or null when none of them expire: a student has to be
// able to see which coins lapse and when without reading the terms.
type BucketDTO struct {
	Bucket    string     `json:"bucket"`
	Balance   int64      `json:"balance"`
	ExpiresAt *time.Time `json:"expires_at"`
	LotCount  int64      `json:"lot_count"`
}

// SpendOrderDTO is the FEFO plan: what a spend would burn, in the order it
// would burn it. It is derived from the same rows as Buckets, so the two cannot
// describe different wallets.
type SpendOrderDTO struct {
	Bucket    string     `json:"bucket"`
	Coins     int64      `json:"coins"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// AllowanceDTO is the allowance block of §2.1, and the whole body of
// GET /api/v1/coins/allowance.
//
// Granted/Used per class rather than a total, because the classes have separate
// quotas and "2 left" is meaningless without knowing which two. Used is derived
// from resource_unlock and is never stored; see unlock_model.go.
type AllowanceDTO struct {
	GrantedAt       *time.Time `json:"granted_at"`
	ExpiresAt       *time.Time `json:"expires_at"`
	DocumentUnlocks int64      `json:"document_unlocks"`
	DocumentUsed    int64      `json:"document_used"`
	VideoUnlocks    int64      `json:"video_unlocks"`
	VideoUsed       int64      `json:"video_used"`
	MockTestUnlocks int64      `json:"mock_test_unlocks"`
	MockTestUsed    int64      `json:"mock_test_used"`
}

// ── GET /api/v1/coins/transactions ───────────────────────────────────────────

// TransactionsResponse is §2.2. NextCursor is empty (not null) on the last
// page, so a client can branch on it without a nil check that would then have to
// be re-checked on the next page.
type TransactionsResponse struct {
	Items      []TransactionDTO `json:"items"`
	NextCursor string           `json:"next_cursor"`
}

// TransactionDTO is one journal, newest first.
//
// Amount is SIGNED from the caller's perspective: a spend is negative, a grant
// positive, a clawback negative again. Reversals appear as their own entries
// referencing the original rather than editing it, so a student who disputes a
// clawback can see both sides of it.
type TransactionDTO struct {
	JournalID    string    `json:"journal_id"`
	EntryType    string    `json:"entry_type"`
	ReasonCode   string    `json:"reason_code"`
	Amount       int64     `json:"amount"`
	BalanceAfter int64     `json:"balance_after"`
	Ref          *RefDTO   `json:"ref"`
	Description  string    `json:"description"`
	Reversed     bool      `json:"reversed"`
	CreatedAt    time.Time `json:"created_at"`
}

// RefDTO is the resource a journal is about. Null for a journal with no ref, and
// a pointer so that absent is absent rather than an object of empty strings that
// a client would have to learn to distrust.
type RefDTO struct {
	Type string `json:"type"`
	ID   uint64 `json:"id"`
}

// transactionCursor is the keyset for §2.2's cursor pagination.
//
// It is (created_at, id) rather than an OFFSET, and both halves are load-bearing:
// created_at alone is not unique — two journals can be written in the same
// microsecond by two requests for the same user — and an OFFSET re-reads and
// re-skips rows when a new journal lands mid-scroll, so a student scrolling
// their history while they spend would see a row twice or miss one.
//
// It is base64url'd rather than a bare timestamp so the client treats it as
// opaque, which is what lets this change shape without a frontend change.
type transactionCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

func encodeTransactionCursor(c transactionCursor) string {
	encoded, err := json.Marshal(c)
	if err != nil {
		// A struct of two scalar fields cannot fail to marshal. Returning an
		// empty cursor degrades to "first page", which is the safe direction:
		// the client re-reads from the top rather than being sent somewhere
		// arbitrary.
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(encoded)
}

// decodeTransactionCursor is strict on purpose. A cursor the server did not
// write is a client bug or a stale build, and guessing at it would silently
// return the wrong page of somebody's money; ErrInvalidArgument is a 400 the
// caller can act on.
func decodeTransactionCursor(raw string) (transactionCursor, error) {
	if raw == "" {
		return transactionCursor{}, nil
	}
	encoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		// Tolerate the padded alphabet too. It costs one fallback and removes a
		// whole class of "it works in the app but not on the web" bug report.
		encoded, err = base64.URLEncoding.DecodeString(raw)
		if err != nil {
			return transactionCursor{}, fmt.Errorf("%w: the cursor is not base64url", ErrInvalidArgument)
		}
	}
	var cursor transactionCursor
	if err := json.Unmarshal(encoded, &cursor); err != nil {
		return transactionCursor{}, fmt.Errorf("%w: the cursor is not a page token", ErrInvalidArgument)
	}
	if cursor.ID == "" || cursor.CreatedAt.IsZero() {
		return transactionCursor{}, fmt.Errorf("%w: the cursor is missing its key", ErrInvalidArgument)
	}
	return cursor, nil
}

// ── the 402 body ────────────────────────────────────────────────────────────

// InsufficientCoinsData is the designed object of §2.3.
//
// required is the server-resolved price for the class. available is the wallet's
// cached projection as of AFTER the refusal rolled back — the ledger returns an
// empty SpendResult alongside an error, so the two figures are re-read rather
// than carried out of the failed call. unlock_api.go documents what that costs
// and why it is safe.
//
// Shortfall is derived here rather than being asked for, so it cannot disagree
// with the two numbers above it.
type InsufficientCoinsData struct {
	Required      int64            `json:"required"`
	Available     int64            `json:"available"`
	Shortfall     int64            `json:"shortfall"`
	ExpiresInDays *int64           `json:"expires_in_days"`
	WaysToEarn    []WayToEarnDTO   `json:"ways_to_earn"`
	Unavailable   []UnavailableDTO `json:"unavailable_routes"`
}

// WayToEarnDTO is one route to coins that this student can still take, with the
// coins it is actually worth to them.
//
// Potential is the REMAINING value of the route, not the configured award: a
// profile paid four instalments of five is worth one instalment, not the whole
// 25. A number that overstates what is on offer is how a "you only need 15
// more" screen becomes a support ticket.
type WayToEarnDTO struct {
	Code      string `json:"code"`
	Label     string `json:"label"`
	Potential int64  `json:"potential"`
}

// UnavailableDTO is a route that exists on paper and cannot be taken yet, with
// the reason. It exists so the client can say so rather than inventing a
// destination: a button for a feature that is not launched is worse than no
// button. See the referral note on the waysToEarn builder.
type UnavailableDTO struct {
	Code   string `json:"code"`
	Label  string `json:"label"`
	Reason string `json:"reason"`
}

// Route codes. They are constants rather than literals because the frontend
// switches on them, and a typo in one place would be a route the UI never
// renders.
const (
	RouteReferral = "REFERRAL"
	RouteProfile  = "PROFILE"
	RouteUpload   = "UPLOAD"

	// ReasonNotLaunched is the only reason an earning route can be unavailable
	// today. It is deliberately specific: a second value later ("ALREADY_DONE",
	// "CAPPED") would mean the route is not unavailable, it is ineligible, and
	// that belongs in ways_to_earn's absence instead.
	ReasonNotLaunched = "NOT_LAUNCHED"
)

// ── the error envelope ───────────────────────────────────────────────────────

// ErrorEnvelope is what every failure of POST /api/v1/coins/unlock returns.
//
// Data is a pointer and omitempty so the 400 and the 409 carry only a message
// while the 402 carries the designed object, which is what §2.3 specifies for
// the 402 and says nothing about for the others.
type ErrorEnvelope struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    any         `json:"data,omitempty"`
	Error   ErrorDetail `json:"error"`
}

// ErrorDetail is the promoted error object: the code from the §2.3 table, a
// student-safe message, and the payload when there is one.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ── shared value types ───────────────────────────────────────────────────────

// refFromPointers renders a journal's ref pair, or nil when it has none.
func refFromPointers(refType *string, refID *uint64) *RefDTO {
	if refType == nil || *refType == "" || refID == nil || *refID == 0 {
		return nil
	}
	return &RefDTO{Type: *refType, ID: *refID}
}

// describeReason renders a journal as a line of English for the wallet's
// history list.
//
// It is server-side and derived from the reason code rather than a title looked
// up from the resource module, because this package does not know the resource
// tables — a cross-module read here would couple the ledger to four modules and
// would have to be redone by each of them. The wording is therefore honest about
// what it knows: the event and the class. The gate slice replaces it with the
// real title once a gate knows it.
func describeReason(reasonCode string, ref *RefDTO) string {
	what := "your account"
	if ref != nil {
		what = ref.Type + " " + strconv.FormatUint(ref.ID, 10)
	}
	switch reasonCode {
	case ReasonResourceUnlock:
		return "Unlocked: " + what
	case ReasonProfileComplete:
		return "Profile progress: " + what
	case ReasonReferralQualified:
		return "Referral qualified: " + what
	case ReasonResourceApproved:
		return "Resource published: " + what
	case ReasonGrantReversal:
		return "Reversed: " + what
	case ReasonReferralHold:
		return "Referral placed on hold: " + what
	}
	return reasonCode
}
