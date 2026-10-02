// internal/coins/support_api.go
//
// The HTTP surface of 04 §6's support view.
//
// Small on purpose: the whole value of this endpoint is the data in support_view.go,
// and a handler that reshapes it is a second contract to keep in step. So the body
// IS the SupportView struct, and the only decisions here are which refusals map to
// which statuses.

package coins

import (
	"net/http"
	"strconv"

	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

// SupportUser handles GET /api/v1/admin/coins/users/:id.
//
// Unauthenticated and non-admins are refused by the middleware before this runs, so
// this handler answers only three things: a malformed id, a user 0, and a read
// failure. There is no "no such user" answer — an unknown id returns an EMPTY view,
// which is a true answer (this student has no coin history) and is the same rule the
// referral endpoint follows. A 404 here would leak which user ids exist, which is
// an enumeration surface on a table of students.
func (a *AdminAPI) SupportUser(c *gin.Context) {
	// The id is parsed BEFORE the ledger is consulted, and that order is deliberate:
	// a malformed id is a client error that needs no dependency, so it must read as
	// 400 whether or not the ledger is wired. Checking the wiring first would report
	// a server problem for a request the caller got wrong, which sends support
	// looking in the wrong place entirely.
	raw := c.Param("id")
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "Invalid user id")
		return
	}

	// A nil ledger is a wiring mistake and must be a 500 rather than a panic: the
	// route is mounted on a *AdminAPI that RegisterRoutes substitutes when the real
	// one is absent.
	if a == nil || a.ledger == nil {
		response.Error(c, http.StatusInternalServerError, "Coin history is not available")
		return
	}

	// limit is read defensively: a junk value falls back to the default rather than
	// to 0, and 0 would mean "unbounded" to SupportQuery.
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))
	view, err := a.ledger.SupportView(c.Request.Context(), SupportQuery{
		UserID: uint(id),
		Limit:  limit,
	})
	if err != nil {
		// A wiring problem is 500 and says nothing about the user: the id was fine.
		response.Error(c, http.StatusInternalServerError, "Could not read the coin history")
		return
	}
	response.Success(c, http.StatusOK, "Coin history", view)
}

// AdminAPI serves the admin coin endpoints that need a Ledger rather than a Service.
//
// It is a separate type from Handler for the same reason RegisterRoutes takes a
// third argument: the config endpoints operate on the settings row and need no
// ledger at all, and giving them one would let a config read reach into the balance
// tables.
type AdminAPI struct {
	ledger *Ledger
}

// NewAdminAPI wires the admin surface. A nil ledger makes every route 500 rather
// than pretending to work.
func NewAdminAPI(ledger *Ledger) *AdminAPI {
	return &AdminAPI{ledger: ledger}
}
