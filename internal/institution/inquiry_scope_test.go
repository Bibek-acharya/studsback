package institution

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"studsphere/backend/internal/system"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// The two tenant inquiry WRITES, mounted on the real internal/institution routes.
//
// The system package pins the ownership rule. This pins that internal/institution
// is actually wired to the guarded methods, which is a separate claim and the one
// that was false: both handlers called the unscoped pair, so the rule existed
// nowhere on this path.
//
// A regression here is the original vulnerability returning in its original shape —
// an institution naming another institution's inquiry id in the URL.

// Returns the service, the handle to read back through, and the two seeded ids.
func inquiryTestSystem(t *testing.T, ownerID uint) (*system.Service, *gorm.DB, uint, uint) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// The schema comes from the model rather than from production migrations:
	// internal/system owns this table, and these tests are about the institution
	// module's WIRING, not about the table existing.
	if err := db.AutoMigrate(&system.ContactInquiry{}); err != nil {
		t.Fatalf("auto migrate contact_inquiries: %v", err)
	}
	// A nil notifier: the tenant write path calls notifyInquiryReplied, which
	// returns early on a nil notifier rather than panicking. SubmitContactInquiry
	// does the opposite, which is a different route and not exercised here.
	svc := system.NewService(system.NewRepository(db), nil)

	// One inquiry addressed to ownerID, one addressed to nobody (a platform-level
	// guest message, which is the realistic majority).
	owned := system.ContactInquiry{
		InstitutionID: &ownerID, Name: "Alan Turing", Email: "alan@example.com",
		Subject: "Tour", Message: "Can we visit?", Type: "general", Status: "new",
	}
	guest := system.ContactInquiry{
		Name: "Grace Hopper", Email: "grace@example.com",
		Subject: "Entry requirements", Message: "What do I need?", Type: "admission", Status: "new",
	}
	for _, row := range []interface{}{&owned, &guest} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	return svc, db, owned.ID, guest.ID
}

// authedAs installs the institution identity the way authMW does: user_id IS the
// institution id on these routes, which is what getInstID reads.
func authedAs(instID uint) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", instID)
		c.Set("user_role", "institution")
		c.Next()
	}
}

func inquiryRoutes(svc *system.Service, instID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(nil, svc)
	g := r.Group("/api/v1/institution")
	g.Use(authedAs(instID))
	{
		g.PUT("/inquiries/:id/status", h.UpdateInquiryStatus)
		g.DELETE("/inquiries/:id", h.DeleteInquiry)
	}
	return r
}

func call(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// The load-bearing test. Institution 9 names the id of a platform-level inquiry it
// does not own and gets 404, with the row untouched.
func TestUpdateInquiryStatusRefusesAnInquiryTheCallerDoesNotOwn(t *testing.T) {
	svc, db, ownedID, guestID := inquiryTestSystem(t, 9)
	r := inquiryRoutes(svc, 9)

	w := call(t, r, "PUT", "/api/v1/institution/inquiries/"+u64(guestID)+"/status", `{"status":"closed"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status write on an unowned inquiry returned %d, want 404 — body %s", w.Code, w.Body.String())
	}
	if got := inquiryStatus(t, db, guestID); got != "new" {
		t.Errorf("the refused write changed the row: status = %q, want %q", got, "new")
	}

	// The owner's own inquiry still works. A guard that refused this would lock
	// institutions out of their own inbox, which is the other half of the fix.
	w = call(t, r, "PUT", "/api/v1/institution/inquiries/"+u64(ownedID)+"/status", `{"status":"resolved"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("owner writing its own inquiry returned %d, want 200 — body %s", w.Code, w.Body.String())
	}
	if got := inquiryStatus(t, db, ownedID); got != "resolved" {
		t.Errorf("owner's write did not land: status = %q, want %q", got, "resolved")
	}
}

func TestDeleteInquiryRefusesAnInquiryTheCallerDoesNotOwn(t *testing.T) {
	svc, db, ownedID, guestID := inquiryTestSystem(t, 9)
	r := inquiryRoutes(svc, 9)

	w := call(t, r, "DELETE", "/api/v1/institution/inquiries/"+u64(guestID), "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("delete of an unowned inquiry returned %d, want 404 — body %s", w.Code, w.Body.String())
	}
	if !inquiryExists(t, db, guestID) {
		t.Error("the refused delete removed the row anyway")
	}

	w = call(t, r, "DELETE", "/api/v1/institution/inquiries/"+u64(ownedID), "")
	if w.Code != http.StatusOK {
		t.Fatalf("owner deleting its own inquiry returned %d, want 200 — body %s", w.Code, w.Body.String())
	}
	if inquiryExists(t, db, ownedID) {
		t.Error("owner's delete did not take effect")
	}
}

// Another TENANT's inquiry — distinct from the platform-level case above, and the
// one the vulnerability was actually about.
func TestInquiryRoutesRefuseAnotherTenantsInquiry(t *testing.T) {
	svc, db, otherOwnedID, _ := inquiryTestSystem(t, 77) // the row belongs to 77
	r := inquiryRoutes(svc, 9)                           // the caller is 9

	w := call(t, r, "PUT", "/api/v1/institution/inquiries/"+u64(otherOwnedID)+"/status", `{"status":"closed"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("institution 9 wrote institution 77's inquiry: %d, want 404 — body %s", w.Code, w.Body.String())
	}
	w = call(t, r, "DELETE", "/api/v1/institution/inquiries/"+u64(otherOwnedID), "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("institution 9 deleted institution 77's inquiry: %d, want 404 — body %s", w.Code, w.Body.String())
	}
	if got := inquiryStatus(t, db, otherOwnedID); got != "new" {
		t.Errorf("the refused calls changed institution 77's row: status = %q, want %q", got, "new")
	}
	if !inquiryExists(t, db, otherOwnedID) {
		t.Error("the refused delete removed institution 77's row")
	}
}

// An invalid status is still a 400, not a 404. Otherwise a caller could not tell
// "bad value" from "not yours", and the ownership refusal would stop being
// informative about its own cause.
func TestAnInvalidStatusIsStillABadRequest(t *testing.T) {
	svc, _, ownedID, _ := inquiryTestSystem(t, 9)
	r := inquiryRoutes(svc, 9)

	w := call(t, r, "PUT", "/api/v1/institution/inquiries/"+u64(ownedID)+"/status", `{"status":"nonsense"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("an invalid status returned %d, want 400 — body %s", w.Code, w.Body.String())
	}
}
