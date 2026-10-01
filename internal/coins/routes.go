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

// RegisterWalletRoutes mounts the student-facing wallet surface:
// GET /balance, GET /transactions, GET /allowance and POST /unlock
// (03-api-contract.md §2.1-§2.3).
//
// It is a separate function rather than two more parameters on RegisterRoutes
// because the two groups have different authorisation. The admin group is behind
// its own superadmin gate; the wallet group is behind authMW alone, because every
// one of these endpoints exposes only the CALLER's own wallet and moves nothing.
// Sharing a group would mean either locking a student out of their own balance
// or letting an institution account read coin pricing, and both are worse than
// two mounts.
//
// No role middleware here, deliberately: a student has no special role in this
// codebase beyond being authenticated, and RequireRole would need a list of
// every role that is allowed to own a wallet.
//
// POST /unlock is mounted but dark — it answers 503 until an admin sets
// EconomyConfig.UnlockEndpointEnabled. See the field's comment and
// unlock_api.go's header for why the resource gates that would make a purchase
// meaningful land with the next slice.
func RegisterWalletRoutes(r *gin.Engine, authMW gin.HandlerFunc, api *UnlockAPI) {
	if r == nil || api == nil {
		return
	}

	wallet := r.Group("/api/v1/coins")
	wallet.Use(authMW)
	{
		wallet.GET("/balance", api.Balance)
		wallet.GET("/transactions", api.ListTransactions)
		wallet.GET("/allowance", api.Allowance)
		wallet.POST("/unlock", api.UnlockResource)
	}
}

// RegisterReferralRoutes mounts GET /api/v1/referrals (03-api-contract.md §2.4).
//
// A third mount rather than a group inside RegisterWalletRoutes, and the reason is
// the one the function above already gives: these are different resources with a
// different disclosure surface, and lumping a referral summary in with the wallet
// would make "add a role gate here" a question about coin pricing.
//
// It is authMW alone, for the wallet's reason inverted. The wallet needs no role gate
// because a student has no special role beyond being authenticated; referrals need no
// role gate because every row returned belongs to the CALLER — `institution` and
// `scholarship_provider` are legitimate authenticated principals here and get their
// own (empty) view, which is exactly what they get from the wallet. Adding
// RequireRole here would be an entitlement decision about which account types may
// hold StudsTokens, and that is not this route's to make.
func RegisterReferralRoutes(r *gin.Engine, authMW gin.HandlerFunc, api *ReferralAPI) {
	if r == nil || api == nil {
		return
	}

	referrals := r.Group("/api/v1/referrals")
	referrals.Use(authMW)
	{
		referrals.GET("", api.Me)
	}
}
