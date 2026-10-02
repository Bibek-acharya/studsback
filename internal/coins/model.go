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

// VersionStore appends AND reads config audit rows. It is an interface so the
// service can be exercised without a database.
//
// The read half exists because 04 §6 asks for a config version history and a table
// nobody can query is a write-only log. It was added with the read side of Phase 4;
// every row the append side has written since Phase 2 becomes readable at that point,
// so no backfill is involved.
type VersionStore interface {
	AppendConfigVersion(version *ConfigVersion) error
	// RecentConfigVersions returns at most limit rows, NEWEST FIRST.
	//
	// Newest first because the question an admin opens the page for is "what did I
	// just change" — reading oldest-first makes them scroll to the bottom to answer
	// it. limit is applied in the QUERY, not by slicing afterwards, so a large table
	// is never fully read to return a page of it.
	RecentConfigVersions(limit int) ([]ConfigVersion, error)
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

// RecentConfigVersions returns the newest rows first.
//
// ORDER BY id DESC rather than created_at DESC, and that is deliberate: created_at is
// a timestamp two rows can share to the microsecond on a fast machine, and an audit
// trail whose order is ambiguous is not an audit trail. id is the insertion sequence,
// so it is total and monotonic.
func (s *gormVersionStore) RecentConfigVersions(limit int) ([]ConfigVersion, error) {
	if s == nil || s.db == nil {
		return nil, ErrNoDatabase
	}
	if limit <= 0 {
		limit = ConfigVersionHistoryDefault
	}
	var rows []ConfigVersion
	if err := s.db.Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
