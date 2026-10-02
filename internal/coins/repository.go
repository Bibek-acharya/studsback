// internal/coins/repository.go
//
// Every SQL statement and every db.Transaction() call in the StudsToken ledger
// lives in this file. That is the house layering, not a new architectural layer:
// 02-architecture.md §8 puts "GORM queries + all db.Transaction() calls (repo
// layer only)" here, and notes that ledger.go's split is an internal
// decomposition of the repository layer. The precedent is 17 existing call sites
// (internal/mocktests/repository.go:141,162,237,263; internal/studyresources/
// repository.go:128; internal/forum/repository.go:316).
//
// ledger.go holds the decisions; this file holds the statements. Nothing in here
// decides whether a spend is affordable or which lots pay for it.
//
// ── the concurrency mechanism ────────────────────────────────────────────────
//
// InUserTx is the only way to mutate a balance, and it is the whole of §3 of the
// architecture spec in one function:
//
//	SET LOCAL lock_timeout = '3s'
//	SET LOCAL idle_in_transaction_session_timeout = '10s'
//	SELECT pg_advisory_xact_lock(hashtextextended('coin:user:' || $1, 0))
//
// One lock per user, so there is no lock ordering to get wrong between two
// balance-mutating transactions and therefore no deadlock. The timeouts are set
// BEFORE the lock, so a pathological request cannot hold a user's lock open
// indefinitely: it fails instead, and it fails before it has touched anything.
//
// ── one lock the spec does not mention ───────────────────────────────────────
//
// coin_posting has UNIQUE (account_id, account_seq), so account_seq must be a
// strictly increasing per-account counter. The per-USER advisory lock covers the
// user's own accounts, but the system accounts (earned_faucet, redeemed_sink) are
// shared by every user in the system, and two users' transactions hold two
// different user locks. They will both want to append to redeemed_sink with
// account_seq = N+1.
//
// insertPostings therefore takes the target coin_account rows FOR UPDATE in
// ascending id order before it reads any counter, and it does so on every call,
// not optionally, so no caller can forget. That makes the global lock order:
//
//  1. the one per-user advisory lock
//  2. coin_account rows, ascending id
//  3. coin_account_balance and coin_lot rows (implied by UPDATE)
//
// Step 1 is held by exactly one transaction, so no cycle can form. The cost is
// that every grant and every spend serialises for the length of its final insert
// against one shared row. That is forced by the schema rather than chosen here,
// and it is the honest consequence of a counter that must be unique across the
// whole system; a sequence-per-account would remove it and needs a migration.
//
// ── that UNIQUE constraint is missing from the shipped schema ────────────────
//
// §2 declares UNIQUE (account_id, account_seq) and UNIQUE (journal_id, seq) as
// table constraints. Neither exists in the schema as built. The GORM model has
// no uniqueIndex tag for AutoMigrate to honour, and EnsurePostgresIndexes adds
// every partial index and every CHECK but not these two table-level UNIQUE
// constraints. Verified against a live instance: pg_constraint on coin_posting
// contains only coin_posting_pkey and chk_coin_posting_nonzero.
//
// Consequences this file has to carry in Go instead:
//
//   - the account_seq read-then-write is safe ONLY because of the FOR UPDATE
//     above, with no constraint behind it. It is the whole mechanism.
//   - (journal_id, seq) density is checked in Go before any insert.
//
// Reported rather than fixed: ledger_model.go and ensure_indexes.go are not this
// unit's to change. The invariant test asserts both properties at the rows, and
// TestConcurrentGrantsAcrossDifferentUsersDoNotCollideOnAccountSeq is the test
// that fails if the FOR UPDATE ever stops taking effect.
//
// ── GORM zero values ─────────────────────────────────────────────────────────
//
// GORM omits zero-valued fields on Create and on struct-based Updates. This has
// already cost this codebase once, twice over: seeding a system account with
// `Liability: false` wrote nothing, the column default (true) won, and the
// no-overdraft CHECK then rejected every single grant. seedChartOfAccounts works
// around it with Create-then-UpdateColumn.
//
// Everything below that needs an explicit zero — liability=false, reserved=0,
// consumed=0, a delta of 0 — is raw SQL, or UpdateColumn. There is no
// struct-based Create or Updates on a ledger table anywhere in this file.
//
// ── and db.Raw is not db.Exec ────────────────────────────────────────────────
//
// `db.Raw(sql).Error` PREPARES a statement and does not send it. It returns a nil
// error. It is a statement that never runs, written in the shape of one that
// did, and it cost this file a lock that appeared to work and did not. Anything
// whose purpose is a side effect — a lock, a mutation — goes through Exec, or
// through Raw followed by Scan/Row, so that the rows have to be consumed.
package coins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ScopeUser is the journal scope for every journal this package writes. The
// UNIQUE idempotency index is (scope, idempotency_key), so the scope is what
// partitions keys.
//
// The spec's scope is 'user' or 'system' and this build only writes user-scope
// journals, including reversals and holds. That means an idempotency key must
// itself be unique across all users, which the caller builds (the spec's own
// example is 'grant:u1:ref:1', user-scoped). The ledger does not silently paper
// over a cross-user collision: the request fingerprint includes the user id, so
// a reused key from another user is rejected as ErrIdempotencyKeyReuse rather
// than replaying somebody else's journal.
const ScopeUser = "user"

// Repository is the ledger's data access. It owns the *gorm.DB and nothing
// else.
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// DB exposes the underlying handle for read-only callers outside the ledger. The
// ledger itself never uses it: a read that has to be consistent with a write
// must run inside InUserTx, or it is a race with a documented cause.
func (r *Repository) DB() *gorm.DB {
	if r == nil {
		return nil
	}
	return r.db
}

// ErrNoDatabase is returned by a Repository constructed with no handle. It is
// distinct from any domain sentinel because it is a wiring mistake, not a
// business outcome.
var ErrNoDatabase = errors.New("coins: repository has no database handle")

// InUserTx runs fn inside one transaction that already holds the per-user
// advisory lock. It is the only entry point to a balance mutation.
//
// Everything the callback does is committed together or not at all, which is what
// makes a rejected spend leave zero rows behind rather than a journal with no
// postings: the journal insert, the lot updates, the postings and the cached
// balance all roll back with the error.
func (r *Repository) InUserTx(ctx context.Context, userID uint, fn func(tx *TxContext) error) error {
	if r == nil || r.db == nil {
		return ErrNoDatabase
	}
	if userID == 0 {
		return fmt.Errorf("%w: a ledger operation needs a user id", ErrInvalidArgument)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := setTransactionGuards(tx); err != nil {
			return err
		}
		if err := lockUser(tx, userID); err != nil {
			return err
		}
		return fn(&TxContext{db: tx, userID: userID})
	})
}

// setTransactionGuards bounds how long one coin transaction may be stuck.
//
// SET LOCAL is transaction-scoped, so there is no pool-poisoning risk: the
// setting evaporates at COMMIT or ROLLBACK and the next user of the connection
// gets the server default. The values are the ones in 02-architecture.md §3.
//
//	lock_timeout = 3s                      a request that cannot get the user lock
//	                                       within three seconds fails instead of
//	                                       queueing behind a pathological one.
//	idle_in_transaction_session_timeout    the companion control for a client
//	                                       that opens a transaction and stalls.
func setTransactionGuards(tx *gorm.DB) error {
	if err := tx.Exec(`SET LOCAL lock_timeout = '3s'`).Error; err != nil {
		return fmt.Errorf("set lock_timeout: %w", err)
	}
	if err := tx.Exec(`SET LOCAL idle_in_transaction_session_timeout = '10s'`).Error; err != nil {
		return fmt.Errorf("set idle_in_transaction_session_timeout: %w", err)
	}
	return nil
}

// lockUser is the entire concurrency strategy for a balance mutation.
//
// hashtextextended gives a stable 64-bit key from the text, so the lock name
// does not have to be allocated in Go. pg_advisory_xact_lock is transaction
// scoped: it releases at COMMIT or ROLLBACK with no explicit unlock, so a
// connection returned to the pool can never be holding one.
//
// The id is rendered to text in Go and the parameter is explicitly cast. Two
// reasons, and the first one is a trap worth naming: pgx infers a placeholder's
// type from the surrounding expression, so `?::text` makes it infer text — and
// then it cannot encode a Go uint as text, and the query fails with "cannot find
// encode plan". Passing a string makes the encoding trivially correct, and the
// cast keeps the lock name obviously the same literal shape on every call site.
func lockUser(tx *gorm.DB, userID uint) error {
	if err := tx.Exec(
		`SELECT pg_advisory_xact_lock(hashtextextended('coin:user:' || CAST(? AS text), 0))`,
		strconv.FormatUint(uint64(userID), 10),
	).Error; err != nil {
		return fmt.Errorf("acquire user advisory lock for user %d: %w", userID, err)
	}
	return nil
}

// TxContext carries the open transaction and the user it is locked for. It is
// handed to the callback rather than stored, so nothing can hold one after its
// transaction has ended.
type TxContext struct {
	db     *gorm.DB
	userID uint
}

// UserID is the user whose advisory lock this transaction holds.
func (tx *TxContext) UserID() uint { return tx.userID }

// DB is the transaction handle. Exposed so ledger.go can read; every WRITE in
// this package goes through a named method below.
func (tx *TxContext) DB() *gorm.DB { return tx.db }

// ── expiry sweep selection ────────────────────────────────────────────────────

// ClaimExpiredLots returns the lots whose expiry has passed and that still hold
// coins, claiming them for this worker.
//
// FOR UPDATE SKIP LOCKED is the whole of the concurrency story, and both halves
// matter. Without SKIP LOCKED, two workers sweeping at once would serialise on the
// first row and one would block behind the other's entire batch; with it, each
// worker gets disjoint rows and they run in parallel.
//
// The lock is taken HERE and released when THIS transaction ends, which is before
// burnLot opens its own — so two workers can genuinely select the same lot. That
// gap is safe because the burn is idempotent: the loser's `expire:lot:<id>` insert
// finds the key taken and takes the replay path. The claim is therefore an
// optimisation for throughput, NOT the correctness mechanism. The unique index is.
//
// `consumed < granted` is what makes a second pass free: a burned lot is marked
// fully consumed, so it is never selected again whatever the journal says.
//
// The caller asks for limit+1 so it can tell "hit the bound" from "finished".
func (r *Repository) ClaimExpiredLots(ctx context.Context, now time.Time, limit int) ([]ExpiringLot, error) {
	var lots []CoinLot
	err := r.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at <= ? AND consumed < granted", now).
		Order("expires_at ASC").
		Limit(limit).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Find(&lots).Error
	if err != nil {
		return nil, err
	}
	if len(lots) == 0 {
		return nil, nil
	}

	// ONE query for the whole batch rather than one per lot: the alternative is N
	// round trips before any work starts, on a table that is large precisely because
	// the economy is working.
	accountIDs := make([]uint, 0, len(lots))
	for _, lot := range lots {
		accountIDs = append(accountIDs, lot.AccountID)
	}
	var owners []struct {
		AccountID   uint
		OwnerUserID *uint
	}
	if err := r.db.WithContext(ctx).
		Model(&CoinAccount{}).
		Select("id AS account_id, owner_user_id").
		Where("id IN ?", accountIDs).
		Find(&owners).Error; err != nil {
		return nil, err
	}
	byAccount := make(map[uint]uint, len(owners))
	for _, row := range owners {
		if row.OwnerUserID != nil {
			byAccount[row.AccountID] = *row.OwnerUserID
		}
	}

	out := make([]ExpiringLot, 0, len(lots))
	for _, lot := range lots {
		out = append(out, ExpiringLot{Lot: lot, OwnerUserID: byAccount[lot.AccountID]})
	}
	return out, nil
}

// ── support view reads ────────────────────────────────────────────────────────
//
// Four queries, all keyed on the user, and none of them is a SUM over the whole
// table. Each returns the CACHED projection alongside the COMPUTED one where both
// exist, because the comparison between them is the product: a support engineer
// handed a single number has been told what the wallet already said.

// SupportAccountsForUser returns every bucket account a user holds, with its cached
// balance and the same figure summed from the postings.
//
// The correlated SUM is per account rather than one total so the endpoint can name
// WHICH account drifted. A single "these don't match" with no account is a support
// ticket that goes nowhere.
func (r *Repository) SupportAccountsForUser(ctx context.Context, userID uint) ([]SupportAccountView, error) {
	var rows []SupportAccountView
	err := r.db.WithContext(ctx).Raw(`
		SELECT a.id            AS account_id,
		       COALESCE(a.bucket, '') AS bucket,
		       a.kind          AS kind,
		       (a.closed_at IS NOT NULL) AS closed,
		       COALESCE(b.posted_balance, 0) AS cached_posted,
		       COALESCE(b.reserved, 0)      AS cached_reserved,
		       COALESCE((SELECT SUM(p.amount) FROM coin_posting p
		                 WHERE p.account_id = a.id), 0) AS computed_posted
		  FROM coin_account a
		  LEFT JOIN coin_account_balance b ON b.account_id = a.id
		 WHERE a.owner_user_id = ?
		 ORDER BY a.id`, userID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	// reserved is only on the balance row and has no postings-side equivalent: a
	// reservation is a column on the projection, not a posting. The computed value
	// is therefore the cached one by definition, and Drifted only compares posted.
	for i := range rows {
		rows[i].ComputedReserved = rows[i].CachedReserved
		rows[i].Drifted = rows[i].CachedPosted != rows[i].ComputedPosted
	}
	return rows, nil
}

// SupportLotsForUser returns every lot a user holds, oldest first.
//
// Oldest first because the question support is usually asking is "which grant is
// this?" and the oldest unexpired lot is the one a student thinks of as "the one
// from my profile".
func (r *Repository) SupportLotsForUser(ctx context.Context, userID uint) ([]CoinLot, error) {
	var lots []CoinLot
	err := r.db.WithContext(ctx).Raw(`
		SELECT l.id, l.account_id, l.journal_id, l.bucket, l.granted, l.consumed, l.expires_at, l.created_at
		  FROM coin_lot l
		  JOIN coin_account a ON a.id = l.account_id
		 WHERE a.owner_user_id = ?
		 ORDER BY l.created_at ASC, l.id ASC`, userID).Scan(&lots).Error
	if err != nil {
		return nil, err
	}
	return lots, nil
}

// SupportJournalsForUser returns a user's journals, newest first, each with its
// legs and the net it moved FOR THAT USER.
//
// The net is a correlated SUM filtered on the user's own accounts, not the sum of
// the legs — a grant is two legs totalling zero across a user account and the
// faucet, so summing the legs would report zero for every award.
//
// An EXPIRE journal is included alongside the others rather than being filtered out
// as an internal detail: a student whose coins lapsed is asking why, and "we burned
// them" is the answer they need to hear from the same screen.
func (r *Repository) SupportJournalsForUser(ctx context.Context, userID uint, limit int) ([]SupportJournalView, error) {
	var rows []SupportJournalView
	err := r.db.WithContext(ctx).Raw(`
		SELECT j.id, j.entry_type, j.state, j.reason_code, j.ref_type, j.ref_id,
		       j.idempotency_key, j.effective_at, j.created_by, j.metadata,
		       COALESCE((SELECT SUM(p.amount) FROM coin_posting p
		                  JOIN coin_account pa ON pa.id = p.account_id
		                 WHERE p.journal_id = j.id AND pa.owner_user_id = ?), 0) AS amount,
		       COALESCE((SELECT COUNT(*) FROM coin_posting p2
		                  JOIN coin_account pa2 ON pa2.id = p2.account_id
		                 WHERE p2.journal_id = j.id AND pa2.owner_user_id = ?), 0) AS legs
		  FROM coin_journal j
		 WHERE j.scope = ? AND j.created_by IS NOT NULL
		   AND (j.id IN (SELECT p3.journal_id FROM coin_posting p3
		                   JOIN coin_account pa3 ON pa3.id = p3.account_id
		                  WHERE pa3.owner_user_id = ?))
		 ORDER BY j.effective_at DESC, j.id DESC
		 LIMIT ?`, userID, userID, ScopeUser, userID, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []SupportJournalView{}, nil
	}

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	var postings []struct {
		JournalID string `gorm:"column:journal_id"`
		Seq       int16  `gorm:"column:seq"`
		AccountID uint   `gorm:"column:account_id"`
		Amount    int64  `gorm:"column:amount"`
		LotID     *uint  `gorm:"column:lot_id"`
	}
	if err := r.db.WithContext(ctx).Raw(
		`SELECT journal_id, seq, account_id, amount, lot_id FROM coin_posting
		  WHERE journal_id IN ? ORDER BY journal_id, seq`, ids).Scan(&postings).Error; err != nil {
		return nil, err
	}
	byJournal := make(map[string][]SupportPostingView, len(rows))
	for _, posting := range postings {
		byJournal[posting.JournalID] = append(byJournal[posting.JournalID], SupportPostingView{
			Seq: posting.Seq, AccountID: posting.AccountID, Amount: posting.Amount, LotID: posting.LotID,
		})
	}
	for i := range rows {
		legs := byJournal[rows[i].ID]
		if legs == nil {
			legs = []SupportPostingView{}
		}
		rows[i].Postings = legs
	}
	return rows, nil
}

// ── journal ──────────────────────────────────────────────────────────────────

// InsertJournal writes the journal head, or reports that the (scope,
// idempotency_key) pair is already taken.
//
// This is the whole of §12 idempotency, and it is the UNIQUE index rather than
// application logic. A SELECT-then-INSERT check in Go races: two transactions
// for the same key both find nothing and both insert, and the loser's failure
// arrives as a unique violation instead of a replay. ON CONFLICT DO NOTHING is
// resolved by the index under the same lock the inserter takes, so exactly one
// transaction ever inserts.
//
// `created` is false when the key was already present. Under READ COMMITTED, a
// conflicting row that is still uncommitted makes the INSERT wait for the other
// transaction and then re-evaluate, so a replay is only reported once the
// original has actually committed. There is no window in which a rolled-back
// original is mistaken for a committed one.
//
// The id is generated in Go rather than by the column's gen_random_uuid()
// default so the caller knows it before the insert, and so a replay can be
// reported with the original id without a second round trip.
func (tx *TxContext) InsertJournal(j *CoinJournal) (created bool, err error) {
	metadata := []byte("{}")
	if len(j.Metadata) > 0 {
		if metadata, err = json.Marshal(j.Metadata); err != nil {
			return false, fmt.Errorf("marshal journal metadata: %w", err)
		}
	}

	var anyRefType any
	if j.RefType != nil {
		anyRefType = *j.RefType
	}
	var anyRefID any
	if j.RefID != nil {
		anyRefID = *j.RefID
	}
	var anyReversalOf any
	if j.ReversalOf != nil {
		anyReversalOf = *j.ReversalOf
	}

	var id string
	res := tx.db.Raw(
		`INSERT INTO coin_journal
			(id, entry_type, state, scope, idempotency_key, request_fingerprint, reason_code,
			 ref_type, ref_id, reversal_of, effective_at, created_at, created_by, metadata)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?::jsonb)
		 ON CONFLICT (scope, idempotency_key) DO NOTHING
		 RETURNING id`,
		j.ID, j.EntryType, j.State, j.Scope, j.IdempotencyKey, j.RequestFingerprint,
		j.ReasonCode, anyRefType, anyRefID, anyReversalOf,
		j.EffectiveAt, j.CreatedAt, j.CreatedBy, string(metadata),
	).Scan(&id)
	if res.Error != nil {
		return false, fmt.Errorf("insert journal: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return false, nil
	}
	j.ID = id
	return true, nil
}

// FindJournalByKey reads back the journal that owns an idempotency key. It is
// only ever called after InsertJournal reported a replay, inside the same
// transaction and therefore after the original has committed.
func (tx *TxContext) FindJournalByKey(scope, key string) (*CoinJournal, error) {
	var j CoinJournal
	if err := tx.db.Raw(
		`SELECT id, entry_type, state, scope, idempotency_key, request_fingerprint, reason_code,
		        ref_type, ref_id, reversal_of, effective_at, created_at, created_by, metadata
		   FROM coin_journal
		  WHERE scope = ? AND idempotency_key = ?`,
		scope, key,
	).Scan(&j).Error; err != nil {
		return nil, fmt.Errorf("read journal by idempotency key: %w", err)
	}
	if j.ID == "" {
		return nil, nil
	}
	return &j, nil
}

// FindJournal reads one journal by id.
func (tx *TxContext) FindJournal(id string) (*CoinJournal, error) {
	var j CoinJournal
	if err := tx.db.Raw(
		`SELECT id, entry_type, state, scope, idempotency_key, request_fingerprint, reason_code,
		        ref_type, ref_id, reversal_of, effective_at, created_at, created_by, metadata
		   FROM coin_journal WHERE id = ?`,
		id,
	).Scan(&j).Error; err != nil {
		return nil, fmt.Errorf("read journal %s: %w", id, err)
	}
	if j.ID == "" {
		return nil, nil
	}
	return &j, nil
}

// FindOpenHold returns the PENDING hold journal for a (reason, ref) pair, or nil.
//
// It is the lookup ReleaseReserved needs, and it is scoped to PENDING because a
// hold that has already been released is not releasable again. journal_id has no
// index on (reason_code, ref_id) — the partial index is on (ref_type, ref_id) —
// so this is a scan of the ref index, which is acceptable because a caller
// releasing a hold always supplies the ref.
func (tx *TxContext) FindOpenHold(reasonCode, refType string, refID *uint64) (*CoinJournal, error) {
	var j CoinJournal
	args := []any{reasonCode, refType}
	query := `SELECT id, entry_type, state, scope, idempotency_key, request_fingerprint, reason_code,
	                 ref_type, ref_id, reversal_of, effective_at, created_at, created_by, metadata
	            FROM coin_journal
	           WHERE reason_code = ? AND state = 'PENDING' AND ref_type = ?`
	if refID != nil {
		query += ` AND ref_id = ?`
		args = append(args, *refID)
	}
	query += ` ORDER BY created_at DESC LIMIT 1`

	if err := tx.db.Raw(query, args...).Scan(&j).Error; err != nil {
		return nil, fmt.Errorf("read open hold: %w", err)
	}
	if j.ID == "" {
		return nil, nil
	}
	return &j, nil
}

// SetJournalState performs the one mutation the journal trigger permits:
// PENDING -> REVERSED, stamped with who released it and when.
//
// It is a compare-and-set on state. RowsAffected == 0 means another transaction
// already released the hold, which is the natural idempotency of
// ReleaseReserved: the second caller sees the hold as done rather than
// decrementing `reserved` twice.
//
// metadata is merged with `||` rather than replaced, and it is not one of the
// frozen provenance columns, so the release stamp lands without touching
// anything the audit trail depends on. CoinPosting is append-only; the journal
// is append-only apart from exactly this transition.
func (tx *TxContext) SetJournalState(id, from, to string, stamp map[string]any) (bool, error) {
	patch, err := json.Marshal(stamp)
	if err != nil {
		return false, fmt.Errorf("marshal release stamp: %w", err)
	}
	res := tx.db.Exec(
		`UPDATE coin_journal
		    SET state = ?, metadata = metadata || ?::jsonb
		  WHERE id = ? AND state = ?`,
		to, string(patch), id, from,
	)
	if res.Error != nil {
		return false, fmt.Errorf("transition journal %s %s->%s: %w", id, from, to, res.Error)
	}
	return res.RowsAffected > 0, nil
}

// CountReversalsOf counts the REVERSAL journals pointing at a journal. Used to
// refuse a second reversal: POSTED is terminal, so nothing marks the original as
// reversed, and the existence of the reversal is the only record that it
// happened.
func (tx *TxContext) CountReversalsOf(journalID string) (int64, error) {
	var n int64
	if err := tx.db.Raw(
		`SELECT count(*) FROM coin_journal WHERE entry_type = 'REVERSAL' AND reversal_of = ?`,
		journalID,
	).Scan(&n).Error; err != nil {
		return 0, fmt.Errorf("count reversals of %s: %w", journalID, err)
	}
	return n, nil
}

// JournalOwnerUserID returns the user a journal moved money for, read from its
// postings' account.
//
// This is on Repository rather than TxContext because Reverse has to know the
// owner BEFORE it can open a transaction: the lock is per user, and a journal
// does not carry an owner column. The read is outside the transaction on purpose
// and is safe there — it joins a posting (append-only) to an account whose owner
// never changes, so nothing that happens afterwards can alter the answer — and
// the journal is re-read, authoritatively, inside the transaction.
//
// A journal with no user leg is not reversible and returns ErrNotFound, because
// the caller has no user to lock and no coins to claw back.
func (r *Repository) JournalOwnerUserID(journalID string) (uint, error) {
	var owner uint
	res := r.db.Raw(
		`SELECT a.owner_user_id
		   FROM coin_posting p
		   JOIN coin_account a ON a.id = p.account_id
		  WHERE p.journal_id = ? AND a.kind = 'USER'
		  ORDER BY p.seq
		  LIMIT 1`,
		journalID,
	).Scan(&owner)
	if res.Error != nil {
		return 0, fmt.Errorf("read owner of journal %s: %w", journalID, res.Error)
	}
	if owner == 0 {
		return 0, fmt.Errorf("%w: journal %s has no user leg, so there is nobody to lock", ErrNotFound, journalID)
	}
	return owner, nil
}

// UserAccountIDs returns the set of a user's bucket account ids, for the places
// that need to know which legs of a journal are the user's and which are the
// system side.
func (tx *TxContext) UserAccountIDs(userID uint) (map[uint]struct{}, error) {
	var ids []uint
	res := tx.db.Raw(
		`SELECT id FROM coin_account WHERE owner_user_id = ? AND kind = 'USER'`, userID,
	).Scan(&ids)
	if res.Error != nil {
		return nil, fmt.Errorf("read account ids for user %d: %w", userID, res.Error)
	}
	set := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set, nil
}

// JournalPostings reads a journal's legs in seq order, for a reversal to negate.
func (tx *TxContext) JournalPostings(journalID string) ([]CoinPosting, error) {
	var postings []CoinPosting
	if err := tx.db.Raw(
		`SELECT id, journal_id, seq, account_id, amount, account_seq, lot_id, created_at
		   FROM coin_posting WHERE journal_id = ? ORDER BY seq`,
		journalID,
	).Scan(&postings).Error; err != nil {
		return nil, fmt.Errorf("read postings of %s: %w", journalID, err)
	}
	return postings, nil
}

// ── accounts ─────────────────────────────────────────────────────────────────

// EnsureUserAccount returns the id of the user's account for a bucket, creating
// it and its balance row on first use.
//
// Find-then-create is safe here, and only here, because the caller holds that
// user's advisory lock. Two different users never race on the same row (the
// partial unique index is on (owner_user_id, bucket)), and the same user cannot
// be in here twice at once.
//
// The balance row is written with raw SQL and liability is set explicitly rather
// than left to the column default. That is the trap documented at the top of this
// file: a liability of true on a system account makes the no-overdraft CHECK
// reject every grant, and a liability of true here would be right but a liability
// of false written by GORM's zero-value omission would not.
func (tx *TxContext) EnsureUserAccount(userID uint, bucket string) (uint, error) {
	var id uint
	err := tx.db.Raw(
		`SELECT id FROM coin_account WHERE owner_user_id = ? AND bucket = ? AND kind = 'USER'`,
		userID, bucket,
	).Scan(&id).Error
	if err != nil {
		return 0, fmt.Errorf("find account for user %d bucket %s: %w", userID, bucket, err)
	}
	if id != 0 {
		return id, nil
	}

	bucketCopy := bucket
	user := uint64(userID)
	if err := tx.db.Exec(
		`INSERT INTO coin_account (kind, owner_user_id, bucket, created_at)
		 VALUES ('USER', ?, ?, ?)`,
		user, bucketCopy, time.Now().UTC(),
	).Error; err != nil {
		return 0, fmt.Errorf("create account for user %d bucket %s: %w", userID, bucket, err)
	}
	if err := tx.db.Raw(
		`SELECT id FROM coin_account WHERE owner_user_id = ? AND bucket = ? AND kind = 'USER'`,
		userID, bucket,
	).Scan(&id).Error; err != nil {
		return 0, fmt.Errorf("resolve new account id: %w", err)
	}
	if id == 0 {
		return 0, fmt.Errorf("account for user %d bucket %s was created but could not be read back", userID, bucket)
	}
	if err := tx.createBalanceRow(id, true); err != nil {
		return 0, err
	}
	return id, nil
}

// SystemAccountID resolves a chart-of-accounts account by system_type.
//
// A missing one is ErrNotFound, not a create: the chart of accounts is seeded by
// EnsurePostgresIndexes, and a ledger that silently invented an account to post
// against would post into a bucket nobody reconciles.
func (tx *TxContext) SystemAccountID(systemType string) (uint, error) {
	var id uint
	if err := tx.db.Raw(
		`SELECT id FROM coin_account WHERE kind = 'SYSTEM' AND system_type = ?`, systemType,
	).Scan(&id).Error; err != nil {
		return 0, fmt.Errorf("find system account %s: %w", systemType, err)
	}
	if id == 0 {
		return 0, fmt.Errorf("%w: system account %q is missing; EnsurePostgresIndexes was not run against this database", ErrNotFound, systemType)
	}
	return id, nil
}

// createBalanceRow inserts a cached-balance row with an explicit liability.
//
// Raw SQL on purpose. GORM's Create omits zero-valued fields, so
// `Liability: false` in a struct literal writes nothing and DEFAULT true wins —
// the exact failure this codebase already shipped once.
func (tx *TxContext) createBalanceRow(accountID uint, liability bool) error {
	if err := tx.db.Exec(
		`INSERT INTO coin_account_balance (account_id, liability, posted_balance, reserved, version, updated_at)
		 VALUES (?, ?, 0, 0, 0, ?)`,
		accountID, liability, time.Now().UTC(),
	).Error; err != nil {
		return fmt.Errorf("create balance row for account %d: %w", accountID, err)
	}
	return nil
}

// AccountBalance is the cached projection as the ledger reads it, plus the one
// fact about the account itself the ledger needs: whether it is frozen.
type AccountBalance struct {
	AccountID     uint
	PostedBalance int64
	Reserved      int64
	Closed        bool
}

// Available is what a spend may draw on.
//
// It is posted - reserved, and NOT posted. That single subtraction is the whole
// difference between a hold and a grant, and getting it wrong means spending
// coins that are already promised to a pending referral: the balance reads
// healthy, the CHECK passes, and the referral is unfundable when it qualifies.
func (b AccountBalance) Available() int64 { return b.PostedBalance - b.Reserved }

// Balances reads the cached projection for the given accounts.
func (tx *TxContext) Balances(accountIDs []uint) (map[uint]AccountBalance, error) {
	out := make(map[uint]AccountBalance, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}
	var rows []AccountBalance
	if err := tx.db.Raw(
		`SELECT b.account_id, b.posted_balance, b.reserved, (a.closed_at IS NOT NULL) AS closed
		   FROM coin_account_balance b
		   JOIN coin_account a ON a.id = b.account_id
		  WHERE b.account_id IN (?)`,
		accountIDs,
	).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("read balances: %w", err)
	}
	for _, r := range rows {
		out[r.AccountID] = r
	}
	return out, nil
}

// UserBalances returns every one of a user's bucket accounts, creating none. A
// user with no account has no balance, and Available is 0 rather than an error.
func (tx *TxContext) UserBalances(userID uint) ([]AccountBalance, error) {
	var rows []AccountBalance
	if err := tx.db.Raw(
		`SELECT b.account_id, b.posted_balance, b.reserved, (a.closed_at IS NOT NULL) AS closed
		   FROM coin_account_balance b
		   JOIN coin_account a ON a.id = b.account_id
		  WHERE a.owner_user_id = ? AND a.kind = 'USER'`,
		userID,
	).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("read balances for user %d: %w", userID, err)
	}
	return rows, nil
}

// ── lots ─────────────────────────────────────────────────────────────────────

// OpenLots returns the spendable lots for the given accounts, in FEFO order.
//
// The WHERE clause is the same filter as coin_lot_open_idx, so the index is used
// and the result stays small as lots close. expires_at IS NULL OR > now puts
// expired lots out of scope entirely: expiry is a later phase that debits them to
// expired_burn, and until it runs an expired lot must simply not be spendable.
//
// The ORDER BY matches fefoBefore. The allocator re-sorts anyway, so a plan
// change here cannot corrupt an allocation; the ordering is here so the returned
// slice is already FEFO for the balance endpoint's spend_order.
func (tx *TxContext) OpenLots(accountIDs []uint, now time.Time) ([]LotBalance, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	var lots []LotBalance
	if err := tx.db.Raw(
		`SELECT l.id, l.account_id, l.bucket, l.granted, l.consumed, l.expires_at
		   FROM coin_lot l
		  WHERE l.account_id IN (?)
		    AND l.consumed < l.granted
		    AND (l.expires_at IS NULL OR l.expires_at > ?)
		  ORDER BY l.expires_at NULLS LAST, l.id`,
		accountIDs, now,
	).Scan(&lots).Error; err != nil {
		return nil, fmt.Errorf("read open lots: %w", err)
	}
	return lots, nil
}

// CreateLot opens a new lot for a grant.
//
// One lot per grant, in the grant's own bucket, so the bucket and the expiry
// policy are properties of the award rather than of the user.
func (tx *TxContext) CreateLot(lot *CoinLot) error {
	var expiresAt any
	if lot.ExpiresAt != nil {
		expiresAt = *lot.ExpiresAt
	}
	// RETURNING id, and assigning it back onto the struct. This used to be a plain
	// Exec, which left lot.ID at its zero value — so EVERY GrantResult.LotID was 0
	// and any consumer that looked the lot up by it found nothing. The insert and
	// the id come back in one round trip, which is why this is not a second query.
	if err := tx.db.Raw(
		`INSERT INTO coin_lot (account_id, journal_id, bucket, granted, consumed, expires_at, created_at)
		 VALUES (?, ?, ?, ?, 0, ?, ?) RETURNING id`,
		lot.AccountID, lot.JournalID, lot.Bucket, lot.Granted, expiresAt, lot.CreatedAt,
	).Scan(&lot.ID).Error; err != nil {
		return fmt.Errorf("create lot: %w", err)
	}
	return nil
}

// ConsumeLot applies one lot's share of a spend.
//
// consumed = consumed + inc, guarded by consumed + inc <= granted. The guard
// makes it a compare-and-set: if the guard fails, zero rows are affected and the
// caller turns that into an error instead of an over-consumed lot. Under the user
// advisory lock it cannot fail, and the guard is here so that a future code path
// that forgets the lock fails loudly rather than writing a lot the CHECK would
// reject three statements later with a much less useful message.
func (tx *TxContext) ConsumeLot(lotID uint, inc int64) error {
	res := tx.db.Exec(
		`UPDATE coin_lot SET consumed = consumed + ? WHERE id = ? AND consumed + ? <= granted`,
		inc, lotID, inc,
	)
	if res.Error != nil {
		return fmt.Errorf("consume lot %d by %d: %w", lotID, inc, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("lot %d could not take %d more: the row changed under this transaction", lotID, inc)
	}
	return nil
}

// LotByID reads one lot. It is a pointer so "no such lot" is nil rather than a
// zero-value lot that looks like a lot of nothing.
func (tx *TxContext) LotByID(id uint) (*CoinLot, error) {
	var lot CoinLot
	res := tx.db.Raw(
		`SELECT id, account_id, journal_id, bucket, granted, consumed, expires_at, created_at
		   FROM coin_lot WHERE id = ?`, id,
	).Scan(&lot)
	if res.Error != nil {
		return nil, fmt.Errorf("read lot %d: %w", id, res.Error)
	}
	if lot.ID == 0 {
		return nil, nil
	}
	return &lot, nil
}

// LotsOfJournal reads every lot a grant opened, oldest first.
//
// This is the only way to find a grant's lots: the grant's own postings carry no
// lot_id, because the lot is created after the postings so there is never a lot
// for a journal that did not post. A reversal therefore finds the lots to shrink
// here rather than from the legs it is negating. The id order is a stable
// "oldest first", which is the order the clawback spreads across.
func (tx *TxContext) LotsOfJournal(journalID string) ([]CoinLot, error) {
	var lots []CoinLot
	res := tx.db.Raw(
		`SELECT id, account_id, journal_id, bucket, granted, consumed, expires_at, created_at
		   FROM coin_lot WHERE journal_id = ? ORDER BY id`,
		journalID,
	).Scan(&lots)
	if res.Error != nil {
		return nil, fmt.Errorf("read lots of journal %s: %w", journalID, res.Error)
	}
	return lots, nil
}

// ShrinkLot reduces a lot's grant by a clawback, keeping the already-consumed
// part intact.
//
//	granted' = max(consumed, granted - reduction)
//
// The floor at `consumed` is what stops the CHECK (consumed <= granted) from
// failing when a clawback lands on coins that have already been spent, and it is
// also the correct answer: the value the lot can still contribute falls by
// exactly the reduction, because the spent part cannot be un-spent.
//
// A lot whose whole remaining value is clawed back has nothing left to represent,
// and setting granted = 0 would violate CHECK (granted > 0), so DeleteLot removes
// it. The grant journal and its postings are the permanent record; the lot was
// only ever the spendable-potential projection, and coin_lot is not append-only.
func (tx *TxContext) ShrinkLot(lotID uint, reduction int64) error {
	if reduction <= 0 {
		return nil
	}
	res := tx.db.Exec(
		`UPDATE coin_lot SET granted = GREATEST(consumed, granted - ?) WHERE id = ?`,
		reduction, lotID,
	)
	if res.Error != nil {
		return fmt.Errorf("shrink lot %d by %d: %w", lotID, reduction, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("lot %d could not be shrunk by %d: the row changed under this transaction", lotID, reduction)
	}
	return nil
}

// DeleteLot removes a lot that has no value left. See ShrinkLot for why deleting
// is correct and why setting granted = 0 is not.
func (tx *TxContext) DeleteLot(lotID uint) error {
	if err := tx.db.Exec(`DELETE FROM coin_lot WHERE id = ?`, lotID).Error; err != nil {
		return fmt.Errorf("delete lot %d: %w", lotID, err)
	}
	return nil
}

// ── postings ─────────────────────────────────────────────────────────────────

// PostingLeg is one row to write. Seq must be dense from 1: UNIQUE
// (journal_id, seq) plus the requirement that a journal's legs sum to zero means
// a gap is a bug, and the caller numbers them as it builds them.
type PostingLeg struct {
	Seq       int16
	AccountID uint
	Amount    int64
	LotID     *uint
}

// InsertPostings writes a journal's legs and is the ONLY way this package writes
// to coin_posting.
//
// It takes the FOR UPDATE lock on the target accounts first. See the file header:
// UNIQUE (account_id, account_seq) makes account_seq a per-account counter, and
// the system accounts are shared across users, so two transactions holding two
// different user locks would otherwise race for the same number. Locking the
// account rows in ascending id also fixes the lock order globally, which is what
// keeps this from being a deadlock source.
//
// Two invariants are enforced here rather than trusted:
//
//   - No zero amount. CHECK (amount <> 0) would reject it, but a zero leg is
//     always a sign that a caller passed the running total where it meant to pass
//     a per-lot increment and it happened to cancel — the shape of the bug in
//     §4. It is refused here with a message that says so.
//   - The legs sum to zero. A journal that does not net to zero breaks the
//     ledger's central property, and it is a two-line check at the point where
//     the numbers are known rather than a nightly reconciliation finding.
func (tx *TxContext) InsertPostings(journalID string, legs []PostingLeg) error {
	if len(legs) == 0 {
		return fmt.Errorf("journal %s has no postings; a money-moving journal must have at least two legs", journalID)
	}

	var sum int64
	accountIDs := make([]uint, 0, len(legs))
	for i, leg := range legs {
		if leg.Amount == 0 {
			return fmt.Errorf("journal %s leg %d has amount 0; post the per-lot increment, never an aggregate or a zero", journalID, leg.Seq)
		}
		if leg.Seq != int16(i+1) {
			return fmt.Errorf("journal %s leg %d has seq %d; seq must be dense from 1", journalID, i, leg.Seq)
		}
		sum += leg.Amount
		accountIDs = append(accountIDs, leg.AccountID)
	}
	if sum != 0 {
		return fmt.Errorf("journal %s does not net to zero: legs sum to %d", journalID, sum)
	}

	if err := tx.lockPostingTargets(accountIDs); err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, leg := range legs {
		accountSeq, err := tx.nextAccountSeq(leg.AccountID)
		if err != nil {
			return err
		}
		var lotID any
		if leg.LotID != nil {
			lotID = *leg.LotID
		}
		if err := tx.db.Exec(
			`INSERT INTO coin_posting (journal_id, seq, account_id, amount, account_seq, lot_id, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			journalID, leg.Seq, leg.AccountID, leg.Amount, accountSeq, lotID, now,
		).Error; err != nil {
			return fmt.Errorf("insert posting seq %d of journal %s: %w", leg.Seq, journalID, err)
		}
	}
	return nil
}

// lockPostingTargets takes the account locks for a set of postings, in ascending
// id order.
//
// One statement, with the ORDER BY, so the rows are locked in the same order in
// every transaction. For SELECT ... FOR UPDATE above a Sort, PostgreSQL locks as
// rows leave the sort, so the order is the sorted one. Sorting the ids in Go and
// using a plain IN would be just as correct and easier to read, so the ORDER BY is
// the mechanism and the sort in SQL is what makes it uniform.
//
// This is the ONLY thing standing between two transactions writing the same
// account_seq. See the file header, and see the note on the missing UNIQUE
// (account_id, account_seq) constraint at the end of this function.
func (tx *TxContext) lockPostingTargets(accountIDs []uint) error {
	unique := make([]uint, 0, len(accountIDs))
	seen := make(map[uint]struct{}, len(accountIDs))
	for _, id := range accountIDs {
		if id == 0 {
			return errors.New("posting leg has no account")
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return errors.New("no posting accounts to lock")
	}
	// FOR UPDATE on coin_account, not on coin_account_balance: nothing else in
	// the system writes coin_account, so this row is exclusively the ledger's
	// serialisation point for the shared system accounts.
	//
	// The result is Scan'd, NOT read off `.Error`. `db.Raw(...)` on its own only
	// PREPARES the statement; it does not send it. `db.Raw(...).Error` is
	// therefore a statement that never runs, silently, with a nil error. That bug
	// shipped here first: the lock appeared to work, every test passed, and the
	// only evidence was a cross-user concurrency test observing duplicate
	// account_seq values. A statement whose whole purpose is to block must be
	// executed by something that has to consume its rows, and the ids are
	// returned so there is no excuse for discarding them.
	var locked []uint
	if err := tx.db.Raw(
		`SELECT id FROM coin_account WHERE id IN (?) ORDER BY id FOR UPDATE`, unique,
	).Scan(&locked).Error; err != nil {
		return fmt.Errorf("lock posting accounts: %w", err)
	}
	if len(locked) != len(unique) {
		// A missing account row means a posting is about to reference account 0
		// or a deleted account, and the lock silently did not cover it.
		return fmt.Errorf("lock posting accounts: wanted %d rows, locked %d; an account referenced by a posting does not exist", len(unique), len(locked))
	}
	return nil
}

// nextAccountSeq reads the next per-account posting sequence.
//
// Only valid while the caller holds that account's row lock from
// lockPostingTargets. MAX(account_seq) + 1 is the whole of it.
//
// The spec's DDL in 02-architecture.md §2 declares UNIQUE (account_id,
// account_seq) alongside this column, and the natural reading is that the
// constraint is what makes the read-then-write safe once the lock is held. That
// is not true of the schema as shipped: the GORM model carries no uniqueIndex tag
// for AutoMigrate to honour, and EnsurePostgresIndexes does not add it either.
// Checked against a live instance — pg_constraint on coin_posting holds only
// coin_posting_pkey and chk_coin_posting_nonzero.
//
// So the row lock is the WHOLE mechanism rather than half of one, which makes it
// load-bearing in a way a database constraint would normally cover. That is
// reported rather than fixed here, because ledger_model.go and
// ensure_indexes.go are not this unit's to change. The invariant test asserts
// account_seq contiguity as the stand-in, and
// TestConcurrentGrantsAcrossDifferentUsersDoNotCollideOnAccountSeq is the test
// that fails when the lock stops working.
func (tx *TxContext) nextAccountSeq(accountID uint) (int64, error) {
	var seq int64
	if err := tx.db.Raw(
		`SELECT COALESCE(MAX(account_seq), 0) + 1 FROM coin_posting WHERE account_id = ?`,
		accountID,
	).Scan(&seq).Error; err != nil {
		return 0, fmt.Errorf("read next account_seq for %d: %w", accountID, err)
	}
	return seq, nil
}

// ── cached balance ───────────────────────────────────────────────────────────

// ApplyPosted moves the cached balance projection by a signed delta per account.
//
// raw SQL, not a struct Update: GORM omits zero-valued fields, and a delta of
// zero written as a struct would be silently dropped, which happens to be
// harmless here and would be catastrophic in the sibling that writes `reserved`.
//
// The no-overdraft CHECK is evaluated on this statement, so an overdraft is
// refused by the database rather than by a Go comparison that could be wrong.
func (tx *TxContext) ApplyPosted(deltas map[uint]int64, journalID string) error {
	return tx.applyDeltas("posted_balance = posted_balance + ?", deltas, journalID)
}

// ApplyReserved moves the hold projection by a signed delta per account.
//
// Same trap, same answer. A release takes reserved back towards zero, and a
// struct-based Update with Reserved: 0 writes nothing at all — the hold would
// stick and the user's coins would stay unspendable forever with no row anywhere
// recording that the hold was lifted.
func (tx *TxContext) ApplyReserved(deltas map[uint]int64, journalID string) error {
	return tx.applyDeltas("reserved = reserved + ?", deltas, journalID)
}

func (tx *TxContext) applyDeltas(assignment string, deltas map[uint]int64, journalID string) error {
	now := time.Now().UTC()
	for accountID, delta := range deltas {
		if delta == 0 {
			continue
		}
		if err := tx.db.Exec(
			`UPDATE coin_account_balance
			    SET `+assignment+`, version = version + 1, last_journal_id = ?, updated_at = ?
			  WHERE account_id = ?`,
			delta, journalID, now, accountID,
		).Error; err != nil {
			return fmt.Errorf("apply balance delta %d to account %d: %w", delta, accountID, err)
		}
	}
	return nil
}
