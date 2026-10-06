// internal/coins/unlock_repository.go
//
// Every SQL statement and every transaction boundary for the entitlement
// domain, for the same reason repository.go holds them for the ledger:
// 02-architecture.md §8 puts "GORM queries + all db.Transaction() calls (repo
// layer only)" in the repository, and unlock.go's split from this file is that
// same internal decomposition rather than a new layer. unlock.go decides; this
// file speaks.
//
// ── how the entitlement domain is serialised ────────────────────────────────
//
// Every write goes through InUserTx, which is the ledger's entry point: SET
// LOCAL lock_timeout, SET LOCAL idle_in_transaction_session_timeout, then
// pg_advisory_xact_lock('coin:user:' || id). The entitlement slice therefore
// shares the ledger's lock namespace rather than inventing a second one, for a
// concrete reason rather than tidiness: the eventual unlock gate will spend
// coins and record the entitlement, and if those two halves took different locks
// a transaction that nested them could deadlock against another request doing
// the same thing in the other order. One lock namespace makes that impossible,
// and re-entering it in the same session is a no-op.
//
// ── the two mechanisms, and what each one covers ────────────────────────────
//
// ConsumeAllowance has two jobs and they are guarded by two different things:
//
//  1. "Does this user already hold this resource?" is guarded by the
//     UNIQUE (user_id, resource_type, resource_id) constraint, via
//     ON CONFLICT DO NOTHING RETURNING. A Go check for it would race, and this
//     is exactly the ledger's InsertJournal shape.
//
//  2. "Does this user have an allowance left?" is a COUNT, and NO constraint can
//     guard a count. The quantity is configuration, so the ceiling moves, and a
//     unique index cannot express "at most the current value of a settings row".
//     The mechanism is serialisation: the per-user advisory lock from InUserTx,
//     plus the row lock on user_free_allowance in LockFreeAllowance. Two
//     requests for the same user are strictly ordered, so the second one's count
//     sees the first one's committed row and the last unlock is consumed once.
//
//     See unlock.go for the full argument and
//     TestConsumeAllowanceWithOneRemainingConsumesItExactlyOnce for the proof.
//
// ── raw SQL, not structs ─────────────────────────────────────────────────────
//
// GORM omits zero-valued fields on Create and on struct Updates, so a struct
// write of `coins_paid: 0` writes nothing at all and the column default wins.
// That trap has already cost this repository once (repository.go file header,
// internal/studyresources/repository.go:121). Every write below is therefore an
// explicit Exec or Raw with its NULLs spelled out, and every read a
// Raw(...).Scan(...) so the rows are actually consumed — a bare db.Raw(sql).Error
// only PREPARES a statement and does not send it.
package coins

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// unlockColumns is the column list for reading a ResourceUnlock. Spelled once so
// the transactional and non-transactional readers cannot drift, which is how a
// revoked_at comes back unset in one path and set in the other.
const unlockColumns = `id, user_id, resource_type, resource_id, journal_id, source,
	coins_paid, unlocked_at, revoked_at, revoke_reason`

// allowanceColumns is the column list for reading a UserFreeAllowance.
const allowanceColumns = `id, user_id, granted_at, expires_at,
	document_unlocks, video_unlocks, mock_test_unlocks`

// ── the derived "used" count ─────────────────────────────────────────────────
//
// Two projections of one predicate, in the two shapes the two callers need.
// Neither reads a stored counter, because there is none: see the file header of
// unlock_model.go for why a stored one would be the first thing to break.

// countAllowanceUnlocksByClass counts a user's un-revoked allowance unlocks per
// resource class, in one grouped query rather than three counts. The three
// classes are read together on the wallet render, and three index scans of the
// same rows is three times the work for no benefit.
func countAllowanceUnlocksByClass(db *gorm.DB, userID uint) (map[string]int64, error) {
	type row struct {
		ResourceType string
		Used         int64
	}
	var rows []row
	if err := db.Raw(
		`SELECT resource_type, count(*) AS used
		   FROM resource_unlock
		  WHERE user_id = ? AND source = 'ALLOWANCE' AND revoked_at IS NULL
		  GROUP BY resource_type`,
		userID,
	).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("count allowance unlocks for user %d: %w", userID, err)
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.ResourceType] = r.Used
	}
	return out, nil
}

// countAllowanceUnlocksOfClass is the single-class form, and it is the exact
// query documented on UserFreeAllowance. It is what ConsumeAllowance runs inside
// its transaction, where it must see rows committed by the transaction ahead of
// it in the per-user lock queue.
func countAllowanceUnlocksOfClass(db *gorm.DB, userID uint, resourceType string) (int64, error) {
	var used int64
	if err := db.Raw(
		`SELECT count(*) FROM resource_unlock
		  WHERE user_id = ? AND source = 'ALLOWANCE' AND resource_type = ?
		    AND revoked_at IS NULL`,
		userID, resourceType,
	).Scan(&used).Error; err != nil {
		return 0, fmt.Errorf("count %s allowance unlocks for user %d: %w", resourceType, userID, err)
	}
	return used, nil
}

// ── reads that do not need a transaction ─────────────────────────────────────

// FindUnlock reads ONE unlock, revoked or not, or nil when there is none.
//
// A caller that needs to know whether the row is ACTIVE must ask HasUnlock, not
// this: a revoked row is still a row, and FindUnlock is documented to return
// it. What FindUnlock no longer promises is WHICH row it returns when a triple
// has more than one — see below.
//
// ORDER BY revoked_at NULLS FIRST, and that is not tidiness. This query
// predates the partial index, when resource_unlock_uniq guaranteed at most one
// row per triple and no ordering was needed. That guarantee is deliberately
// gone: a revoked row no longer occupies the slot, so a triple can now hold a
// revoked row AND a live one, and a read with no ORDER BY returns whichever the
// planner reached first. Unordered, FindUnlock could hand back a year-old
// revoked row to a caller that believes it holds an active unlock.
//
// NULLS FIRST puts the live row first for the ordinary case, and the re-bought
// row over the revoked one it replaced. For a triple that is entirely revoked —
// which is what a caller asking about a revocation wants — it returns the
// oldest of them, so "what is the history here" starts at the beginning. Use
// ReadRevokedUnlock when the newest revocation is what matters.
func (r *Repository) FindUnlock(ctx context.Context, userID uint, resourceType string, resourceID uint64) (*ResourceUnlock, error) {
	if r == nil || r.db == nil {
		return nil, ErrNoDatabase
	}
	var u ResourceUnlock
	if err := r.db.WithContext(ctx).Raw(
		`SELECT `+unlockColumns+` FROM resource_unlock
		  WHERE user_id = ? AND resource_type = ? AND resource_id = ?
		  ORDER BY revoked_at NULLS FIRST
		  LIMIT 1`,
		userID, resourceType, resourceID,
	).Scan(&u).Error; err != nil {
		return nil, fmt.Errorf("read unlock for user %d %s/%d: %w", userID, resourceType, resourceID, err)
	}
	if u.ID == 0 {
		return nil, nil
	}
	return &u, nil
}

// HasUnlock reports whether the user holds an ACTIVE (un-revoked) unlock. It is
// the idempotency check every gate calls before doing anything else, and it is a
// plain indexed read: at most one row can match, because of the UNIQUE.
//
// revoked_at IS NULL is the whole of the revocation semantics. A revoked row
// does not grant access, and the student may re-buy it.
func (r *Repository) HasUnlock(ctx context.Context, userID uint, resourceType string, resourceID uint64) (bool, error) {
	if r == nil || r.db == nil {
		return false, ErrNoDatabase
	}
	var found bool
	if err := r.db.WithContext(ctx).Raw(
		`SELECT EXISTS (
			SELECT 1 FROM resource_unlock
			 WHERE user_id = ? AND resource_type = ? AND resource_id = ?
			   AND revoked_at IS NULL)`,
		userID, resourceType, resourceID,
	).Scan(&found).Error; err != nil {
		return false, fmt.Errorf("check active unlock for user %d %s/%d: %w", userID, resourceType, resourceID, err)
	}
	return found, nil
}

// FreeAllowance reads a user's allowance row, or nil when they have none. It
// creates nothing: "no allowance" and "an allowance of zero" are different
// questions, and RemainingAllowance answers the first without inventing the
// second.
func (r *Repository) FreeAllowance(ctx context.Context, userID uint) (*UserFreeAllowance, error) {
	if r == nil || r.db == nil {
		return nil, ErrNoDatabase
	}
	var a UserFreeAllowance
	if err := r.db.WithContext(ctx).Raw(
		`SELECT `+allowanceColumns+` FROM user_free_allowance WHERE user_id = ?`, userID,
	).Scan(&a).Error; err != nil {
		return nil, fmt.Errorf("read free allowance for user %d: %w", userID, err)
	}
	if a.ID == 0 {
		return nil, nil
	}
	return &a, nil
}

// CountAllowanceUnlocksByClass is the Repository-level reader for the derived
// "used" counts.
func (r *Repository) CountAllowanceUnlocksByClass(ctx context.Context, userID uint) (map[string]int64, error) {
	if r == nil || r.db == nil {
		return nil, ErrNoDatabase
	}
	return countAllowanceUnlocksByClass(r.db.WithContext(ctx), userID)
}

// ── in-transaction reads ─────────────────────────────────────────────────────

// CountAllowanceUnlocks is the single-class count, inside the transaction that is
// about to act on the answer.
func (tx *TxContext) CountAllowanceUnlocks(userID uint, resourceType string) (int64, error) {
	return countAllowanceUnlocksOfClass(tx.db, userID, resourceType)
}

// CountAllowanceUnlocksByClass is the grouped form, inside a transaction.
func (tx *TxContext) CountAllowanceUnlocksByClass(userID uint) (map[string]int64, error) {
	return countAllowanceUnlocksByClass(tx.db, userID)
}

// ReadUnlock reads the LIVE unlock for a triple inside the transaction that is
// deciding something about it: the TxContext twin of Repository.HasUnlock with
// the row rather than a boolean, used after a conflicting insert to find out WHO
// holds the row.
//
// `AND revoked_at IS NULL` is the correct filter here and not a narrowing of
// scope. This is only ever called on the path where InsertUnlock just returned
// created=false, and that boolean is produced by an ON CONFLICT against
// resource_unlock_live_uniq, which by construction conflicts only against a LIVE
// row. The row that caused the conflict is therefore always a live one, and
// asking for exactly that makes the read agree with the write instead of
// relying on a coincidence. Without it, a triple that holds a revoked row and a
// re-bought live row would return whichever came first, and the loser of the
// insert could be told somebody else holds the resource under a journal they
// have never heard of.
//
// A caller that wants the REVOCED row is not a variation of this one, it is a
// different question. Use ReadLatestRevokedUnlock, which Revoke does.
//
// LIMIT 1 is load-bearing for the same reason the ORDER BY is in FindUnlock:
// a Scan into a struct walks the result set, and an unbounded multi-row result
// is not a single answer.
func (tx *TxContext) ReadUnlock(userID uint, resourceType string, resourceID uint64) (*ResourceUnlock, error) {
	return tx.readUnlock(
		`SELECT `+unlockColumns+` FROM resource_unlock
		  WHERE user_id = ? AND resource_type = ? AND resource_id = ?
		    AND revoked_at IS NULL
		  LIMIT 1`,
		userID, resourceType, resourceID)
}

// ReadLatestRevokedUnlock reads the most recent REVOKED unlock for a triple, or
// nil when the user has never had one revoked.
//
// This is the other end of RevokeUnlock's compare-and-set. That method reports
// "nothing updated", and this one answers why: nil means there is no row at all
// and the caller should say ErrNotFound, a row here means the user does hold it
// and it is already revoked and the caller should say ErrUnlockRevoked. Those
// are different answers and a caller that cannot tell them apart retries
// forever, which is the whole reason this method exists.
//
// ORDER BY revoked_at DESC, and the direction is a decision rather than a
// default. A triple can now hold several revoked rows — buy, revoke, re-buy,
// revoke — and the one that answers "when and why did access go away" is the
// LATEST, because that is the revocation that is currently in force. Reporting
// the oldest would describe a state that was superseded and send a support
// agent looking at a reason the student is no longer subject to.
//
// Note this is a live query, not a scan of the partial index: the two indexes
// that cover this table for other reads are both partial on revoked_at IS NULL
// and physically cannot return this row. resource_unlock_user_revoked_idx is the
// one that serves it.
func (tx *TxContext) ReadLatestRevokedUnlock(userID uint, resourceType string, resourceID uint64) (*ResourceUnlock, error) {
	return tx.readUnlock(
		`SELECT `+unlockColumns+` FROM resource_unlock
		  WHERE user_id = ? AND resource_type = ? AND resource_id = ?
		    AND revoked_at IS NOT NULL
		  ORDER BY revoked_at DESC
		  LIMIT 1`,
		userID, resourceType, resourceID)
}

// readUnlock is the shared body of the two readers above. It exists so the
// "did I get a row?" check and the error wrapping cannot drift between them —
// and it is the reason both queries must carry their own LIMIT, because the
// zero-row case is decided here by u.ID == 0 rather than by the caller.
func (tx *TxContext) readUnlock(sql string, userID uint, resourceType string, resourceID uint64) (*ResourceUnlock, error) {
	var u ResourceUnlock
	if err := tx.db.Raw(sql, userID, resourceType, resourceID).Scan(&u).Error; err != nil {
		return nil, fmt.Errorf("read unlock for user %d %s/%d: %w", userID, resourceType, resourceID, err)
	}
	if u.ID == 0 {
		return nil, nil
	}
	return &u, nil
}

// LockFreeAllowance reads a user's allowance row FOR UPDATE, or nil.
//
// This row lock is the second half of ConsumeAllowance's concurrency argument.
// InUserTx already serialises per user with an advisory lock, so under the
// ledger's own convention this cannot block; it is here so that the count which
// follows it is still correct for a caller that reaches this method without the
// advisory lock, and so the guarantee is visible at the statement that depends
// on it rather than inferred from a lock three frames up.
//
// Note the limit of what it can do: a row lock on a row that does not exist locks
// nothing. Two transactions racing to create a user's first allowance are
// separated by UNIQUE (user_id) — `uni_user_free_allowance_user_id`, declared on
// the model — instead, which is why
// EnsureAllowanceRow resolves its conflict with ON CONFLICT DO NOTHING and
// re-reads rather than assuming it won.
func (tx *TxContext) LockFreeAllowance(userID uint) (*UserFreeAllowance, error) {
	var a UserFreeAllowance
	if err := tx.db.Raw(
		`SELECT `+allowanceColumns+` FROM user_free_allowance WHERE user_id = ? FOR UPDATE`,
		userID,
	).Scan(&a).Error; err != nil {
		return nil, fmt.Errorf("lock free allowance for user %d: %w", userID, err)
	}
	if a.ID == 0 {
		return nil, nil
	}
	return &a, nil
}

// ── in-transaction writes ────────────────────────────────────────────────────

// InsertUnlock writes one resource_unlock row, or reports that its
// (user_id, resource_type, resource_id) LIVE slot is already taken.
//
// ON CONFLICT DO NOTHING rather than a read-then-write, for the reason
// repository.go gives for InsertJournal: two transactions for the same resource
// can both find nothing and both insert, and under READ COMMITTED the loser's
// failure arrives as a unique violation instead of as the caller-visible
// "already unlocked" the caller can act on. Resolving it against the index turns
// the race into a returned boolean. The conflicting row must already be
// committed for RowsAffected to be 0, so `created == false` is never reported
// from a transaction that later rolls back.
//
// ── the WHERE clause is load-bearing, not decoration ─────────────────────────
//
// `WHERE revoked_at IS NULL` on the ON CONFLICT is what makes this statement
// legal at all. resource_unlock_live_uniq is a PARTIAL unique index, and
// Postgres index inference does not infer through a partial index from a bare
// column list: a partial index only matches an inference clause that carries a
// predicate the planner can prove is implied by the index's own predicate. With
// the bare column list this statement does not fail at INSERT time with a
// duplicate — it fails at PREPARE time, every time, with:
//
//	ERROR: there is no unique or exclusion constraint matching
//	       the ON CONFLICT specification   (SQLSTATE 42P10)
//
// which means every unlock write in the system fails, on every call, forever.
// This is the same 42P10 that internal/notification/ensure_indexes.go
// documents as a production incident, and it is why the predicate is written out
// in full rather than left to a reader to infer. Do not "simplify" this back to
// a bare column list.
//
// The semantics are the same either way, and they are the right ones: a
// conflicting LIVE row means somebody already holds this resource. A REVOKED
// row is not a conflict and must not be, or a student whose unlock was revoked
// could never buy it back — which is the bug the partial index was introduced
// to fix, reintroduced here as an inference detail.
//
// The funding triple is validated in Go before the write so the caller gets
// ErrInvalidArgument with the field named; chk_resource_unlock_source_funding is
// the backstop for any path that reaches the table without going through here.
func (tx *TxContext) InsertUnlock(u *ResourceUnlock) (created bool, err error) {
	if u == nil {
		return false, fmt.Errorf("%w: no unlock to insert", ErrInvalidArgument)
	}
	// The source is checked first and separately so an unknown one is reported as
	// an unknown source, rather than folded into the funding-shape refusal below
	// where the message does not say which field was wrong.
	if err := validateUnlockSource(u.Source); err != nil {
		return false, err
	}
	if !validUnlockFunding(u.Source, u.JournalID, u.CoinsPaid) {
		return false, fmt.Errorf("%w: source %s with journal %v and coins_paid %d is not a funding shape any unlock can have",
			ErrInvalidArgument, u.Source, u.JournalID, u.CoinsPaid)
	}
	var anyJournal any
	if u.JournalID != nil {
		anyJournal = *u.JournalID
	}
	var id uint
	res := tx.db.Raw(
		`INSERT INTO resource_unlock
			(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (user_id, resource_type, resource_id) WHERE revoked_at IS NULL DO NOTHING
		 RETURNING id`,
		u.UserID, u.ResourceType, u.ResourceID, anyJournal, u.Source, u.CoinsPaid, u.UnlockedAt,
	).Scan(&id)
	if res.Error != nil {
		return false, fmt.Errorf("insert unlock for user %d %s/%d: %w", u.UserID, u.ResourceType, u.ResourceID, res.Error)
	}
	if res.RowsAffected == 0 {
		return false, nil
	}
	u.ID = id
	return true, nil
}

// EnsureAllowanceRow inserts a user's allowance row, or reports that they
// already have one.
//
// The UNIQUE (user_id) constraint is the idempotency mechanism, exactly as
// coin_journal_idem_uniq is for a grant. A SELECT-then-INSERT in Go would race
// two registration paths into two rows, and the second insert's failure would
// surface as a constraint error rather than as "they already have one".
func (tx *TxContext) EnsureAllowanceRow(a *UserFreeAllowance) (created bool, err error) {
	if a == nil {
		return false, fmt.Errorf("%w: no allowance row to ensure", ErrInvalidArgument)
	}
	var id uint
	res := tx.db.Raw(
		`INSERT INTO user_free_allowance
			(user_id, granted_at, expires_at, document_unlocks, video_unlocks, mock_test_unlocks)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT (user_id) DO NOTHING
		 RETURNING id`,
		a.UserID, a.GrantedAt, a.ExpiresAt,
		a.DocumentUnlocks, a.VideoUnlocks, a.MockTestUnlocks,
	).Scan(&id)
	if res.Error != nil {
		return false, fmt.Errorf("ensure allowance for user %d: %w", a.UserID, res.Error)
	}
	if res.RowsAffected == 0 {
		return false, nil
	}
	a.ID = id
	return true, nil
}

// RevokeUnlock stamps an ACTIVE unlock as revoked, and reports whether it did.
//
// The `revoked_at IS NULL` guard in the WHERE clause is a compare-and-set, the
// same shape as SetJournalState: a second caller that revokes an already-revoked
// row affects nothing and is told so, rather than overwriting the first
// revocation's timestamp and reason. Revocation is a reversal, so the first one
// is the record.
//
// Raw SQL rather than a struct Updates, because the reason is a *string and GORM
// would omit it entirely if it were nil; see this file's header.
func (tx *TxContext) RevokeUnlock(userID uint, resourceType string, resourceID uint64, reason string, now time.Time) (bool, error) {
	res := tx.db.Exec(
		`UPDATE resource_unlock
		    SET revoked_at = ?, revoke_reason = ?
		  WHERE user_id = ? AND resource_type = ? AND resource_id = ?
		    AND revoked_at IS NULL`,
		now, reason, userID, resourceType, resourceID,
	)
	if res.Error != nil {
		return false, fmt.Errorf("revoke unlock for user %d %s/%d: %w", userID, resourceType, resourceID, res.Error)
	}
	return res.RowsAffected > 0, nil
}
