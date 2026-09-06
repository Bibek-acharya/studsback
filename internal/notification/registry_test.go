package notification

import "testing"

var p1Keys = []string{
	"application.received", "application.status_changed", "application.submitted",
	"application.interview_scheduled", "application.shortlisted", "application.approved",
	"application.rejected", "scholarship.admit_card_ready", "scholarship.payment_received",
	"scholarship.payment_failed", "scholarship.bank_payment_approved",
	"scholarship.bank_payment_rejected", "scholarship.bank_receipt_submitted",
	"counselling.booking_created", "counselling.booking_confirmed",
	"counselling.booking_cancelled", "counselling.booking_rescheduled",
	"account.welcome", "account.approval_pending", "account.approved", "account.rejected",
	"account.new_login", "account.profile_incomplete", "account.access_granted",
	"account.access_removed", "account.password_changed", "account.email_changed",
	"content.created_own", "system.announcement",
}

func TestValidateRegistryAllP1KeysPresent(t *testing.T) {
	if err := ValidateRegistry(); err != nil {
		t.Fatalf("registry invalid: %v", err)
	}
	for _, key := range p1Keys {
		if _, ok := Registry[key]; !ok {
			t.Errorf("missing P1 registry key %q", key)
		}
	}
}

func TestResolveTemplate(t *testing.T) {
	out, err := ResolveTemplate("Application {{.status}}: {{.program}}",
		map[string]any{"status": "Shortlisted", "program": "BSc CS"})
	if err != nil || out != "Application Shortlisted: BSc CS" {
		t.Fatalf("got %q, %v", out, err)
	}
	if _, err := ResolveTemplate("Application {{.missing_var}}", map[string]any{}); err == nil {
		t.Fatal("expected error for missing template variable")
	}
}

func TestUnknownKeyFailsLookup(t *testing.T) {
	if _, ok := Registry["nonexistent.key"]; ok {
		t.Fatal("unknown key must not be in registry")
	}
}

func TestCriticalKeysAreCritical(t *testing.T) {
	for _, key := range []string{"application.status_changed", "application.interview_scheduled",
		"scholarship.payment_failed", "counselling.booking_cancelled", "account.approved"} {
		if Registry[key].Priority != PriorityCritical {
			t.Errorf("%s must be critical, got %s", key, Registry[key].Priority)
		}
	}
}
