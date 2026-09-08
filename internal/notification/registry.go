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
	EventSocialReviewReported          = "social.review_reported"
	EventModerationForumReport         = "moderation.forum_report"
	EventModerationFeedback            = "moderation.feedback_received"
	EventSystemClaimSubmitted          = "system.claim_submitted"
	EventSystemProviderPending         = "system.provider_pending"
	EventSystemInstitutionPending      = "system.institution_pending"
	EventSystemInquiryReceived         = "system.inquiry_received"
	EventAccountNewDeviceLogin         = "account.new_device_login"
	EventAccountSuspended              = "account.suspended"
	EventAccountReinstated             = "account.reinstated"
	EventAccountDeletionScheduled      = "account.deletion_scheduled"
	EventAccountDeletionCancelled      = "account.deletion_cancelled"
	EventAccountTotpChanged            = "account.totp_changed"
	EventScholarshipDeadlineReminder   = "scholarship.deadline_reminder"
	EventScholarshipExamReminder       = "scholarship.exam_reminder"
	EventCounsellingSessionCancelled   = "counselling.session_cancelled"
	EventSocialNewFollower             = "social.new_follower"
	EventSocialReviewReceived          = "social.review_received"
	EventSocialForumReply              = "social.forum_reply"
	EventSocialInviteAccepted          = "social.invite_accepted"
	EventSocialReviewModerated         = "social.review_moderated"
	EventSocialForumModerated          = "social.forum_moderated"
	EventMessageOfflineFallback        = "message.offline_fallback"
	EventJobsApplicationReceived       = "jobs.application_received"
	EventJobsStatusChanged             = "jobs.status_changed"
	EventProjectshikshaStatusChanged   = "projectshiksha.status_changed"
	EventPaymentSubscriptionRecorded   = "payment.subscription_recorded"
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
	EmailDefault  bool   // true = email channel ON by default (doc 06 per-event channels)
	EmailTmpl     string // emailqueue template id; "" = generic render from Title/Body
}

func ev(key, category, priority, title, body, link, recipientKind string, emailDefault bool, emailTmpl string) EventDef {
	return EventDef{Key: key, Category: category, Priority: priority,
		TitleTpl: title, BodyTpl: body, LinkTpl: link, RecipientKind: recipientKind,
		EmailDefault: emailDefault, EmailTmpl: emailTmpl}
}

// P1 keys. Email defaults are inert until P2 (no email channel in P1).
// Full registry incl. P2 keys: docs/notification-system/06-notification-taxonomy.md
var Registry = map[string]EventDef{
	EventApplicationReceived:           ev(EventApplicationReceived, "application", PriorityNormal, "New Application Received", "{{if .student_name}}{{.student_name}} applied{{else}}A new application was received{{end}}{{if .program}} for {{.program}}{{end}}.", "", RecipientExplicit, false, ""),
	EventApplicationStatusChanged:      ev(EventApplicationStatusChanged, "application", PriorityCritical, "Application Status Updated", "Your application{{if .program}} for {{.program}}{{end}} moved to {{.status}}.", "/user/dashboard/applications", RecipientExplicit, true, ""),
	EventApplicationSubmitted:          ev(EventApplicationSubmitted, "application", PriorityNormal, "Application Submitted", "Your application was submitted successfully.", "/user/dashboard/applications", RecipientExplicit, false, ""),
	EventApplicationInterviewScheduled: ev(EventApplicationInterviewScheduled, "scholarship", PriorityCritical, "Interview Scheduled", "An interview has been scheduled for your application{{if .scholarship}} for {{.scholarship}}{{end}}.", "/user/dashboard/applications", RecipientExplicit, true, ""),
	EventApplicationShortlisted:        ev(EventApplicationShortlisted, "application", PriorityCritical, "Application Shortlisted", "Congratulations — your application{{if .program}} for {{.program}}{{end}} was shortlisted.", "/user/dashboard/applications", RecipientExplicit, true, ""),
	EventApplicationApproved:           ev(EventApplicationApproved, "application", PriorityCritical, "Application Approved", "Your application{{if .program}} for {{.program}}{{end}} was approved.", "/user/dashboard/applications", RecipientExplicit, true, ""),
	EventApplicationRejected:           ev(EventApplicationRejected, "application", PriorityCritical, "Application Update", "Your application{{if .program}} for {{.program}}{{end}} was not successful this time.", "/user/dashboard/applications", RecipientExplicit, true, ""),
	EventScholarshipAdmitCardReady:     ev(EventScholarshipAdmitCardReady, "scholarship", PriorityCritical, "Admit Card Ready", "Your admit card is ready to download.", "/user/dashboard/applications", RecipientExplicit, true, ""),
	EventScholarshipPaymentReceived:    ev(EventScholarshipPaymentReceived, "scholarship", PriorityCritical, "Payment Received", "Payment confirmed for {{.scholarship}}.", "", RecipientExplicit, true, ""),
	EventScholarshipPaymentFailed:      ev(EventScholarshipPaymentFailed, "scholarship", PriorityCritical, "Payment Failed", "Your payment for {{.scholarship}} did not go through. Please retry.", "/scholarship-pay/{{.slug}}", RecipientExplicit, true, ""),
	EventScholarshipBankApproved:       ev(EventScholarshipBankApproved, "scholarship", PriorityCritical, "Bank Payment Approved", "Your bank payment was approved. Your admit card has been issued.", "/user/dashboard/applications", RecipientExplicit, true, ""),
	EventScholarshipBankRejected:       ev(EventScholarshipBankRejected, "scholarship", PriorityCritical, "Bank Payment Rejected", "Your bank payment was rejected{{if .reason}}: {{.reason}}{{end}}.", "/user/dashboard/applications", RecipientExplicit, true, ""),
	EventScholarshipBankReceipt:        ev(EventScholarshipBankReceipt, "scholarship", PriorityNormal, "Bank Receipt Submitted", "A bank receipt was uploaded for {{.scholarship}}.", "", RecipientExplicit, false, ""),
	EventCounsellingBookingCreated:     ev(EventCounsellingBookingCreated, "counselling", PriorityNormal, "New Counselling Booking", "{{if .student_name}}{{.student_name}} requested a session{{else}}A new session was requested{{end}}{{if .when}} on {{.when}}{{end}}.", "", RecipientExplicit, false, ""),
	EventCounsellingBookingConfirmed:   ev(EventCounsellingBookingConfirmed, "counselling", PriorityNormal, "Counselling Confirmed", "Your counselling session was confirmed.", "/user/dashboard", RecipientExplicit, true, ""),
	EventCounsellingBookingCancelled:   ev(EventCounsellingBookingCancelled, "counselling", PriorityCritical, "Counselling Cancelled", "Your counselling session was cancelled.", "/user/dashboard", RecipientExplicit, true, ""),
	EventCounsellingBookingRescheduled: ev(EventCounsellingBookingRescheduled, "counselling", PriorityCritical, "Counselling Rescheduled", "Your counselling session was moved{{if .when}} to {{.when}}{{end}}.", "/user/dashboard", RecipientExplicit, true, ""),
	EventAccountWelcome:                ev(EventAccountWelcome, "account", PriorityNormal, "Welcome to StudsSphere", "Your account is ready.", "/user/dashboard", RecipientExplicit, true, "welcome"),
	EventAccountApprovalPending:        ev(EventAccountApprovalPending, "account", PriorityNormal, "StudSphere — Application Received", "Hi {{.name}}, we received your {{.kind}} registration. Our team will review it and email you the decision.", "", RecipientExplicit, true, ""),
	EventAccountApproved:               ev(EventAccountApproved, "account", PriorityCritical, "Account Approved", "Your account has been approved. Welcome!", "", RecipientExplicit, true, ""),
	EventAccountRejected:               ev(EventAccountRejected, "account", PriorityCritical, "Account Not Approved", "Your account was not approved.", "", RecipientExplicit, true, ""),
	EventAccountNewLogin:               ev(EventAccountNewLogin, "account", PriorityNormal, "New Login", "Access user {{.email}} logged in.", "", RecipientExplicit, false, ""),
	EventAccountProfileIncomplete:      ev(EventAccountProfileIncomplete, "account", PriorityLow, "Profile Incomplete", "Complete your profile to appear in more results.", "", RecipientExplicit, false, ""),
	EventAccountAccessGranted:          ev(EventAccountAccessGranted, "account", PriorityNormal, "Access Granted", "Access has been granted to {{.email}}.", "", RecipientExplicit, false, ""),
	EventAccountAccessRemoved:          ev(EventAccountAccessRemoved, "account", PriorityNormal, "Access Removed", "Access was removed for {{.email}}.", "", RecipientExplicit, false, ""),
	EventAccountPasswordChanged:        ev(EventAccountPasswordChanged, "account", PriorityCritical, "Password Changed", "Your password was changed.", "", RecipientExplicit, true, ""),
	EventAccountEmailChanged:           ev(EventAccountEmailChanged, "account", PriorityCritical, "Email Updated", "Your email was changed.", "", RecipientExplicit, true, ""),
	EventContentCreatedOwn:             ev(EventContentCreatedOwn, "content", PriorityLow, "{{.what}} Created", "Your {{.what}} \"{{.title}}\" was created.", "", RecipientExplicit, false, ""),
	EventSystemAnnouncement:            ev(EventSystemAnnouncement, "system", PriorityCritical, "{{.title}}", "{{.body}}", "{{.link}}", RecipientExplicit, false, ""),
	EventSystemInquiryReceived:         ev(EventSystemInquiryReceived, "moderation", PriorityNormal, "New Inquiry", "{{.name}} ({{.email}}): {{.subject}}", "", RecipientRole, false, ""),
	EventSocialReviewReported:          ev(EventSocialReviewReported, "moderation", PriorityNormal, "Review Reported", "Review #{{.review_id}} reported: {{.reason}}", "", RecipientRole, false, ""),
	EventModerationForumReport:         ev(EventModerationForumReport, "moderation", PriorityNormal, "Forum Content Reported", "{{.kind}} #{{.id}} reported: {{.reason}}", "", RecipientRole, false, ""),
	EventModerationFeedback:            ev(EventModerationFeedback, "moderation", PriorityLow, "New Feedback", "Feedback received from {{.name}}.", "", RecipientRole, false, ""),
	EventSystemClaimSubmitted:          ev(EventSystemClaimSubmitted, "moderation", PriorityNormal, "College Claim Submitted", "{{.college}} claimed by {{.email}}.", "", RecipientRole, false, ""),
	EventSystemProviderPending:         ev(EventSystemProviderPending, "moderation", PriorityNormal, "Provider Pending Approval", "{{.name}} registered and awaits approval.", "", RecipientRole, false, ""),
	EventSystemInstitutionPending:      ev(EventSystemInstitutionPending, "moderation", PriorityNormal, "Institution Pending Approval", "{{.name}} registered and awaits approval.", "", RecipientRole, false, ""),

	// P2 keys (docs/notification-system/06-notification-taxonomy.md). All RecipientExplicit.
	EventAccountNewDeviceLogin:       ev(EventAccountNewDeviceLogin, "account", PriorityLow, "New Device Login", "Your account was just signed in from a new device ({{.email}}). If this wasn't you, change your password.", "/user/security", RecipientExplicit, true, ""),
	EventAccountSuspended:            ev(EventAccountSuspended, "account", PriorityCritical, "Account Suspended", "Your account has been suspended. Contact support if you believe this is a mistake.", "", RecipientExplicit, true, ""),
	EventAccountReinstated:           ev(EventAccountReinstated, "account", PriorityNormal, "Account Reinstated", "Good news — your account has been reinstated. Welcome back!", "", RecipientExplicit, true, ""),
	EventAccountDeletionScheduled:    ev(EventAccountDeletionScheduled, "account", PriorityCritical, "Account Deletion Scheduled", "Your account is scheduled for deletion. It will be deleted in 14 days. Sign in to cancel.", "", RecipientExplicit, true, ""),
	EventAccountDeletionCancelled:    ev(EventAccountDeletionCancelled, "account", PriorityLow, "Deletion Cancelled", "Your account deletion request was cancelled. Your account is safe.", "/user/settings", RecipientExplicit, false, ""),
	EventAccountTotpChanged:          ev(EventAccountTotpChanged, "account", PriorityCritical, "2FA Changed", "Two-factor authentication settings for your account were changed.", "/user/security", RecipientExplicit, true, ""),
	EventScholarshipDeadlineReminder: ev(EventScholarshipDeadlineReminder, "scholarship", PriorityNormal, "Deadline Approaching", "{{.scholarship}} closes on {{.deadline}}.", "/scholarship-pay/{{.slug}}", RecipientExplicit, true, ""),
	EventScholarshipExamReminder:     ev(EventScholarshipExamReminder, "scholarship", PriorityNormal, "Exam Reminder", "Your {{.scholarship}} exam is coming up. Review your admit card.", "/user/dashboard/admit-card", RecipientExplicit, true, ""),
	EventCounsellingSessionCancelled: ev(EventCounsellingSessionCancelled, "counselling", PriorityCritical, "Session Cancelled", "The session on {{.when}} was cancelled by the institution.", "/user/counselling", RecipientExplicit, true, ""),
	EventSocialNewFollower:           ev(EventSocialNewFollower, "social", PriorityLow, "New Follower", "{{.name}} started following you.", "/followers", RecipientExplicit, false, ""),
	EventSocialReviewReceived:        ev(EventSocialReviewReceived, "social", PriorityNormal, "New Review", "{{.name}} left{{if .rating}} a {{.rating}}-star{{end}} review.", "/reviews", RecipientExplicit, false, ""),
	EventSocialForumReply:            ev(EventSocialForumReply, "social", PriorityNormal, "New Reply", "{{.name}} replied to your post.", "/campus-forum/post/{{.post_id}}", RecipientExplicit, false, ""),
	EventSocialInviteAccepted:        ev(EventSocialInviteAccepted, "social", PriorityLow, "Invite Accepted", "{{.name}} {{.response}} your calendar invite.", "/user/calendar", RecipientExplicit, false, ""),
	EventSocialReviewModerated:       ev(EventSocialReviewModerated, "social", PriorityNormal, "Review Removed", "Your review was removed by a moderator.", "", RecipientExplicit, true, ""),
	EventSocialForumModerated:        ev(EventSocialForumModerated, "social", PriorityNormal, "Content Removed", "Your forum content was removed by a moderator.", "/campus-forum", RecipientExplicit, true, ""),
	EventMessageOfflineFallback:      ev(EventMessageOfflineFallback, "message", PriorityNormal, "New Message", "You have a new message from {{.name}}.", "/messages/{{.conversation_id}}", RecipientExplicit, false, ""),
	EventJobsApplicationReceived:     ev(EventJobsApplicationReceived, "moderation", PriorityNormal, "New Job Application", "{{.name}} applied: {{.title}}.", "", RecipientExplicit, false, ""),
	EventJobsStatusChanged:           ev(EventJobsStatusChanged, "account", PriorityNormal, "Application Update", "Your application for {{.job_title}} moved to {{.status}}.", "/careers", RecipientExplicit, true, ""),
	EventProjectshikshaStatusChanged: ev(EventProjectshikshaStatusChanged, "account", PriorityNormal, "Application Update", "Your ProjectShiksha application status: {{.status}}.", "/projectshiksha", RecipientExplicit, true, ""),
	EventPaymentSubscriptionRecorded: ev(EventPaymentSubscriptionRecorded, "account", PriorityNormal, "Subscription Recorded", "Institution subscription for plan \"{{.plan}}\" was recorded.", "", RecipientExplicit, true, ""),
}

// ev() has no Transactional/DedupeWin params; set P2 attrs that differ from
// defaults here (email-only class per doc 06; Task 6 relies on the 1h window).
func init() {
	def := Registry[EventAccountApprovalPending]
	def.Transactional = true
	Registry[EventAccountApprovalPending] = def
	for _, k := range []string{EventAccountNewDeviceLogin, EventMessageOfflineFallback} {
		def := Registry[k]
		def.DedupeWin = time.Hour
		Registry[k] = def
	}
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
		EventSystemProviderPending, EventSystemInstitutionPending,
		EventAccountNewDeviceLogin, EventAccountSuspended, EventAccountReinstated,
		EventAccountDeletionScheduled, EventAccountDeletionCancelled, EventAccountTotpChanged,
		EventScholarshipDeadlineReminder, EventScholarshipExamReminder, EventCounsellingSessionCancelled,
		EventSocialNewFollower, EventSocialReviewReceived, EventSocialForumReply,
		EventSocialInviteAccepted, EventSocialReviewModerated, EventSocialForumModerated,
		EventMessageOfflineFallback, EventJobsApplicationReceived, EventJobsStatusChanged,
		EventProjectshikshaStatusChanged, EventPaymentSubscriptionRecorded} {
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
