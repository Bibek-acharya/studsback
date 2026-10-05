// internal/coins/ensure_indexes.go
package coins

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// EnsurePostgresIndexes creates everything GORM AutoMigrate cannot: partial
// indexes, CHECK constraints, the append-only triggers, and the chart-of-accounts
// seed. It is idempotent, a no-op on SQLite, and a no-op when the tables do not
// exist yet.
//
// This function is not optional. AutoMigrate runs ~110 models at boot and cannot
// create partial or expression indexes, so a fresh `go run ./cmd/server` that
// skips this call produces a server with the tables present but the ledger's
// correctness constraints missing. The repo has shipped that exact bug once:
// internal/notification/ensure_indexes.go documents a fresh-boot server where
// every preferences PUT failed with 42P10 ON CONFLICT without matching
// constraint.
//
// Plain non-unique indexes are declared as `index` tags on the models and are
// created by AutoMigrate. Everything below is deliberately NOT on the models, so
// there is one auditable place to look for a ledger constraint.
func EnsurePostgresIndexes(db *gorm.DB) error {
	if db == nil || db.Dialector == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(db.Dialector.Name()), "sqlite") {
		return nil
	}
	var exists bool
	if err := db.Raw(`SELECT to_regclass('coin_account') IS NOT NULL`).Scan(&exists).Error; err != nil {
		return err
	}
	if !exists {
		return nil
	}

	stmts := []string{
		// ── partial unique indexes ───────────────────────────────────────
		// Two partial indexes, not one UNIQUE over (owner_user_id, kind,
		// bucket): a single unique constraint collides on
		// (NULL, 'SYSTEM', NULL) for every system account. Hit and fixed
		// during schema validation.
		`CREATE UNIQUE INDEX IF NOT EXISTS coin_account_user_bucket_uniq
		   ON coin_account (owner_user_id, bucket) WHERE kind = 'USER'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS coin_account_system_uniq
		   ON coin_account (system_type) WHERE kind = 'SYSTEM'`,

		// Idempotency is enforced by this index alone. A SELECT-then-INSERT
		// check in Go races under concurrency; the constraint cannot.
		`CREATE UNIQUE INDEX IF NOT EXISTS coin_journal_idem_uniq
		   ON coin_journal (scope, idempotency_key)`,

		// The starter allowance's idempotency constraint, and it was MISSING.
		//
		// `UserFreeAllowance.UserID` carries `gorm:"not null;index"` — a plain,
		// NON-unique index. Three separate places in the codebase assert that
		// UNIQUE (user_id) exists and is load-bearing: unlock.go's EnsureAllowance
		// ("Idempotency is UNIQUE (user_id) … resolved with ON CONFLICT DO NOTHING"),
		// unlock_repository.go's EnsureAllowanceRow ("The UNIQUE (user_id) constraint
		// is the idempotency mechanism"), and unlock_model.go's own comment.
		//
		// So `INSERT … ON CONFLICT (user_id) DO NOTHING` failed with
		// SQLSTATE 42P10 — "there is no unique or exclusion constraint matching the
		// ON CONFLICT specification" — on every call, for every student, on any
		// database built by AutoMigrate. Which is every database, because nothing
		// else created it: no migration, no other index statement.
		//
		// The effect was that no student could ever be granted the starter allowance,
		// so `allowance.expiring` / `allowance.expired` had nothing to report and the
		// catalogue's "included with your account" had no allowance behind it.
		//
		// Found while wiring the catalogue access block, by a test that grants an
		// allowance and reads it back.
		//
		// Wrapped in a DO block rather than `CREATE UNIQUE INDEX IF NOT EXISTS`,
		// because that form still FAILS when the table does not exist, and several
		// integration harnesses in this package migrate only LedgerModels. The guard is
		// on the table as well as the index name.
		//
		// In production the table always exists — cmd/server/main.go lists
		// &coins.UserFreeAllowance{} in the AutoMigrate set — so the guard never skips
		// anything that matters; it stops an unrelated test harness from failing on a
		// relation it never created.
		`DO $$
		BEGIN
		    IF to_regclass('user_free_allowance') IS NOT NULL
		       AND NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'user_free_allowance_user_uniq')
		    THEN
		        CREATE UNIQUE INDEX user_free_allowance_user_uniq ON user_free_allowance (user_id);
		    END IF;
		END $$`,

		// ── partial indexes ──────────────────────────────────────────────
		`CREATE INDEX IF NOT EXISTS coin_journal_ref_idx
		   ON coin_journal (ref_type, ref_id) WHERE ref_type IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS coin_posting_lot_idx
		   ON coin_posting (lot_id) WHERE lot_id IS NOT NULL`,
		// FEFO support. Stays small as lots close, and NULLS LAST is what
		// makes never-expiring coins sort after everything that does expire.
		`CREATE INDEX IF NOT EXISTS coin_lot_open_idx
		   ON coin_lot (account_id, expires_at NULLS LAST, id) WHERE consumed < granted`,

		// ── CHECK constraints ────────────────────────────────────────────
		// Only USER accounts may not go negative. SYSTEM accounts are the
		// "outside the world" side: earned_faucet is legitimately negative
		// after an issue. Applying this to system accounts rejects every
		// grant. Hit and fixed during schema validation.
		addCheck("chk_coin_no_overdraft", `coin_account_balance`, `
			NOT liability OR posted_balance - reserved >= 0`),
		addCheck("chk_coin_account_shape", `coin_account`, `
			(kind = 'USER'   AND owner_user_id IS NOT NULL AND system_type IS NULL AND bucket IS NOT NULL)
		 OR (kind = 'SYSTEM' AND owner_user_id IS NULL     AND system_type IS NOT NULL AND bucket IS NULL)`),
		addCheck("chk_coin_posting_nonzero", `coin_posting`, `amount <> 0`),
		addCheck("chk_coin_lot_not_overconsumed", `coin_lot`,
			`consumed >= 0 AND consumed <= granted AND granted > 0`),
		addCheck("chk_coin_journal_ref", `coin_journal`,
			`ref_type IS NULL OR ref_id IS NOT NULL`),
		addCheck("chk_coin_account_reversal", `coin_journal`,
			`(entry_type = 'REVERSAL') = (reversal_of IS NOT NULL)`),

		// Bucket values are constrained rather than a Postgres ENUM so the
		// models stay plain strings. PURCHASED is listed but unreachable
		// under D1; widening this list is the whole cost of revisiting that
		// decision.
		addCheck("chk_coin_bucket", `coin_account`, `bucket IN ('FREE', 'EARNED', 'PURCHASED')`),
		addCheck("chk_coin_lot_bucket", `coin_lot`, `bucket IN ('FREE', 'EARNED', 'PURCHASED')`),
		addCheck("chk_coin_kind", `coin_account`, `kind IN ('USER', 'SYSTEM')`),
		addCheck("chk_coin_state", `coin_journal`, `state IN ('PENDING', 'POSTED', 'REVERSED')`),
		addCheck("chk_coin_entry_type", `coin_journal`,
			`entry_type IN ('GRANT', 'SPEND', 'EXPIRE', 'REVERSAL', 'ADJUST')`),

		// ── UNIQUE constraints declared by the spec but previously never
		//    created ──────────────────────────────────────────────────────
		// 02-architecture.md section 2 declares both of these. They were not
		// in the models, and AutoMigrate cannot create them without a
		// uniqueIndex tag, so a live schema had NEITHER. The ledger carried on
		// with only the row lock preventing a duplicate account_seq, which
		// makes correctness depend on Go rather than on the database. The
		// ledger-core build found this by inspecting pg_constraint rather than
		// trusting the spec, which is the right way round.
		//
		// (account_id, account_seq) is the counter-uniqueness guarantee. The
		// per-user advisory lock does not protect this: the system accounts are
		// global, so two different users holding two different user locks both
		// write earned_faucet. Without this constraint two concurrent journals
		// can hand the same account the same sequence number.
		addConstraint("uq_coin_posting_account_seq", `coin_posting`,
			`UNIQUE (account_id, account_seq)`),
		addConstraint("uq_coin_posting_journal_seq", `coin_posting`,
			`UNIQUE (journal_id, seq)`),

		// ── foreign keys ──────────────────────────────────────────────────
		// ON DELETE RESTRICT is the default but is stated explicitly. The
		// append-only trigger already blocks deleting a journal or posting, so
		// this is the second of two independent guards rather than the only one.
		addConstraint("fk_coin_posting_journal", `coin_posting`,
			`FOREIGN KEY (journal_id) REFERENCES coin_journal(id) ON DELETE RESTRICT`),
		addConstraint("fk_coin_posting_account", `coin_posting`,
			`FOREIGN KEY (account_id) REFERENCES coin_account(id) ON DELETE RESTRICT`),
		addConstraint("fk_coin_lot_journal", `coin_lot`,
			`FOREIGN KEY (journal_id) REFERENCES coin_journal(id) ON DELETE RESTRICT`),
		addConstraint("fk_coin_lot_account", `coin_lot`,
			`FOREIGN KEY (account_id) REFERENCES coin_account(id) ON DELETE RESTRICT`),

		// ── immutability ─────────────────────────────────────────────────
		// A mutable posting row destroys the only audit trail that makes a
		// balance explainable. Triggers alone are bypassable by a superuser
		// or by session_replication_role, so the grant below is the real
		// control and the trigger is the ergonomic one.
		`CREATE OR REPLACE FUNCTION coin_deny_mutation() RETURNS trigger
		 LANGUAGE plpgsql AS $$
		 BEGIN
		   RAISE EXCEPTION 'coin_posting is append-only (attempted %)', TG_OP
		     USING ERRCODE = '25006';
		 END; $$`,
		`DROP TRIGGER IF EXISTS coin_posting_append_only ON coin_posting`,
		`CREATE TRIGGER coin_posting_append_only
		   BEFORE UPDATE OR DELETE ON coin_posting
		   FOR EACH ROW EXECUTE FUNCTION coin_deny_mutation()`,

		// Restricted immutability, not absolute: PENDING may become POSTED
		// or REVERSED exactly once, POSTED is terminal, and the provenance
		// columns are frozen outright.
		`CREATE OR REPLACE FUNCTION coin_journal_guard() RETURNS trigger
		 LANGUAGE plpgsql AS $$
		 BEGIN
		   IF TG_OP = 'DELETE' THEN
		     RAISE EXCEPTION 'coin_journal is append-only' USING ERRCODE = '25006';
		   END IF;
		   IF (OLD.id, OLD.entry_type, OLD.scope, OLD.idempotency_key, OLD.request_fingerprint,
		       OLD.reason_code, OLD.ref_type, OLD.ref_id, OLD.created_at, OLD.created_by,
		       OLD.reversal_of)
		      IS DISTINCT FROM
		      (NEW.id, NEW.entry_type, NEW.scope, NEW.idempotency_key, NEW.request_fingerprint,
		       NEW.reason_code, NEW.ref_type, NEW.ref_id, NEW.created_at, NEW.created_by,
		       NEW.reversal_of) THEN
		     RAISE EXCEPTION 'coin_journal: provenance columns are frozen' USING ERRCODE = '25006';
		   END IF;
		   IF OLD.state = 'POSTED' AND NEW.state <> 'POSTED' THEN
		     RAISE EXCEPTION 'coin_journal: POSTED is terminal' USING ERRCODE = '25006';
		   END IF;
		   RETURN NEW;
		 END; $$`,
		`DROP TRIGGER IF EXISTS coin_journal_immutable ON coin_journal`,
		`CREATE TRIGGER coin_journal_immutable
		   BEFORE UPDATE OR DELETE ON coin_journal
		   FOR EACH ROW EXECUTE FUNCTION coin_journal_guard()`,
	}

	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	return seedChartOfAccounts(db)
}

// addConstraint runs ALTER TABLE ... ADD CONSTRAINT only if that constraint is
// not already on that table, using the same idempotent DO $$ shape the
// notification module uses, because Postgres has no CREATE CONSTRAINT IF NOT
// EXISTS. conname and conrelid are both qualified so a same-named constraint on
// another table cannot be mistaken for ours.
//
// Built with Sprintf over one template rather than by concatenating backtick
// raw strings. Interpolating into a raw string next to SQL containing quotes
// and `::regclass` is how you end up with an unterminated literal.
//
// clause is the part after ADD CONSTRAINT <name>, e.g.
// "UNIQUE (account_id, account_seq)" or
// "FOREIGN KEY (journal_id) REFERENCES coin_journal(id) ON DELETE RESTRICT".
func addConstraint(name, table, clause string) string {
	return fmt.Sprintf(`DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = '%s'
          AND conrelid = '%s'::regclass
    ) THEN
        ALTER TABLE %s ADD CONSTRAINT %s %s;
    END IF;
END $$`, name, table, table, name, clause)
}

// addCheck is addConstraint specialised to a CHECK expression.
func addCheck(name, table, expression string) string {
	return addConstraint(name, table, "CHECK ("+expression+")")
}

// seedChartOfAccounts creates the three SYSTEM accounts the ledger posts against.
//
// The liability flag is false for all of them, which is what allows
// earned_faucet to go negative on issue. It is set here rather than relying on
// the column default (true) because GORM omits zero-valued fields on Create and
// the database default would otherwise win. This is the same trap documented at
// internal/studyresources/repository.go:121 and internal/mocktests/repository.go:136.
func seedChartOfAccounts(db *gorm.DB) error {
	now := time.Now().UTC()
	for _, sysType := range SystemAccounts {
		var existing int64
		if err := db.Raw(
			`SELECT count(*) FROM coin_account WHERE kind = 'SYSTEM' AND system_type = ?`, sysType,
		).Scan(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			continue
		}
		st := sysType
		if err := db.Create(&CoinAccount{
			Kind:       AccountSystem,
			SystemType: &st,
			CreatedAt:  now,
		}).Error; err != nil {
			return err
		}
		// The balance row must exist with liability = false, and it cannot be
		// set on Create: GORM omits zero-valued fields, so a struct literal
		// carrying `Liability: false` writes nothing and the column default
		// (true) wins. That is not cosmetic. With liability true, the
		// no-overdraft CHECK applies to a system account, and since
		// earned_faucet is legitimately negative after an issue, EVERY GRANT
		// IS REJECTED. The integration test caught exactly this.
		//
		// Same trap and same remedy as internal/studyresources/repository.go:121,
		// which does Create-then-UpdateColumn for the is_published case.
		var accountID uint
		if err := db.Raw(
			`SELECT id FROM coin_account WHERE kind = 'SYSTEM' AND system_type = ?`, sysType,
		).Scan(&accountID).Error; err != nil {
			return err
		}
		if err := db.Create(&CoinAccountBalance{AccountID: accountID}).Error; err != nil {
			return err
		}
		if err := db.Model(&CoinAccountBalance{}).
			Where("account_id = ?", accountID).
			UpdateColumn("liability", false).Error; err != nil {
			return err
		}
	}
	return nil
}
