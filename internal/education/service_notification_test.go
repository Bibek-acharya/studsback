package education

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
	Roles [][]string
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
	return []notification.Ref{{Type: "user", ID: 1}}, nil
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

func testDBEducation(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&Blog{}, &BlogComment{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestCreateBlogCommentNotifiesAdmins(t *testing.T) {
	db := testDBEducation(t)
	blog := Blog{Title: "Studying Abroad", Slug: "studying-abroad", Published: true}
	if err := db.Create(&blog).Error; err != nil {
		t.Fatalf("seed blog: %v", err)
	}
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), &testInstProgramRepo{db: db}, nil, notif)

	if _, err := svc.CreateBlogComment(BlogCommentInput{BlogID: blog.ID, Author: "Ada", Message: "Great read"}); err != nil {
		t.Fatalf("comment: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventSystemInquiryReceived {
		t.Fatalf("expected %s, got %+v", notification.EventSystemInquiryReceived, notif.Last)
	}
	if len(notif.Roles) != 1 || len(notif.Roles[0]) != 2 || notif.Roles[0][0] != "superadmin" || notif.Roles[0][1] != "admin" {
		t.Fatalf("ForRoles called with %v, want [superadmin admin]", notif.Roles)
	}
	if notif.Last.Data["name"] != "Ada" || notif.Last.Data["subject"] != "Studying Abroad" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	if _, ok := notif.Last.Data["email"]; !ok {
		t.Fatalf("email key missing from data: %+v", notif.Last.Data)
	}
	assertTemplates(t, *notif.Last)
}
