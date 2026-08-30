package forum

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newForumModerationTestRepository(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&ForumCommunity{}, &ForumPost{}, &ForumComment{}, &ForumReport{}, &ForumNotInterested{}); err != nil {
		t.Fatalf("migrate forum models: %v", err)
	}

	return NewRepository(db), db
}

func TestDeleteCommentTreeDeletesAllDescendants(t *testing.T) {
	repo, db := newForumModerationTestRepository(t)
	post := ForumPost{Title: "Post", Content: "Body", Category: "General", CommentCount: 5}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}

	root := ForumComment{PostID: post.ID, UserID: 1, Content: "root"}
	if err := db.Create(&root).Error; err != nil {
		t.Fatalf("create root: %v", err)
	}
	child := ForumComment{PostID: post.ID, UserID: 2, Content: "child", ParentID: &root.ID}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create child: %v", err)
	}
	grandchild := ForumComment{PostID: post.ID, UserID: 3, Content: "grandchild", ParentID: &child.ID}
	if err := db.Create(&grandchild).Error; err != nil {
		t.Fatalf("create grandchild: %v", err)
	}
	greatGrandchild := ForumComment{PostID: post.ID, UserID: 4, Content: "great-grandchild", ParentID: &grandchild.ID}
	if err := db.Create(&greatGrandchild).Error; err != nil {
		t.Fatalf("create great-grandchild: %v", err)
	}
	unrelated := ForumComment{PostID: post.ID, UserID: 5, Content: "unrelated"}
	if err := db.Create(&unrelated).Error; err != nil {
		t.Fatalf("create unrelated comment: %v", err)
	}

	postID, deleted, err := repo.DeleteCommentTree(child.ID)
	if err != nil {
		t.Fatalf("delete comment tree: %v", err)
	}
	if postID != post.ID {
		t.Fatalf("post ID = %d, want %d", postID, post.ID)
	}
	if deleted != 3 {
		t.Fatalf("deleted comments = %d, want 3", deleted)
	}

	var remaining []ForumComment
	if err := db.Order("id ASC").Find(&remaining).Error; err != nil {
		t.Fatalf("load remaining comments: %v", err)
	}
	if len(remaining) != 2 || remaining[0].ID != root.ID || remaining[1].ID != unrelated.ID {
		t.Fatalf("remaining comment IDs = %v, want [%d %d]", commentIDs(remaining), root.ID, unrelated.ID)
	}

	var updatedPost ForumPost
	if err := db.First(&updatedPost, post.ID).Error; err != nil {
		t.Fatalf("load updated post: %v", err)
	}
	if updatedPost.CommentCount != 2 {
		t.Fatalf("comment count = %d, want 2", updatedPost.CommentCount)
	}
}

func TestCreateNotInterestedIsIdempotent(t *testing.T) {
	repo, db := newForumModerationTestRepository(t)
	preference := &ForumNotInterested{PostID: 7, UserID: 11}

	if err := repo.CreateNotInterested(preference); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if err := repo.CreateNotInterested(&ForumNotInterested{PostID: 7, UserID: 11}); err != nil {
		t.Fatalf("second create: %v", err)
	}

	var count int64
	if err := db.Model(&ForumNotInterested{}).Where("post_id = ? AND user_id = ?", 7, 11).Count(&count).Error; err != nil {
		t.Fatalf("count preferences: %v", err)
	}
	if count != 1 {
		t.Fatalf("preference count = %d, want 1", count)
	}
}

func TestCreateReportUpdatesExistingUserReport(t *testing.T) {
	repo, db := newForumModerationTestRepository(t)

	if err := repo.CreateReport(&ForumReport{PostID: 7, UserID: 11, Reasons: "Spam"}); err != nil {
		t.Fatalf("first report: %v", err)
	}
	if err := repo.CreateReport(&ForumReport{PostID: 7, UserID: 11, Reasons: "Privacy", OtherText: "Updated details"}); err != nil {
		t.Fatalf("updated report: %v", err)
	}

	var reports []ForumReport
	if err := db.Where("post_id = ? AND user_id = ?", 7, 11).Find(&reports).Error; err != nil {
		t.Fatalf("load reports: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("report count = %d, want 1", len(reports))
	}
	if reports[0].Reasons != "Privacy" || reports[0].OtherText != "Updated details" {
		t.Fatalf("report was not updated: %#v", reports[0])
	}
}

func TestBuildCommentTreePreservesNestedReplies(t *testing.T) {
	rootID := uint(1)
	childID := uint(2)
	comments := []ForumComment{
		{ID: rootID, User: User{ID: 1, FirstName: "Root"}, Content: "root"},
		{ID: childID, ParentID: &rootID, User: User{ID: 2, FirstName: "Child"}, Content: "child"},
		{ID: 3, ParentID: &childID, User: User{ID: 3, FirstName: "Grandchild"}, Content: "grandchild"},
	}

	tree := buildCommentTree(comments)
	if len(tree) != 1 || len(tree[0].Replies) != 1 || len(tree[0].Replies[0].Replies) != 1 {
		t.Fatalf("unexpected comment tree: %#v", tree)
	}
	if tree[0].Replies[0].ParentUserName != "Root" {
		t.Fatalf("child parent user = %q, want Root", tree[0].Replies[0].ParentUserName)
	}
	if tree[0].Replies[0].Replies[0].ParentUserName != "Child" {
		t.Fatalf("grandchild parent user = %q, want Child", tree[0].Replies[0].Replies[0].ParentUserName)
	}
}

func commentIDs(comments []ForumComment) []uint {
	ids := make([]uint, len(comments))
	for i, comment := range comments {
		ids[i] = comment.ID
	}
	return ids
}
