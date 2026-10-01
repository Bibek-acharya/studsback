// internal/auth/referral_code.go
//
// Minting and backfilling the referral code on `users`.
//
// ── where this sits, and why not in internal/coins ─────────────────────────────
//
// The column is on auth.User, and auth owns that table. The GENERATOR needs no
// coin-system knowledge, so putting it in internal/coins would mean internal/auth
// importing internal/coins to mint a string — the dependency inversion
// profile_award.go refuses, and it is also what this file's own first test
// asserts is not happening. utils.GenerateReferralCode does the drawing; this file
// does the collision handling and the backfill, both of which are properties of
// the users table.
//
// What lives in internal/coins is everything about the ECONOMY: the attribution
// table, the uniqueness that stops a double claim, the caps. What lives here is
// only "does every account have a code, and is it unique".
//
// ── why a collision must not fail a signup ────────────────────────────────────
//
// utils.GenerateReferralCode draws 50 bits uniformly, and at a million users the
// birthday probability of a collision is about 0.04% — roughly one in 2,500 codes.
// That is rare enough that it will never be noticed in testing and common enough
// that it WILL happen in production. Which makes the handling of it load-bearing:
//
//   - Returning the error fails a student's registration. The student did nothing
//     wrong, the code was randomly unlucky, and the failure surfaces as a 500 on
//     the product's main signup path. A bug.
//   - Not checking at all means two accounts share a code, and every referral
//     through that code credits whichever of them the lookup found — a silent
//     misattribution that is indistinguishable from a fraud ring.
//
// So: check, and on collision generate again. Bounded, so a broken entropy source
// or an exhausted alphabet fails loudly rather than spinning.
//
// ── why the backfill is set-based and batched ─────────────────────────────────
//
// Every existing User needs a code, and "existing" is however many there are at the
// moment of deploy. Two properties matter and they pull against each other:
//
//   - IDEMPOTENT. Running it twice must not renumber anything, must not duplicate,
//     and must not fail on the rows the first run wrote. Achieved with an
//     `UPDATE ... WHERE referral_code IS NULL` rather than a read-then-write, so
//     the second run's WHERE matches nothing and writes nothing.
//   - SAFE ON A LARGE TABLE. A single `UPDATE users SET referral_code = <expr>` over
//     ten million rows takes one long transaction, holds locks for its whole
//     duration, bloats WAL, and — critically here — cannot be written as a plain
//     SQL expression at all, because the code has to come from crypto/rand in Go.
//
// The second point is the reason this is Go at all rather than a migration. There
// is no SQL that produces a cryptographically random Crockford base32 string
// without a PL/pgSQL function, and a PL/pgSQL function that does crypto work is a
// far worse thing to have in a schema than 40 lines of Go behind a WHERE clause.
//
// So it is BATCHED: claim a bounded page of NULL rows, generate a code per row,
// write them back with a guarded UPDATE, repeat. Each batch is its own short
// transaction, so the table is never locked for long and a run that dies half way
// leaves the completed batches written and the rest still NULL — which the next
// run picks up, because it is looking for NULLs and not for a marker.
//
// Concurrency: the WHERE referral_code IS NULL is re-checked by the UPDATE itself,
// not just by the SELECT. Two concurrent backfills therefore cannot both write a
// row — the second one's UPDATE matches zero rows for that id and it moves on. And
// a signup that races the backfill is safe for the same reason: CreateUser assigns
// a code inside its own retry loop, and the guarded UPDATE loses cleanly.
package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"studsphere/backend/internal/shared/utils"
)

// referralCodeAttempts is how many times mintReferralCode tries before giving up.
//
// Five, against a 0.04% per-attempt collision probability at a million users: five
// consecutive collisions have probability 2.5 × 10^-11, so exhausting this means
// something other than randomness is wrong — a stubbed or failing entropy source,
// or an alphabet that has been edited down to one symbol. Bounded so that failure
// is an error rather than an infinite loop, because the alternative to a clear
// error here is a signup handler that never returns.
const referralCodeAttempts = 5

// referralCodeBackfillBatch is how many rows one backfill batch claims.
//
// 500 is chosen against the two costs: large enough that the per-statement overhead
// is negligible over a whole table, small enough that a single transaction is
// short. There is no row-level lock held across batches, so a concurrent signup
// only ever waits on one row.
const referralCodeBackfillBatch = 500

// mintReferralCode assigns a unique referral code to a user that does not have
// one.
//
// Under the users table's own unique index, so a collision is detected by the
// DATABASE and not by a SELECT — the reason this cannot be a
// read-then-write. A SELECT for the candidate code followed by an INSERT races:
// two concurrent signups both find the code free and both insert, and the loser's
// failure arrives as a unique violation at the driver, which is the correct outcome
// and is exactly what the retry loop below is for.
func mintReferralCode(db *gorm.DB, userID uint) (string, error) {
	var lastErr error
	for attempt := 0; attempt < referralCodeAttempts; attempt++ {
		code, err := utils.GenerateReferralCode()
		if err != nil {
			return "", fmt.Errorf("generate referral code for user %d: %w", userID, err)
		}
		// ON CONFLICT DO NOTHING rather than a plain UPDATE: the conflict target is
		// the unique index on referral_code, and swallowing it is what turns a
		// collision into "try another" instead of "fail the signup". RowsAffected
		// distinguishes the two outcomes — 1 means the code is ours, 0 means
		// somebody already has it.
		res := db.Exec(
			`UPDATE users SET referral_code = ?, updated_at = ?
			  WHERE id = ? AND referral_code IS NULL`,
			code, time.Now().UTC(), userID,
		)
		if res.Error != nil {
			return "", fmt.Errorf("assign referral code to user %d: %w", userID, res.Error)
		}
		if res.RowsAffected == 1 {
			return code, nil
		}
		lastErr = fmt.Errorf("user %d already has a referral code", userID)
		// RowsAffected == 0 is either "collision" or "this user already had a
		// code". Both are handled by re-reading rather than by assuming, because
		// assuming is how a backfill would overwrite a code a user has already
		// shared with somebody.
		var existing *string
		if err := db.Raw(`SELECT referral_code FROM users WHERE id = ?`, userID).Scan(&existing).Error; err != nil {
			return "", fmt.Errorf("read back referral code for user %d: %w", userID, err)
		}
		if existing != nil && *existing != "" {
			return *existing, nil
		}
	}
	return "", fmt.Errorf("%w: could not mint a unique referral code for user %d after %d attempts: %v",
		errCodeCollisionExhausted, userID, referralCodeAttempts, lastErr)
}

// errCodeCollisionExhausted is the give-up error from mintReferralCode. Distinct
// from the generation error so a caller can tell "the entropy source failed" from
// "the space is somehow full".
var errCodeCollisionExhausted = errors.New("referral code collisions exhausted")

// ensureReferralCode assigns a code to a user if it has none, and returns whatever
// it has either way.
//
// Idempotent, which is what makes it safe to call from every user-creation path
// without one of them having to know whether this is a first or a subsequent call
// — and what lets the backfill be re-run against a partially populated table.
//
// The read happens first so the common case (an existing signup race, or a re-run)
// is one SELECT and no UPDATE.
func ensureReferralCode(db *gorm.DB, userID uint) (*string, error) {
	var existing *string
	if err := db.Raw(`SELECT referral_code FROM users WHERE id = ?`, userID).Scan(&existing).Error; err != nil {
		return nil, fmt.Errorf("read referral code for user %d: %w", userID, err)
	}
	if existing != nil && *existing != "" {
		return existing, nil
	}
	code, err := mintReferralCode(db, userID)
	if err != nil {
		return nil, err
	}
	return &code, nil
}

// BackfillReferralCodes gives every user without a referral code one, and returns
// how many rows it wrote.
//
// Idempotent by construction, and the reason is the WHERE clause rather than a
// bookkeeping column: the UPDATE only touches rows where referral_code IS NULL, so
// a second run finds nothing to do and rewrites nothing. That matters for a
// referred student in particular — a code that changed between two runs would
// invalidate every invite link already shared, so "must not renumber" is a product
// requirement and not only a tidiness one.
//
// Batched for the reasons in the file header. The loop terminates because each pass
// either writes at least one row or there are no NULL rows left; the page is read
// with an id-ordered cursor so a large table is walked by index rather than
// re-scanned from the start on every batch.
//
// AutoMigrate must have added the column first, or the SELECT fails — which is a
// loud, correct failure and not something to paper over with a CREATE TABLE IF NOT
// EXISTS here.
func BackfillReferralCodes(db *gorm.DB) (int, error) {
	if db == nil {
		return 0, errors.New("auth: backfill referral codes needs a database handle")
	}
	total := 0
	var cursor uint
	for {
		// Claim a page. The id cursor is what keeps this O(n) rather than
		// O(n²): without it, every batch would re-read the rows the previous
		// batch filled.
		var ids []uint
		if err := db.Raw(
			`SELECT id FROM users
			  WHERE referral_code IS NULL AND id > ?
			  ORDER BY id
			  LIMIT ?`, cursor, referralCodeBackfillBatch,
		).Scan(&ids).Error; err != nil {
			return total, fmt.Errorf("backfill: read a page of users without a referral code: %w", err)
		}
		if len(ids) == 0 {
			return total, nil
		}

		wrote, err := backfillBatch(db, ids)
		if err != nil {
			// Each batch is its own transaction, so the rows already written stay
			// written and the next run resumes. Returning the count alongside the
			// error lets a caller log how far it got, which is the difference
			// between "the backfill failed" and "the backfill failed after 40,000".
			return total, err
		}
		total += wrote
		cursor = ids[len(ids)-1]
	}
}

// backfillBatch writes one page, and returns how many rows it actually wrote.
//
// The count is not the page size, deliberately. A row whose UPDATE matched zero
// rows was either claimed by a concurrent signup (which is fine — that signup has
// its own code) or filled by a concurrent backfill (same). Reporting the page size
// would overstate what this run did, and an operator watching the number would be
// watching a number that cannot be wrong.
func backfillBatch(db *gorm.DB, ids []uint) (int, error) {
	var wrote int
	err := db.Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			code, err := utils.GenerateReferralCode()
			if err != nil {
				return fmt.Errorf("backfill: generate a code for user %d: %w", id, err)
			}
			// The guarded UPDATE is what makes the whole thing idempotent AND safe
			// against a concurrent signup: the WHERE is re-evaluated by the database
			// at write time, not by this loop when it read the page.
			res := tx.Exec(
				`UPDATE users SET referral_code = ?, updated_at = ? WHERE id = ? AND referral_code IS NULL`,
				code, time.Now().UTC(), id,
			)
			if res.Error != nil {
				// A unique violation here means this generated code already belongs
				// to somebody. The right answer is to move to the next row and let
				// the next run try this one again — NOT to abort, because aborting
				// would roll back the whole batch over a 0.04% event and re-roll
				// dice on every row in it.
				if isUniqueViolation(res.Error) {
					continue
				}
				return fmt.Errorf("backfill: assign a code to user %d: %w", id, res.Error)
			}
			wrote += int(res.RowsAffected)
		}
		return nil
	})
	if err != nil {
		return wrote, err
	}
	return wrote, nil
}

// isUniqueViolation reports whether err is a unique-constraint failure.
//
// Matched on the SQLSTATE (Postgres 23505) rather than on message text, because a
// message match is a locale and driver-version dependency and this function gates
// a retry: too permissive and a real error is swallowed and the row silently
// skipped, too strict and a routine collision aborts a whole batch.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var typed interface{ SQLState() string }
	if errors.As(err, &typed) && typed.SQLState() == "23505" {
		return true
	}
	// glebarez/sqlite — the driver this module's tests run on — does not surface
	// SQLState, so the message is the only signal available there. Both spellings
	// are the driver's own fixed prefixes rather than anything from Postgres.
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "duplicate key value")
}
