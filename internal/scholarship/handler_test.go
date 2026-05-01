package scholarship

import "testing"

func TestToApplicationResponse_AllowsAnonymousUser(t *testing.T) {
	resp := toApplicationResponse(ScholarshipApplication{
		ScholarshipID: 99,
		UserID:        nil,
		FullName:      "Guest Applicant",
		Gender:        "Female",
		Status:        "pending",
	})

	if resp.UserID != 0 {
		t.Fatalf("UserID = %d, want 0 for anonymous application", resp.UserID)
	}

	if resp.FullName != "Guest Applicant" {
		t.Fatalf("FullName = %q, want %q", resp.FullName, "Guest Applicant")
	}
}
