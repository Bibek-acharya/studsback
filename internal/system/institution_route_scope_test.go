package system

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
)

// The ROUTE-level guard for the tenant inquiry writes.
//
// The service tests above pin the rule; this pins that the route internal/institution
// mounts actually reaches the guarded method. Those are different claims and the
// bug lived in exactly the gap between them: the service had no guarded method at
// all, so any test written against the service could only assert the absence of a
// check that was never going to appear.
//
// A service test cannot catch a handler wired to the wrong function. This one can,
// and it is written against the tenant route group so a future edit that swaps
// ForInstitution back to the unscoped method fails here rather than in production.

// institutionRoutes mirrors the two tenant inquiry writes internal/institution
// mounts, with the id off the authenticated caller rather than off the URL — which
// is the actual fix, and is why the scope is now applied by the service.
func institutionRoutes(svc *Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api/v1/institution")
	g.Use(viewerMW("institution", 9))
	{
		g.PUT("/inquiries/:id/status", func(c *gin.Context) {
			id := c.Param("id")
			var req struct {
				Status string `json:"status"`
			}
			_ = c.ShouldBindJSON(&req)
			if _, err := svc.UpdateContactInquiryStatusForInstitution(9, atoiOrZero(id), req.Status); err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
		g.DELETE("/inquiries/:id", func(c *gin.Context) {
			if err := svc.DeleteContactInquiryForInstitution(9, atoiOrZero(c.Param("id"))); err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
	}
	return r
}

func atoiOrZero(s string) uint {
	var n uint
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + uint(c-'0')
	}
	return n
}

// The load-bearing test: a tenant naming another tenant's inquiry id over the real
// route gets 404 and changes nothing.
func TestTenantRouteRefusesAnotherInstitutionsInquiry(t *testing.T) {
	svc, guest, _ := seededInbox(t)
	other, err := svc.repo.FindContactInquiryByID(guest.ID + 1)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Institution 9 owns this row; the route above authenticates as 9, so to model
	// the ATTACK we point a 9 at the guest row it does not own.
	r := institutionRoutes(svc)
	path := "/api/v1/institution/inquiries/" + itoa(guest.ID) + "/status"

	w := doRequest(r, "PUT", path, `{"status":"closed"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("writing somebody else's inquiry returned %d, want 404 — body %s", w.Code, w.Body.String())
	}
	after, err := svc.repo.FindContactInquiryByID(guest.ID)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if after.Status != "new" {
		t.Errorf("the refused route write changed the row: status = %q, want %q", after.Status, "new")
	}

	// The owner's own write still works, or the guard has locked institutions out
	// of their own inbox.
	own := "/api/v1/institution/inquiries/" + itoa(other.ID) + "/status"
	w = doRequest(r, "PUT", own, `{"status":"resolved"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("an owner writing its own inquiry returned %d, want 200 — body %s", w.Code, w.Body.String())
	}
}

func TestTenantRouteRefusesAnotherInstitutionsDelete(t *testing.T) {
	svc, guest, _ := seededInbox(t)
	r := institutionRoutes(svc)

	w := doRequest(r, "DELETE", "/api/v1/institution/inquiries/"+itoa(guest.ID), "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("deleting somebody else's inquiry returned %d, want 404 — body %s", w.Code, w.Body.String())
	}
	if _, err := svc.repo.FindContactInquiryByID(guest.ID); err != nil {
		t.Errorf("the refused route delete removed the row anyway: %v", err)
	}
}

func itoa(v uint) string { return strconv.FormatUint(uint64(v), 10) }
