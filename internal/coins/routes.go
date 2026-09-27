package coins

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the admin coin-economy config endpoints.
//
// adminRoleMW is passed in rather than built here, and it is deliberately NOT
// the shared roleMW from cmd/server/main.go. That gate is
// RequireRole("admin", "super_admin", "scholarship_provider", …, "institution"),
// so putting coin pricing behind it would let an institution account rewrite
// the price of every unlock on the site. Coin config gets its own
// RequireRole("superadmin", "super_admin") group, exactly like
// studyResourcesRoleMW and the notifications gate
// (02-architecture.md §7, 03-api-contract.md §1).
//
// The other admin coin endpoints in 03-api-contract.md §3 — economy-daily,
// users/:id, adjust — belong to the ledger slice and are not here.
func RegisterRoutes(r *gin.Engine, authMW, adminRoleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	admin := v1.Group("/admin/coins")
	admin.Use(authMW)
	admin.Use(adminRoleMW)
	{
		admin.GET("/economy", h.GetEconomyConfig)
		admin.PUT("/economy", h.UpdateEconomyConfig)
	}
}
