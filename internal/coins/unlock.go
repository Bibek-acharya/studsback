// internal/coins/unlock.go
//
// The entitlement domain: 02-architecture.md §5, and the shapes in
// 03-api-contract.md §2.1 and §2.3. No HTTP is added here on purpose — the
// gates that call these are a later slice, and what this file owns is the
// decision layer plus the concurrency argument that makes the decision safe.
//
// ── why this is not a balance ─────────────────────────────────────────────────
//
// See unlock_model.go. Three lines carry it: an unlock is not a coin movement, a
// revocation is not a reversal, and the allowance is a cap on a count. A ledger
// expresses none of the three.
//
// ── the concurrency argument for ConsumeAllowance ────────────────────────────
//
// The naive version is `count remaining; if > 0 then insert`, and it double-spends
// the last unlock: two requests for one user both read "1 left", both see 1 > 0,
// and both insert. The window is the whole of the count-to-insert gap.
//
// Two mechanisms, and it is worth being precise about which covers which,
// because "we took a lock" is not an answer on its own:
//
//  1. THE CONSTRAINT covers the duplicate resource.
//     resource_unlock_live_uniq — UNIQUE (user_id, resource_type, resource_id)
//     WHERE revoked_at IS NULL — makes the insert itself the decision. ON CONFLICT
//     DO NOTHING RETURNING id is resolved against that index, so exactly one of
//     two concurrent inserts for the same (user, class, resource) gets a live
//     row, and the other is told so as ErrAlreadyUnlocked rather than as a raw
//     unique violation. No read precedes it that could be stale.
//
//     The ON CONFLICT carries `WHERE revoked_at IS NULL` and MUST, because
//     Postgres will not infer a partial index from a bare column list: without
//     the predicate the statement fails to prepare with 42P10 on every call.
//     See InsertUnlock.
//
//     "Live" is the whole of the scope, and it is deliberate. A revoked row
//     grants no access, so it does not occupy the one-unlock cap, and a student
//     whose unlock was revoked can buy it again — at full price, so the cap is
//     not weakened. The revoked row is kept, not overwritten: it is the record
//     of what was granted and what was taken away.
//
//  2. THE LOCK covers the count, because no constraint can.
//     The allowance ceiling is a NUMBER IN CONFIGURATION, not a schema constant,
//     so there is no index to build: "at most the current value of a settings
//     row" is not an expressible constraint. The only way to make read-then-insert
//     atomic is to stop anything else from interleaving, and this file does that
//     with the ledger's own mechanism — pg_advisory_xact_lock('coin:user:' || id)
//     via InUserTx, the same lock Spend and Grant take, plus the FOR UPDATE on
//     user_free_allowance in LockFreeAllowance. Two requests for one user are
//     strictly ordered; the second one's count runs under READ COMMITTED after
//     the first one has committed, so it sees the first one's row and refuses.
//
//     Why the same lock as the ledger rather than a new one: the eventual gate
//     spends coins and records the entitlement, and two lock namespaces across
//     those halves is a deadlock waiting for the first request that nests them in
//     the other order.
//
// The proof is TestConsumeAllowanceWithOneRemainingConsumesItExactlyOnce: eight
// concurrent requests, one allowance left, one winner, seven
// ErrNoAllowanceRemaining, one row.
//
// ── a deliberate non-feature: no stored `used` counter ───────────────────────
//
// See unlock_model.go. The short version: it would be a second copy of a truth
// that resource_unlock already holds, and the two disagree the first time an
// unlock is revoked.
package coins

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// NewServiceWithRepository returns a Service that can also reach the database.
//
// It exists alongside NewService rather than replacing it, because NewService is
// the admin-config wiring and its two existing callers should not have to know
// about a repository they never use. A Service built with NewService has a nil
// repo, and every entitlement method on it returns ErrNoDatabase rather than
// panicking — the same way the ledger treats a missing config store.
func NewServiceWithRepository(repo *Repository, config *ConfigStore, versions VersionStore) *Service {
	return &Service{config: config, versions: versions, repo: repo}
}

// ── the allowance shape ──────────────────────────────────────────────────────

// allowanceClass binds a resource class to the allowance column that holds its
// entitlement. One table rather than a switch in three places, so "which class
// does the allowance cover, and where is the limit stored" has one answer.
//
// The three classes are exactly ResourceTypes, and chk_resource_unlock_type
// restates them as SQL. A fourth class is: one line here, one in ResourceTypes,
// one in the CHECK, one in Prices and one in AllowanceConfig — five edits, all of
// which are refused if forgotten, because ValidateEconomyConfig and the CHECK
// both fail loudly rather than silently granting nothing.
type allowanceClass struct {
	name  string
	quota func(AllowanceConfig) int64
}

var allowanceClasses = []allowanceClass{
	{ResourceTypeStudyResource, func(c AllowanceConfig) int64 { return c.DocumentUnlocks }},
	{ResourceTypeVideo, func(c AllowanceConfig) int64 { return c.VideoUnlocks }},
	{ResourceTypeMockTest, func(c AllowanceConfig) int64 { return c.MockTestUnlocks }},
}

// allowanceClassFor resolves a resource class to its allowance column. The slice
// is scanned rather than indexed by map because it holds three entries and
// because the ordered form is what ResourceTypes and the error messages want.
func allowanceClassFor(resourceType string) (allowanceClass, bool) {
	for _, c := range allowanceClasses {
		if c.name == resourceType {
			return c, true
		}
	}
	return allowanceClass{}, false
}

// ClassAllowance is one class's entitlement arithmetic: what was granted, what
// has been burned, and what is left. Used and Remaining are always both present
// because the balance response in 03-api-contract.md §2.1 renders the used count
// next to the granted one, and a caller that had to re-derive one of them from
// the other could get the expiry rule wrong.
type ClassAllowance struct {
	ResourceType string
	Granted      int64
	Used         int64
	Remaining    int64
}

// AllowanceStatus is the per-class view of one user's free allowance, plus the
// window it lives in. Classes is keyed by resource class and always holds an
// entry for every class in ResourceTypes, including the ones with a grant of
// zero, so a caller can render "0 of 0" rather than having to know that absence
// means zero.
type AllowanceStatus struct {
	GrantedAt time.Time
	ExpiresAt time.Time
	// Expired is the window verdict at the instant the status was computed.
	// ExpiresAt is still reported, because "your allowance ran out on the 26th"
	// is more useful to a student than a bare zero.
	Expired        bool
	Classes        map[string]ClassAllowance
	TotalRemaining int64
}

// Remaining returns the class's remaining count. Absent classes read as zero.
func (s AllowanceStatus) Remaining(resourceType string) int64 {
	if c, ok := s.Classes[resourceType]; ok {
		return c.Remaining
	}
	return 0
}

// ── pure helpers ─────────────────────────────────────────────────────────────
//
// These carry the rules and no I/O, which is what makes the rules testable
// without a database. Each one is the Go mirror of a constraint or of a
// documented sentence, and each says which.

// allowanceExpired is the expiry rule, and the boundary is the part that matters:
// an allowance whose ExpiresAt is EXACTLY now is expired. `now < expires_at` is
// the usable window, so the usable window is half-open and there is no instant at
// which an allowance is simultaneously expired and usable. An inclusive
// comparison here would make a request landing on the boundary behave
// differently from the microsecond before it.
func allowanceExpired(now, expiresAt time.Time) bool {
	return !now.Before(expiresAt)
}

// allowanceExpiry is granted_at plus the configured lifetime in days.
//
// A negative lifetime is refused by the caller rather than turned into a date in
// the past: `allowance.expires_in_days` is validated as non-negative by
// ValidateEconomyConfig, so a negative value can only arrive from a bug, and
// silently producing an already-expired allowance would hide it. Zero is
// accepted and means "usable for no time at all", which is the literal meaning of
// a zero-day allowance and a legitimate way for an admin to disable it.
func allowanceExpiry(grantedAt time.Time, days int64) (time.Time, error) {
	if days < 0 {
		return time.Time{}, fmt.Errorf("%w: allowance.expires_in_days is %d", ErrInvalidArgument, days)
	}
	return grantedAt.AddDate(0, 0, int(days)), nil
}

// remainingUnlocks is the whole of the per-class arithmetic: granted minus used,
// floored at zero, and zero when the window has closed.
//
// An expired allowance contributes nothing REGARDLESS of what is unused. That is
// the rule that stops "save them for the weekend" from being a strategy, and it
// is why Expired is an input to the arithmetic rather than something the caller
// checks separately afterwards.
func remainingUnlocks(granted, used int64, expired bool) int64 {
	if expired {
		return 0
	}
	if left := granted - used; left > 0 {
		return left
	}
	return 0
}

// computeAllowanceStatus is the derivation, as a pure function of the stored
// row, the derived used counts and the instant being asked about.
//
// A nil allowance row is a zero status rather than an error: "this user has no
// allowance" is a thing the wallet renders every day for a user who has not
// registered through the hook yet, and it is not a failure. It is also
// deliberately NOT Expired, because an allowance that was never granted has not
// lapsed — reporting it as expired would tell a student their free unlocks
// expired when they were never issued.
func computeAllowanceStatus(a *UserFreeAllowance, used map[string]int64, now time.Time) AllowanceStatus {
	status := AllowanceStatus{Classes: make(map[string]ClassAllowance, len(allowanceClasses))}
	if a == nil {
		for _, c := range allowanceClasses {
			status.Classes[c.name] = ClassAllowance{ResourceType: c.name}
		}
		return status
	}
	status.GrantedAt = a.GrantedAt
	status.ExpiresAt = a.ExpiresAt
	status.Expired = allowanceExpired(now, a.ExpiresAt)
	for _, c := range allowanceClasses {
		// The row holds the grant, not the config: a student granted three
		// document unlocks has three even after an admin lowers the default.
		granted := c.quota(AllowanceConfig{
			DocumentUnlocks: a.DocumentUnlocks,
			VideoUnlocks:    a.VideoUnlocks,
			MockTestUnlocks: a.MockTestUnlocks,
		})
		burned := used[c.name]
		remaining := remainingUnlocks(granted, burned, status.Expired)
		status.Classes[c.name] = ClassAllowance{
			ResourceType: c.name,
			Granted:      granted,
			Used:         burned,
			Remaining:    remaining,
		}
		status.TotalRemaining += remaining
	}
	return status
}

// validateResourceType refuses anything that is not one of the three classes.
//
// This is ErrInvalidResourceType rather than ErrInvalidArgument because it is the
// one validation error in this slice with its own HTTP mapping coming: a request
// naming an un-unlockable class is a 400 about a named field, and the handler
// will want to say which field without string-matching a message.
func validateResourceType(resourceType string) error {
	if _, ok := allowanceClassFor(resourceType); ok {
		return nil
	}
	return fmt.Errorf("%w: %q is not an unlockable class; it is one of %s",
		ErrInvalidResourceType, resourceType, strings.Join(ResourceTypes, ", "))
}

// validateUnlockSource refuses an unknown source.
//
// It runs ahead of validUnlockFunding in InsertUnlock, and the two are separate
// checks on purpose. validUnlockFunding is a boolean that folds the source into a
// funding-shape test, so an unknown source reaches it and is refused as "not a
// funding shape" — true, but the message does not say WHICH field was wrong. This
// names the source first, so a caller that typo'd it is told so.
//
// ErrInvalidArgument rather than a new sentinel: an unknown source is a
// programming error in a caller of this package, and it is the same class of
// mistake as an unknown reason code reaching Grant.
func validateUnlockSource(source string) error {
	for _, s := range UnlockSources {
		if s == source {
			return nil
		}
	}
	return fmt.Errorf("%w: %q is not an unlock source; it is one of %s",
		ErrInvalidArgument, source, strings.Join(UnlockSources, ", "))
}

// validUnlockFunding is the Go mirror of chk_resource_unlock_source_funding, and
// the reason the CHECK is a backstop rather than the only guard.
//
//	COINS     -> a journal, and any non-negative amount
//	ALLOWANCE -> no journal, and no coins
//
// An allowance unlock that names a journal claims coins were charged for it; a
// coin unlock with no journal is money that moved and left no trace of what for.
// Neither is reconstructable afterwards, so both are refused before the write
// with the field named, and again by the database for any path that does not
// come through here.
//
// A zero-amount COINS unlock is allowed by this rule, and the reason is that
// refusing it here would be a second policy: a price of zero is the ledger's
// business (spendPrice, and Spend's own refusal of a zero price), and this
// package records what the ledger settled rather than re-deciding it.
func validUnlockFunding(source string, journalID *string, coinsPaid int64) bool {
	switch source {
	case UnlockSourceCoins:
		return journalID != nil && *journalID != "" && coinsPaid >= 0
	case UnlockSourceAllowance:
		return journalID == nil && coinsPaid == 0
	}
	return false
}

// validateResourceRef is the argument check every method shares: a real user, a
// real class and a real resource. A zero resource_id is rejected here rather than
// becoming a row that unlocks resource 0, which is nothing.
func validateResourceRef(userID uint, resourceType string, resourceID uint64) error {
	if err := validateUser(userID); err != nil {
		return err
	}
	if err := validateResourceType(resourceType); err != nil {
		return err
	}
	if resourceID == 0 {
		return fmt.Errorf("%w: an unlock needs a resource id, and 0 is not one", ErrInvalidArgument)
	}
	return nil
}

// requireRepo is the nil-repository guard shared by every method that touches
// the database. ErrNoDatabase rather than a panic: a Service wired by NewService
// alone is a legitimate construction for the admin-config slice, and calling an
// entitlement method on one is a wiring mistake that should read as one.
func (s *Service) requireRepo() error {
	if s == nil || s.repo == nil || s.repo.db == nil {
		return ErrNoDatabase
	}
	return nil
}

// ── EnsureAllowance ──────────────────────────────────────────────────────────

// EnsureAllowance creates the one-time starter allowance for a user, sized from
// the economy config, and is idempotent per user.
//
// Idempotency is UNIQUE (user_id) on user_free_allowance, resolved with ON
// CONFLICT DO NOTHING — the same shape as the ledger's idempotency key. A
// SELECT-then-INSERT here would race two registration paths, which is a real
// shape in this codebase: all four user-creation paths call applyAttribution
// (02-architecture.md §10.1), and a future retry of a verification would be the
// second caller.
//
// An existing row is returned UNCHANGED. That is the load-bearing decision and
// it cuts two ways, both of which argue for it:
//
//   - Re-granting would reset granted_at and expires_at, so a user could extend
//     their allowance forever by triggering a code path that calls this again.
//     The starter allowance is once per lifetime, and a re-grant is how "once per
//     lifetime" stops being true.
//   - Re-sizing it upward to the current config would be the same bug with a
//     friendlier name: an admin raising document_unlocks from 3 to 5 would hand
//     five to every existing user who happened to call this again.
//
// So the quantities are a snapshot taken at first grant, and the config is read
// again only for a user who has never had one. The expiry is
// granted_at + allowance.expires_in_days, recorded once.
func (s *Service) EnsureAllowance(ctx context.Context, userID uint, now time.Time) (*UserFreeAllowance, error) {
	if err := validateUser(userID); err != nil {
		return nil, err
	}
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if s.config == nil {
		return nil, ErrNoDatabase
	}
	// The config is loaded BEFORE the transaction, for the reason Grant gives:
	// holding a user advisory lock across a round trip to another module's table
	// is how a 3-second lock_timeout starts firing in production. The cache
	// makes it free on the hot path.
	cfg, err := s.config.Load()
	if err != nil {
		return nil, err
	}
	now = now.UTC()
	expiresAt, err := allowanceExpiry(now, cfg.Allowance.ExpiresInDays)
	if err != nil {
		return nil, err
	}
	row := &UserFreeAllowance{
		UserID:          userID,
		GrantedAt:       now,
		ExpiresAt:       expiresAt,
		DocumentUnlocks: cfg.Allowance.DocumentUnlocks,
		VideoUnlocks:    cfg.Allowance.VideoUnlocks,
		MockTestUnlocks: cfg.Allowance.MockTestUnlocks,
	}

	var out *UserFreeAllowance
	if err := s.repo.InUserTx(ctx, userID, func(tx *TxContext) error {
		created, err := tx.EnsureAllowanceRow(row)
		if err != nil {
			return err
		}
		if created {
			out = row
			return nil
		}
		// Someone already has one — a concurrent registration, or an earlier
		// call. Read theirs back rather than reporting ours, because theirs is
		// the one that is on disk and the one every later read will see.
		existing, err := tx.LockFreeAllowance(userID)
		if err != nil {
			return err
		}
		if existing == nil {
			// The conflict was with a row that is not visible. Under READ
			// COMMITTED an ON CONFLICT loser blocks until the winner commits, so
			// this should be unreachable; saying so is better than returning a
			// row that is not there.
			return fmt.Errorf("%w: the allowance for user %d conflicted but could not be read back", ErrImmutable, userID)
		}
		out = existing
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// ── RemainingAllowance ───────────────────────────────────────────────────────

// RemainingAllowance reports, per class, how many starter unlocks are unexpired
// and unused.
//
// "Used" is derived from resource_unlock, never stored: see the header of
// unlock_model.go. The whole thing is two indexed reads — the allowance row and
// one grouped count — and neither can drift from the rows that made it true.
//
// An expired allowance contributes zero to every class regardless of what is
// unused, which is the half-open window rule from allowanceExpired.
func (s *Service) RemainingAllowance(ctx context.Context, userID uint, now time.Time) (*AllowanceStatus, error) {
	if err := validateUser(userID); err != nil {
		return nil, err
	}
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	now = now.UTC()

	allowance, err := s.repo.FreeAllowance(ctx, userID)
	if err != nil {
		return nil, err
	}
	used, err := s.repo.CountAllowanceUnlocksByClass(ctx, userID)
	if err != nil {
		return nil, err
	}
	// The economy config is NOT consulted here, and that is the point worth
	// stating: the row holds the grant, not the current default, so lowering
	// allowance.document_unlocks from 3 to 1 does not retroactively take two
	// unlocks away from every existing student. Reading the config here would be
	// the first step of that bug.
	status := computeAllowanceStatus(allowance, used, now)
	return &status, nil
}

// ── HasAccess ────────────────────────────────────────────────────────────────

// HasAccess reports whether the user already holds an ACTIVE (un-revoked) unlock
// for the resource.
//
// This is the idempotency check every gate calls FIRST, before it resolves a
// price, spends a coin or burns an allowance. It is one indexed existence check
// and it cannot be stale in a way that matters, because the row it would be stale
// about is covered by resource_unlock_live_uniq: at most one LIVE row per
// (user, class, resource), so "no active row" means "never unlocked" or "revoked",
// and in both cases the answer is that there is no access right now.
//
// A revoked row does not occupy the uniqueness cap, so "no active row" after a
// revocation genuinely does invite a re-purchase — which is the intent, not a
// hole. The re-purchase costs the buyer again and leaves the revoked row in
// place as history.
func (s *Service) HasAccess(ctx context.Context, userID uint, resourceType string, resourceID uint64) (bool, error) {
	if err := validateResourceRef(userID, resourceType, resourceID); err != nil {
		return false, err
	}
	if err := s.requireRepo(); err != nil {
		return false, err
	}
	return s.repo.HasUnlock(ctx, userID, resourceType, resourceID)
}

// ── ConsumeAllowance ─────────────────────────────────────────────────────────

// ConsumeAllowance burns one starter unlock of a class and writes the
// resource_unlock row with source=ALLOWANCE and coins_paid=0.
//
// The concurrency mechanism is described in full at the top of this file. In one
// paragraph: the insert is made safe against a duplicate resource by
// resource_unlock_live_uniq via ON CONFLICT, and the read-then-insert against the
// allowance CEILING is made safe by serialisation — the ledger's per-user
// advisory lock from InUserTx plus the FOR UPDATE on user_free_allowance —
// because a configurable count has no constraint that could enforce it.
//
// The whole read and the insert are in one transaction, so a refusal writes
// nothing at all: a student who is refused leaves no resource_unlock row, no
// allowance row and no partial state. That is the property the concurrency test
// asserts alongside the winner count.
//
// The allowance row is created if it is missing rather than refused, which is
// what makes this method safe to call from a gate that has not yet been wired to
// the registration hook: a user with no allowance row gets the configured
// starter allowance, once, exactly as EnsureAllowance would have given them.
// Creating it inside the same transaction means the row that is counted against
// is the row this call made.
func (s *Service) ConsumeAllowance(ctx context.Context, userID uint, resourceType string, resourceID uint64, now time.Time) (*ResourceUnlock, error) {
	if err := validateResourceRef(userID, resourceType, resourceID); err != nil {
		return nil, err
	}
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if s.config == nil {
		return nil, ErrNoDatabase
	}
	cfg, err := s.config.Load()
	if err != nil {
		return nil, err
	}
	class, _ := allowanceClassFor(resourceType) // validated above
	now = now.UTC()

	var out *ResourceUnlock
	err = s.repo.InUserTx(ctx, userID, func(tx *TxContext) error {
		allowance, err := loadOrCreateAllowance(tx, userID, cfg, now)
		if err != nil {
			return err
		}
		used, err := tx.CountAllowanceUnlocks(userID, resourceType)
		if err != nil {
			return err
		}

		// Order of these two refusals is deliberate, and it follows 03-api-contract.md
		// §2.3: the allowance is checked before anything else, because it is the
		// cheapest gate and the one that is checked before the balance. A caller
		// that reaches here already holding the resource should have called
		// HasAccess; if it did not, it hears about the exhaustion, which is the
		// condition that would have blocked it anyway.
		if allowanceExpired(now, allowance.ExpiresAt) {
			// Both sentinels, deliberately. ErrAllowanceExpired is what the handler
			// maps to 423 ALLOWANCE_EXPIRED and ErrNoAllowanceRemaining is the
			// "cannot be covered" answer that 03-api-contract.md §2.3 groups it
			// with. A caller that wants to know "did the starter allowance run out"
			// and a caller that wants to know "which of the two" can both be right.
			return fmt.Errorf("%w: %w: the allowance for user %d expired at %s",
				ErrNoAllowanceRemaining, ErrAllowanceExpired, userID, allowance.ExpiresAt.UTC().Format(time.RFC3339))
		}
		granted := class.quota(cfg.Allowance)
		if remainingUnlocks(granted, used, false) <= 0 {
			return fmt.Errorf("%w: user %d has used all %d %s allowance unlocks",
				ErrNoAllowanceRemaining, userID, granted, resourceType)
		}

		unlock := &ResourceUnlock{
			UserID:       userID,
			ResourceType: resourceType,
			ResourceID:   resourceID,
			// No journal and no coins. That is not an omission — it is the whole
			// definition of an allowance unlock, and chk_resource_unlock_source_funding
			// refuses any other combination.
			JournalID:  nil,
			Source:     UnlockSourceAllowance,
			CoinsPaid:  0,
			UnlockedAt: now,
		}
		created, err := tx.InsertUnlock(unlock)
		if err != nil {
			return err
		}
		if !created {
			existing, err := tx.ReadUnlock(userID, resourceType, resourceID)
			if err != nil {
				return err
			}
			if existing == nil {
				return fmt.Errorf("%w: the unlock for user %d %s/%d conflicted but the row could not be read back",
					ErrImmutable, userID, resourceType, resourceID)
			}
			return fmt.Errorf("%w: user %d already holds %s/%d (unlocked at %s)",
				ErrAlreadyUnlocked, userID, resourceType, resourceID, existing.UnlockedAt.UTC().Format(time.RFC3339))
		}
		out = unlock
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// loadOrCreateAllowance returns the user's allowance row, creating it from config
// if it does not exist, and is the shared body of EnsureAllowance's conflict
// resolution and ConsumeAllowance's precondition.
//
// The row is taken FOR UPDATE, so the count that follows it is safe for a caller
// that is not already holding the per-user advisory lock. A create that loses the
// race re-reads rather than assuming it won — see LockFreeAllowance for why the
// constraint, not the row lock, is what separates two first-time creators.
func loadOrCreateAllowance(tx *TxContext, userID uint, cfg EconomyConfig, now time.Time) (*UserFreeAllowance, error) {
	existing, err := tx.LockFreeAllowance(userID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	expiresAt, err := allowanceExpiry(now, cfg.Allowance.ExpiresInDays)
	if err != nil {
		return nil, err
	}
	row := &UserFreeAllowance{
		UserID:          userID,
		GrantedAt:       now,
		ExpiresAt:       expiresAt,
		DocumentUnlocks: cfg.Allowance.DocumentUnlocks,
		VideoUnlocks:    cfg.Allowance.VideoUnlocks,
		MockTestUnlocks: cfg.Allowance.MockTestUnlocks,
	}
	created, err := tx.EnsureAllowanceRow(row)
	if err != nil {
		return nil, err
	}
	if created {
		return row, nil
	}
	loser, err := tx.LockFreeAllowance(userID)
	if err != nil {
		return nil, err
	}
	if loser == nil {
		return nil, fmt.Errorf("%w: the allowance for user %d conflicted but could not be read back", ErrImmutable, userID)
	}
	return loser, nil
}

// ── RecordCoinUnlock ─────────────────────────────────────────────────────────

// RecordCoinUnlock writes the entitlement half of a purchase the ledger has
// already settled.
//
// journalID and coinsPaid come from the CALLER because the ledger already knows
// them: this method is not a gate and takes no money, it records that a spend
// journal exists and what it moved. It is the entitlement half of the two
// transactions the eventual gate will run — Spend, commit, then RecordCoinUnlock
// — and the seam between them is documented here rather than left to be
// rediscovered:
//
//   - A spend that commits and an unlock that does not leave a student who paid
//     and has no access. That is a support ticket, and the recovery is a refund
//     plus a manual insert, which is why the journal_id is stored: the two can
//     be reconciled from the rows.
//   - The gate therefore calls HasAccess after a failure and retries the record
//     rather than re-spending. Re-spending is the expensive mistake, and the
//     UNIQUE on the unlock is what stops a retry from making it invisible.
//
// coinsPaid is a snapshot taken here and never recomputed, so a later price
// change does not retroactively re-price the purchase
// (02-architecture.md §5, 03-api-contract.md §3.1).
//
// Idempotency is the UNIQUE constraint, and the shape of it is worth stating
// because a mobile retry is the normal case and not the exception:
//
//   - same journal, same resource -> the EXISTING row is returned. Nothing
//     moves, and nothing is charged. This is the retry.
//   - a DIFFERENT journal for the same resource -> ErrAlreadyUnlocked, because
//     somebody has already paid for this and recording a second unlock would
//     hide a double charge behind a successful-looking response. The caller
//     should surface the existing unlock, not pay again.
//
// One consequence of the UNIQUE is worth stating precisely, because the scope
// changed and the old wording is now wrong in the dangerous direction. The
// uniqueness is over LIVE rows only (resource_unlock_live_uniq, partial on
// revoked_at IS NULL). A revoked row does NOT occupy the cap, so a student whose
// access was revoked can simply buy it again: this method will create a second
// row, charge them a second time, and leave the revoked row untouched as the
// record of the first decision.
//
// That is deliberate, and the fraud argument is unchanged — see unlock_model.go.
// One live unlock per resource is still one live unlock per resource. What it
// means in practice is that a revocation is recoverable at the buyer's expense,
// which is the right outcome for a revocation made in error, and the wrong one
// for a revocation meant to be permanent. Deciding which is which is not this
// method's job and is not yet a job anywhere: a gate that wants a permanent
// revocation must not simply call Revoke and let a re-buy through, it has to
// block the purchase. This slice does not have that gate, and this note is here
// so the next one does not inherit the assumption that Revoke alone is enough.
func (s *Service) RecordCoinUnlock(ctx context.Context, userID uint, resourceType string, resourceID uint64, journalID string, coinsPaid int64, now time.Time) (*ResourceUnlock, error) {
	// Every argument is checked before the repository is consulted, so a
	// malformed call is a typed argument error rather than a wiring error that
	// happens to be reported first. Validation that runs after requireRepo would
	// make "you passed a bad journal id" indistinguishable from "this Service was
	// never wired to a database", and only one of those is the caller's bug.
	if err := validateResourceRef(userID, resourceType, resourceID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(journalID) == "" {
		return nil, fmt.Errorf("%w: a coin unlock must name the journal that settled it", ErrInvalidArgument)
	}
	// Parsed here, not left to the foreign key: the FK would catch it, but three
	// statements later and as an opaque constraint violation, and a typed "that is
	// not a journal id" names the field the caller got wrong.
	if _, err := uuid.Parse(journalID); err != nil {
		return nil, fmt.Errorf("%w: %q is not a journal id", ErrInvalidArgument, journalID)
	}
	if coinsPaid < 0 {
		return nil, fmt.Errorf("%w: coins_paid is %d and a snapshot cannot be negative", ErrInvalidArgument, coinsPaid)
	}
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	now = now.UTC()
	journal := journalID

	var out *ResourceUnlock
	err := s.repo.InUserTx(ctx, userID, func(tx *TxContext) error {
		unlock := &ResourceUnlock{
			UserID:       userID,
			ResourceType: resourceType,
			ResourceID:   resourceID,
			JournalID:    &journal,
			Source:       UnlockSourceCoins,
			CoinsPaid:    coinsPaid,
			UnlockedAt:   now,
		}
		created, err := tx.InsertUnlock(unlock)
		if err != nil {
			return err
		}
		if created {
			out = unlock
			return nil
		}
		existing, err := tx.ReadUnlock(userID, resourceType, resourceID)
		if err != nil {
			return err
		}
		if existing == nil {
			return fmt.Errorf("%w: the unlock for user %d %s/%d conflicted but the row could not be read back",
				ErrImmutable, userID, resourceType, resourceID)
		}
		if existing.JournalID == nil || *existing.JournalID != journal {
			// A different payment already owns this resource. The existing row is
			// returned in the error so a caller can show the student what they
			// already have rather than charging again for it.
			return fmt.Errorf("%w: user %d already paid for %s/%d under a different journal (%v, %d coins)",
				ErrAlreadyUnlocked, userID, resourceType, resourceID, existing.JournalID, existing.CoinsPaid)
		}
		// The retry. The original row is the answer and nothing moves.
		out = existing
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ── Revoke ───────────────────────────────────────────────────────────────────

// Revoke marks an unlock revoked with a reason. It never deletes.
//
// Two refusals, and the second is the interesting one:
//
//   - ErrNotFound: the user never held this resource.
//   - ErrUnlockRevoked: they held it and it is already revoked.
//
// The second is not idempotent-success on purpose. A revocation is a reversal,
// and the first one is the record: it is what an auditor reads to find out when
// access was taken away and why. Overwriting it with a later, friendlier reason
// would destroy the evidence, so the update is a compare-and-set on
// `revoked_at IS NULL` and a second caller changes nothing.
//
// The reason is mandatory. A revoked_at with no reason is a support ticket that
// cannot be answered, and chk_resource_unlock_revocation refuses the pair being
// half-present at the database as well.
//
// Who may call this is not decided here. It is an admin/support operation and the
// gate slice is where the authorisation lives; what this method guarantees is
// that it is recorded rather than erased.
//
// A re-grant after a revocation is now possible — the uniqueness that used to
// forbid it is partial on revoked_at IS NULL, so a revoked row no longer holds
// the slot and a fresh purchase makes a second row rather than being refused.
// See RecordCoinUnlock for why that is the right default and where the open
// question sits.
func (s *Service) Revoke(ctx context.Context, userID uint, resourceType string, resourceID uint64, reason string, now time.Time) error {
	// Validated before the repository, for the same reason as RecordCoinUnlock:
	// a missing reason is the caller's bug and must not be reported as a wiring
	// error.
	if err := validateResourceRef(userID, resourceType, resourceID); err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("%w: a revocation needs a recorded reason", ErrInvalidArgument)
	}
	if err := s.requireRepo(); err != nil {
		return err
	}
	now = now.UTC()

	return s.repo.InUserTx(ctx, userID, func(tx *TxContext) error {
		revoked, err := tx.RevokeUnlock(userID, resourceType, resourceID, reason, now)
		if err != nil {
			return err
		}
		if revoked {
			return nil
		}
		// Nothing was updated. Either there was never an unlock, or there was one
		// and it is already revoked; those are different answers and a caller
		// that cannot tell them apart will retry forever.
		//
		// ReadLatestRevokedUnlock rather than ReadUnlock, and the choice is the
		// point: the row we need here is the REVOKED one, which is exactly the row
		// ReadUnlock now refuses to return. A live-row read would come back nil
		// on this path — the compare-and-set above guarantees there is no live
		// row when it reports no update — and a nil is reported to the caller as
		// ErrNotFound. That would turn "you already revoked this, here is when
		// and why" into "you never had this", which is a false answer about a
		// student's own history and is precisely the confusion this branch
		// exists to prevent.
		existing, err := tx.ReadLatestRevokedUnlock(userID, resourceType, resourceID)
		if err != nil {
			return err
		}
		if existing == nil {
			return fmt.Errorf("%w: user %d holds no unlock for %s/%d", ErrNotFound, userID, resourceType, resourceID)
		}
		return fmt.Errorf("%w: user %d's %s/%d unlock was already revoked at %s (%s)",
			ErrUnlockRevoked, userID, resourceType, resourceID,
			existing.RevokedAt.UTC().Format(time.RFC3339), derefString(existing.RevokeReason))
	})
}

// derefString renders a nullable column for an error message. A message that
// says "already revoked" without saying when and why is a message that produces
// a second support ticket.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
