package auth

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/register", h.Register)
			auth.POST("/login", h.Login)
			auth.POST("/send-otp", h.SendOTP)
			auth.POST("/verify-otp", h.VerifyOTP)
			auth.POST("/reset-password", h.ResetPassword)
			auth.GET("/google", h.GoogleLogin)
			auth.GET("/google/callback", h.GoogleCallback)
		}

		institutionAuth := v1.Group("/institutions/auth")
		{
			institutionAuth.POST("/register", h.InstitutionRegister)
			institutionAuth.POST("/login", h.InstitutionLogin)
			institutionAuth.GET("/google", h.InstitutionGoogleLogin)
			institutionAuth.GET("/google/callback", h.InstitutionGoogleCallback)
		}

		scholarshipProviderAuth := v1.Group("/scholarship-providers/auth")
		{
			scholarshipProviderAuth.POST("/register", h.ScholarshipProviderRegister)
			scholarshipProviderAuth.POST("/login", h.ScholarshipProviderLogin)
			scholarshipProviderAuth.GET("/google", h.ScholarshipProviderGoogleLogin)
			scholarshipProviderAuth.GET("/google/callback", h.ScholarshipProviderGoogleCallback)
		}

		superadminAuth := v1.Group("/superadmin/auth")
		{
			superadminAuth.POST("/register", h.SuperadminRegister)
			superadminAuth.POST("/login", h.SuperadminLogin)
		}

		protected := v1.Group("")
		protected.Use(authMW)
		{
			protected.GET("/profile", h.GetProfile)
			protected.PUT("/profile", h.UpdateProfile)
			protected.POST("/preferences", h.SavePreferences)
		}
	}
}
