package forum

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		forum := v1.Group("/forum")
		{
			forum.GET("/posts", h.GetForumPosts)
			forum.GET("/posts/trending", h.GetTrendingForumPosts)
			forum.GET("/posts/:id/comments", h.GetForumPostComments)
			forum.GET("/communities", h.GetForumCommunities)

			protectedForum := forum.Group("")
			protectedForum.Use(authMW)
			{
				protectedForum.POST("/posts", h.CreateForumPost)
				protectedForum.POST("/posts/:id/like", h.LikeForumPost)
				protectedForum.POST("/posts/:id/dislike", h.DislikeForumPost)
				protectedForum.POST("/posts/:id/save", h.SaveForumPost)
				protectedForum.PUT("/posts/:id", h.UpdateForumPost)
				protectedForum.DELETE("/posts/:id", h.DeleteForumPost)
				protectedForum.POST("/posts/:id/comments", h.CreateForumComment)
				protectedForum.POST("/posts/:id/poll/vote", h.VoteForumPoll)
				protectedForum.POST("/posts/:id/report", h.ReportForumPost)
				protectedForum.POST("/posts/:id/not-interested", h.NotInterestedForumPost)
				protectedForum.POST("/upload", h.UploadForumMedia)
				protectedForum.POST("/communities", h.CreateForumCommunity)
				protectedForum.POST("/communities/:id/join", h.JoinForumCommunity)
				protectedForum.PUT("/communities/:id", h.UpdateForumCommunity)
				protectedForum.DELETE("/communities/:id", h.DeleteForumCommunity)
			}

			adminForum := forum.Group("/admin")
			adminForum.Use(authMW)
			adminForum.Use(roleMW)
			{
				adminForum.GET("/reports", h.GetAdminForumReports)
				adminForum.GET("/posts/:id/comments", h.GetAdminForumPostComments)
				adminForum.DELETE("/posts/:id", h.AdminDeleteForumPost)
				adminForum.DELETE("/comments/:id", h.AdminDeleteForumComment)
			}
		}
	}
}
