package institution

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"studsphere/backend/internal/education"
	"studsphere/backend/internal/notification"
)

type captureNotifier struct {
	Last  *notification.NotifyRequest
	Calls []notification.NotifyRequest
}

func (c *captureNotifier) Notify(_ context.Context, req notification.NotifyRequest) error {
	c.Last = &req
	c.Calls = append(c.Calls, req)
	return nil
}

func (c *captureNotifier) NotifyTx(ctx context.Context, _ *gorm.DB, req notification.NotifyRequest) error {
	return c.Notify(ctx, req)
}

func (c *captureNotifier) ForRoles(_ context.Context, _ ...string) ([]notification.Ref, error) {
	return nil, nil
}

// assertTemplates enforces the missingkey=error data contract: every key
// referenced by the registry's Title/Body templates must be present in Data.
func assertTemplates(t *testing.T, req notification.NotifyRequest) {
	t.Helper()
	def, ok := notification.Registry[req.EventKey]
	if !ok {
		t.Fatalf("unknown event key %q", req.EventKey)
	}
	if _, err := notification.ResolveTemplate(def.TitleTpl, req.Data); err != nil {
		t.Errorf("title template for %s: %v", req.EventKey, err)
	}
	if _, err := notification.ResolveTemplate(def.BodyTpl, req.Data); err != nil {
		t.Errorf("body template for %s: %v", req.EventKey, err)
	}
}

func TestBookingConfirmNotifiesStudent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&InstitutionCounsellingSession{}, &InstitutionCounsellingBooking{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), education.NewRepository(db), nil, notif)

	session := &InstitutionCounsellingSession{InstitutionID: 1, Title: "Career Guidance", Status: "scheduled"}
	if err := db.Create(session).Error; err != nil {
		t.Fatalf("seed session: %v", err)
	}
	booking := &InstitutionCounsellingBooking{SessionID: session.ID, UserID: 7, Status: "pending"}
	if err := db.Create(booking).Error; err != nil {
		t.Fatalf("seed booking: %v", err)
	}

	if _, err := svc.UpdateBookingStatus(1, booking.ID, "confirmed", "", ""); err != nil {
		t.Fatalf("confirm booking: %v", err)
	}
	if notif.Last == nil || notif.Last.EventKey != notification.EventCounsellingBookingConfirmed {
		t.Fatalf("expected %s, got %+v", notification.EventCounsellingBookingConfirmed, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: booking.UserID}) {
		t.Fatalf("recipient wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}
