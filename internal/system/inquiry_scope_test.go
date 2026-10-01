package system

import (
	"errors"
	"testing"
)

// Who may WRITE an inquiry, as pinned by the tests below:
//
//	platform admin                              — any inquiry
//	the institution the inquiry is addressed to   — its own only
//	any other tenant                             — nothing, and told 404
//
// The read side was scoped before the write side was, which is why this was
// exploitable: GetInstitutionInquiries has always filtered on institution_id, so
// an institution could see only its own inbox, yet it could still change or delete
// any inquiry on the platform by id. A row it cannot read was a row it could
// write.
//
// WHY 404 AND NOT 403. The refusal is an ownership answer, not a role one —
// `system`'s own ErrForbidden comment says a 403 is for role refusals where
// "there is no ownership question here to hide". Here there is: a 403 would
// confirm that an inquiry id exists, which is the one piece of information a
// caller probing for another tenant's inbox must not get. This matches how
// internal/jobs scopes applicant routes.

func TestUpdateContactInquiryStatusForInstitutionIsScopedToTheCaller(t *testing.T) {
	svc, guest, _ := seededInbox(t)
	claimed, err := svc.repo.FindContactInquiryByID(guest.ID + 1)
	if err != nil {
		t.Fatalf("read the institution's own inquiry: %v", err)
	}

	// The owner's own inquiry: this is the write that must keep working, and a
	// guard that refused it would lock institutions out of their own inbox.
	got, err := svc.UpdateContactInquiryStatusForInstitution(9, claimed.ID, "resolved")
	if err != nil {
		t.Fatalf("institution 9 could not resolve its own inquiry: %v", err)
	}
	if got.Status != "resolved" {
		t.Errorf("status = %q, want %q", got.Status, "resolved")
	}

	// Somebody else's inquiry, named by id. Institution 77 does not exist in the
	// seed and owns nothing, which is the shape of the attack: the id came from
	// the URL and the caller has no relationship to the row at all.
	if _, err := svc.UpdateContactInquiryStatusForInstitution(77, claimed.ID, "closed"); !errors.Is(err, ErrInquiryNotFound) {
		t.Errorf("a non-owner got %v, want ErrInquiryNotFound", err)
	}

	// And it must not have changed on the way past.
	after, err := svc.repo.FindContactInquiryByID(claimed.ID)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if after.Status != "resolved" {
		t.Errorf("a refused write still changed the row: status = %q, want %q", after.Status, "resolved")
	}
}

// A guest inquiry — one with no institution at all, which is most of them — must
// be unreachable to every tenant, not merely to the wrong one.
func TestPlatformLevelInquiryIsInvisibleToEveryTenant(t *testing.T) {
	svc, guest, _ := seededInbox(t)

	for _, instID := range []uint{9, 77} {
		if _, err := svc.UpdateContactInquiryStatusForInstitution(instID, guest.ID, "closed"); !errors.Is(err, ErrInquiryNotFound) {
			t.Errorf("institution %d reached a platform-level inquiry: %v, want ErrInquiryNotFound", instID, err)
		}
		if err := svc.DeleteContactInquiryForInstitution(instID, guest.ID); !errors.Is(err, ErrInquiryNotFound) {
			t.Errorf("institution %d deleted a platform-level inquiry: %v, want ErrInquiryNotFound", instID, err)
		}
	}
	if err := svc.repo.db.First(guest, guest.ID).Error; err != nil {
		t.Errorf("a refused delete removed the row anyway: %v", err)
	}
}

func TestDeleteContactInquiryForInstitutionIsScopedToTheCaller(t *testing.T) {
	svc, guest, _ := seededInbox(t)
	claimed, err := svc.repo.FindContactInquiryByID(guest.ID + 1)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// Refused for a non-owner, and the row survives.
	if err := svc.DeleteContactInquiryForInstitution(77, claimed.ID); !errors.Is(err, ErrInquiryNotFound) {
		t.Errorf("a non-owner got %v, want ErrInquiryNotFound", err)
	}
	if _, err := svc.repo.FindContactInquiryByID(claimed.ID); err != nil {
		t.Fatalf("a refused delete removed the row anyway: %v", err)
	}

	// Allowed for the owner.
	if err := svc.DeleteContactInquiryForInstitution(9, claimed.ID); err != nil {
		t.Fatalf("institution 9 could not delete its own inquiry: %v", err)
	}
	if _, err := svc.repo.FindContactInquiryByID(claimed.ID); err == nil {
		t.Error("the owner's own delete did not take effect")
	}
}

// A platform admin keeps the whole inbox. The scoping must not cost the operator
// the route they already had, or this fix trades a tenant leak for an outage.
func TestPlatformAdminStillReachesEveryInquiry(t *testing.T) {
	svc, guest, _ := seededInbox(t)
	admin := Viewer{UserID: 1, Role: "superadmin"}

	got, err := svc.UpdateContactInquiryStatusAsAdmin(admin, guest.ID, "in_progress")
	if err != nil {
		t.Fatalf("an admin could not write a platform-level inquiry: %v", err)
	}
	if got.Status != "in_progress" {
		t.Errorf("status = %q, want %q", got.Status, "in_progress")
	}
	if err := svc.DeleteContactInquiryAsAdmin(admin, guest.ID); err != nil {
		t.Fatalf("an admin could not delete a platform-level inquiry: %v", err)
	}
}

// An unauthenticated caller is not an institution with id 0. `getInstID` reads
// the id off the context with a bare type assertion, and this pins that a
// missing or zero caller cannot reach a row.
func TestInstitutionZeroCannotWrite(t *testing.T) {
	svc, guest, _ := seededInbox(t)

	if _, err := svc.UpdateContactInquiryStatusForInstitution(0, guest.ID, "closed"); !errors.Is(err, ErrInquiryNotFound) {
		t.Errorf("a caller with no institution id got %v, want ErrInquiryNotFound", err)
	}
	if err := svc.DeleteContactInquiryForInstitution(0, guest.ID); !errors.Is(err, ErrInquiryNotFound) {
		t.Errorf("a caller with no institution id got %v, want ErrInquiryNotFound", err)
	}
}
