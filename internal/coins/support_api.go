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
	"errors"
	"net/http"
	"strconv"
	"strings"

	"studsphere/backend/internal/shared/httpx"
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

// AdjustRequestBody is POST /admin/coins/adjust's request.
//
// There is NO balance field and no bucket field. Amount is signed, so the direction
// cannot disagree with itself, and there is deliberately no way to express "make the
// balance 40" — see adjust.go.
type AdjustRequestBody struct {
	// Amount is a JSON NUMBER, not a string. A string would invite a client to send
	// "40" and "forty" to the same field.
	Amount int64 `json:"amount" binding:"required"`
	// Reason must be one of AdjustmentReasons. Validated in the service, not by a
	// binding tag: the tag can only check "present", and the closed set is a
	// package-level fact that the DDL CHECK also reads.
	Reason string `json:"reason" binding:"required"`
	// Note is free text alongside the reason code — the specifics the closed set
	// cannot carry. Never grouped by.
	Note string `json:"note"`
}

// AdjustCoins handles POST /api/v1/admin/coins/adjust.
//
// The AUTHORITY is the caller's own identity, taken from the session and never from
// the body. 03 §3.2 specifies `created_by = 'admin:<id>'`, and the reason this
// matters is the whole reason the endpoint is safe: a body-supplied author would make
// the audit trail a self-report, and a compromised admin session could attribute its
// corrections to a colleague. A body field named `admin_id` is exactly the thing a
// future well-meaning PR would add.
//
// The Idempotency-Key header is REQUIRED rather than defaulted. An operator clicking
// twice is the normal case for a support console, and without a key the second click
// is a second movement.
func (a *AdminAPI) AdjustCoins(c *gin.Context) {
	adminID, ok := httpx.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	// The TARGET comes from the path, never the body.
	//
	// A correction aimed at the wrong student is the worst thing this endpoint can
	// do, and the path is where the operator saw it. It also means the request body
	// has no field naming who to change, which is one fewer thing an attacker with a
	// captured session can vary, and one fewer field a future well-meaning PR adds.
	// The route is therefore POST /admin/coins/adjust/:userID.
	target, err := strconv.ParseUint(c.Param("userId"), 10, 64)
	if err != nil || target == 0 {
		response.Error(c, http.StatusBadRequest, "Invalid user id")
		return
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		// 400 rather than a server-generated key: a generated key would make a double
		// click apply twice, which is the exact failure the header exists to prevent.
		response.Error(c, http.StatusBadRequest, "An Idempotency-Key header is required")
		return
	}

	var body AdjustRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if a == nil || a.ledger == nil {
		response.Error(c, http.StatusInternalServerError, "Adjustments are not available")
		return
	}

	result, err := a.ledger.Adjust(c.Request.Context(), AdjustRequest{
		UserID:         uint(target),
		Amount:         body.Amount,
		Reason:         body.Reason,
		CreatedBy:      "admin:" + strconv.FormatUint(uint64(adminID), 10),
		IdempotencyKey: key,
		Note:           body.Note,
	})
	if err != nil {
		writeAdjustError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Adjustment recorded", result)
}

// writeAdjustError maps the ledger's refusals onto statuses.
//
// 400 for a bad request (an invented reason, a missing author, zero), 409 for a key
// reuse — which is a CONFLICT rather than a bad request, and saying so is what tells
// an operator their key collided with someone else's correction rather than that
// they typed it wrong — and 400 for an overdraft with the shortfall in the message,
// because "would go below zero" is the single most useful thing to tell an operator
// who was about to take back an award.
func writeAdjustError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrIdempotencyKeyReuse):
		response.Error(c, http.StatusConflict, "That idempotency key was used for a different adjustment")
	case errors.Is(err, ErrInsufficientCoins):
		response.Error(c, http.StatusBadRequest, "That adjustment would take the balance below zero; issue a credit instead")
	case errors.Is(err, ErrInvalidArgument):
		response.Error(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNoDatabase):
		response.Error(c, http.StatusInternalServerError, "Adjustments are not available")
	default:
		response.Error(c, http.StatusInternalServerError, "Could not record the adjustment")
	}
}
