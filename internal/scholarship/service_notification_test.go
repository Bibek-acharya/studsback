package scholarship

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"studsphere/backend/internal/notification"
	"studsphere/backend/internal/shared/config"
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

func newPaymentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&Scholarship{}, &ScholarshipApplication{}, &Payment{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func seedPaymentScenario(t *testing.T, db *gorm.DB) (*Scholarship, *ScholarshipApplication, *Payment) {
	t.Helper()
	sch := &Scholarship{Slug: "merit-award", Title: "Merit Award", Provider: "Test Provider", Status: "published"}
	if err := db.Create(sch).Error; err != nil {
		t.Fatalf("seed scholarship: %v", err)
	}
	uid := uint(7)
	app := &ScholarshipApplication{
		ScholarshipID: sch.ID,
		UserID:        &uid,
		FullName:      "Test Student",
		Gender:        "male",
		Email:         "student@example.com",
		Status:        ApplicationStatusPendingPayment,
	}
	if err := db.Create(app).Error; err != nil {
		t.Fatalf("seed application: %v", err)
	}
	pay := &Payment{
		ApplicationID: app.ID,
		ScholarshipID: sch.ID,
		UserID:        &uid,
		Method:        "esewa",
		Amount:        100,
		Status:        "pending",
		TransactionID: "TX-FAIL-1",
	}
	if err := db.Create(pay).Error; err != nil {
		t.Fatalf("seed payment: %v", err)
	}
	return sch, app, pay
}

func TestEsewaFailureNotifiesStudent(t *testing.T) {
	db := newPaymentTestDB(t)
	sch, app, pay := seedPaymentScenario(t, db)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":           "PENDING",
			"total_amount":     "100",
			"transaction_uuid": pay.TransactionID,
		})
	}))
	defer ts.Close()
	origURL := esewaStatusAPIURL
	esewaStatusAPIURL = func() string { return ts.URL }
	defer func() { esewaStatusAPIURL = origURL }()

	config.AppConfig = &config.Config{}

	notif := &captureNotifier{}
	svc := NewPaymentService(db, notif)

	if _, err := svc.VerifyEsewaPayment(EsewaVerifyRequest{
		ApplicationID:   app.ID,
		TransactionUUID: pay.TransactionID,
		TotalAmount:     "100",
		ProductCode:     "EPAYTEST",
		Status:          "PENDING",
	}); err == nil {
		t.Fatal("expected verify error for non-COMPLETE status")
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventScholarshipPaymentFailed {
		t.Fatalf("expected %s, got %+v", notification.EventScholarshipPaymentFailed, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: 7}) {
		t.Fatalf("recipient wrong: %+v", notif.Last.Recipients)
	}
	if notif.Last.Data["slug"] != sch.Slug {
		t.Fatalf("slug data wrong: %+v", notif.Last.Data)
	}
	assertTemplates(t, *notif.Last)
}

func TestBankRejectionNotifiesStudent(t *testing.T) {
	db := newPaymentTestDB(t)
	_, _, pay := seedPaymentScenario(t, db)
	pay.Status = "pending_approval"
	pay.Method = "bank"
	if err := db.Save(pay).Error; err != nil {
		t.Fatalf("update payment: %v", err)
	}

	notif := &captureNotifier{}
	svc := NewPaymentService(db, notif)

	if err := svc.ApproveBankPayment(pay.ID, 1, "Receipt unreadable"); err != nil {
		t.Fatalf("approve bank payment: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventScholarshipBankRejected {
		t.Fatalf("expected %s, got %+v", notification.EventScholarshipBankRejected, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: 7}) {
		t.Fatalf("recipient wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}
