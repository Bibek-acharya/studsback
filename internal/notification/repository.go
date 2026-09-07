// internal/notification/repository.go
package notification

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) InsertNotifications(tx *gorm.DB, rows []AccountNotification) error {
	if tx == nil {
		tx = r.db
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Clauses(clause.Returning{}).Create(&rows).Error
}

func (r *Repository) FindByOccurrenceKey(tx *gorm.DB, accountType string, accountID uint, key string) (*AccountNotification, error) {
	if tx == nil {
		tx = r.db
	}
	var row AccountNotification
	err := tx.Where("account_type = ? AND account_id = ? AND occurrence_key = ?", accountType, accountID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}

func (r *Repository) ListInbox(accountType string, accountID uint, page, limit int, category string, unreadOnly, archived bool) ([]AccountNotification, int64, int, error) {
	limit = clampInt(limit, 1, 50)
	if page < 1 {
		page = 1
	}
	archFilter := "archived_at IS NULL"
	if archived {
		archFilter = "archived_at IS NOT NULL"
	}
	q := r.db.Model(&AccountNotification{}).
		Where("account_type = ? AND account_id = ?", accountType, accountID).
		Where(archFilter)
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if unreadOnly {
		q = q.Where("read_at IS NULL")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, 0, err
	}
	var unread int64
	if err := r.db.Model(&AccountNotification{}).
		Where("account_type = ? AND account_id = ? AND read_at IS NULL AND archived_at IS NULL", accountType, accountID).
		Count(&unread).Error; err != nil {
		return nil, 0, 0, err
	}
	order := "read_at NULLS FIRST, created_at DESC"
	if archived {
		order = "created_at DESC"
	}
	var rows []AccountNotification
	err := q.Order(order).Offset((page - 1) * limit).Limit(limit).Find(&rows).Error
	return rows, total, int(unread), err
}

func (r *Repository) UnreadCount(accountType string, accountID uint) (int64, error) {
	var n int64
	err := r.db.Model(&AccountNotification{}).
		Where("account_type = ? AND account_id = ? AND read_at IS NULL AND archived_at IS NULL", accountType, accountID).
		Count(&n).Error
	return n, err
}

func (r *Repository) MarkRead(accountType string, accountID, id uint) (int64, error) {
	res := r.db.Model(&AccountNotification{}).
		Where("account_type = ? AND account_id = ? AND id = ? AND read_at IS NULL", accountType, accountID, id).
		Update("read_at", time.Now())
	return res.RowsAffected, res.Error
}

func (r *Repository) MarkAllRead(accountType string, accountID uint) (int64, error) {
	res := r.db.Model(&AccountNotification{}).
		Where("account_type = ? AND account_id = ? AND read_at IS NULL AND archived_at IS NULL", accountType, accountID).
		Update("read_at", time.Now())
	return res.RowsAffected, res.Error
}

func (r *Repository) SetArchived(accountType string, accountID, id uint, archived bool) (int64, error) {
	var t *time.Time
	if archived {
		now := time.Now()
		t = &now
	}
	res := r.db.Model(&AccountNotification{}).
		Where("account_type = ? AND account_id = ? AND id = ?", accountType, accountID, id).
		Update("archived_at", t)
	return res.RowsAffected, res.Error
}

func (r *Repository) SoftDelete(accountType string, accountID, id uint) (int64, error) {
	res := r.db.Where("account_type = ? AND account_id = ?", accountType, accountID).Delete(&AccountNotification{}, id)
	return res.RowsAffected, res.Error
}

// AcquireLease implements doc 04 §4d: one atomic statement, three outcomes.
//   - inserted=true            → new lease; caller inserts the inbox row, then
//     PointLeaseAtNotification back-fills the lease.
//   - inserted=false, err=nil, leaseID>0 → expired lease RENEWED (row returned);
//     caller may refresh the referenced notification or insert fresh.
//   - inserted=false, err=nil, leaseID=0  → zero rows: active lease; SUPPRESS.
func (r *Repository) AcquireLease(tx *gorm.DB, accountType string, accountID uint, dedupeKey string, win time.Duration) (uint, *uint, bool, error) {
	if tx == nil {
		tx = r.db
	}
	var id uint
	var nid *uint
	var inserted bool
	err := tx.Raw(`
INSERT INTO notification_dedupe_leases (account_type, account_id, dedupe_key, notification_id, expires_at)
VALUES (?, ?, ?, NULL, now() + make_interval(secs => ?))
ON CONFLICT (account_type, account_id, dedupe_key) DO UPDATE
  SET expires_at = now() + make_interval(secs => ?),
      superseded_count = notification_dedupe_leases.superseded_count + 1
WHERE notification_dedupe_leases.expires_at < now()
RETURNING id, notification_id, (xmax = 0) AS inserted`,
		accountType, accountID, dedupeKey, win.Seconds(), win.Seconds(),
	).Row().Scan(&id, &nid, &inserted)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) { // zero rows = active lease = suppress
			return 0, nil, false, nil
		}
		return 0, nil, false, err
	}
	return id, nid, inserted, nil
}

func (r *Repository) PointLeaseAtNotification(tx *gorm.DB, leaseID, notificationID uint) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Model(&NotificationDedupeLease{}).Where("id = ?", leaseID).Update("notification_id", notificationID).Error
}

func (r *Repository) DeleteLease(tx *gorm.DB, leaseID uint) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Delete(&NotificationDedupeLease{}, leaseID).Error
}

func (r *Repository) InsertOutbox(tx *gorm.DB, row NotificationOutbox) error {
	if tx == nil {
		tx = r.db
	}
	if row.AvailableAt.IsZero() {
		row.AvailableAt = time.Now()
	}
	return tx.Create(&row).Error
}

// ClaimDueOutbox leases up to limit due rows — doc 03 §3 claim protocol
// (due-time filter + lease expiry + SKIP LOCKED).
func (r *Repository) ClaimDueOutbox(limit int) ([]NotificationOutbox, string, error) {
	token := newToken()
	var rows []NotificationOutbox
	err := r.db.Raw(`
UPDATE notification_outbox
   SET claimed_at = now(), claim_token = ?, lease_expires_at = now() + interval '2 minutes', attempts = attempts + 1
 WHERE id IN (
   SELECT id FROM notification_outbox
    WHERE done = false AND available_at <= now()
      AND (claimed_at IS NULL OR lease_expires_at < now())
    ORDER BY available_at, id
    LIMIT ?
    FOR UPDATE SKIP LOCKED
 )
RETURNING *`, token, limit).Scan(&rows).Error
	return rows, token, err
}

// CompleteOutbox marks a claimed row done — only while the caller still holds
// a live (unexpired) lease; an expired lease counts as a lost claim so the row
// can be reclaimed by another worker (doc 03 §3).
func (r *Repository) CompleteOutbox(id uint, token string) error {
	res := r.db.Model(&NotificationOutbox{}).
		Where("id = ? AND claim_token = ? AND (lease_expires_at IS NULL OR lease_expires_at > now())", id, token).
		Updates(map[string]any{"done": true, "lease_expires_at": nil})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("outbox claim lost (id=%d)", id)
	}
	return nil
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
