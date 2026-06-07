package scholarship

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		education := v1.Group("/education")
		{
			education.GET("/scholarships", h.GetScholarships)
			education.GET("/scholarships/:id", h.GetScholarshipByID)
			education.GET("/scholarships/:id/similar", h.GetSimilarScholarships)
			education.GET("/scholarships/:id/exam-centers", h.GetAvailableExamCenters)
			education.POST("/scholarships/:id/apply", h.ApplyScholarship)
			education.POST("/scholarships/recommend", h.RecommendScholarships)
		}

		public := v1.Group("")
		{
			public.POST("/scholarships/upload", h.UploadFile)
			public.POST("/scholarships/:id/pay", h.ProcessPayment)
			public.POST("/scholarships/pay/:id/confirm", h.ConfirmPayment)
			public.POST("/scholarships/pay/:id/receipt", h.uploadBankReceipt)
			public.POST("/scholarships/pay/esewa/initiate", h.InitiateEsewaPayment)
			public.POST("/scholarships/pay/esewa/verify", h.VerifyEsewaPayment)
		}

		protected := v1.Group("")
		protected.Use(authMW)
		{
			protected.GET("/scholarships/my-applications", h.GetMyApplications)
			protected.GET("/scholarships/applications/:id", h.GetApplication)
			protected.PUT("/scholarships/applications/:id", h.UpdateApplication)
			protected.DELETE("/scholarships/applications/:id", h.DeleteApplication)
		}

		provider := v1.Group("/providers")
		provider.Use(authMW)
		{
			provider.POST("/payments/:id/approve", h.ApprovePayment)
		}

		admin := v1.Group("/admin")
		admin.Use(authMW)
		admin.Use(roleMW)
		{
			admin.GET("/scholarships", h.GetAllApplications)
			admin.GET("/scholarships/list", h.AdminListScholarships)
			admin.GET("/scholarships/:id", h.GetScholarshipByID)
			admin.POST("/scholarships", h.AdminCreateScholarship)
			admin.PUT("/scholarships/:id", h.AdminUpdateScholarship)
			admin.DELETE("/scholarships/:id", h.AdminDeleteScholarship)

			admin.GET("/scholarship-applications", h.GetAllApplications)
			admin.GET("/scholarship-applications/:id", h.GetApplication)
			admin.PUT("/scholarship-applications/:id/status", h.UpdateApplicationStatus)
			admin.GET("/scholarship-applications/scholarship/:scholarshipId", h.GetApplicationsByScholarship)
			admin.POST("/payments/verify-esewa", h.VerifyPendingEsewaPayments)
			admin.POST("/payments/send-admit-cards", h.SendAdmitCards)
		}
	}
}
