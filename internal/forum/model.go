package forum

import (
	"time"

	"gorm.io/gorm"
)

type ForumCommunity struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Name        string         `gorm:"unique;not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	Icon        string         `json:"icon"`
	BgColor     string         `json:"bg_color"`
	IsGeneral   bool           `gorm:"default:false" json:"is_general"`
	MemberCount int            `gorm:"-" json:"member_count"`
	IsMember    bool           `gorm:"-" json:"is_member"`
	PostCount   int            `gorm:"-" json:"post_count"`
}

type ForumCommunityMember struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	CommunityID uint      `gorm:"uniqueIndex:idx_community_user_member;not null" json:"community_id"`
	UserID      uint      `gorm:"uniqueIndex:idx_community_user_member;not null" json:"user_id"`
}

type ForumPost struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	UserID       uint           `gorm:"not null" json:"user_id"`
	User         User           `gorm:"foreignKey:UserID" json:"user"`
	CommunityID  uint           `json:"community_id"`
	Community    ForumCommunity `gorm:"foreignKey:CommunityID" json:"community"`
	Category     string         `gorm:"not null" json:"category"`
	Title        string         `gorm:"not null" json:"title"`
	Content      string         `gorm:"type:text;not null" json:"content"`
	ImageURL     string         `json:"image_url"`
	VideoURL     string         `json:"video_url"`
	PollOptions  string         `gorm:"type:text" json:"poll_options"`
	Upvotes      int            `gorm:"default:0" json:"upvotes"`
	Downvotes    int            `gorm:"default:0" json:"downvotes"`
	CommentCount int            `gorm:"default:0" json:"comment_count"`
	IsPoll       bool           `gorm:"default:false" json:"is_poll"`
	IsLiked      bool           `gorm:"-" json:"is_liked"`
	IsDisliked   bool           `gorm:"-" json:"is_disliked"`
	IsSaved      bool           `gorm:"-" json:"is_saved"`
	VotedOption  *int           `gorm:"-" json:"voted_option"`
	PollResults  map[int]int    `gorm:"-" json:"poll_results"`
	TotalVotes   int            `gorm:"-" json:"total_votes"`
}

type ForumComment struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	PostID    uint           `gorm:"not null" json:"post_id"`
	UserID    uint           `gorm:"not null" json:"user_id"`
	User      User           `gorm:"foreignKey:UserID" json:"user"`
	Content   string         `gorm:"type:text;not null" json:"content"`
	ParentID  *uint          `json:"parent_id"`
	Replies   []ForumComment `gorm:"-" json:"replies"`
}

type ForumVote struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	PostID    uint      `gorm:"uniqueIndex:idx_user_post" json:"post_id"`
	UserID    uint      `gorm:"uniqueIndex:idx_user_post" json:"user_id"`
	Vote      int       `gorm:"type:smallint" json:"vote"`
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

type User struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	ImageURL  string `json:"image_url"`
}
