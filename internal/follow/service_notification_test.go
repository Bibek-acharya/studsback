package follow

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

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

// usersRow is a projection of auth.User covering only the columns the
// follower-name lookup touches. Kept local to avoid importing the auth module.
type usersRow struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	FirstName string         `json:"first_name"`
	LastName  string         `json:"last_name"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (usersRow) TableName() string { return "users" }

// institutionUsersRow is a projection of institution.InstitutionUser covering
// only the columns the recipient-resolution query touches. Kept local to avoid
// importing the institution module.
type institutionUsersRow struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Status    string         `gorm:"default:'pending'" json:"status"`
	CollegeID uint           `gorm:"default:0" json:"college_id"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
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

func testDBFollow(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&UserFollow{}, &usersRow{}, &institutionUsersRow{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func seedFollower(t *testing.T, db *gorm.DB, id uint, first, last string) {
	t.Helper()
	if err := db.Create(&usersRow{ID: id, FirstName: first, LastName: last}).Error; err != nil {
		t.Fatalf("seed follower: %v", err)
	}
}

func TestFollowNotifiesInstitutionOwner(t *testing.T) {
	db := testDBFollow(t)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)
	seedFollower(t, db, 7, "Ada", "Lovelace")
	if err := db.Create(&institutionUsersRow{ID: 42, Status: "approved", CollegeID: 3}).Error; err != nil {
		t.Fatalf("seed institution user: %v", err)
	}

	if err := svc.Follow(7, 3, "institution"); err != nil {
		t.Fatalf("follow: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventSocialNewFollower {
		t.Fatalf("expected %s, got %+v", notification.EventSocialNewFollower, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "institution", ID: 42}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	if notif.Last.Data["name"] != "Ada Lovelace" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	assertTemplates(t, *notif.Last)
}

func TestFollowSkipsUnclaimedInstitution(t *testing.T) {
	db := testDBFollow(t)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)
	seedFollower(t, db, 7, "Ada", "Lovelace")

	// No institution_users row claims college 3 — no inbox identity, no
	// emission (doc 15 Q6 guard).
	if err := svc.Follow(7, 3, "institution"); err != nil {
		t.Fatalf("follow: %v", err)
	}
	if len(notif.Calls) != 0 {
		t.Fatalf("expected no emissions, got %+v", notif.Calls)
	}
}

func TestFollowUniversityNoEmission(t *testing.T) {
	db := testDBFollow(t)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)
	seedFollower(t, db, 7, "Ada", "Lovelace")

	// Universities have no inbox identity on the platform (D-Q6: providers
	// are not followable either) — the follow succeeds silently.
	if err := svc.Follow(7, 5, "university"); err != nil {
		t.Fatalf("follow: %v", err)
	}
	if len(notif.Calls) != 0 {
		t.Fatalf("expected no emissions, got %+v", notif.Calls)
	}
}

func TestDuplicateFollowNoEmission(t *testing.T) {
	db := testDBFollow(t)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)
	seedFollower(t, db, 7, "Ada", "Lovelace")
	if err := db.Create(&institutionUsersRow{ID: 42, Status: "approved", CollegeID: 3}).Error; err != nil {
		t.Fatalf("seed institution user: %v", err)
	}

	if err := svc.Follow(7, 3, "institution"); err != nil {
		t.Fatalf("first follow: %v", err)
	}
	if err := svc.Follow(7, 3, "institution"); err != nil {
		t.Fatalf("duplicate follow: %v", err)
	}

	if len(notif.Calls) != 1 {
		t.Fatalf("expected exactly one emission, got %d: %+v", len(notif.Calls), notif.Calls)
	}
}
