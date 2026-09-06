// internal/notification/service.go
package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Ref struct {
	Type string // user | institution | provider
	ID   uint
}

type NotifyRequest struct {
	EventKey      string
	Actor         *Ref
	Recipients    []Ref
	Data          map[string]any
	Link          string
	OccurrenceKey string
	DedupeKey     string
	CorrelationID string
}

// Notifier is what business modules consume (constructor-injected).
type Notifier interface {
	Notify(ctx context.Context, req NotifyRequest) error
	NotifyTx(ctx context.Context, tx *gorm.DB, req NotifyRequest) error
	ForRoles(ctx context.Context, roles ...string) ([]Ref, error)
}

type Service struct {
	repo *Repository
	db   *gorm.DB
}

func NewService(db *gorm.DB) *Service { return &Service{repo: NewRepository(db), db: db} }

// ForRoles resolves active accounts by role — audience rules (doc 03 §3).
// Role spellings are normalized: superadmin/super_admin both match.
func (s *Service) ForRoles(ctx context.Context, roles ...string) ([]Ref, error) {
	return s.RecipientsForRole(roles...)
}

func (s *Service) RecipientsForRole(roles ...string) ([]Ref, error) {
	set := map[string]bool{}
	for _, r := range roles {
		if r = strings.ToLower(strings.TrimSpace(r)); r != "" {
			set[r] = true
		}
	}
	// superadmin/super_admin are one audience under two spellings.
	if set["superadmin"] || set["super_admin"] {
		set["superadmin"] = true
		set["super_admin"] = true
	}
	lowered := make([]string, 0, len(set))
	for r := range set {
		lowered = append(lowered, r)
	}
	var rows []struct{ ID uint }
	err := s.db.Raw(
		`SELECT id FROM users WHERE lower(role) IN ? AND status = 'active' AND deleted_at IS NULL`,
		lowered,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	refs := make([]Ref, 0, len(rows))
	for _, r := range rows {
		refs = append(refs, Ref{Type: "user", ID: r.ID})
	}
	return refs, nil
}

// ResolveAccount maps JWT role claims to an inbox identity (doc 03 §4).
func ResolveAccount(role string, userID, providerID uint) (string, uint, bool) {
	switch role {
	case "student", "admin", "superadmin", "super_admin":
		return "user", userID, true
	case "institution":
		return "institution", userID, true
	case "scholarship_provider":
		return "provider", userID, true
	case "scholarship_provider_subuser":
		return "provider", providerID, true
	default:
		return "", 0, false
	}
}

func (s *Service) Notify(ctx context.Context, req NotifyRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.NotifyTx(ctx, tx, req)
	})
}

// NotifyTx writes inbox rows (+ outbox dispatch row) inside the caller's
// transaction — emission is atomic with the business mutation (doc 03 §3).
func (s *Service) NotifyTx(ctx context.Context, tx *gorm.DB, req NotifyRequest) error {
	def, ok := Registry[req.EventKey]
	if !ok {
		return fmt.Errorf("notification: unknown event key %q", req.EventKey)
	}
	if len(req.Recipients) == 0 {
		return errors.New("notification: no recipients")
	}
	if req.Data == nil {
		req.Data = map[string]any{}
	}
	payload, _ := json.Marshal(req)
	now := time.Now()

	for _, rec := range req.Recipients {
		if def.Transactional {
			continue // email-only class: never an inbox row (P2 sends the email)
		}
		if req.DedupeKey != "" {
			leaseID, nid, inserted, err := s.repo.AcquireLease(tx, rec.Type, rec.ID, req.DedupeKey, def.DedupeWinOr(24*time.Hour))
			if err != nil {
				return fmt.Errorf("notification: lease: %w", err)
			}
			if !inserted {
				if leaseID == 0 {
					continue // active lease — suppress
				}
				// Expired-lease renewal. ReNudge events refresh the referenced
				// row; everything else inserts fresh and re-points the lease.
				if def.ReNudge && nid != nil {
					if err := s.refreshRow(tx, *nid, def, req); err != nil {
						return err
					}
					continue
				}
			}
			row, err := s.buildRow(def, req, rec, now)
			if err != nil {
				return err
			}
			if err := s.repo.InsertNotifications(tx, []AccountNotification{*row}); err != nil {
				return err
			}
			if leaseID != 0 {
				if err := s.repo.PointLeaseAtNotification(tx, leaseID, row.ID); err != nil {
					return err
				}
			}
			continue
		}
		if req.OccurrenceKey != "" {
			existing, err := s.repo.FindByOccurrenceKey(tx, rec.Type, rec.ID, req.OccurrenceKey)
			if err != nil {
				return err
			}
			if existing != nil {
				continue // caller retry — already emitted
			}
		}
		row, err := s.buildRow(def, req, rec, now)
		if err != nil {
			return err
		}
		if err := s.repo.InsertNotifications(tx, []AccountNotification{*row}); err != nil {
			if isUniqueViolation(err) {
				continue // concurrent duplicate on occurrence key — treat as emitted
			}
			return err
		}
	}

	return s.repo.InsertOutbox(tx, NotificationOutbox{
		Kind:          "dispatch",
		Payload:       datatypes.JSON(payload),
		OccurrenceKey: req.OccurrenceKey,
	})
}

func (s *Service) refreshRow(tx *gorm.DB, notificationID uint, def EventDef, req NotifyRequest) error {
	// Preserve read_at/archived_at; refresh updated_at/body (doc 04 §4d).
	body, err := ResolveTemplate(def.BodyTpl, req.Data)
	if err != nil {
		body = ""
	}
	return tx.Model(&AccountNotification{}).
		Where("id = ? AND deleted_at IS NULL", notificationID).
		Updates(map[string]any{"body": body, "updated_at": time.Now()}).Error
}

func (s *Service) buildRow(def EventDef, req NotifyRequest, rec Ref, now time.Time) (*AccountNotification, error) {
	title, err := ResolveTemplate(def.TitleTpl, req.Data)
	if err != nil || title == "" {
		title = humanizeKey(def.Key) // fallback: never block in-app on template bugs
	}
	body, err := ResolveTemplate(def.BodyTpl, req.Data)
	if err != nil {
		body = title
	}
	link := def.LinkTpl
	if req.Link != "" {
		link = req.Link
	}
	row := &AccountNotification{
		AccountType: rec.Type, AccountID: rec.ID,
		EventKey: def.Key, Category: def.Category, Priority: def.Priority,
		Title: title, Body: body, Link: link,
		OccurrenceKey: req.OccurrenceKey,
	}
	if req.Actor != nil {
		row.ActorType, row.ActorID = req.Actor.Type, req.Actor.ID
	}
	if req.CorrelationID != "" {
		row.CorrelationID = req.CorrelationID
	}
	return row, nil
}

func humanizeKey(key string) string {
	b := []rune(key)
	for i, c := range b {
		if c == '.' || c == '_' {
			b[i] = ' '
		}
	}
	if len(b) > 0 {
		b[0] = []rune(strings.ToUpper(string(b[0])))[0]
	}
	return string(b)
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, gorm.ErrDuplicatedKey) ||
		strings.Contains(err.Error(), "duplicate key") ||
		strings.Contains(err.Error(), "uq_an_occurrence")
}

func (s *Service) CreateBroadcast(createdBy uint, title, body, link, priority string, audience []string, idemKey string) (*NotificationBroadcast, bool, error) {
	valid := map[string]bool{"user": true, "institution": true, "provider": true, "all": true}
	for _, a := range audience {
		if !valid[a] {
			return nil, false, fmt.Errorf("notification: invalid audience %q", a)
		}
	}
	campaign := NotificationBroadcast{
		Title: title, Body: body, Link: link, Priority: priority,
		Audience: datatypes.JSON(mustJSON(audience)), CreatedBy: createdBy, IdempotencyKey: idemKey,
	}
	created := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if idemKey != "" {
			var existing NotificationBroadcast
			err := tx.Where("created_by = ? AND idempotency_key = ?", createdBy, idemKey).First(&existing).Error
			if err == nil {
				campaign = existing // idempotent replay: return the original
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if err := tx.Create(&campaign).Error; err != nil {
			return err
		}
		created = true
		return s.repo.InsertOutbox(tx, NotificationOutbox{
			Kind:    "fanout",
			Payload: datatypes.JSON(mustJSON(map[string]any{"broadcast_id": campaign.ID})),
		})
	})
	if err != nil {
		return nil, false, err
	}
	return &campaign, created, nil
}

// ExpandFanoutPending expands every un-done fanout row (poller + test entry).
func (s *Service) ExpandFanoutPending(ctx context.Context) error {
	var rows []NotificationOutbox
	if err := s.db.WithContext(ctx).Where("done = false AND kind = 'fanout'").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if err := s.expandFanoutRow(row); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) expandFanoutRow(row NotificationOutbox) error {
	var payload struct {
		BroadcastID uint `json:"broadcast_id"`
	}
	_ = json.Unmarshal(row.Payload, &payload)
	var campaign NotificationBroadcast
	if err := s.db.First(&campaign, payload.BroadcastID).Error; err != nil {
		return err
	}
	if campaign.Status != "sending" {
		return s.repo.CompleteOutbox(row.ID, row.ClaimToken) // cancelled — nothing to fan out
	}
	batch := 500
	offset := 0
	for {
		var targets []struct {
			AccountType string
			AccountID   uint
		}
		q := s.db.Raw(buildAudienceQuery(campaign.Audience)+` ORDER BY account_type, account_id OFFSET ? LIMIT ?`, offset, batch)
		if err := q.Scan(&targets).Error; err != nil {
			return err
		}
		if len(targets) == 0 {
			break
		}
		err := s.db.Transaction(func(tx *gorm.DB) error {
			for _, t := range targets {
				occ := fmt.Sprintf("system.announcement:b%d:%s:%d", campaign.ID, t.AccountType, t.AccountID)
				existing, err := s.repo.FindByOccurrenceKey(tx, t.AccountType, t.AccountID, occ)
				if err != nil {
					return err
				}
				if existing != nil {
					continue // batch replay is harmless (doc 03 §3)
				}
				title, _ := ResolveTemplate(Registry[EventSystemAnnouncement].TitleTpl, map[string]any{"title": campaign.Title})
				b, _ := ResolveTemplate(Registry[EventSystemAnnouncement].BodyTpl, map[string]any{"body": campaign.Body})
				cid := campaign.ID
				if err := s.repo.InsertNotifications(tx, []AccountNotification{{
					AccountType: t.AccountType, AccountID: t.AccountID,
					EventKey: EventSystemAnnouncement, Category: "system", Priority: campaign.Priority,
					Title: title, Body: b, Link: campaign.Link,
					OccurrenceKey: occ, BroadcastID: &cid, ActorType: "user", ActorID: campaign.CreatedBy,
				}}); err != nil {
					return err
				}
			}
			return tx.Model(&NotificationBroadcast{}).Where("id = ?", campaign.ID).
				Update("sent_count", gorm.Expr("sent_count + ?", len(targets))).Error
		})
		if err != nil {
			return err
		}
		offset += batch
		if len(targets) < batch {
			break
		}
	}
	s.db.Model(&NotificationBroadcast{}).Where("id = ? AND status = 'sending'", campaign.ID).Update("status", "completed")
	return s.repo.CompleteOutbox(row.ID, row.ClaimToken)
}

// buildAudienceQuery returns the eligibility-filtered UNION for the audience
// enum (user | institution | provider | all) — doc 04 §4c. Eligibility:
// suspended/deletion-queued/soft-deleted excluded for every member set.
func buildAudienceQuery(audience datatypes.JSON) string {
	var sets []string
	_ = json.Unmarshal(audience, &sets)
	has := func(v string) bool {
		for _, s := range sets {
			if s == v {
				return true
			}
		}
		return false
	}
	all := has("all")
	var parts []string
	if all || has("user") {
		parts = append(parts, `SELECT 'user' AS account_type, id AS account_id FROM users
			WHERE status = 'active' AND deleted_at IS NULL AND lower(role) IN ('student','admin','superadmin','super_admin')`)
	}
	if all || has("institution") {
		parts = append(parts, `SELECT 'institution' AS account_type, id AS account_id FROM institution_users
			WHERE status = 'approved' AND deleted_at IS NULL`)
	}
	if all || has("provider") {
		parts = append(parts, `SELECT 'provider' AS account_type, id AS account_id FROM scholarship_provider_users
			WHERE status = 'approved' AND deleted_at IS NULL`)
	}
	return strings.Join(parts, " UNION ")
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
