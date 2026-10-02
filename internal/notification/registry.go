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
	EventSystemInquiryReplied          = "system.inquiry_replied"
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
	EventContentSaved                  = "content.saved"
	EventCoinsDebited                  = "coins.debited"
	EventCoinsCredited                 = "coins.credited"
	// EventStudyResourceApproved and EventStudyResourceRejected are the two halves
	// of the §5.3 moderation decision, and they are separate keys rather than one
	// with a status in the body because they are different events to a student: one
	// is their material going live, the other is a refusal they can act on.
	EventStudyResourceApproved = "studyresource.approved"
	EventStudyResourceRejected = "studyresource.rejected"

	// ── Phase 5: the referral programme's own three events ─────────────────────
	//
	// These existed in 03-api-contract.md §5 from the start and were never added to
	// the registry. The referral payout was announced through the generic
	// EventCoinsCredited instead, which is a real defect rather than a cosmetic gap:
	// a student whose friend qualified was told "200 StudsTokens were added to your
	// balance" with no indication that a person they invited had done anything. The
	// cause is the entire content of the event.
	//
	// Separate keys rather than one with a status in the body, for the same reason
	// the two moderation events are separate: these are three different facts to a
	// student, and one of them is bad news.
	//
	//   qualified — the invitee met the conditions, and the award is being held.
	//   released  — the hold has passed and the coins are now the referrer's.
	//   revoked   — the payout was taken back. A student's balance went DOWN, which
	//               is why this one is not a credit event under any circumstances.
	EventReferralQualified = "referral.qualified"
	EventReferralReleased  = "referral.released"
	EventReferralRevoked   = "referral.revoked"

	// ── Phase 5: the per-lot expiry reminders ────────────────────────────────
	//
	// 04 §7 asks for "reminders at 30 / 7 / 1 days, per lot". This is deliberately
	// NOT EventAllowanceExpiring: that event is about the included starter unlocks,
	// which are a single per-account entitlement with one date. These are about EARNED
	// coin lots, each with its own expiry and its own balance, and a student told
	// "your included unlocks end on 12 November" must not read that as their earned
	// coins ending on the 12th.
	//
	// One event with a `days` value rather than three keys (coins.expiring_30 etc.)
	// because the copy differs only in that one figure, and a third key would triple
	// the registry for no extra information.
	EventCoinsExpiring = "coins.expiring"
	// EventCoinsExpired is the sweep telling a student their coins lapsed. 04 §7
	// says "never silently delete an expired balance … and always tell the student",
	// and this event is the second half of that sentence made executable.
	EventCoinsExpired = "coins.expired"

	// ── 03 §5, still missing ─────────────────────────────────────────────────
	//
	// Both named in the §5 table from the beginning and neither implemented. The
	// allowance is an entitlement rather than a grant (02-architecture.md §5), so
	// "expiring" is about losing access to included unlocks, not about coins.
	EventAllowanceExpiring = "allowance.expiring"
	EventAllowanceExpired  = "allowance.expired"
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
	EventScholarshipPaymentReceived:    ev(EventScholarshipPaymentReceived, "scholarship", PriorityCritical, "Payment Received", "Payment for {{.scholarship}} received.", "/scholarship-pay/{{.slug}}", RecipientExplicit, true, ""),
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
	EventSystemInquiryReplied:          ev(EventSystemInquiryReplied, "system", PriorityNormal, "Inquiry Update", "Response received regarding your inquiry: {{.subject}}", "/user/inquiries", RecipientExplicit, true, ""),
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
	EventSocialReviewReceived:        ev(EventSocialReviewReceived, "social", PriorityNormal, "New Review", "{{.name}} left{{if index . \"rating\"}} a {{.rating}}-star{{end}} review.", "/reviews", RecipientExplicit, false, ""),
	EventSocialForumReply:            ev(EventSocialForumReply, "social", PriorityNormal, "New Reply", "{{.name}} replied to your post.", "/campus-forum/post/{{.post_id}}", RecipientExplicit, false, ""),
	EventSocialInviteAccepted:        ev(EventSocialInviteAccepted, "social", PriorityLow, "Invite Accepted", "{{.name}} {{.response}} your calendar invite.", "/user/calendar", RecipientExplicit, false, ""),
	EventSocialReviewModerated:       ev(EventSocialReviewModerated, "social", PriorityNormal, "Review Removed", "Your review was removed by a moderator.", "", RecipientExplicit, true, ""),
	EventSocialForumModerated:        ev(EventSocialForumModerated, "social", PriorityNormal, "Content Removed", "Your forum content was removed by a moderator.", "/campus-forum", RecipientExplicit, true, ""),
	EventMessageOfflineFallback:      ev(EventMessageOfflineFallback, "message", PriorityNormal, "New Message", "You have a new message from {{.name}}.", "/messages/{{.conversation_id}}", RecipientExplicit, false, ""),
	EventJobsApplicationReceived:     ev(EventJobsApplicationReceived, "moderation", PriorityNormal, "New Job Application", "{{.name}} applied: {{.title}}.", "", RecipientExplicit, false, ""),
	EventJobsStatusChanged:           ev(EventJobsStatusChanged, "account", PriorityNormal, "Application Update", "Your application for {{.job_title}} moved to {{.status}}.", "/careers", RecipientExplicit, true, ""),
	EventProjectshikshaStatusChanged: ev(EventProjectshikshaStatusChanged, "account", PriorityNormal, "Application Update", "Your ProjectShiksha application status: {{.status}}.", "/projectshiksha", RecipientExplicit, true, ""),
	EventPaymentSubscriptionRecorded: ev(EventPaymentSubscriptionRecorded, "account", PriorityNormal, "Subscription Recorded", "Institution subscription for plan \"{{.plan}}\" was recorded.", "", RecipientExplicit, true, ""),
	EventContentSaved:                ev(EventContentSaved, "content", PriorityLow, "Saved", "{{.item}} saved to your bookmarks.", "/user/dashboard/bookmarks", RecipientExplicit, false, ""),
	// coins.debited (03-api-contract.md §5): a spend went through, so this is a
	// receipt a student will look for when a balance does not match what they
	// expected. account, like payment.subscription_recorded, because it is a
	// movement on their own account and not a message about anyone else's.
	// PriorityNormal: it confirms something the student just did on purpose, so
	// it must not force in_app on the way. EmailDefault true because 09
	// §"If the student says an email never arrived" lists this event as one that
	// emails, and a missing receipt for a real debit is exactly the question that
	// list exists to answer. Every string is from the 06 §10 copy deck, and the
	// coin figures beside them are coin figures, never a currency one.
	EventCoinsDebited: ev(EventCoinsDebited, "account", PriorityNormal, "StudsTokens used",
		"{{.coins}} StudsTokens were used to unlock a {{.item}}. Your balance is {{.balance}}.",
		"/user/dashboard/resources", RecipientExplicit, true, ""),
	// coins.credited (03-api-contract.md §5): coins arrived without being spent.
	// Deliberately the mirror of coins.debited — same category, same priority, same
	// account link — because a credit and a debit are the same event seen from two
	// sides, and a student whose balance moves needs to be able to answer "why".
	//
	// EmailDefault true for the same reason as the debit: 09's "if the student says
	// an email never arrived" list is about a movement on their own account, and a
	// missing credit notice is exactly as confusing as a missing receipt.
	//
	// The copy obeys 09's three hard bans, and the constraint is statutory rather
	// than stylistic: no "free" (CPA 2075 s.16), no currency figure beside the coin
	// figure, and none of prize/award/win/raffle/draw (Income Tax Act 2058 s.5/88A
	// defines windfall gain to include "lottery, gift, prize, baksis, award for
	// winning"). The word "earned" is the approved substitute and appears in 09's
	// own reply block, which says a student "earn StudsTokens by completing your
	// profile".
	//
	// The body says nothing about WHY the coins arrived. The profile ladder pays in
	// five instalments, so a credit notice that named a profile step would be wrong
	// for the referral and upload awards that share this event key, and one event
	// key must render for every producer of a credit. The cause is in the wallet's
	// transaction list, which is where a student looks for it.
	EventCoinsCredited: ev(EventCoinsCredited, "account", PriorityNormal, "StudsTokens earned",
		"{{.coins}} StudsTokens were added to your balance. Your balance is {{.balance}}.",
		"/user/dashboard/resources", RecipientExplicit, true, ""),

	// The moderation decision. Both are Transactional, because the state machine
	// writes the row and then announces inside the same transaction and a commit
	// that silently dropped the announcement would leave a student who uploaded
	// material with no word about what happened to it — the exact gap 09's support
	// list exists to close.
	EventStudyResourceApproved: ev(EventStudyResourceApproved, "studyresource", PriorityNormal,
		"Your upload is live",
		"{{.title}} is now available to students. StudsTokens have been added to your balance.",
		"/user/dashboard/resources", RecipientExplicit, true, ""),
	// The rejection body carries the reason, and says "was not approved" rather
	// than "failed": 06 §7 forbids dressing a moderation decision as a loss, and
	// the reason is the only part the student can act on.
	EventStudyResourceRejected: ev(EventStudyResourceRejected, "studyresource", PriorityNormal,
		"Your upload was not approved",
		"{{.title}} was not approved. Reason: {{.reason}}.",
		"/user/dashboard/resources", RecipientExplicit, true, ""),

	// ── Phase 5 rows ─────────────────────────────────────────────────────────
	//
	// The copy on every one of these obeys 09's three hard bans, and the bans are
	// asserted by coin_events_registry_test.go against the template text rather than
	// left to this review — a template is data, so it can be edited by anyone with
	// registry access without anyone reading it as prose.
	//
	// 06 §1.4 is the other constraint and it shapes two of these bodies: expiry is
	// stated as a DATE or a day count, never as a countdown and never with urgency.
	// 09's row on this is unambiguous — "limited time (on earned StudsTokens): delete
	// the phrase" — because a student who earned a coin over twelve months is not
	// being sold anything.

	// referral.qualified: the invitee did the thing, and the coins are being held
	// rather than paid. The hold is named because the wait is a policy the student
	// would otherwise read as a delay with no end.
	EventReferralQualified: ev(EventReferralQualified, "referral", PriorityNormal,
		"Friend signed up",
		"Your friend joined and {{.coins}} StudsTokens are held for you. They are added to your balance on {{.releases_on}}.",
		"/user/dashboard/referral", RecipientExplicit, true, ""),

	// referral.released: the hold has passed and the coins are the student's.
	EventReferralReleased: ev(EventReferralReleased, "referral", PriorityNormal,
		"Referral StudsTokens added",
		"{{.coins}} StudsTokens were added to your balance for {{.friend}}. Your balance is {{.balance}}.",
		"/user/dashboard/referral", RecipientExplicit, true, ""),

	// referral.revoked: the payout was taken back. This one is the reason the three
	// are separate events — a balance going DOWN must never be announced as a credit,
	// and it must say why, or the student sees coins disappear with no explanation.
	//
	// The balance is stated because after a revocation the student needs to know what
	// they are left with, not just what went.
	EventReferralRevoked: ev(EventReferralRevoked, "referral", PriorityCritical,
		"Referral StudsTokens removed",
		"{{.coins}} StudsTokens were removed from your balance for a referral that was reversed. Your balance is {{.balance}}.",
		"/user/dashboard/referral", RecipientExplicit, true, ""),

	// coins.expiring: the 30 / 7 / 1-day notice for ONE lot.
	//
	// `days` is what makes this one event rather than three, and `coins` is the lot's
	// OWN remaining balance rather than the student's total — telling someone
	// "5 StudsTokens expire on 12 November" when their balance is 300 is actionable,
	// and "you have StudsTokens expiring" is not.
	//
	// The FEFO note is in the body because it is the one thing a student cannot
	// derive: which of their lots goes first is a policy decision, not arithmetic,
	// and a student who assumes it is arithmetic will be surprised the wrong way.
	EventCoinsExpiring: ev(EventCoinsExpiring, "account", PriorityNormal,
		"StudsTokens expiring soon",
		"{{.coins}} StudsTokens expire on {{.expires_at}}, in {{.days}} days. We spend the StudsTokens that expire soonest first.",
		"/user/dashboard/coins", RecipientExplicit, true, ""),

	// coins.expired: the sweep has burned them.
	//
	// PriorityCritical and stated in the past tense, because unlike every other coin
	// event this one reports value that is ALREADY GONE. There is nothing to do with
	// this notification — it exists so the balance a student sees is explainable, and
	// because 04 §7 requires it: "never silently delete an expired balance … and
	// always tell the student."
	EventCoinsExpired: ev(EventCoinsExpired, "account", PriorityCritical,
		"StudsTokens expired",
		"{{.coins}} StudsTokens expired on {{.expires_at}} and were removed from your balance. Your balance is {{.balance}}.",
		"/user/dashboard/coins", RecipientExplicit, true, ""),

	// allowance.expiring / allowance.expired: losing INCLUDED UNLOCKS, which is not
	// the same thing as losing coins — a student whose allowance lapses still has
	// every coin they earned. The copy says "included unlocks" throughout rather
	// than "StudsTokens", because conflating the two is how a student ends up
	// believing a balance they can see has been taken away.
	EventAllowanceExpiring: ev(EventAllowanceExpiring, "account", PriorityNormal,
		"Included unlocks expiring soon",
		"Your included unlocks expire on {{.expires_at}}, in {{.days}} days.",
		"/user/dashboard/coins", RecipientExplicit, true, ""),
	EventAllowanceExpired: ev(EventAllowanceExpired, "account", PriorityCritical,
		"Included unlocks expired",
		"Your included unlocks expired on {{.expires_at}}. You can still unlock anything using StudsTokens in your balance.",
		"/user/dashboard/coins", RecipientExplicit, true, ""),
}

// ev() has no Transactional/DedupeWin params; set P2 attrs that differ from
// defaults here (email-only class per doc 06; Task 6 relies on the 1h window).
func init() {
	def := Registry[EventAccountApprovalPending]
	def.Transactional = true
	Registry[EventAccountApprovalPending] = def
	// Both §5.3 moderation decisions are transactional: the row is written and the
	// announcement sent inside one transaction, so a dropped announcement on a
	// committed row would be the gap this is here to close.
	for _, k := range []string{EventStudyResourceApproved, EventStudyResourceRejected} {
		d := Registry[k]
		d.Transactional = true
		Registry[k] = d
	}
	for _, k := range []string{EventAccountNewDeviceLogin, EventMessageOfflineFallback} {
		def := Registry[k]
		def.DedupeWin = time.Hour
		Registry[k] = def
	}

	// The coin events that fire from the hourly sweep get a dedupe window, and are
	// deliberately NOT Transactional.
	//
	// The "not transactional" is the part worth being sure about, because 03 §5 reads
	// as though it means the opposite. `Transactional` does NOT mean "rolled back with
	// the transaction that caused it" — see service.go, where a transactional
	// definition is `continue`d straight past the delivery loop: "Transactional events
	// handled by their own email paths; notification deliveries only for
	// inbox-bearing events." It means NO INBOX ROW.
	//
	// So marking a reminder transactional would do the opposite of what a reminder is
	// for: a student who never opens their email would get no notice at all that their
	// coins are about to lapse. These belong in the notification centre, which is
	// where a student looks, and email is the copy that arrives on its own.
	//
	// (03 §5's sentence about being emitted inside the same transaction is therefore
	// NOT implemented anywhere — Notify enqueues in its own transaction regardless of
	// this flag. Recorded rather than fixed here: closing it means restructuring the
	// notify path so an enqueue can enlist in a caller's transaction, which is a
	// change to internal/notification's contract and not a Phase 5 one.)
	//
	// The dedupe window is what makes an hourly job safe. 24 hours rather than the 1
	// hour the other jobs use, and deliberately so: the thresholds are 30/7/1 days, so
	// the window needs to be long enough that a job which runs twice inside one day
	// cannot double-send, and short enough that a genuine restart still delivers.
	// The caller's OccurrenceKey — keyed on lot AND threshold — is the first line of
	// defence; this is the second.
	for _, k := range []string{
		EventReferralQualified, EventReferralReleased, EventReferralRevoked,
		EventCoinsExpiring, EventCoinsExpired,
		EventAllowanceExpiring, EventAllowanceExpired,
	} {
		d := Registry[k]
		d.DedupeWin = 24 * time.Hour
		Registry[k] = d
	}

	// The two moderation decisions, and the two coin movements, get a dedupe window
	// for the same reason: they are emitted from inside a state transition that a
	// retry can repeat, so without one a retried approve tells the uploader twice.
	//
	// coins.credited and coins.debited are ALSO added here rather than being left
	// alone. They are emitted from the profile-award and referral-settle transactions
	// (profile_award.go, referral.go), both of which a client can retry, and neither
	// had a window — so a double-submitted profile completion announced two credits.
	for _, k := range []string{
		EventStudyResourceApproved, EventStudyResourceRejected,
		EventCoinsCredited, EventCoinsDebited,
	} {
		d := Registry[k]
		if d.DedupeWin <= 0 {
			d.DedupeWin = 24 * time.Hour
		}
		Registry[k] = d
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
		EventSystemAnnouncement, EventSystemInquiryReceived, EventSystemInquiryReplied, EventSocialReviewReported,
		EventModerationForumReport, EventModerationFeedback, EventSystemClaimSubmitted,
		EventSystemProviderPending, EventSystemInstitutionPending,
		EventAccountNewDeviceLogin, EventAccountSuspended, EventAccountReinstated,
		EventAccountDeletionScheduled, EventAccountDeletionCancelled, EventAccountTotpChanged,
		EventScholarshipDeadlineReminder, EventScholarshipExamReminder, EventCounsellingSessionCancelled,
		EventSocialNewFollower, EventSocialReviewReceived, EventSocialForumReply,
		EventSocialInviteAccepted, EventSocialReviewModerated, EventSocialForumModerated,
		EventMessageOfflineFallback, EventJobsApplicationReceived, EventJobsStatusChanged,
		EventProjectshikshaStatusChanged, EventPaymentSubscriptionRecorded, EventContentSaved,
		EventCoinsDebited, EventCoinsCredited,
		EventStudyResourceApproved, EventStudyResourceRejected,
		EventReferralQualified, EventReferralReleased, EventReferralRevoked,
		EventCoinsExpiring, EventCoinsExpired,
		EventAllowanceExpiring, EventAllowanceExpired} {
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
