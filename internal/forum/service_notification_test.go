package forum

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"studsphere/backend/internal/notification"
)

type captureNotifier struct {
	Last     *notification.NotifyRequest
	Calls    []notification.NotifyRequest
	Roles    [][]string
	Audience []notification.Ref
}

func (c *captureNotifier) Notify(_ context.Context, req notification.NotifyRequest) error {
	c.Last = &req
	c.Calls = append(c.Calls, req)
	return nil
}

func (c *captureNotifier) NotifyTx(ctx context.Context, _ *gorm.DB, req notification.NotifyRequest) error {
	return c.Notify(ctx, req)
}

func (c *captureNotifier) ForRoles(_ context.Context, roles ...string) ([]notification.Ref, error) {
	c.Roles = append(c.Roles, roles)
	return c.Audience, nil
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

func testDBForum(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&ForumCommunity{}, &ForumPost{}, &ForumComment{}, &ForumReport{}, &ForumNotInterested{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestReportPostNotifiesSuperadmins(t *testing.T) {
	db := testDBForum(t)
	notif := &captureNotifier{Audience: []notification.Ref{{Type: "user", ID: 1}}}
	svc := NewService(NewRepository(db), notif)

	post := ForumPost{UserID: 9, Title: "Suspicious Post", Content: "Body", Category: "General"}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("seed post: %v", err)
	}

	if err := svc.ReportPost(post.ID, 2, ReportPostRequest{Reasons: []string{"Spam"}}); err != nil {
		t.Fatalf("report: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventModerationForumReport {
		t.Fatalf("expected %s, got %+v", notification.EventModerationForumReport, notif.Last)
	}
	if notif.Last.Data["kind"] != "post" || notif.Last.Data["id"] != post.ID || notif.Last.Data["reason"] != "Spam" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	if len(notif.Roles) != 1 || len(notif.Roles[0]) != 2 || notif.Roles[0][0] != "superadmin" || notif.Roles[0][1] != "admin" {
		t.Fatalf("ForRoles called with %v, want [superadmin admin]", notif.Roles)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: 1}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}
