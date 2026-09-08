package scholarship

import (
	"context"
	"encoding/json"
	"fmt"
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
	if err := db.AutoMigrate(&Scholarship{}, &ScholarshipApplication{}, &Payment{}, &ProviderScholarship{}); err != nil {
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
			"status":           "CANCELED",
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
		Status:          "CANCELED",
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

// TestEsewaPendingDoesNotNotifyStudent: an in-flight transaction (PENDING) is
// not a failure — no payment_failed emission while the poller re-verifies it.
func TestEsewaPendingDoesNotNotifyStudent(t *testing.T) {
	db := newPaymentTestDB(t)
	_, app, pay := seedPaymentScenario(t, db)

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

	if len(notif.Calls) != 0 {
		t.Fatalf("PENDING must not emit payment_failed, got %+v", notif.Calls)
	}
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
	if notif.Last.DedupeKey != fmt.Sprintf("bank_rejected:%d", pay.ID) {
		t.Fatalf("dedupe key wrong: %q", notif.Last.DedupeKey)
	}
	assertTemplates(t, *notif.Last)
}

// stubEsewaServer points the status-check seam at a stub returning status.
func stubEsewaServer(t *testing.T, status, transactionUUID string) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":           status,
			"total_amount":     "100",
			"transaction_uuid": transactionUUID,
		})
	}))
	t.Cleanup(ts.Close)
	origURL := esewaStatusAPIURL
	esewaStatusAPIURL = func() string { return ts.URL }
	t.Cleanup(func() { esewaStatusAPIURL = origURL })
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = nil })
}

// seedProviderPipeline attaches a provider-owned scholarship mirror to sch so
// emissions can resolve the provider org (scholarship_provider_users row id).
func seedProviderPipeline(t *testing.T, db *gorm.DB, sch *Scholarship) *ProviderScholarship {
	t.Helper()
	ps := &ProviderScholarship{ID: 900, ProviderID: 55, Title: sch.Title}
	if err := db.Create(ps).Error; err != nil {
		t.Fatalf("seed provider scholarship: %v", err)
	}
	sch.ProviderScholarshipID = &ps.ID
	if err := db.Save(sch).Error; err != nil {
		t.Fatalf("attach provider pipeline: %v", err)
	}
	t.Cleanup(func() { db.Delete(ps) })
	return ps
}

// TestEsewaCompleteNotifiesStudentAndProvider: the terminal COMPLETE handler
// sends the payment receipt copy to the student and, because a provider
// pipeline owns this scholarship, to the provider org too.
func TestEsewaCompleteNotifiesStudentAndProvider(t *testing.T) {
	db := newPaymentTestDB(t)
	sch, app, pay := seedPaymentScenario(t, db)
	ps := seedProviderPipeline(t, db, sch)
	stubEsewaServer(t, "COMPLETE", pay.TransactionID)

	notif := &captureNotifier{}
	svc := NewPaymentService(db, notif)

	if _, err := svc.VerifyEsewaPayment(EsewaVerifyRequest{
		ApplicationID:   app.ID,
		TransactionUUID: pay.TransactionID,
		TotalAmount:     "100",
		ProductCode:     "EPAYTEST",
		Status:          "COMPLETE",
	}); err != nil {
		t.Fatalf("verify: %v", err)
	}

	var student, provider *notification.NotifyRequest
	for i := range notif.Calls {
		switch notif.Calls[i].Recipients[0].Type {
		case "user":
			student = &notif.Calls[i]
		case "provider":
			provider = &notif.Calls[i]
		}
	}
	if student == nil || provider == nil {
		t.Fatalf("expected student + provider copies, got %+v", notif.Calls)
	}
	if student.Recipients[0] != (notification.Ref{Type: "user", ID: 7}) {
		t.Fatalf("student recipient wrong: %+v", student.Recipients)
	}
	if provider.Recipients[0] != (notification.Ref{Type: "provider", ID: ps.ProviderID}) {
		t.Fatalf("provider recipient wrong: %+v", provider.Recipients)
	}
	for _, req := range []*notification.NotifyRequest{student, provider} {
		if req.Data["slug"] != sch.Slug || req.Data["scholarship"] != sch.Title {
			t.Fatalf("data wrong: %+v", req.Data)
		}
		assertTemplates(t, *req)
	}
}

// TestEsewaCompleteSkipsProviderWithoutPipeline: admin/institution-created
// scholarships have no provider inbox — student copy only.
func TestEsewaCompleteSkipsProviderWithoutPipeline(t *testing.T) {
	db := newPaymentTestDB(t)
	_, app, pay := seedPaymentScenario(t, db)
	stubEsewaServer(t, "COMPLETE", pay.TransactionID)

	notif := &captureNotifier{}
	svc := NewPaymentService(db, notif)

	if _, err := svc.VerifyEsewaPayment(EsewaVerifyRequest{
		ApplicationID:   app.ID,
		TransactionUUID: pay.TransactionID,
		TotalAmount:     "100",
		ProductCode:     "EPAYTEST",
		Status:          "COMPLETE",
	}); err != nil {
		t.Fatalf("verify: %v", err)
	}

	if len(notif.Calls) != 1 {
		t.Fatalf("expected only the student copy, got %+v", notif.Calls)
	}
	if notif.Calls[0].Recipients[0] != (notification.Ref{Type: "user", ID: 7}) {
		t.Fatalf("recipient wrong: %+v", notif.Calls[0].Recipients)
	}
	assertTemplates(t, notif.Calls[0])
}

// TestBankReceiptNotifiesProvider: a student's bank-receipt upload notifies
// the provider org whose pipeline owns the scholarship.
func TestBankReceiptNotifiesProvider(t *testing.T) {
	db := newPaymentTestDB(t)
	sch, _, pay := seedPaymentScenario(t, db)
	ps := seedProviderPipeline(t, db, sch)
	pay.Method = "bank"
	if err := db.Save(pay).Error; err != nil {
		t.Fatalf("update payment: %v", err)
	}

	notif := &captureNotifier{}
	svc := NewPaymentService(db, notif)

	if err := svc.UploadBankReceipt(pay.ID, "https://example.com/receipt.png"); err != nil {
		t.Fatalf("upload receipt: %v", err)
	}

	if len(notif.Calls) != 1 {
		t.Fatalf("expected 1 emission, got %+v", notif.Calls)
	}
	req := notif.Calls[0]
	if req.EventKey != notification.EventScholarshipBankReceipt {
		t.Fatalf("expected %s, got %+v", notification.EventScholarshipBankReceipt, req)
	}
	if req.Recipients[0] != (notification.Ref{Type: "provider", ID: ps.ProviderID}) {
		t.Fatalf("recipient wrong: %+v", req.Recipients)
	}
	assertTemplates(t, req)
}

// TestBankReceiptWithoutPipelineEmitsNothing: no provider pipeline → no
// provider org to notify (receipt review falls back to the manual queue).
func TestBankReceiptWithoutPipelineEmitsNothing(t *testing.T) {
	db := newPaymentTestDB(t)
	_, _, pay := seedPaymentScenario(t, db)
	pay.Method = "bank"
	if err := db.Save(pay).Error; err != nil {
		t.Fatalf("update payment: %v", err)
	}

	notif := &captureNotifier{}
	svc := NewPaymentService(db, notif)

	if err := svc.UploadBankReceipt(pay.ID, "https://example.com/receipt.png"); err != nil {
		t.Fatalf("upload receipt: %v", err)
	}

	if len(notif.Calls) != 0 {
		t.Fatalf("expected no emission without a provider pipeline, got %+v", notif.Calls)
	}
}
