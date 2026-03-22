package handlers

import (
	"encoding/json"
	"strconv"
	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/utils"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetForumPosts returns a list of all forum posts
func GetForumPosts(c *gin.Context) {
	var posts []models.ForumPost
	query := config.GetDB().Preload("User").Preload("Community")

	// Filter by category or community if provided
	category := c.Query("category")
	communityID := c.Query("community_id")

	if communityID != "" {
		query = query.Where("community_id = ?", communityID)
	}

	// Handle authentication for "Saved" category and virtual fields
	authHeader := c.GetHeader("Authorization")
	var currentUserID uint
	if authHeader != "" {
		tokenString := ""
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		}

		if tokenString != "" {
			claims, err := utils.ValidateToken(tokenString)
			if err == nil {
				currentUserID = claims.UserID
			}
		}
	}

	if category == "Saved" {
		if currentUserID == 0 {
			utils.ErrorResponse(c, 401, "Authentication required to view saved posts")
			return
		}
		query = query.Joins("JOIN forum_saves ON forum_saves.post_id = forum_posts.id").Where("forum_saves.user_id = ?", currentUserID)
	} else if category != "" && category != "Home Feed" && category != "Latest" && category != "Trending" {
		query = query.Where("category = ?", category)
	}

	if category == "Trending" {
		// Trending: Most upvotes (vote=1) in last 24 hours
		last24h := time.Now().Add(-24 * time.Hour)
		query = query.Model(&models.ForumPost{}).
			Select("forum_posts.*, (SELECT COUNT(id) FROM forum_votes WHERE forum_votes.post_id = forum_posts.id AND forum_votes.vote = 1 AND forum_votes.created_at > ?) as trending_votes", last24h).
			Order("trending_votes DESC, forum_posts.upvotes DESC, forum_posts.created_at DESC")
	} else {
		query = query.Order("created_at desc")
	}

	if err := query.Find(&posts).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch posts")
		return
	}

	// Populate virtual fields if user is authenticated
	if currentUserID != 0 {
		var votes []models.ForumVote
		config.GetDB().Where("user_id = ?", currentUserID).Find(&votes)

		voteMap := make(map[uint]int)
		for _, v := range votes {
			voteMap[v.PostID] = v.Vote
		}

		var saves []models.ForumSave
		config.GetDB().Where("user_id = ?", currentUserID).Find(&saves)
		saveMap := make(map[uint]bool)
		for _, s := range saves {
			saveMap[s.PostID] = true
		}

		for i := range posts {
			vote := voteMap[posts[i].ID]
			posts[i].IsLiked = vote == 1
			posts[i].IsDisliked = vote == -1
			posts[i].IsSaved = saveMap[posts[i].ID]
		}
	}

	// Populate poll results
	pollPostIDs := []uint{}
	for _, p := range posts {
		if p.IsPoll {
			pollPostIDs = append(pollPostIDs, p.ID)
		}
	}

	if len(pollPostIDs) > 0 {
		var allPollVotes []models.ForumPollVote
		config.GetDB().Where("post_id IN ?", pollPostIDs).Find(&allPollVotes)
		
		pollMap := make(map[uint]map[int]int)
		pollTotalMap := make(map[uint]int)
		for _, v := range allPollVotes {
			if _, ok := pollMap[v.PostID]; !ok {
				pollMap[v.PostID] = make(map[int]int)
			}
			pollMap[v.PostID][v.OptionIdx]++
			pollTotalMap[v.PostID]++
		}
		
		myPollVoteMap := make(map[uint]int)
		hasVotedMap := make(map[uint]bool)
		if currentUserID != 0 {
			var myPollVotes []models.ForumPollVote
			config.GetDB().Where("post_id IN ? AND user_id = ?", pollPostIDs, currentUserID).Find(&myPollVotes)
			for _, v := range myPollVotes {
				myPollVoteMap[v.PostID] = v.OptionIdx
				hasVotedMap[v.PostID] = true
			}
		}
		
		for i := range posts {
			id := posts[i].ID
			if posts[i].IsPoll {
				posts[i].PollResults = pollMap[id]
				posts[i].TotalVotes = pollTotalMap[id]
				if hasVotedMap[id] {
					opt := myPollVoteMap[id]
					posts[i].VotedOption = &opt
				}
			}
		}
	}

	utils.SuccessResponse(c, 200, "Posts retrieved successfully", posts)
}

// CreateForumPost handles the creation of a new forum post
func CreateForumPost(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req models.CreatePostRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	pollOptionsJSON, _ := json.Marshal(req.PollOptions)

	post := models.ForumPost{
		UserID:      userID.(uint),
		CommunityID: req.CommunityID,
		Category:    req.Category,
		Title:       req.Title,
		Content:     req.Content,
		ImageURL:    req.ImageURL,
		VideoURL:    req.VideoURL,
		PollOptions: string(pollOptionsJSON),
		IsPoll:      req.IsPoll,
	}

	if err := config.GetDB().Create(&post).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to create post")
		return
	}

	// Preload user and community for the response
	config.GetDB().Preload("User").Preload("Community").First(&post, post.ID)

	utils.SuccessResponse(c, 201, "Post created successfully", post)
}

// GetForumCommunities returns a list of available forum communities
func GetForumCommunities(c *gin.Context) {
	var communities []models.ForumCommunity
	if err := config.GetDB().Find(&communities).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch communities")
		return
	}
	utils.SuccessResponse(c, 200, "Communities retrieved successfully", communities)
}

// UpdateForumPost updates an existing post
func UpdateForumPost(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")
	var req models.UpdatePostRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	var post models.ForumPost
	if err := config.GetDB().First(&post, id).Error; err != nil {
		utils.ErrorResponse(c, 404, "Post not found")
		return
	}

	if post.UserID != userID.(uint) {
		utils.ErrorResponse(c, 403, "You can only edit your own posts")
		return
	}

	if req.Title != nil {
		post.Title = *req.Title
	}
	if req.Content != nil {
		post.Content = *req.Content
	}

	if err := config.GetDB().Save(&post).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to update post")
		return
	}

	config.GetDB().Preload("User").First(&post, post.ID)
	utils.SuccessResponse(c, 200, "Post updated successfully", post)
}

// DeleteForumPost deletes a post
func DeleteForumPost(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")

	var post models.ForumPost
	if err := config.GetDB().First(&post, id).Error; err != nil {
		utils.ErrorResponse(c, 404, "Post not found")
		return
	}

	if post.UserID != userID.(uint) {
		utils.ErrorResponse(c, 403, "You can only delete your own posts")
		return
	}

	if err := config.GetDB().Delete(&post).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to delete post")
		return
	}

	utils.SuccessResponse(c, 200, "Post deleted successfully", nil)
}

// LikeForumPost handles liking and unliking of posts
func LikeForumPost(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")

	var post models.ForumPost
	if err := config.GetDB().First(&post, id).Error; err != nil {
		utils.ErrorResponse(c, 404, "Post not found")
		return
	}

	var existingVote models.ForumVote
	err := config.GetDB().Where("post_id = ? AND user_id = ?", post.ID, userID).First(&existingVote).Error

	if err == nil {
		// Already voted
		if existingVote.Vote == 1 {
			// Was liked, so toggle off
			config.GetDB().Delete(&existingVote)
			if post.Upvotes > 0 {
				post.Upvotes--
			}
			post.IsLiked = false
		} else {
			// Was disliked, change to liked
			existingVote.Vote = 1
			config.GetDB().Save(&existingVote)
			post.Upvotes++
			if post.Downvotes > 0 {
				post.Downvotes--
			}
			post.IsLiked = true
			post.IsDisliked = false
		}
	} else {
		// New vote
		vote := models.ForumVote{
			PostID: post.ID,
			UserID: userID.(uint),
			Vote:   1,
		}
		config.GetDB().Create(&vote)
		post.Upvotes++
		post.IsLiked = true
	}

	if err := config.GetDB().Model(&post).Updates(map[string]interface{}{
		"upvotes":   post.Upvotes,
		"downvotes": post.Downvotes,
	}).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to update post votes")
		return
	}

	utils.SuccessResponse(c, 200, "Post liked successfully", post)
}

// DislikeForumPost handles disliking and undisliking of posts
func DislikeForumPost(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")

	var post models.ForumPost
	if err := config.GetDB().First(&post, id).Error; err != nil {
		utils.ErrorResponse(c, 404, "Post not found")
		return
	}

	var existingVote models.ForumVote
	err := config.GetDB().Where("post_id = ? AND user_id = ?", post.ID, userID).First(&existingVote).Error

	if err == nil {
		// Already voted
		if existingVote.Vote == -1 {
			// Was disliked, so toggle off
			config.GetDB().Delete(&existingVote)
			if post.Downvotes > 0 {
				post.Downvotes--
			}
			post.IsDisliked = false
		} else {
			// Was liked, change to disliked
			existingVote.Vote = -1
			config.GetDB().Save(&existingVote)
			post.Downvotes++
			if post.Upvotes > 0 {
				post.Upvotes--
			}
			post.IsLiked = false
			post.IsDisliked = true
		}
	} else {
		// New vote
		vote := models.ForumVote{
			PostID: post.ID,
			UserID: userID.(uint),
			Vote:   -1,
		}
		config.GetDB().Create(&vote)
		post.Downvotes++
		post.IsDisliked = true
	}

	if err := config.GetDB().Model(&post).Updates(map[string]interface{}{
		"upvotes":   post.Upvotes,
		"downvotes": post.Downvotes,
	}).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to update post votes")
		return
	}

	utils.SuccessResponse(c, 200, "Post disliked successfully", post)
}

// SaveForumPost handles saving and unsaving of posts
func SaveForumPost(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")

	var post models.ForumPost
	if err := config.GetDB().First(&post, id).Error; err != nil {
		utils.ErrorResponse(c, 404, "Post not found")
		return
	}

	var existingSave models.ForumSave
	err := config.GetDB().Where("post_id = ? AND user_id = ?", post.ID, userID).First(&existingSave).Error

	if err == nil {
		// Already saved, so unsave it (toggle)
		config.GetDB().Delete(&existingSave)
		post.IsSaved = false
	} else {
		// Not saved yet, create save
		save := models.ForumSave{
			PostID: post.ID,
			UserID: userID.(uint),
		}
		config.GetDB().Create(&save)
		post.IsSaved = true
	}

	utils.SuccessResponse(c, 200, "Post save status updated", post)
}

// GetForumPostComments returns a list of comments for a given post
func GetForumPostComments(c *gin.Context) {
	postID := c.Param("id")
	limit := c.DefaultQuery("limit", "10")
	offset := c.DefaultQuery("offset", "0")

	// Get total count of comments for the post
	var totalCount int64
	config.GetDB().Model(&models.ForumComment{}).Where("post_id = ?", postID).Count(&totalCount)

	limitInt, _ := strconv.Atoi(limit)
	offsetInt, _ := strconv.Atoi(offset)

	var comments []models.ForumComment
	if err := config.GetDB().Preload("User").
		Where("post_id = ?", postID).
		Limit(limitInt).
		Offset(offsetInt).
		Order("created_at desc").
		Find(&comments).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch comments")
		return
	}

	response := gin.H{
		"comments":    comments,
		"total_count": totalCount,
	}

	utils.SuccessResponse(c, 200, "Comments retrieved successfully", response)
}

// CreateForumComment handles the creation of a new comment
func CreateForumComment(c *gin.Context) {
	userID, _ := c.Get("user_id")
	postID := c.Param("id")
	var req models.CreateCommentRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	postIDInt, _ := strconv.Atoi(postID)
	comment := models.ForumComment{
		PostID:   uint(postIDInt),
		UserID:   userID.(uint),
		Content:  req.Content,
		ParentID: req.ParentID,
	}

	if err := config.GetDB().Create(&comment).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to create comment")
		return
	}

	// Update post comment count
	config.GetDB().Model(&models.ForumPost{}).Where("id = ?", postID).Update("comment_count", gorm.Expr("comment_count + 1"))

	// Preload user for the response
	config.GetDB().Preload("User").First(&comment, comment.ID)

	utils.SuccessResponse(c, 201, "Comment added successfully", comment)
}

// VoteForumPoll handles voting on a poll
func VoteForumPoll(c *gin.Context) {
	userID, _ := c.Get("user_id")
	postID := c.Param("id")

	var req struct {
		OptionIdx int `json:"option_idx"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, "Invalid option index")
		return
	}

	var post models.ForumPost
	if err := config.GetDB().First(&post, postID).Error; err != nil {
		utils.ErrorResponse(c, 404, "Post not found")
		return
	}

	if !post.IsPoll {
		utils.ErrorResponse(c, 400, "Post is not a poll")
		return
	}

	// Check if user already voted and update if so, or create new
	var existingVote models.ForumPollVote
	err := config.GetDB().Where("post_id = ? AND user_id = ?", post.ID, userID).First(&existingVote).Error
	if err == nil {
		existingVote.OptionIdx = req.OptionIdx
		config.GetDB().Save(&existingVote)
	} else {
		vote := models.ForumPollVote{
			PostID:    post.ID,
			UserID:    userID.(uint),
			OptionIdx: req.OptionIdx,
		}
		config.GetDB().Create(&vote)
	}

	// Return updated post results
	var allPollVotes []models.ForumPollVote
	config.GetDB().Where("post_id = ?", post.ID).Find(&allPollVotes)

	results := make(map[int]int)
	total := 0
	for _, v := range allPollVotes {
		results[v.OptionIdx]++
		total++
	}

	opt := req.OptionIdx
	post.PollResults = results
	post.TotalVotes = total
	post.VotedOption = &opt

	utils.SuccessResponse(c, 200, "Vote cast successfully", post)
}
