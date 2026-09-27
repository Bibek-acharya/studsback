// internal/coins/ledger_model.go
//
// Schema notes that matter when reading these models:
//
//   - Bucket values are strings with CHECK constraints rather than a Postgres
//     ENUM. GORM has no enum precedent in this repo and its ENUM support needs a
//     driver.Valuer/sql.Scanner pair per type, which is a lot of surface for
//     identical enforcement. The constraints live in ensure_indexes.go because
//     AutoMigrate cannot create CHECK constraints either.
//
//   - PURCHASED is deliberately absent from the grantable buckets. Decision D1
//     forbids purchasing coins. Adding the value later is a non-event: one CHECK
//     constraint widens, one chart-of-accounts row is added, and no existing row
//     changes meaning.
//
//   - Liability is load-bearing. Only USER accounts are constrained to stay
//     non-negative. SYSTEM accounts are the "outside the world" side of the
//     ledger and are legitimately signed against us: earned_faucet goes negative
//     when coins are issued and redeemed_sink goes positive when they are
//     destroyed. Applying the no-overdraft CHECK to system accounts rejects
//     every single grant. This was hit and fixed during schema validation.
package coins

import (
	"time"
)

// Grantable buckets. Purchased is named for the CHECK constraint value lists
// but is not reachable while D1 holds.
const (
	BucketFree   = "FREE"
	BucketEarned = "EARNED"
	BucketBought = "PURCHASED"
)

// Account kinds.
const (
	AccountUser   = "USER"
	AccountSystem = "SYSTEM"
)

// Journal states. PENDING may become POSTED or REVERSED exactly once; POSTED is
// terminal. This is restricted immutability, not a contradiction of it.
const (
	StatePending  = "PENDING"
	StatePosted   = "POSTED"
	StateReversed = "REVERSED"
)

// Entry types. EXPIRE is a journal like any other: expiry debits the user and
// credits expired_burn, it never deletes a lot.
const (
	EntryGrant    = "GRANT"
	EntrySpend    = "SPEND"
	EntryExpire   = "EXPIRE"
	EntryReversal = "REVERSAL"
	EntryAdjust   = "ADJUST"
)

// The chart of accounts, seeded by EnsurePostgresIndexes. These are the P&L side
// of the ledger; the global invariant is that the sum of every account's
// posted_balance is exactly zero.
const (
	SystemEarnedFaucet = "earned_faucet" // goes negative on issue
	SystemRedeemedSink = "redeemed_sink" // goes positive on destroy
	SystemExpiredBurn  = "expired_burn"  // goes positive on expiry
)

// SystemAccounts is the seed list for the chart of accounts.
var SystemAccounts = []string{SystemEarnedFaucet, SystemRedeemedSink, SystemExpiredBurn}

// CoinAccount is one side of a double-entry posting. A user has at most one
// account per bucket; the platform has a fixed set of system accounts.
//
// Two partial unique indexes enforce "one user account per bucket" and "one
// account per system type". A single UNIQUE over (owner_user_id, kind, bucket)
// does NOT work: every system account collides on (NULL, 'SYSTEM', NULL). That
// was hit and fixed during schema validation.
type CoinAccount struct {
	ID          uint       `gorm:"primarykey" json:"id"`
	Kind        string     `gorm:"type:varchar(16);not null" json:"kind"`
	OwnerUserID *uint      `gorm:"index" json:"owner_user_id,omitempty"`     // USER only
	SystemType  *string    `json:"system_type,omitempty"`                    // SYSTEM only
	Bucket      *string    `gorm:"type:varchar(16)" json:"bucket,omitempty"` // USER only
	CreatedAt   time.Time  `json:"created_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
}

// CoinAccountBalance is the cached balance projection. It is NOT the source of
// truth; coin_posting is. This table is rebuildable from postings, which is the
// property that makes a nightly drift check meaningful.
//
// Version is the optimistic-concurrency token. This build serialises per user
// with pg_advisory_xact_lock instead, so Version is written but not relied on.
type CoinAccountBalance struct {
	AccountID     uint      `gorm:"primarykey;autoIncrement:false" json:"account_id"`
	Liability     bool      `gorm:"not null;default:true" json:"liability"`
	PostedBalance int64     `gorm:"not null;default:0" json:"posted_balance"`
	Reserved      int64     `gorm:"not null;default:0" json:"reserved"`
	Version       int64     `gorm:"not null;default:0" json:"version"`
	LastJournalID *string   `gorm:"type:uuid" json:"last_journal_id,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CoinJournal is the head of a double-entry event: one business fact, with an
// idempotency key and a reason code. It is append-only apart from the single
// one-way PENDING -> POSTED|REVERSED transition, enforced by trigger.
//
// The UNIQUE (scope, idempotency_key) created in ensure_indexes.go is the entire
// idempotency mechanism. An application-level SELECT-then-INSERT check in Go
// races under concurrency; the constraint cannot.
type CoinJournal struct {
	ID                 string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	EntryType          string         `gorm:"type:varchar(16);not null;index" json:"entry_type"`
	State              string         `gorm:"type:varchar(16);not null;default:POSTED;index" json:"state"`
	Scope              string         `gorm:"type:varchar(64);not null" json:"scope"`
	IdempotencyKey     string         `gorm:"type:varchar(255);not null" json:"idempotency_key"`
	RequestFingerprint []byte         `gorm:"type:bytea;not null" json:"-"`
	ReasonCode         string         `gorm:"type:varchar(64);not null;index" json:"reason_code"`
	RefType            *string        `gorm:"type:varchar(64);index" json:"ref_type,omitempty"`
	RefID              *uint64        `json:"ref_id,omitempty"`
	ReversalOf         *string        `gorm:"type:uuid;index" json:"reversal_of,omitempty"`
	EffectiveAt        time.Time      `gorm:"not null;default:now()" json:"effective_at"`
	CreatedAt          time.Time      `json:"created_at"`
	CreatedBy          string         `gorm:"type:varchar(64)" json:"created_by"`
	Metadata           map[string]any `gorm:"type:jsonb;serializer:json;default:'{}'" json:"metadata,omitempty"`
}

// CoinPosting is one signed leg of a journal. Amount is SIGNED and every journal
// must sum to exactly zero, which is asserted as an invariant rather than
// trusted. Append-only, enforced by trigger, because a mutable posting row
// destroys the only audit trail that makes a balance explainable.
type CoinPosting struct {
	ID         int64     `gorm:"primarykey" json:"id"`
	JournalID  string    `gorm:"type:uuid;not null;index" json:"journal_id"`
	Seq        int16     `gorm:"not null" json:"seq"`
	AccountID  uint      `gorm:"not null;index" json:"account_id"`
	Amount     int64     `gorm:"not null" json:"amount"`
	AccountSeq int64     `gorm:"not null" json:"account_seq"`
	LotID      *uint     `gorm:"index" json:"lot_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// CoinLot is a grant with its own independent expiry. This is what makes "which
// bucket gets spent first" a query instead of a column, and it is what lets the
// free allowance expire on 30 days while earned coins live a year.
type CoinLot struct {
	ID        uint       `gorm:"primarykey" json:"id"`
	AccountID uint       `gorm:"not null;index" json:"account_id"`
	JournalID string     `gorm:"type:uuid;not null" json:"journal_id"`
	Bucket    string     `gorm:"type:varchar(16);not null" json:"bucket"`
	Granted   int64      `gorm:"not null" json:"granted"`
	Consumed  int64      `gorm:"not null;default:0" json:"consumed"`
	ExpiresAt *time.Time `gorm:"index" json:"expires_at,omitempty"` // nil = never expires
	CreatedAt time.Time  `json:"created_at"`
}

// TableName pins table names explicitly. GORM's default pluraliser would produce
// "coin_account_balances" and similar, and the ledger DDL was validated against
// the singular names.
func (CoinAccount) TableName() string        { return "coin_account" }
func (CoinAccountBalance) TableName() string { return "coin_account_balance" }
func (CoinJournal) TableName() string        { return "coin_journal" }
func (CoinPosting) TableName() string        { return "coin_posting" }
func (CoinLot) TableName() string            { return "coin_lot" }

// LedgerModels is what cmd/server/main.go adds to the AutoMigrate list.
// AutoMigrate creates the tables and the plain non-unique indexes; everything
// partial, every CHECK constraint, the append-only triggers, and the
// chart-of-accounts seed come from EnsurePostgresIndexes, which main.go must
// call in the same non-fatal block as the other post-migration hooks.
var LedgerModels = []any{
	&CoinAccount{},
	&CoinAccountBalance{},
	&CoinJournal{},
	&CoinPosting{},
	&CoinLot{},
}
