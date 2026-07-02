package forum

type CreateCommunityRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Emoji       string `json:"emoji"`
	BgColor     string `json:"bg_color"`
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

type VotePollRequest struct {
	OptionIdx int `json:"option_idx" binding:"required"`
}

type CommunityResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Emoji       string `json:"emoji"`
	BgColor     string `json:"bg_color"`
	MemberCount int    `json:"member_count"`
	IsMember    bool   `json:"is_member"`
	PostCount   int    `json:"post_count"`
}

type PostResponse struct {
	ID           uint        `json:"id"`
	CreatedAt    string      `json:"created_at"`
	UpdatedAt    string      `json:"updated_at"`
	UserID       uint        `json:"user_id"`
	UserName     string      `json:"user_name"`
	CommunityID  uint        `json:"community_id"`
	Category     string      `json:"category"`
	Title        string      `json:"title"`
	Content      string      `json:"content"`
	ImageURL     string      `json:"image_url"`
	VideoURL     string      `json:"video_url"`
	Upvotes      int         `json:"upvotes"`
	Downvotes    int         `json:"downvotes"`
	CommentCount int         `json:"comment_count"`
	IsPoll       bool        `json:"is_poll"`
	IsLiked      bool        `json:"is_liked"`
	IsDisliked   bool        `json:"is_disliked"`
	IsSaved      bool        `json:"is_saved"`
	VotedOption  *int        `json:"voted_option,omitempty"`
	PollResults  map[int]int `json:"poll_results,omitempty"`
	TotalVotes   int         `json:"total_votes"`
}

type CommentResponse struct {
	ID        uint              `json:"id"`
	CreatedAt string            `json:"created_at"`
	PostID    uint              `json:"post_id"`
	UserID    uint              `json:"user_id"`
	UserName  string            `json:"user_name"`
	Content   string            `json:"content"`
	ParentID  *uint             `json:"parent_id"`
	Replies   []CommentResponse `json:"replies"`
}
