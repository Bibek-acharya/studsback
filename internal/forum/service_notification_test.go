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

func seedUsers(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, u := range []User{
		{ID: 1, Email: "author@example.com", FirstName: "Post", LastName: "Author"},
		{ID: 2, Email: "commenter@example.com", FirstName: "Bob", LastName: "Reply"},
		{ID: 3, Email: "parent@example.com", FirstName: "Pat", LastName: "Parent"},
	} {
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("seed user %d: %v", u.ID, err)
		}
	}
}

func TestCommentOnPostNotifiesPostAuthor(t *testing.T) {
	db := testDBForum(t)
	seedUsers(t, db)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)
	post := ForumPost{UserID: 1, Title: "T", Content: "C", Category: "General"}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("seed post: %v", err)
	}

	if _, err := svc.CreateForumComment(post.ID, 2, CreateCommentRequest{Content: "Nice"}); err != nil {
		t.Fatalf("comment: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventSocialForumReply {
		t.Fatalf("expected %s, got %+v", notification.EventSocialForumReply, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: 1}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	if notif.Last.Data["name"] != "Bob Reply" || notif.Last.Data["post_id"] != post.ID {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	assertTemplates(t, *notif.Last)
}

func TestReplyToCommentNotifiesParentAuthor(t *testing.T) {
	db := testDBForum(t)
	seedUsers(t, db)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)
	post := ForumPost{UserID: 1, Title: "T", Content: "C", Category: "General"}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("seed post: %v", err)
	}
	parent := &ForumComment{PostID: post.ID, UserID: 3, Content: "Parent"}
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("seed parent: %v", err)
	}

	if _, err := svc.CreateForumComment(post.ID, 2, CreateCommentRequest{Content: "Reply", ParentID: &parent.ID}); err != nil {
		t.Fatalf("reply: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventSocialForumReply {
		t.Fatalf("expected %s, got %+v", notification.EventSocialForumReply, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: 3}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}

func TestSelfCommentNoNotification(t *testing.T) {
	db := testDBForum(t)
	seedUsers(t, db)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)
	post := ForumPost{UserID: 1, Title: "T", Content: "C", Category: "General"}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("seed post: %v", err)
	}

	// Author comments on their own post — never notifies.
	if _, err := svc.CreateForumComment(post.ID, 1, CreateCommentRequest{Content: "Self"}); err != nil {
		t.Fatalf("comment: %v", err)
	}
	if len(notif.Calls) != 0 {
		t.Fatalf("expected no emissions, got %+v", notif.Calls)
	}
}

func TestSelfReplyNoNotification(t *testing.T) {
	db := testDBForum(t)
	seedUsers(t, db)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)
	post := ForumPost{UserID: 1, Title: "T", Content: "C", Category: "General"}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("seed post: %v", err)
	}
	parent := &ForumComment{PostID: post.ID, UserID: 2, Content: "Parent"}
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("seed parent: %v", err)
	}

	// Replying to your own comment — never notifies.
	if _, err := svc.CreateForumComment(post.ID, 2, CreateCommentRequest{Content: "Self reply", ParentID: &parent.ID}); err != nil {
		t.Fatalf("reply: %v", err)
	}
	if len(notif.Calls) != 0 {
		t.Fatalf("expected no emissions, got %+v", notif.Calls)
	}
}

func TestAdminDeletePostNotifiesOwner(t *testing.T) {
	db := testDBForum(t)
	seedUsers(t, db)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)
	post := ForumPost{UserID: 1, Title: "T", Content: "C", Category: "General"}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("seed post: %v", err)
	}

	if err := svc.AdminDeleteForumPost(post.ID); err != nil {
		t.Fatalf("admin delete: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventSocialForumModerated {
		t.Fatalf("expected %s, got %+v", notification.EventSocialForumModerated, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: 1}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}

func TestAdminDeleteCommentNotifiesOwner(t *testing.T) {
	db := testDBForum(t)
	seedUsers(t, db)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)
	post := ForumPost{UserID: 1, Title: "T", Content: "C", Category: "General"}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("seed post: %v", err)
	}
	comment := &ForumComment{PostID: post.ID, UserID: 2, Content: "Target"}
	if err := db.Create(comment).Error; err != nil {
		t.Fatalf("seed comment: %v", err)
	}

	if _, err := svc.AdminDeleteForumComment(comment.ID); err != nil {
		t.Fatalf("admin delete comment: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventSocialForumModerated {
		t.Fatalf("expected %s, got %+v", notification.EventSocialForumModerated, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: 2}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}

func TestReportPostAppendsOtherText(t *testing.T) {
	db := testDBForum(t)
	seedUsers(t, db)
	notif := &captureNotifier{Audience: []notification.Ref{{Type: "user", ID: 1}}}
	svc := NewService(NewRepository(db), notif)
	post := ForumPost{UserID: 1, Title: "T", Content: "C", Category: "General"}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("seed post: %v", err)
	}

	if err := svc.ReportPost(post.ID, 2, ReportPostRequest{Reasons: []string{"Spam"}, OtherText: "custom detail"}); err != nil {
		t.Fatalf("report: %v", err)
	}
	if notif.Last == nil || notif.Last.Data["reason"] != "Spam; custom detail" {
		t.Fatalf("reason wrong: %+v", notif.Last)
	}
	assertTemplates(t, *notif.Last)
}

func TestReplyNameFallbackToSomeone(t *testing.T) {
	db := testDBForum(t)
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("auto migrate users: %v", err)
	}
	// Author + a commenter whose names are entirely blank.
	for _, u := range []User{{ID: 1, Email: "author@example.com"}, {ID: 2, Email: "anon@example.com"}} {
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("seed user %d: %v", u.ID, err)
		}
	}
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)
	post := ForumPost{UserID: 1, Title: "T", Content: "C", Category: "General"}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("seed post: %v", err)
	}

	if _, err := svc.CreateForumComment(post.ID, 2, CreateCommentRequest{Content: "Nice"}); err != nil {
		t.Fatalf("comment: %v", err)
	}
	if notif.Last == nil {
		t.Fatal("no emission")
	}
	if notif.Last.Data["name"] != "Someone" {
		t.Fatalf("name fallback wrong: %+v", notif.Last.Data)
	}
	assertTemplates(t, *notif.Last)
}
