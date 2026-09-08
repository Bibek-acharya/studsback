package application

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"studsphere/backend/internal/messaging/domain"
	"studsphere/backend/internal/messaging/repository"
	"studsphere/backend/internal/notification"
)

type fakePresence struct {
	online map[string]bool
}

func (f *fakePresence) IsOnline(userType string, userID uint) (bool, error) {
	return f.online[fmt.Sprintf("%s:%d", userType, userID)], nil
}

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

// usersRow is a projection of auth.User covering only the columns the
// sender-name lookup touches. Kept local to avoid importing the auth module.
type usersRow struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	FirstName string         `json:"first_name"`
	LastName  string         `json:"last_name"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (usersRow) TableName() string { return "users" }

// institutionUsersRow is a projection of institution.InstitutionUser covering
// only the columns the sender-name lookup touches. Kept local to avoid
// importing the institution module.
type institutionUsersRow struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	InstitutionName string         `json:"institution_name"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (institutionUsersRow) TableName() string { return "institution_users" }

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

func testMemDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(
		&domain.Message{}, &domain.Conversation{}, &domain.Participant{},
		&domain.Attachment{}, &domain.PendingUpload{}, &domain.OutboxEvent{},
		&usersRow{}, &institutionUsersRow{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func newTestMessageService(db *gorm.DB, pres PresenceChecker, ntf notification.Notifier) MessageService {
	return NewMessageService(
		repository.NewMessageRepository(db),
		repository.NewParticipantRepository(db),
		repository.NewConversationRepository(db),
		repository.NewAttachmentRepository(db),
		repository.NewOutboxRepository(db),
		pres, ntf,
	)
}

// seedConversation creates a conversation with a student and an institution
// participant (plus any extra participants the test appends).
func seedConversation(t *testing.T, db *gorm.DB, convID, studentID, institutionID uint, extra ...domain.Participant) {
	t.Helper()
	if err := db.Create(&domain.Conversation{ID: convID, StudentID: studentID, InstitutionID: institutionID}).Error; err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	parts := []domain.Participant{
		{ConversationID: convID, ParticipantType: "student", ParticipantID: studentID},
		{ConversationID: convID, ParticipantType: "institution", ParticipantID: institutionID},
	}
	parts = append(parts, extra...)
	if err := db.Create(&parts).Error; err != nil {
		t.Fatalf("seed participants: %v", err)
	}
}

func TestSendMessageOfflineEmitsFallback(t *testing.T) {
	db := testMemDB(t)
	notif := &captureNotifier{}
	svc := newTestMessageService(db, &fakePresence{}, notif)
	seedConversation(t, db, 1, 7, 42)
	if err := db.Create(&usersRow{ID: 7, FirstName: "Ada", LastName: "Lovelace"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	if _, err := svc.SendMessage(1, "student", 7, "hello there", "cm-1", nil); err != nil {
		t.Fatalf("send: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventMessageOfflineFallback {
		t.Fatalf("expected %s, got %+v", notification.EventMessageOfflineFallback, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "institution", ID: 42}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	if notif.Last.Data["name"] != "Ada Lovelace" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	if notif.Last.Data["conversation_id"] != uint(1) {
		t.Fatalf("conversation_id wrong: %+v", notif.Last.Data["conversation_id"])
	}
	if notif.Last.DedupeKey != "offline_msg:1-42" {
		t.Fatalf("dedupe key wrong: %q", notif.Last.DedupeKey)
	}
	assertTemplates(t, *notif.Last)
}

func TestSendMessageOnlineNoEmission(t *testing.T) {
	db := testMemDB(t)
	notif := &captureNotifier{}
	svc := newTestMessageService(db, &fakePresence{online: map[string]bool{"institution:42": true}}, notif)
	seedConversation(t, db, 1, 7, 42)

	if _, err := svc.SendMessage(1, "student", 7, "hello", "cm-1", nil); err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(notif.Calls) != 0 {
		t.Fatalf("expected no emissions, got %+v", notif.Calls)
	}
}

func TestSendMessageUnmappedParticipantSkipped(t *testing.T) {
	db := testMemDB(t)
	notif := &captureNotifier{}
	svc := newTestMessageService(db, &fakePresence{}, notif)
	// Only the sender plus a guest participant — the guest has no inbox
	// identity (doc 03 §4), so no emission happens at all.
	if err := db.Create(&domain.Conversation{ID: 1, StudentID: 7, InstitutionID: 42}).Error; err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	if err := db.Create(&[]domain.Participant{
		{ConversationID: 1, ParticipantType: "student", ParticipantID: 7},
		{ConversationID: 1, ParticipantType: "guest", ParticipantID: 99},
	}).Error; err != nil {
		t.Fatalf("seed participants: %v", err)
	}

	if _, err := svc.SendMessage(1, "student", 7, "hello", "cm-1", nil); err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(notif.Calls) != 0 {
		t.Fatalf("expected no emissions for unmapped participant, got %+v", notif.Calls)
	}
}

// testPgDB mirrors notification's testutil: PostgreSQL-only integration tests
// against TEST_DATABASE_DSN. Skips (never fails) without the DSN.
func testPgDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set; dedupe integration test requires PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&domain.Message{}, &domain.Conversation{}, &domain.Participant{},
		&domain.Attachment{}, &domain.PendingUpload{}, &domain.OutboxEvent{},
		&notification.AccountNotification{}, &notification.NotificationOutbox{},
		&notification.NotificationDedupeLease{}, &notification.NotificationPreference{},
		&notification.NotificationDelivery{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id bigserial PRIMARY KEY,
		email text,
		first_name text,
		last_name text,
		role text DEFAULT 'student',
		status text DEFAULT 'active',
		deleted_at timestamptz,
		created_at timestamptz,
		updated_at timestamptz)`)
	return db
}

func TestSendMessageOfflineBurstDedupes(t *testing.T) {
	db := testPgDB(t)
	svc := notification.NewService(db)
	notifSvc := &burstNotifier{svc: svc}

	// Random high IDs isolate this run on the shared test database.
	base := uint(time.Now().UnixNano()%1_000_000) + 10_000_000
	convID, studentID, instID := base, base+1, base+2

	pg := testPgDB(t)
	msgSvc := newTestMessageService(pg, &fakePresence{}, notifSvc)
	seedConversation(t, pg, convID, studentID, instID)
	email := fmt.Sprintf("msg-burst-%d@test.local", studentID)
	if err := pg.Exec(`INSERT INTO users (email, first_name, last_name, role, status) VALUES (?, 'Ada', 'Lovelace', 'student', 'active')`, email).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		pg.Exec(`DELETE FROM users WHERE email = ?`, email)
		pg.Exec(`DELETE FROM conversation_participants WHERE conversation_id = ?`, convID)
		pg.Exec(`DELETE FROM messages WHERE conversation_id = ?`, convID)
		pg.Exec(`DELETE FROM conversations WHERE id = ?`, convID)
		pg.Exec(`DELETE FROM account_notifications WHERE event_key = ? AND account_type = 'institution' AND account_id = ?`, notification.EventMessageOfflineFallback, instID)
		pg.Exec(`DELETE FROM notification_dedupe_leases WHERE dedupe_key LIKE ?`, fmt.Sprintf("offline_msg:%d-%%", convID))
		pg.Exec(`DELETE FROM notification_outbox WHERE payload::text LIKE ?`, fmt.Sprintf("%%offline_msg:%d-%%", convID))
	})

	for i := 0; i < 3; i++ {
		if _, err := msgSvc.SendMessage(convID, "student", studentID, fmt.Sprintf("burst %d", i), fmt.Sprintf("cm-%d", i), nil); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}

	var n int64
	pg.Table("account_notifications").
		Where("event_key = ? AND account_type = 'institution' AND account_id = ?", notification.EventMessageOfflineFallback, instID).
		Count(&n)
	if n != 1 {
		t.Fatalf("inbox rows=%d want 1 (1h dedupe window collapses the burst)", n)
	}
}

type burstNotifier struct {
	svc notification.Notifier
}

func (b *burstNotifier) Notify(ctx context.Context, req notification.NotifyRequest) error {
	return b.svc.Notify(ctx, req)
}

func (b *burstNotifier) NotifyTx(ctx context.Context, tx *gorm.DB, req notification.NotifyRequest) error {
	return b.svc.NotifyTx(ctx, tx, req)
}

func (b *burstNotifier) ForRoles(ctx context.Context, roles ...string) ([]notification.Ref, error) {
	return b.svc.ForRoles(ctx, roles...)
}
