package coins

// PlatformAdminRoles are the roles that run the coin economy.
//
// COINECONOMY-SPECIFIC AND NARROWER THAN ELSEWHERE, and that is deliberate and
// contested rather than settled. Every other module that guards an admin surface —
// system, university, admission, scholarship, review — answers with
//
//	{"admin", "super_admin", "superadmin"}
//
// and this list is
//
//	{"admin", "super_admin", "superadmin"}
//
// while the ACTUAL gate in cmd/server/main.go is
//
//	RequireRole("superadmin", "super_admin")
//
// — no "admin" alias. So this list and the middleware disagree, and the middleware
// wins because it runs.
//
// WHY THE DISAGREEMENT IS DOCUMENTED RATHER THAN RESOLVED: coin balances are the
// most sensitive thing this codebase holds that is not applicant PII, and the
// support view added in 04 §6 reads any student's full coin history. The tightest
// gate in the codebase is defensible for that reason alone. But it is also plausibly
// an oversight from copying the analytics gate, and an operator account with role
// "admin" who can manage the inquiry inbox but not the coin economy is the kind of
// thing nobody discovers until they need it.
//
// TestCoinAdminGateAndPlatformAdminRolesDisagree in support_routes_test.go keeps the
// question visible. Deciding it is a product call about who operates the economy.
//
// It lives here, in the coins package, rather than only in main.go, so that the
// next person to add an admin coin route reads the disagreement instead of copying
// whichever list they find first.
func PlatformAdminRoles() []string {
	return []string{"admin", "super_admin", "superadmin"}
}
