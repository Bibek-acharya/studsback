package models

import (
	"time"

	"gorm.io/gorm"
)

type ForumCommunity struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Name      string         `gorm:"unique;not null" json:"name"`
	Emoji     string         `json:"emoji"`
	BgColor   string         `json:"bg_color"`
}

type ForumPost struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	UserID       uint           `gorm:"not null" json:"user_id"`
	User         User           `json:"user"`
	CommunityID  uint           `json:"community_id"`
	Community    ForumCommunity `json:"community"`
	Category     string         `gorm:"not null" json:"category"`
	Title        string         `gorm:"not null" json:"title"`
	Content      string         `gorm:"type:text;not null" json:"content"`
	ImageURL     string         `json:"image_url"`
	VideoURL     string         `json:"video_url"`
	PollOptions  string         `gorm:"type:text" json:"poll_options"` // JSON string of options
	Upvotes      int            `gorm:"default:0" json:"upvotes"`
	Downvotes    int            `gorm:"default:0" json:"downvotes"`
	CommentCount int            `gorm:"default:0" json:"comment_count"`
	IsPoll       bool             `gorm:"default:false" json:"is_poll"`
	IsLiked      bool             `gorm:"-" json:"is_liked"`
	IsDisliked   bool             `gorm:"-" json:"is_disliked"`
	IsSaved      bool             `gorm:"-" json:"is_saved"`
	VotedOption  *int             `gorm:"-" json:"voted_option"`
	PollResults  map[int]int      `gorm:"-" json:"poll_results"`
	TotalVotes   int              `gorm:"-" json:"total_votes"`
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
	ParentID  *uint          `json:"parent_id"`
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

type ForumPollVote struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	PostID    uint      `gorm:"uniqueIndex:idx_user_post_poll" json:"post_id"`
	UserID    uint      `gorm:"uniqueIndex:idx_user_post_poll" json:"user_id"`
	OptionIdx int       `json:"option_idx"`
}

type CreatePostRequest struct {
	CommunityID uint     `json:"community_id"`
	Category    string   `json:"category" binding:"required"`
	Title       string   `json:"title" binding:"required"`
	Content     string   `json:"content" binding:"required"`
	ImageURL    string   `json:"image_url"`
	VideoURL    string   `json:"video_url"`
	PollOptions []string `json:"poll_options"`
	IsPoll      bool     `json:"is_poll"`
}

type UpdatePostRequest struct {
	Title   *string `json:"title"`
	Content *string `json:"content"`
}

type CreateCommentRequest struct {
	Content  string `json:"content" binding:"required"`
	ParentID *uint  `json:"parent_id"`
}
