package counselling

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		protected := v1.Group("")
		protected.Use(authMW)
		{
			protected.POST("/counselling/bookings", h.CreateCounsellingBooking)
			protected.GET("/counselling/bookings/my", h.GetMyCounsellingBookings)
		}
	}
}
