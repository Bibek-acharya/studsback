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

func (r *Repository) CreatePublicNotification(n *PublicNotification) error {
	return r.db.Create(n).Error
}

func (r *Repository) FindPublicNotificationByID(id uint) (*PublicNotification, error) {
	var n PublicNotification
	if err := r.db.First(&n, id).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

func (r *Repository) UpdatePublicNotification(id uint, updates map[string]interface{}) (*PublicNotification, error) {
	n, err := r.FindPublicNotificationByID(id)
	if err != nil {
		return nil, err
	}
	if err := r.db.Model(n).Updates(updates).Error; err != nil {
		return nil, err
	}
	return r.FindPublicNotificationByID(id)
}

// ListAllPublicNotifications returns every non-deleted banner, active or not.
func (r *Repository) ListAllPublicNotifications() ([]PublicNotification, error) {
	var rows []PublicNotification
	err := r.db.Order("created_at desc").Find(&rows).Error
	return rows, err
}

func (r *Repository) SoftDeletePublicNotification(id uint) (int64, error) {
	res := r.db.Delete(&PublicNotification{}, id)
	return res.RowsAffected, res.Error
}

func (r *Repository) InsertDelivery(tx *gorm.DB, d NotificationDelivery) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&d).Error
}

func (r *Repository) UpsertPreference(pref NotificationPreference) error {
	// Build assignment map for only non-nil fields (sparse upsert).
	assignments := map[string]interface{}{
		"updated_at": time.Now(),
	}
	if pref.InApp != nil {
		assignments["in_app"] = pref.InApp
	}
	if pref.Email != nil {
		assignments["email"] = pref.Email
	}
	if pref.Realtime != nil {
		assignments["realtime"] = pref.Realtime
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "account_type"}, {Name: "account_id"}, {Name: "pref_key"}},
		DoUpdates: clause.Assignments(assignments),
	}).Create(&pref).Error
}

func (r *Repository) GetPreferences(accountType string, accountID uint) ([]NotificationPreference, error) {
	var prefs []NotificationPreference
	err := r.db.Where("account_type = ? AND account_id = ?", accountType, accountID).Find(&prefs).Error
	return prefs, err
}

func (r *Repository) DeletePreference(accountType string, accountID uint, prefKey string) error {
	return r.db.Where("account_type = ? AND account_id = ? AND pref_key = ?",
		accountType, accountID, prefKey).Delete(&NotificationPreference{}).Error
}

// EmailForAccount returns the email address for an account.
func (r *Repository) EmailForAccount(accountType string, accountID uint) (string, error) {
	switch accountType {
	case "user":
		var u struct{ Email string }
		if err := r.db.Raw("SELECT email FROM users WHERE id = ?", accountID).Scan(&u).Error; err != nil {
			return "", err
		}
		return u.Email, nil
	case "institution":
		var u struct{ Email string }
		if err := r.db.Raw("SELECT email FROM institution_users WHERE id = ?", accountID).Scan(&u).Error; err != nil {
			return "", err
		}
		return u.Email, nil
	case "provider":
		var u struct{ Email string }
		if err := r.db.Raw("SELECT email FROM scholarship_provider_users WHERE id = ?", accountID).Scan(&u).Error; err != nil {
			return "", err
		}
		return u.Email, nil
	default:
		return "", fmt.Errorf("notification: unsupported account type %q", accountType)
	}
}

// ClaimDelivery CAS: pending → dispatching with a reservation lease.
// Returns false if the row is no longer pending (already claimed/skipped).
func (r *Repository) ClaimDelivery(id uint, token string, expiry time.Duration) (bool, error) {
	res := r.db.Exec(`UPDATE notification_deliveries
		SET status = 'dispatching', dispatch_expires_at = now() + make_interval(secs => ?),
		    attempts = attempts + 1
		WHERE id = ? AND status = 'pending'`,
		expiry.Seconds(), id)
	return res.RowsAffected > 0, res.Error
}

// CompleteDelivery sets a delivery's terminal status (handed_off | failed |
// skipped) or reverts it to pending; sent_at is stamped on hand-off.
func (r *Repository) CompleteDelivery(id uint, status string, errMsg string) error {
	updates := map[string]any{"status": status, "error": errMsg}
	if status == "handed_off" {
		updates["sent_at"] = time.Now()
	}
	return r.db.Model(&NotificationDelivery{}).Where("id = ?", id).Updates(updates).Error
}

// ReopenExpiredDeliveries resets dispatching rows whose lease expired → pending
// (recovery sweep for crashed workers).
func (r *Repository) ReopenExpiredDeliveries() (int64, error) {
	res := r.db.Exec(`UPDATE notification_deliveries
		SET status = 'pending', dispatch_expires_at = NULL
		WHERE status = 'dispatching' AND dispatch_expires_at < now()`)
	return res.RowsAffected, res.Error
}

// StuckPendingDeliveries returns email deliveries that have sat pending longer
// than beforeMinutes — their process task exhausted its retries and nothing
// else will hand them off (the poller's stranded-pending sweep re-kicks them).
// Bounded per sweep so a backlog can't grow the query unboundedly.
func (r *Repository) StuckPendingDeliveries(beforeMinutes int) ([]NotificationDelivery, error) {
	var rows []NotificationDelivery
	err := r.db.Raw(`SELECT d.* FROM notification_deliveries d
		WHERE d.channel = 'email' AND d.status = 'pending'
		  AND d.created_at < now() - make_interval(mins => ?)
		ORDER BY d.created_at
		LIMIT 200`,
		beforeMinutes).Scan(&rows).Error
	return rows, err
}

// PendingDeliveriesForOutbox returns pending/stale-dispatching email deliveries
// for the given correlation IDs (one NotifyRequest correlation per outbox row).
func (r *Repository) PendingDeliveriesForOutbox(correlationIDs []string) ([]NotificationDelivery, error) {
	var deliveries []NotificationDelivery
	err := r.db.Where(`channel = 'email' AND status IN ('pending', 'dispatching')
		AND correlation_id IN (?)`, correlationIDs).Find(&deliveries).Error
	return deliveries, err
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
