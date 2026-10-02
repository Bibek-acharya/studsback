// internal/coins/public_api.go
//
// GET /api/v1/coins/table — the public coin table (CPA 2075 s.16(2)(n), 04 §6).
//
// The projection and every exclusion decision live in public_table.go; this file is
// the HTTP shell and deliberately holds no policy of its own.
//
// ── why this route carries NO auth middleware ───────────────────────────────────
//
// The disclosure has to be readable by someone who has not registered yet, because
// they are the person deciding whether to. 04 §6 says it "needs to be a page a
// student can actually read before they spend" — a student who has not signed up is
// exactly the audience, and a table behind a login does not reach them.
//
// The route is a separate mount rather than a branch of RegisterRoutes for the reason
// every other mount in this file is separate: the groups have different authorisation,
// and the risk being managed is the admin config leaking. GET /admin/coins/economy
// returns the WHOLE economy — earn rates, the monthly referral cap, the lifetime coin
// cap, the internal unlock switch. One flag added to the wrong mount, or one route
// added to the wrong group, and the fraud thresholds stop being secret. The inventory
// test in public_table_routes_test.go walks both surfaces and fails if either grows.

package coins

import (
	"net/http"

	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

// PublicTableAPI serves the public coin table.
//
// It holds a *Service rather than a *Ledger because it needs the CONFIG and nothing
// else: no account, no balance, no journal. That is a deliberate narrowing — the widest
// thing this endpoint can reach is the same config object an admin editor reads, and
// the projection is the only thing standing between them.
type PublicTableAPI struct {
	service *Service
}

// NewPublicTableAPI builds the API. A nil service is permitted and answered 500 rather
// than refused at construction, so a missing constructor call shows up as one broken
// endpoint rather than a boot that refuses to start over a public page.
func NewPublicTableAPI(service *Service) *PublicTableAPI {
	return &PublicTableAPI{service: service}
}

// Table handles GET /api/v1/coins/table.
func (a *PublicTableAPI) Table(c *gin.Context) {
	if a == nil || a.service == nil {
		response.Error(c, http.StatusInternalServerError, "The coin table is not available")
		return
	}
	cfg, err := a.service.GetEconomyConfig()
	if err != nil {
		// 500 and nothing more. The error is logged elsewhere; the body must not
		// describe the storage failure to a reader who came for a price.
		response.Error(c, http.StatusInternalServerError, "The coin table is not available")
		return
	}
	// A short cache only. The price must not outlive a change to it by much, because
	// a published price that is not the charged price is the advertisement problem
	// again — this time produced by staleness rather than by omission.
	c.Header("Cache-Control", "public, max-age=300, must-revalidate")
	response.Success(c, http.StatusOK, "Coin table", PublicCoinTableFrom(cfg))
}

// RegisterPublicRoutes mounts the unauthenticated coin-table read.
//
// NO auth middleware, and that is the design rather than an omission. Every other
// mount in this package sits behind authMW, and RegisterWalletRoutes' header gives the
// reasoning for the wallet (it exposes only the caller's own coins); here there is no
// caller at all, because the endpoint discloses product terms rather than anything
// about a person.
//
// The unauthenticated surface is exactly one read-only GET that projects the config.
// It is worth keeping it that narrow: the moment this route grows a parameter that
// selects a user, a coin, or a config version, it needs authMW and a different name.
func RegisterPublicRoutes(r *gin.Engine, api *PublicTableAPI) {
	if r == nil || api == nil {
		return
	}
	coins := r.Group("/api/v1/coins")
	{
		coins.GET("/table", api.Table)
	}
}
