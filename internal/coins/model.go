package coins

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ConfigVersion is the audit row appended by every successful economy write:
// who changed it, when, the full previous JSON and the full new JSON.
//
// Storing both full objects rather than a diff is deliberate. A pricing
// question is always "what was the price at 14:02, and who set it", and a diff
// has to be replayed against a chain of other diffs to answer that. The rows
// are small and the write rate is a handful a year, so reconstructibility wins
// over space.
//
// The table is created by AutoMigrate (cmd/server/main.go) and mirrored by
// migrations.CreateCoinEconomyConfigVersion for the SQL-migration path. Its
// only index is a plain btree on created_at, which AutoMigrate can create —
// there is no partial or expression index here, so no EnsurePostgresIndexes
// helper is needed (02-architecture.md §9).
type ConfigVersion struct {
	ID uint `gorm:"primarykey" json:"id"`
	// CreatedAt is indexed so the audit trail is ordered without a sort over
	// the whole table.
	CreatedAt       time.Time `json:"created_at"`
	PreviousJSON    string    `gorm:"column:previous_json;type:text;default:''" json:"previous_json"`
	NewJSON         string    `gorm:"column:new_json;type:text;default:''" json:"new_json"`
	ChangedByUserID uint      `gorm:"column:changed_by_user_id;not null;default:0" json:"changed_by_user_id"`
	// ChangedBy is the human-readable actor, formatted like the ledger's
	// created_by convention ("admin:<id>") so the two audit trails read the
	// same way. The id is kept alongside it for joins.
	ChangedBy string `gorm:"column:changed_by;type:varchar(64);default:''" json:"changed_by"`
}

// TableName pins the singular name from 03-api-contract.md §3.1
// ("a coin_economy_config_version row") rather than GORM's plural default, so
// the Go model, AutoMigrate and the SQL migration all agree.
func (ConfigVersion) TableName() string { return "coin_economy_config_version" }

// actorLabel renders the actor for ChangedBy.
func actorLabel(userID uint) string { return fmt.Sprintf("admin:%d", userID) }

// VersionStore appends config audit rows. It is an interface so the service
// can be exercised without a database.
type VersionStore interface {
	AppendConfigVersion(version *ConfigVersion) error
}

type gormVersionStore struct {
	db *gorm.DB
}

func NewVersionStore(db *gorm.DB) VersionStore {
	return &gormVersionStore{db: db}
}

func (s *gormVersionStore) AppendConfigVersion(version *ConfigVersion) error {
	return s.db.Create(version).Error
}
