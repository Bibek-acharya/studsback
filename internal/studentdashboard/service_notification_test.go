package studentdashboard

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
// invitee-name lookup touches. Kept local to avoid importing the auth module.
type usersRow struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	FirstName string         `json:"first_name"`
	LastName  string         `json:"last_name"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (usersRow) TableName() string { return "users" }

// institutionUsersRow is a projection of institution.InstitutionUser covering
// only the columns the inviter-resolution query touches. Kept local to avoid
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

func testDBDashboard(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&SphereInvite{}, &usersRow{}, &institutionUsersRow{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func seedInvite(t *testing.T, db *gorm.DB) *SphereInvite {
	t.Helper()
	invite := &SphereInvite{UserID: 7, InstitutionID: 3, Title: "Open Day", Status: "pending"}
	if err := db.Create(invite).Error; err != nil {
		t.Fatalf("seed invite: %v", err)
	}
	return invite
}

func seedInvitee(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Create(&usersRow{ID: 7, FirstName: "Ada", LastName: "Lovelace"}).Error; err != nil {
		t.Fatalf("seed invitee: %v", err)
	}
}

func TestAcceptInviteNotifiesInviter(t *testing.T) {
	db := testDBDashboard(t)
	seedInvitee(t, db)
	invite := seedInvite(t, db)
	if err := db.Create(&institutionUsersRow{ID: 42, Status: "approved", CollegeID: 3}).Error; err != nil {
		t.Fatalf("seed institution user: %v", err)
	}
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)

	if _, err := svc.AcceptInvite(invite.ID, 7); err != nil {
		t.Fatalf("accept: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventSocialInviteAccepted {
		t.Fatalf("expected %s, got %+v", notification.EventSocialInviteAccepted, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "institution", ID: 42}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	if notif.Last.Data["name"] != "Ada Lovelace" || notif.Last.Data["response"] != "accepted" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	assertTemplates(t, *notif.Last)
}

func TestDeclineInviteNotifiesInviter(t *testing.T) {
	db := testDBDashboard(t)
	seedInvitee(t, db)
	invite := seedInvite(t, db)
	if err := db.Create(&institutionUsersRow{ID: 42, Status: "approved", CollegeID: 3}).Error; err != nil {
		t.Fatalf("seed institution user: %v", err)
	}
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)

	if _, err := svc.DeclineInvite(invite.ID, 7); err != nil {
		t.Fatalf("decline: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventSocialInviteAccepted {
		t.Fatalf("expected %s, got %+v", notification.EventSocialInviteAccepted, notif.Last)
	}
	if notif.Last.Data["name"] != "Ada Lovelace" || notif.Last.Data["response"] != "declined" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	assertTemplates(t, *notif.Last)
}

func TestInviteSkipsWithoutInstitutionOwner(t *testing.T) {
	db := testDBDashboard(t)
	seedInvitee(t, db)
	invite := seedInvite(t, db)
	// Claimed but unapproved (D-Q13) — no inbox identity, no emission.
	if err := db.Create(&institutionUsersRow{ID: 43, Status: "pending", CollegeID: 3}).Error; err != nil {
		t.Fatalf("seed institution user: %v", err)
	}
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)

	if _, err := svc.AcceptInvite(invite.ID, 7); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if len(notif.Calls) != 0 {
		t.Fatalf("expected no emissions, got %+v", notif.Calls)
	}
}

// invite.InstitutionID semantics are data-dependent (no backend creation
// site). Interpretation A: the value IS an institution_users.id.
func TestAcceptInviteResolvesDirectInstitutionID(t *testing.T) {
	db := testDBDashboard(t)
	seedInvitee(t, db)
	// InstitutionID 42 matches an approved institution_users row by id only
	// (no college_id linkage anywhere).
	invite := seedInvite(t, db)
	invite.InstitutionID = 42
	if err := db.Save(invite).Error; err != nil {
		t.Fatalf("point invite at institution id: %v", err)
	}
	if err := db.Create(&institutionUsersRow{ID: 42, Status: "approved", CollegeID: 999}).Error; err != nil {
		t.Fatalf("seed institution user: %v", err)
	}
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)

	if _, err := svc.AcceptInvite(invite.ID, 7); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if notif.Last == nil || notif.Last.EventKey != notification.EventSocialInviteAccepted {
		t.Fatalf("expected %s, got %+v", notification.EventSocialInviteAccepted, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "institution", ID: 42}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}

// Interpretation B: the value is a college id resolved via a claimed row.
func TestAcceptInviteResolvesCollegeIDFallback(t *testing.T) {
	db := testDBDashboard(t)
	seedInvitee(t, db)
	// Invite.InstitutionID=3 has no matching institution_users row with id 3,
	// so the college claim path must resolve it.
	invite := seedInvite(t, db)
	if err := db.Create(&institutionUsersRow{ID: 42, Status: "approved", CollegeID: 3}).Error; err != nil {
		t.Fatalf("seed institution user: %v", err)
	}
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)

	if _, err := svc.AcceptInvite(invite.ID, 7); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if notif.Last == nil || notif.Last.EventKey != notification.EventSocialInviteAccepted {
		t.Fatalf("expected %s, got %+v", notification.EventSocialInviteAccepted, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "institution", ID: 42}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}
