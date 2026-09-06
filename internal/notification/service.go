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
