package forum

import (
	"strings"

	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetForumCommunities(c *gin.Context) {
	currentUserID := getUserID(c)

	communities, err := h.service.GetForumCommunities(currentUserID)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Communities retrieved successfully", communities)
}

func (h *Handler) CreateForumCommunity(c *gin.Context) {
	var req CreateCommunityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	community, err := h.service.CreateForumCommunity(req)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 201, "Community created successfully", community)
}

func (h *Handler) JoinForumCommunity(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, 401, "Authentication required")
		return
	}

	communityID, err := ParseUint(c.Param("id"))
	if err != nil || communityID <= 0 {
		response.Error(c, 400, "Invalid community ID")
		return
	}

	community, err := h.service.JoinForumCommunity(communityID, userID.(uint))
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "Community membership updated", community)
}

func (h *Handler) UpdateForumCommunity(c *gin.Context) {
	communityID, err := ParseUint(c.Param("id"))
	if err != nil || communityID <= 0 {
		response.Error(c, 400, "Invalid community ID")
		return
	}

	var req UpdateCommunityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	community, err := h.service.UpdateForumCommunity(communityID, req)
	if err != nil {
		if err.Error() == "community not found" {
			response.Error(c, 404, err.Error())
		} else {
			response.Error(c, 500, err.Error())
		}
		return
	}

	response.Success(c, 200, "Community updated successfully", community)
}

func (h *Handler) DeleteForumCommunity(c *gin.Context) {
	communityID, err := ParseUint(c.Param("id"))
	if err != nil || communityID <= 0 {
		response.Error(c, 400, "Invalid community ID")
		return
	}

	if err := h.service.DeleteForumCommunity(communityID); err != nil {
		if err.Error() == "community not found" {
			response.Error(c, 404, err.Error())
		} else if err.Error() == "the General community cannot be deleted" {
			response.Error(c, 403, err.Error())
		} else {
			response.Error(c, 500, err.Error())
		}
		return
	}

	response.Success(c, 200, "Community deleted successfully", nil)
}

func (h *Handler) GetForumPosts(c *gin.Context) {
	category := c.Query("category")
	communityID := c.Query("community_id")
	currentUserID := getUserID(c)

	if category == "Saved" && currentUserID == 0 {
		response.Error(c, 401, "Authentication required to view saved posts")
		return
	}

	posts, err := h.service.GetForumPosts(category, communityID, currentUserID)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Posts retrieved successfully", posts)
}

func (h *Handler) GetTrendingForumPosts(c *gin.Context) {
	posts, err := h.service.GetTrendingForumPosts()
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Trending posts retrieved successfully", posts)
}

func (h *Handler) CreateForumPost(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, 401, "Authentication required")
		return
	}

	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	post, err := h.service.CreateForumPost(req, userID.(uint))
	if err != nil {
		msg := err.Error()
		switch msg {
		case "you must join this community before posting":
			response.Error(c, 403, msg)
		case "community not found", "General community not found":
			response.Error(c, 404, msg)
		default:
			response.Error(c, 500, msg)
		}
		return
	}

	response.Success(c, 201, "Post created successfully", post)
}

func (h *Handler) UpdateForumPost(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, 401, "Authentication required")
		return
	}

	postID, err := ParseUint(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid post ID")
		return
	}

	var req UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	post, err := h.service.UpdateForumPost(postID, userID.(uint), req)
	if err != nil {
		if err.Error() == "post not found" {
			response.Error(c, 404, err.Error())
		} else {
			response.Error(c, 403, err.Error())
		}
		return
	}

	response.Success(c, 200, "Post updated successfully", post)
}

func (h *Handler) DeleteForumPost(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, 401, "Authentication required")
		return
	}

	postID, err := ParseUint(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid post ID")
		return
	}

	if err := h.service.DeleteForumPost(postID, userID.(uint)); err != nil {
		if err.Error() == "post not found" {
			response.Error(c, 404, err.Error())
		} else {
			response.Error(c, 403, err.Error())
		}
		return
	}

	response.Success(c, 200, "Post deleted successfully", nil)
}

func (h *Handler) AdminDeleteForumPost(c *gin.Context) {
	postID, err := ParseUint(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid post ID")
		return
	}

	if err := h.service.AdminDeleteForumPost(postID); err != nil {
		if err.Error() == "post not found" {
			response.Error(c, 404, err.Error())
		} else {
			response.Error(c, 500, err.Error())
		}
		return
	}

	response.Success(c, 200, "Post deleted successfully", nil)
}

func (h *Handler) LikeForumPost(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, 401, "Authentication required")
		return
	}

	postID, err := ParseUint(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid post ID")
		return
	}

	post, err := h.service.LikeForumPost(postID, userID.(uint))
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "Post liked successfully", post)
}

func (h *Handler) DislikeForumPost(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, 401, "Authentication required")
		return
	}

	postID, err := ParseUint(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid post ID")
		return
	}

	post, err := h.service.DislikeForumPost(postID, userID.(uint))
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "Post disliked successfully", post)
}

func (h *Handler) SaveForumPost(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, 401, "Authentication required")
		return
	}

	postID, err := ParseUint(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid post ID")
		return
	}

	post, err := h.service.SaveForumPost(postID, userID.(uint))
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "Post save status updated", post)
}

func (h *Handler) GetForumPostComments(c *gin.Context) {
	postID, err := ParseUint(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid post ID")
		return
	}

	limit := c.DefaultQuery("limit", "10")
	offset := c.DefaultQuery("offset", "0")

	limitInt := 10
	offsetInt := 0

	if l, err := ParseUint(limit); err == nil {
		limitInt = int(l)
	}
	if o, err := ParseUint(offset); err == nil {
		offsetInt = int(o)
	}

	result, err := h.service.GetForumPostComments(postID, limitInt, offsetInt)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Comments retrieved successfully", result)
}

func (h *Handler) CreateForumComment(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, 401, "Authentication required")
		return
	}

	postID, err := ParseUint(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid post ID")
		return
	}

	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	comment, err := h.service.CreateForumComment(postID, userID.(uint), req)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 201, "Comment added successfully", comment)
}

func (h *Handler) VoteForumPoll(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, 401, "Authentication required")
		return
	}

	postID, err := ParseUint(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid post ID")
		return
	}

	var req VotePollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "Invalid option index")
		return
	}

	post, err := h.service.VoteForumPoll(postID, userID.(uint), req.OptionIdx)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, 200, "Vote cast successfully", post)
}

func (h *Handler) UploadForumMedia(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		response.Error(c, 400, "Failed to parse multipart form")
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		response.Error(c, 400, "No files provided")
		return
	}

	urls, err := h.service.UploadForumMedia(files)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, 200, "Files uploaded successfully", gin.H{"urls": urls})
}

func getUserID(c *gin.Context) uint {
	userID, exists := c.Get("user_id")
	if exists {
		return userID.(uint)
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			if claims, err := utils.ValidateToken(parts[1]); err == nil {
				return claims.UserID
			}
		}
	}

	if cookieToken, err := c.Cookie("token"); err == nil && cookieToken != "" {
		if claims, err := utils.ValidateToken(cookieToken); err == nil {
			return claims.UserID
		}
	}

	return 0
}
