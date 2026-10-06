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

		// The document sample: the first few pages of a published PDF, inline,
		// for anyone — the buying decision should not need a session, and the
		// withheld remainder is what the download route charges for. Public for
		// the same reason the metadata list is, and safe for the two reasons
		// documented at the top of preview.go.
		v1.GET("/study-resources/:id/preview", h.PreviewResource)

		// Document downloads and playback tokens both require a session.
		//
		// The download route used to sit here, unauthenticated, and the only
		// check in the handler was IsPublished — so any anonymous caller could
		// fetch any published file, and so could any caller once coin gating
		// made file delivery worth protecting.
		//
		// It is safe behind authMW even though the frontend opens it with
		// window.open and therefore sends no Authorization header: Auth falls
		// back to the HttpOnly `token` cookie that every login/refresh path
		// sets, and a cookie is sent on a plain top-level navigation. That is
		// the difference from the <video> route above, which cannot carry a
		// cookie set on a cross-origin media request the same way and so relies
		// on the playback token instead.
		authenticated := v1.Group("/study-resources")
		authenticated.Use(authMW)
		{
			authenticated.GET("/:id/download", h.DownloadResource)
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
			// The §5.3 moderation queue and its two decisions. They live on the
			// SAME superadmin gate as the CRUD above rather than on a new one,
			// because moderation IS admin work and there is no tenant whose claim
			// to it exists.
			admin.GET("/pending", h.PendingReviewQueue)
			admin.POST("/:id/approve", h.ApproveResource)
			admin.POST("/:id/reject", h.RejectResource)
		}

		// The STUDENT upload and my-uploads surface. Separate from the admin group
		// above and mounted on authMW alone, which is the whole point: a student's
		// resource arrives as pending_review, unpublished, and pays nothing. It
		// becomes visible and paid only when an admin approves it.
		student := v1.Group("/study-resources")
		student.Use(authMW)
		{
			student.POST("", h.SubmitResource)
			student.GET("/mine", h.MyUploads)
		}
	}
}
