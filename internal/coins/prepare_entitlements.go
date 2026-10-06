// internal/coins/prepare_entitlements.go
//
// The one statement that has to happen BEFORE GORM sees the entitlement tables,
// and the boot failure it exists to prevent.
//
// ── the failure ──────────────────────────────────────────────────────────────
//
// UNIQUE (user_free_allowance.user_id) was hand-created for a long time — twice,
// under two bespoke names (`user_free_allowance_user_uniq`, a unique INDEX, and
// `uq_user_free_allowance_user`, a unique CONSTRAINT) — because the model carried
// a plain `index` tag and AutoMigrate would not make it unique on its own.
//
// AutoMigrate does not tolerate an index it did not create. On the boot AFTER the
// ones that added those objects, it enumerated the live indexes, matched the extra
// unique object to the field by COLUMNS rather than by name, decided the name was
// not the one its tag asked for, and issued a drop by ITS OWN canonical name —
// `ALTER TABLE "user_free_allowance" DROP CONSTRAINT "uni_user_free_allowance_user_id"`
// — a constraint no one had ever created. Postgres answered 42704, and because
// AutoMigrate's error is a logger.Fatal in cmd/server/main.go, the server died on
// every boot from then on: a database that had started perfectly once could not
// start again.
//
// The first boot did not fail, which is what made this so confusing: the damage was
// done by the very migration that appeared to succeed.
//
// ── the fix, and why it is here rather than in a migration file ──────────────
//
// `UserFreeAllowance.UserID` now carries `uniqueIndex`, so AutoMigrate owns the
// constraint and reconciles nothing. That alone does not rescue a database that
// ALREADY carries the legacy objects: their presence is what triggers the drop, so
// they have to go before AutoMigrate runs — and every migration file in this repo
// executes after it. Hence one function, called explicitly, before the
// AutoMigrate block.
//
// It is deliberately narrow. It drops exactly the two names this package invented,
// and it does nothing at all when the table is absent, when the dialect is not
// Postgres, or when the canonical index already exists. The uniqueness it leaves
// behind is never absent for even a moment: the canonical index is created before
// the legacy objects are dropped, so there is no window in which two rows for one
// user could be inserted.
package coins

import (
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

// legacyAllowanceUniqueObjects are the two bespoke names this package used to
// create for UNIQUE (user_free_allowance.user_id). Both are dropped; neither is
// ever created again.
var legacyAllowanceUniqueObjects = []string{
	"user_free_allowance_user_uniq", // the DO block in ensure_indexes.go
	"uq_user_free_allowance_user",   // the addConstraint in unlock_indexes.go
}

// canonicalAllowanceUniqueIndex is GORM's own name for a single-column unique
// index, and the one AutoMigrate's `uniqueIndex` tag produces and expects. See
// UserFreeAllowance's comment for why the name is itself load-bearing.
const canonicalAllowanceUniqueIndex = "uni_user_free_allowance_user_id"

// PrepareEntitlementSchema retires the hand-created UNIQUE (user_id) objects so
// AutoMigrate can own the constraint.
//
// MUST run before db.AutoMigrate on any deployment that has ever booted the
// hand-created versions — which is every deployment that ran this package before
// the model's `uniqueIndex` tag, and any database where the previous boot created
// them. A no-op on a database that predates the entitlement tables entirely.
//
// Errors are returned rather than swallowed: this runs before the migrator, so
// failing loudly here is what lets an operator see a schema that needs a manual
// drop, instead of the server dying later with a name they have never heard of.
func PrepareEntitlementSchema(db *gorm.DB) error {
	if db == nil || db.Dialector == nil {
		return nil
	}
	if db.Dialector.Name() != "postgres" {
		return nil
	}

	var tableExists bool
	if err := db.Raw(`SELECT to_regclass('user_free_allowance') IS NOT NULL`).Scan(&tableExists).Error; err != nil {
		return err
	}
	if !tableExists {
		return nil
	}

	// Uniqueness FIRST, so the window in which the table has neither the legacy
	// object nor the canonical one never opens.
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS ` + canonicalAllowanceUniqueIndex +
		` ON user_free_allowance (user_id)`).Error; err != nil {
		return err
	}

	// A constraint cannot be dropped with DROP INDEX ("constraint ... requires it"),
	// and an index created as a constraint cannot be dropped with the constraint
	// name. Both statements run, both are IF EXISTS, and whichever spelling does
	// not apply is the one that no-ops.
	dropped := make([]string, 0, len(legacyAllowanceUniqueObjects))
	for _, name := range legacyAllowanceUniqueObjects {
		var exists bool
		if err := db.Raw(`SELECT to_regclass(?) IS NOT NULL`, name).Scan(&exists).Error; err != nil {
			return err
		}
		if !exists {
			continue
		}
		if err := db.Exec(`ALTER TABLE user_free_allowance DROP CONSTRAINT IF EXISTS ` + name).Error; err != nil {
			return err
		}
		if err := db.Exec(`DROP INDEX IF EXISTS ` + name).Error; err != nil {
			return err
		}
		dropped = append(dropped, name)
	}
	// Only on a run that actually retired something: a log line claiming a drop on
	// every boot of a healthy database trains an operator to ignore the one boot
	// where it matters.
	if len(dropped) > 0 {
		logger.Info("Retired legacy starter-allowance unique objects", "dropped", dropped)
	}
	return nil
}
