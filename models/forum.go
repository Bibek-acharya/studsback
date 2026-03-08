package models

import (
	"time"

	"gorm.io/gorm"
)

type ForumPost struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	UserID       uint           `gorm:"not null" json:"user_id"`
	User         User           `json:"user"`
	Category     string         `gorm:"not null" json:"category"`
	Title        string         `gorm:"not null" json:"title"`
	Content      string         `gorm:"type:text;not null" json:"content"`
	Upvotes      int            `gorm:"default:0" json:"upvotes"`
	Downvotes    int            `gorm:"default:0" json:"downvotes"`
	CommentCount int            `gorm:"default:0" json:"comment_count"`
	IsPoll       bool           `gorm:"default:false" json:"is_poll"`
	IsLiked      bool           `gorm:"-" json:"is_liked"`
	IsDisliked   bool           `gorm:"-" json:"is_disliked"`
	IsSaved      bool           `gorm:"-" json:"is_saved"`
}

type ForumComment struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	PostID    uint           `gorm:"not null" json:"post_id"`
	UserID    uint           `gorm:"not null" json:"user_id"`
	User      User           `json:"user"`
	Content   string         `gorm:"type:text;not null" json:"content"`
}

type ForumVote struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	PostID    uint      `gorm:"uniqueIndex:idx_user_post" json:"post_id"`
	UserID    uint      `gorm:"uniqueIndex:idx_user_post" json:"user_id"`
	Vote      int       `gorm:"type:smallint" json:"vote"` // 1 for like, -1 for dislike
}

type ForumSave struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	PostID    uint      `gorm:"uniqueIndex:idx_user_post_save" json:"post_id"`
	UserID    uint      `gorm:"uniqueIndex:idx_user_post_save" json:"user_id"`
}

type CreatePostRequest struct {
	Category string `json:"category" binding:"required"`
	Title    string `json:"title" binding:"required"`
	Content  string `json:"content" binding:"required"`
}

type UpdatePostRequest struct {
	Title   *string `json:"title"`
	Content *string `json:"content"`
}

type CreateCommentRequest struct {
	Content string `json:"content" binding:"required"`
}
