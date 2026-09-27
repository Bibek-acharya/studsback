// Package httpx holds gin-context helpers shared across modules. It exists so
// that every module reads the same context keys the same way instead of
// re-deriving them per handler.
package httpx

import (
	"github.com/gin-gonic/gin"
)

// CurrentUserID reads the authenticated user id that middleware.Auth stores
// under "user_id" (see shared/middleware/auth.go). It reports whether the id was
// actually resolved.
//
// The bool is load-bearing and callers must branch on it. The alternative
// `c.Get("user_id")` followed by a bare type assertion is *not* equivalent: it
// yields 0 on a type mismatch with no signal, and 0 is indistinguishable from a
// legitimately-resolved id. For a provenance column such as
// studyresources.StudyResource.UploadedBy that means silently recording a wrong
// owner instead of failing, so callers that write the id to storage should
// reject the request when ok is false.
//
// Type handling: the concrete type in use today is uint, because
// utils.Claims.UserID is a uint and that is the value the middleware stores.
// The remaining cases are defensive only — no current middleware writes a
// different concrete type, but a future one could, and widening here is
// cheaper than debugging a silent zero in a foreign key.
func CurrentUserID(c *gin.Context) (uint, bool) {
	value, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	switch v := value.(type) {
	case uint:
		return v, true
	case uint64:
		return uint(v), true
	case int:
		return uint(v), true
	case int64:
		return uint(v), true
	default:
		return 0, false
	}
}
