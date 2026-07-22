package auth

import (
	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

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
			auth.POST("/logout", h.Logout)
			auth.POST("/send-otp", h.SendOTP)
			auth.POST("/verify-otp", h.VerifyOTP)
			auth.POST("/reset-password", h.ResetPassword)
			auth.GET("/google", h.GoogleLogin)
			auth.GET("/google/callback", h.GoogleCallback)
		}

		institutionAuth := v1.Group("/institutions/auth")
		{
			institutionAuth.POST("/register", h.InstitutionRegister)
			institutionAuth.POST("/claim", h.ClaimRegister)
			institutionAuth.POST("/login", h.InstitutionLogin)
			institutionAuth.POST("/send-otp", h.SendOTP)
			institutionAuth.POST("/reset-password", h.ResetPassword)
			institutionAuth.GET("/google", h.InstitutionGoogleLogin)
			institutionAuth.GET("/google/callback", h.InstitutionGoogleCallback)
		}

		scholarshipProviderAuth := v1.Group("/scholarship-providers/auth")
		{
			scholarshipProviderAuth.POST("/register", h.ScholarshipProviderRegister)
			scholarshipProviderAuth.POST("/login", h.ScholarshipProviderLogin)
			scholarshipProviderAuth.POST("/send-otp", h.SendOTP)
			scholarshipProviderAuth.POST("/reset-password", h.ResetPassword)
			scholarshipProviderAuth.GET("/google", h.ScholarshipProviderGoogleLogin)
			scholarshipProviderAuth.GET("/google/callback", h.ScholarshipProviderGoogleCallback)
		}

		superadminAuth := v1.Group("/superadmin/auth")
		{
			superadminAuth.POST("/register", h.SuperadminRegister)
			superadminAuth.POST("/login", h.SuperadminLogin)
		}

		superadmin := v1.Group("/superadmin")
		superadmin.Use(authMW, superadminOnly())
		{
			superadmin.GET("/dashboard/stats", h.GetDashboardStats)
			superadmin.GET("/users", h.ListAllUsers)
			superadmin.GET("/users/:id/education", h.GetUserEducation)
			superadmin.GET("/users/:id", h.GetUserDetail)
			superadmin.PUT("/users/:id/suspend", h.SuspendUser)
			superadmin.PUT("/users/:id/reinstate", h.ReinstateUser)
			superadmin.GET("/pending-providers", h.ListPendingScholarshipProviders)
			superadmin.GET("/providers", h.ListVerifiedScholarshipProviders)
			superadmin.POST("/providers/approve", h.ApproveScholarshipProvider)
			superadmin.GET("/pending-institutions", h.ListPendingInstitutions)
			superadmin.GET("/institutions", h.ListVerifiedInstitutions)
			superadmin.GET("/rejected-institutions", h.ListRejectedInstitutions)
			superadmin.POST("/institutions", h.CreateInstitution)
			superadmin.GET("/institutions/:id", h.GetInstitution)
			superadmin.POST("/institutions/approve", h.ApproveInstitution)
			superadmin.POST("/institutions/claim-approve", h.ApproveClaimRequest)
			superadmin.POST("/institutions/claim-reject", h.RejectClaimRequest)
			superadmin.PUT("/institutions/:id", h.UpdateInstitution)
			superadmin.PUT("/institutions/:id/access", h.UpdateInstitutionProfileAccess)
			superadmin.PUT("/institutions/:id/payment", h.RecordInstitutionPayment)
			superadmin.PUT("/institutions/:id/verify", h.VerifyInstitution)
			superadmin.PUT("/institutions/:id/feature", h.ToggleInstitutionFeatured)
			superadmin.PUT("/institutions/:id/suspend", h.SuspendInstitution)
			superadmin.DELETE("/institutions/:id", h.DeleteInstitution)
			superadmin.POST("/upload", h.SuperadminUploadFile)

			superadmin.GET("/programs", h.ListAllPrograms)
			superadmin.GET("/programs/:id", h.GetProgramForInstitution)
			superadmin.POST("/programs", h.CreateProgramForInstitution)
			superadmin.PUT("/programs/:id", h.UpdateProgramForInstitution)
			superadmin.DELETE("/programs/:id", h.DeleteProgramForInstitution)

			superadmin.GET("/entrances", h.ListAllEntrances)
			superadmin.GET("/entrances/:id", h.GetEntranceForInstitution)
			superadmin.POST("/entrances", h.CreateEntranceForInstitution)
			superadmin.PUT("/entrances/:id", h.UpdateEntranceForInstitution)
			superadmin.DELETE("/entrances/:id", h.DeleteEntranceForInstitution)
			superadmin.GET("/entrances/:id/applicants", h.GetEntranceApplicantsForInstitution)

			superadmin.GET("/admission-pages", h.ListAllAdmissionPages)
			superadmin.POST("/admission-pages", h.CreateAdmissionPageForInstitution)
			superadmin.PUT("/admission-pages/:id", h.UpdateAdmissionPageForInstitution)
			superadmin.DELETE("/admission-pages/:id", h.DeleteAdmissionPageForInstitution)
		}

		institutionProtected := v1.Group("/institutions")
		institutionProtected.Use(authMW)
		{
			institutionProtected.GET("/profile-access", h.GetMyProfileAccess)
		}

		protected := v1.Group("")
		protected.Use(authMW)
		{
			protected.GET("/profile", h.GetProfile)
			protected.PUT("/profile", h.UpdateProfile)
			protected.PUT("/auth/change-password", h.ChangePassword)
			protected.GET("/profile/education", h.GetEducationEntries)
			protected.POST("/profile/education", h.CreateEducationEntry)
			protected.PUT("/profile/education/:id", h.UpdateEducationEntry)
			protected.DELETE("/profile/education/:id", h.DeleteEducationEntry)
			protected.GET("/profile/documents", h.GetProfileDocuments)
			protected.POST("/profile/documents", h.UploadProfileDocument)
			protected.DELETE("/profile/documents/:id", h.DeleteProfileDocument)
			protected.POST("/preferences", h.SavePreferences)
			protected.POST("/auth/profile/picture", h.UploadProfilePicture)
			protected.GET("/auth/sessions", h.GetSessions)
			protected.DELETE("/auth/sessions/:id", h.RevokeSession)
			protected.DELETE("/auth/sessions", h.RevokeAllSessions)
		}
	}
}

func superadminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists || (role.(string) != "superadmin" && role.(string) != "super_admin") {
			response.Error(c, 403, "Superadmin access required")
			c.Abort()
			return
		}
		c.Next()
	}
}
