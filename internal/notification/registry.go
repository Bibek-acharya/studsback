package notification

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"
)

const (
	PriorityLow      = "low"
	PriorityNormal   = "normal"
	PriorityCritical = "critical"

	RecipientExplicit     = "explicit"
	RecipientRole         = "role"
	RecipientFollowers    = "followers"
	RecipientParticipants = "participants"
)

// Event keys (P1). Every constant must appear exactly once in Registry.
const (
	EventApplicationReceived           = "application.received"
	EventApplicationStatusChanged      = "application.status_changed"
	EventApplicationSubmitted          = "application.submitted"
	EventApplicationInterviewScheduled = "application.interview_scheduled"
	EventApplicationShortlisted        = "application.shortlisted"
	EventApplicationApproved           = "application.approved"
	EventApplicationRejected           = "application.rejected"
	EventScholarshipAdmitCardReady     = "scholarship.admit_card_ready"
	EventScholarshipPaymentReceived    = "scholarship.payment_received"
	EventScholarshipPaymentFailed      = "scholarship.payment_failed"
	EventScholarshipBankApproved       = "scholarship.bank_payment_approved"
	EventScholarshipBankRejected       = "scholarship.bank_payment_rejected"
	EventScholarshipBankReceipt        = "scholarship.bank_receipt_submitted"
	EventCounsellingBookingCreated     = "counselling.booking_created"
	EventCounsellingBookingConfirmed   = "counselling.booking_confirmed"
	EventCounsellingBookingCancelled   = "counselling.booking_cancelled"
	EventCounsellingBookingRescheduled = "counselling.booking_rescheduled"
	EventAccountWelcome                = "account.welcome"
	EventAccountApprovalPending        = "account.approval_pending"
	EventAccountApproved               = "account.approved"
	EventAccountRejected               = "account.rejected"
	EventAccountNewLogin               = "account.new_login"
	EventAccountProfileIncomplete      = "account.profile_incomplete"
	EventAccountAccessGranted          = "account.access_granted"
	EventAccountAccessRemoved          = "account.access_removed"
	EventAccountPasswordChanged        = "account.password_changed"
	EventAccountEmailChanged           = "account.email_changed"
	EventContentCreatedOwn             = "content.created_own"
	EventSystemAnnouncement            = "system.announcement"
	EventSystemInquiryReceived         = "system.inquiry_received"
	EventSocialReviewReported          = "social.review_reported"
	EventModerationForumReport         = "moderation.forum_report"
	EventModerationFeedback            = "moderation.feedback_received"
	EventSystemClaimSubmitted          = "system.claim_submitted"
	EventSystemProviderPending         = "system.provider_pending"
	EventSystemInstitutionPending      = "system.institution_pending"
)

type EventDef struct {
	Key           string
	Category      string
	Priority      string
	TitleTpl      string
	BodyTpl       string
	LinkTpl       string
	Transactional bool
	MinPermission string
	DedupeWin     time.Duration
	ReNudge       bool
	RecipientKind string
}

func ev(key, category, priority, title, body, link, recipientKind string) EventDef {
	return EventDef{Key: key, Category: category, Priority: priority,
		TitleTpl: title, BodyTpl: body, LinkTpl: link, RecipientKind: recipientKind}
}

// P1 keys. Email defaults are inert until P2 (no email channel in P1).
// Full registry incl. P2 keys: docs/notification-system/06-notification-taxonomy.md
var Registry = map[string]EventDef{
	EventApplicationReceived:           ev(EventApplicationReceived, "application", PriorityNormal, "New Application Received", "{{if .student_name}}{{.student_name}} applied{{else}}A new application was received{{end}}{{if .program}} for {{.program}}{{end}}.", "", RecipientExplicit),
	EventApplicationStatusChanged:      ev(EventApplicationStatusChanged, "application", PriorityCritical, "Application Status Updated", "Your application{{if .program}} for {{.program}}{{end}} moved to {{.status}}.", "/user/dashboard/applications", RecipientExplicit),
	EventApplicationSubmitted:          ev(EventApplicationSubmitted, "application", PriorityNormal, "Application Submitted", "Your application was submitted successfully.", "/user/dashboard/applications", RecipientExplicit),
	EventApplicationInterviewScheduled: ev(EventApplicationInterviewScheduled, "scholarship", PriorityCritical, "Interview Scheduled", "An interview has been scheduled for your application{{if .scholarship}} for {{.scholarship}}{{end}}.", "/user/dashboard/applications", RecipientExplicit),
	EventApplicationShortlisted:        ev(EventApplicationShortlisted, "application", PriorityCritical, "Application Shortlisted", "Congratulations — your application{{if .program}} for {{.program}}{{end}} was shortlisted.", "/user/dashboard/applications", RecipientExplicit),
	EventApplicationApproved:           ev(EventApplicationApproved, "application", PriorityCritical, "Application Approved", "Your application{{if .program}} for {{.program}}{{end}} was approved.", "/user/dashboard/applications", RecipientExplicit),
	EventApplicationRejected:           ev(EventApplicationRejected, "application", PriorityCritical, "Application Update", "Your application{{if .program}} for {{.program}}{{end}} was not successful this time.", "/user/dashboard/applications", RecipientExplicit),
	EventScholarshipAdmitCardReady:     ev(EventScholarshipAdmitCardReady, "scholarship", PriorityCritical, "Admit Card Ready", "Your admit card is ready to download.", "/user/dashboard/applications", RecipientExplicit),
	EventScholarshipPaymentReceived:    ev(EventScholarshipPaymentReceived, "scholarship", PriorityCritical, "Payment Received", "Payment confirmed for {{.scholarship}}.", "", RecipientExplicit),
	EventScholarshipPaymentFailed:      ev(EventScholarshipPaymentFailed, "scholarship", PriorityCritical, "Payment Failed", "Your payment for {{.scholarship}} did not go through. Please retry.", "/scholarship-pay/{{.slug}}", RecipientExplicit),
	EventScholarshipBankApproved:       ev(EventScholarshipBankApproved, "scholarship", PriorityCritical, "Bank Payment Approved", "Your bank payment was approved. Your admit card has been issued.", "/user/dashboard/applications", RecipientExplicit),
	EventScholarshipBankRejected:       ev(EventScholarshipBankRejected, "scholarship", PriorityCritical, "Bank Payment Rejected", "Your bank payment was rejected{{if .reason}}: {{.reason}}{{end}}.", "/user/dashboard/applications", RecipientExplicit),
	EventScholarshipBankReceipt:        ev(EventScholarshipBankReceipt, "scholarship", PriorityNormal, "Bank Receipt Submitted", "A bank receipt was uploaded for {{.scholarship}}.", "", RecipientExplicit),
	EventCounsellingBookingCreated:     ev(EventCounsellingBookingCreated, "counselling", PriorityNormal, "New Counselling Booking", "{{if .student_name}}{{.student_name}} requested a session{{else}}A new session was requested{{end}}{{if .when}} on {{.when}}{{end}}.", "", RecipientExplicit),
	EventCounsellingBookingConfirmed:   ev(EventCounsellingBookingConfirmed, "counselling", PriorityNormal, "Counselling Confirmed", "Your counselling session was confirmed.", "/user/dashboard", RecipientExplicit),
	EventCounsellingBookingCancelled:   ev(EventCounsellingBookingCancelled, "counselling", PriorityCritical, "Counselling Cancelled", "Your counselling session was cancelled.", "/user/dashboard", RecipientExplicit),
	EventCounsellingBookingRescheduled: ev(EventCounsellingBookingRescheduled, "counselling", PriorityCritical, "Counselling Rescheduled", "Your counselling session was moved{{if .when}} to {{.when}}{{end}}.", "/user/dashboard", RecipientExplicit),
	EventAccountWelcome:                ev(EventAccountWelcome, "account", PriorityNormal, "Welcome to StudsSphere", "Your account is ready.", "/user/dashboard", RecipientExplicit),
	EventAccountApprovalPending:        ev(EventAccountApprovalPending, "account", PriorityNormal, "Account Under Review", "Your registration is being reviewed. We will notify you once approved.", "", RecipientExplicit),
	EventAccountApproved:               ev(EventAccountApproved, "account", PriorityCritical, "Account Approved", "Your account has been approved. Welcome!", "", RecipientExplicit),
	EventAccountRejected:               ev(EventAccountRejected, "account", PriorityCritical, "Account Not Approved", "Your account was not approved.", "", RecipientExplicit),
	EventAccountNewLogin:               ev(EventAccountNewLogin, "account", PriorityNormal, "New Login", "Access user {{.email}} logged in.", "", RecipientExplicit),
	EventAccountProfileIncomplete:      ev(EventAccountProfileIncomplete, "account", PriorityLow, "Profile Incomplete", "Complete your profile to appear in more results.", "", RecipientExplicit),
	EventAccountAccessGranted:          ev(EventAccountAccessGranted, "account", PriorityNormal, "Access Granted", "Access has been granted to {{.email}}.", "", RecipientExplicit),
	EventAccountAccessRemoved:          ev(EventAccountAccessRemoved, "account", PriorityNormal, "Access Removed", "Access was removed for {{.email}}.", "", RecipientExplicit),
	EventAccountPasswordChanged:        ev(EventAccountPasswordChanged, "account", PriorityCritical, "Password Changed", "Your password was changed.", "", RecipientExplicit),
	EventAccountEmailChanged:           ev(EventAccountEmailChanged, "account", PriorityCritical, "Email Updated", "Your email was changed.", "", RecipientExplicit),
	EventContentCreatedOwn:             ev(EventContentCreatedOwn, "content", PriorityLow, "{{.what}} Created", "Your {{.what}} \"{{.title}}\" was created.", "", RecipientExplicit),
	EventSystemAnnouncement:            ev(EventSystemAnnouncement, "system", PriorityCritical, "{{.title}}", "{{.body}}", "{{.link}}", RecipientExplicit),
	EventSystemInquiryReceived:         ev(EventSystemInquiryReceived, "moderation", PriorityNormal, "New Inquiry", "{{.name}} ({{.email}}): {{.subject}}", "", RecipientRole),
	EventSocialReviewReported:          ev(EventSocialReviewReported, "moderation", PriorityNormal, "Review Reported", "Review #{{.review_id}} reported: {{.reason}}", "", RecipientRole),
	EventModerationForumReport:         ev(EventModerationForumReport, "moderation", PriorityNormal, "Forum Content Reported", "{{.kind}} #{{.id}} reported: {{.reason}}", "", RecipientRole),
	EventModerationFeedback:            ev(EventModerationFeedback, "moderation", PriorityLow, "New Feedback", "Feedback received from {{.name}}.", "", RecipientRole),
	EventSystemClaimSubmitted:          ev(EventSystemClaimSubmitted, "moderation", PriorityNormal, "College Claim Submitted", "{{.college}} claimed by {{.email}}.", "", RecipientRole),
	EventSystemProviderPending:         ev(EventSystemProviderPending, "moderation", PriorityNormal, "Provider Pending Approval", "{{.name}} registered and awaits approval.", "", RecipientRole),
	EventSystemInstitutionPending:      ev(EventSystemInstitutionPending, "moderation", PriorityNormal, "Institution Pending Approval", "{{.name}} registered and awaits approval.", "", RecipientRole),
}

// DedupeWinOr returns the registry dedupe window or the provided default.
func (d EventDef) DedupeWinOr(def time.Duration) time.Duration {
	if d.DedupeWin > 0 {
		return d.DedupeWin
	}
	return def
}

func ValidateRegistry() error {
	for key, def := range Registry {
		if key != def.Key {
			return fmt.Errorf("registry key mismatch: map=%s def=%s", key, def.Key)
		}
		for _, tpl := range []string{def.TitleTpl, def.BodyTpl} {
			if _, err := template.New(key).Parse(tpl); err != nil {
				return fmt.Errorf("bad template for %s: %v", key, err)
			}
		}
		switch def.Priority {
		case PriorityLow, PriorityNormal, PriorityCritical:
		default:
			return fmt.Errorf("bad priority for %s: %s", key, def.Priority)
		}
		switch def.RecipientKind {
		case RecipientExplicit, RecipientRole, RecipientFollowers, RecipientParticipants:
		default:
			return fmt.Errorf("bad recipient kind for %s: %s", key, def.RecipientKind)
		}
	}
	// Every constant must exist in the map.
	for _, k := range []string{EventApplicationReceived, EventApplicationStatusChanged, EventApplicationSubmitted,
		EventApplicationInterviewScheduled, EventApplicationShortlisted, EventApplicationApproved, EventApplicationRejected,
		EventScholarshipAdmitCardReady, EventScholarshipPaymentReceived, EventScholarshipPaymentFailed,
		EventScholarshipBankApproved, EventScholarshipBankRejected, EventScholarshipBankReceipt,
		EventCounsellingBookingCreated, EventCounsellingBookingConfirmed, EventCounsellingBookingCancelled,
		EventCounsellingBookingRescheduled, EventAccountWelcome, EventAccountApprovalPending, EventAccountApproved,
		EventAccountRejected, EventAccountNewLogin, EventAccountProfileIncomplete, EventAccountAccessGranted,
		EventAccountAccessRemoved, EventAccountPasswordChanged, EventAccountEmailChanged, EventContentCreatedOwn,
		EventSystemAnnouncement, EventSystemInquiryReceived, EventSocialReviewReported,
		EventModerationForumReport, EventModerationFeedback, EventSystemClaimSubmitted,
		EventSystemProviderPending, EventSystemInstitutionPending} {
		if _, ok := Registry[k]; !ok {
			return fmt.Errorf("constant %s missing from Registry", k)
		}
	}
	return nil
}

func ResolveTemplate(tpl string, data map[string]any) (string, error) {
	t, err := template.New("t").Parse(tpl)
	if err != nil {
		return "", err
	}
	t.Option("missingkey=error")
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}
