package forum

import (
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetAllCommunities() ([]ForumCommunity, error) {
	var communities []ForumCommunity
	err := r.db.Find(&communities).Error
	return communities, err
}

func (r *Repository) GetCommunityByID(id uint) (*ForumCommunity, error) {
	var community ForumCommunity
	err := r.db.First(&community, id).Error
	return &community, err
}

func (r *Repository) GetMemberCount(communityID uint) (int64, error) {
	var count int64
	err := r.db.Model(&ForumCommunityMember{}).Where("community_id = ?", communityID).Count(&count).Error
	return count, err
}

func (r *Repository) GetPostCount(communityID uint) (int64, error) {
	var count int64
	err := r.db.Model(&ForumPost{}).Where("community_id = ?", communityID).Count(&count).Error
	return count, err
}

func (r *Repository) GetMembershipsByUserID(userID uint) ([]ForumCommunityMember, error) {
	var memberships []ForumCommunityMember
	err := r.db.Where("user_id = ?", userID).Find(&memberships).Error
	return memberships, err
}

func (r *Repository) FindMembership(communityID, userID uint) (*ForumCommunityMember, error) {
	var member ForumCommunityMember
	err := r.db.Where("community_id = ? AND user_id = ?", communityID, userID).First(&member).Error
	return &member, err
}

func (r *Repository) CreateMembership(member *ForumCommunityMember) error {
	return r.db.Create(member).Error
}

func (r *Repository) DeleteMembership(member *ForumCommunityMember) error {
	return r.db.Delete(member).Error
}

func (r *Repository) CreateCommunity(community *ForumCommunity) error {
	return r.db.Create(community).Error
}

func (r *Repository) GetAllPosts(category, communityID string, currentUserID uint) ([]ForumPost, error) {
	var posts []ForumPost
	query := r.db.Preload("User").Preload("Community")

	if communityID != "" {
		query = query.Where("community_id = ?", communityID)
	}

	if category == "Saved" {
		query = query.Joins("JOIN forum_saves ON forum_saves.post_id = forum_posts.id").
			Where("forum_saves.user_id = ?", currentUserID)
	} else if category != "" && category != "Home Feed" && category != "Latest" && category != "Trending" {
		query = query.Where("category = ?", category)
	}

	if category == "Trending" {
		query = query.Order("upvotes DESC, created_at DESC")
	} else {
		query = query.Order("created_at DESC")
	}

	err := query.Find(&posts).Error
	return posts, err
}

func (r *Repository) GetPostByID(id uint) (*ForumPost, error) {
	var post ForumPost
	err := r.db.Preload("User").Preload("Community").First(&post, id).Error
	return &post, err
}

func (r *Repository) CreatePost(post *ForumPost) error {
	return r.db.Create(post).Error
}

func (r *Repository) UpdatePost(post *ForumPost) error {
	return r.db.Save(post).Error
}

func (r *Repository) DeletePost(post *ForumPost) error {
	return r.db.Delete(post).Error
}

func (r *Repository) UpdatePostVotes(postID uint, upvotes, downvotes int) error {
	return r.db.Model(&ForumPost{}).Where("id = ?", postID).Updates(map[string]interface{}{
		"upvotes":   upvotes,
		"downvotes": downvotes,
	}).Error
}

func (r *Repository) FindVote(postID, userID uint) (*ForumVote, error) {
	var vote ForumVote
	err := r.db.Where("post_id = ? AND user_id = ?", postID, userID).First(&vote).Error
	return &vote, err
}

func (r *Repository) CreateVote(vote *ForumVote) error {
	return r.db.Create(vote).Error
}

func (r *Repository) UpdateVote(vote *ForumVote) error {
	return r.db.Save(vote).Error
}

func (r *Repository) DeleteVote(vote *ForumVote) error {
	return r.db.Delete(vote).Error
}

func (r *Repository) FindSave(postID, userID uint) (*ForumSave, error) {
	var save ForumSave
	err := r.db.Where("post_id = ? AND user_id = ?", postID, userID).First(&save).Error
	return &save, err
}

func (r *Repository) CreateSave(save *ForumSave) error {
	return r.db.Create(save).Error
}

func (r *Repository) DeleteSave(save *ForumSave) error {
	return r.db.Delete(save).Error
}

func (r *Repository) GetVotesByUserID(userID uint) ([]ForumVote, error) {
	var votes []ForumVote
	err := r.db.Where("user_id = ?", userID).Find(&votes).Error
	return votes, err
}

func (r *Repository) GetSavesByUserID(userID uint) ([]ForumSave, error) {
	var saves []ForumSave
	err := r.db.Where("user_id = ?", userID).Find(&saves).Error
	return saves, err
}

func (r *Repository) GetPollVotesByPostIDs(postIDs []uint) ([]ForumPollVote, error) {
	var votes []ForumPollVote
	err := r.db.Where("post_id IN ?", postIDs).Find(&votes).Error
	return votes, err
}

func (r *Repository) GetPollVoteByPostAndUser(postID, userID uint) (*ForumPollVote, error) {
	var vote ForumPollVote
	err := r.db.Where("post_id = ? AND user_id = ?", postID, userID).First(&vote).Error
	return &vote, err
}

func (r *Repository) CreatePollVote(vote *ForumPollVote) error {
	return r.db.Create(vote).Error
}

func (r *Repository) UpdatePollVote(vote *ForumPollVote) error {
	return r.db.Save(vote).Error
}

func (r *Repository) GetAllPollVotesByPostID(postID uint) ([]ForumPollVote, error) {
	var votes []ForumPollVote
	err := r.db.Where("post_id = ?", postID).Find(&votes).Error
	return votes, err
}

func (r *Repository) GetTopComments(postID uint, limit, offset int) ([]ForumComment, error) {
	var comments []ForumComment
	err := r.db.Preload("User").
		Where("post_id = ? AND parent_id IS NULL", postID).
		Limit(limit).
		Offset(offset).
		Order("created_at ASC").
		Find(&comments).Error
	return comments, err
}

func (r *Repository) GetCommentCount(postID uint) (int64, error) {
	var count int64
	err := r.db.Model(&ForumComment{}).Where("post_id = ? AND parent_id IS NULL", postID).Count(&count).Error
	return count, err
}

func (r *Repository) GetRepliesByParentID(parentID uint) ([]ForumComment, error) {
	var replies []ForumComment
	err := r.db.Preload("User").
		Where("parent_id = ?", parentID).
		Order("created_at ASC").
		Find(&replies).Error
	return replies, err
}

func (r *Repository) CreateComment(comment *ForumComment) error {
	return r.db.Create(comment).Error
}

func (r *Repository) GetCommentByID(id uint) (*ForumComment, error) {
	var comment ForumComment
	err := r.db.Preload("User").First(&comment, id).Error
	return &comment, err
}

func (r *Repository) IncrementCommentCount(postID uint) error {
	return r.db.Model(&ForumPost{}).Where("id = ?", postID).Update("comment_count", gorm.Expr("comment_count + 1")).Error
}
