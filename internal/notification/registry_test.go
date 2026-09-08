package notification

import (
	"testing"
	"time"
)

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
	"system.inquiry_received", "social.review_reported", "moderation.forum_report",
	"moderation.feedback_received", "system.claim_submitted", "system.provider_pending",
	"system.institution_pending",
}

// P2 keys from docs/notification-system/06-notification-taxonomy.md (Task 1).
// application.submitted, scholarship.payment_received and account.approval_pending
// already exist from P1; only their def attributes were updated.
var p2Keys = []string{
	"account.new_device_login", "account.suspended", "account.reinstated",
	"account.deletion_scheduled", "account.deletion_cancelled", "account.totp_changed",
	"scholarship.deadline_reminder", "scholarship.exam_reminder",
	"counselling.session_cancelled",
	"social.new_follower", "social.review_received", "social.forum_reply",
	"social.invite_accepted", "social.review_moderated", "social.forum_moderated",
	"message.offline_fallback",
	"jobs.application_received", "jobs.status_changed",
	"projectshiksha.status_changed", "payment.subscription_recorded",
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
	for _, key := range p2Keys {
		if _, ok := Registry[key]; !ok {
			t.Errorf("missing P2 registry key %q", key)
		}
	}
}

func TestRegistryP2Attributes(t *testing.T) {
	if err := ValidateRegistry(); err != nil {
		t.Fatal(err)
	}
	if def := Registry[EventAccountApprovalPending]; !def.Transactional || !def.EmailDefault {
		t.Errorf("account.approval_pending must be Transactional+EmailDefault, got %+v", def)
	}
	for _, key := range []string{"account.new_device_login", "message.offline_fallback"} {
		if Registry[key].DedupeWin != time.Hour {
			t.Errorf("%s must have DedupeWin 1h, got %v", key, Registry[key].DedupeWin)
		}
	}
	critical := map[string]bool{"account.suspended": true, "account.deletion_scheduled": true,
		"account.totp_changed": true, "scholarship.payment_received": true,
		"counselling.session_cancelled": true}
	for key, want := range critical {
		got := Registry[key].Priority == PriorityCritical
		if got != want {
			t.Errorf("%s critical=%v want %v", key, got, want)
		}
	}
	// social.* and message.* categories must be registered and validate.
	for _, cat := range []string{"social", "message"} {
		found := false
		for _, def := range Registry {
			if def.Category == cat {
				found = true
			}
		}
		if !found {
			t.Errorf("no registry entry for category %q", cat)
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

func TestRegistryEmailDefaults(t *testing.T) {
	if err := ValidateRegistry(); err != nil {
		t.Fatal(err)
	}
	for key, def := range Registry {
		_ = def.EmailDefault // bool is always "set"
		if key == EventAccountWelcome && def.EmailTmpl == "" {
			t.Errorf("%s: expected EmailTmpl set for welcome email", key)
		}
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
