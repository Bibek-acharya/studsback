package studyresources

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, superadminRoleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		// Public metadata browsing. The public list and detail expose metadata
		// only — no video bytes.
		v1.GET("/study-resources", h.ListResources)
		v1.GET("/study-resources/:id", h.GetResource)
		v1.GET("/study-resources/:id/download", h.DownloadResource)

		// Range-aware inline playback for video lectures.
		//
		// This route is deliberately NOT behind the session authMW: a <video>
		// element can only request a URL, so it cannot attach an Authorization
		// header. The caller instead presents the short-lived, resource-bound
		// playback token minted by /:id/playback-token (see StreamResource),
		// which validates signature, purpose, audience, expiry and resource
		// binding before any database or object-storage access. The handler is
		// therefore the authorization gate, and documents keep using the
		// attachment-oriented /:id/download endpoint.
		v1.GET("/study-resources/:id/stream", h.StreamResource)

		// Mints the short-lived, resource-bound token that unlocks the stream.
		// Behind the session Auth middleware: an anonymous caller gets 401.
		authenticated := v1.Group("/study-resources")
		authenticated.Use(authMW)
		{
			authenticated.GET("/:id/playback-token", h.IssuePlaybackToken)
		}

		admin := v1.Group("/admin/study-resources")
		admin.Use(authMW)
		admin.Use(superadminRoleMW)
		{
			admin.POST("", h.CreateResource)
			admin.GET("", h.AdminListResources)
			admin.PUT("/:id", h.UpdateResource)
			admin.POST("/:id/file", h.ReplaceResourceFile)
			admin.DELETE("/:id", h.DeleteResource)
		}
	}
}
