package scholarship

import "testing"

func TestScholarshipAdminRolesDoNotAdmitTenants(t *testing.T) {
	// This list is the only thing standing between a paying tenant and a
	// minor's household financial profile, so it is pinned on its own rather than
	// only through the routes that read it. Every entry below was admitted before.
	for _, tenant := range []string{
		"institution", "scholarship_provider", "scholarship-provider",
		"Scholarship Provider", "scholarship_provider_subuser", "student", "user", "",
	} {
		if IsPlatformAdmin(tenant) {
			t.Errorf("IsPlatformAdmin(%q) = true", tenant)
		}
	}
	for _, admin := range []string{"admin", "superadmin", "super_admin", "Admin", " SUPER_ADMIN "} {
		if !IsPlatformAdmin(admin) {
			t.Errorf("IsPlatformAdmin(%q) = false; the operator would be locked out", admin)
		}
	}
}
