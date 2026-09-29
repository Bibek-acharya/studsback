# 01 — Current State & Feasibility

What StudSphere has today, what a coin system needs that it does not have, and an
honest verdict on whether this is buildable on the current codebase without
destabilising it.

---

## 1. Starting position: this is a greenfield build

A repository-wide search of both applications for `coin`, `credit`, `wallet`,
`balance`, `reward`, `redeem`, `unlock`, and `monetize` returns **no domain code
whatsoever**. The only hits are unrelated: college fit-score `point` fields, a
`Credits` string on a course DTO (`studsback/internal/education/dto.go:247`), and
prose. There is no partial coin implementation to extend, no half-migrated
schema, and no prior art inside the repo. Every table, endpoint, and screen in
this feature is new.

That is a real cost, but it is also good news in one specific way: there is no
legacy data model to unwind and no migration of existing balances to reconcile.
We can design the schema correctly from day one rather than retrofitting it.

### 1.1 Two applications, two nested Git repositories

| Directory | Stack | Notes |
|---|---|---|
| `studsnew/` | Next.js App Router, React, TypeScript | Run `npm run lint`, `npm test -- --runInBand` |
| `studsback/` | Go, gin, GORM, PostgreSQL, MinIO | Run `go test ./...`, `go build ./...` |

Per `AGENTS.md`, run Git commands from the directory whose history you intend to
change — they are separate repositories.

---

## 2. What already exists and is worth reusing

The coin feature is not starting from nothing. Five specific pieces of the current
codebase are directly load-bearing.

### 2.1 Profile completeness is already computed

`studsback/internal/studentdashboard/service.go:327` implements
`computeProfileCompletion(user *auth.User, educationCount int64) int` — a
twelve-check score surfaced to the frontend as `ProfileCompletion int`
(`internal/studentdashboard/dto.go:13`).

**Why this matters:** the "complete your profile" earn rule does not need a new
definition of profile completeness. It needs the existing one, called at the
right moment.

**The catch, and it is a real one:** this function is recomputed on every
dashboard request and its result is never persisted. It is also *decreasing* — a
user who deletes their address drops back below 100%. Awarding coins on
"completion reached 100%" therefore needs an explicit, durable award record, not
a comparison against a recomputed number. The design in [02](02-architecture.md)
handles this with a `reward_grant` table carrying a unique constraint on
`(user_id, award_code)`, so a grant happens at most once per user per award type
regardless of how the profile oscillates.

### 2.2 A composite-unique-index pattern to copy

`internal/follow/model.go:5` is the idiom this codebase already uses to prevent
duplicate edges between two entities:

```go
type UserFollow struct {
    ID         uint   `gorm:"primaryKey"`
    UserID     uint   `gorm:"index:idx_follow_user_target,unique;not null"`
    TargetID   uint   `gorm:"index:idx_follow_user_target,unique;not null"`
    TargetType string `gorm:"index:idx_follow_user_target,unique;not null;default:'institution'"`
    CreatedAt  time.Time
}
```

The referral table in [02](02-architecture.md) uses the same shape to guarantee
one inviter per invitee, which is the first of several database-level fraud
controls described in [05](05-economy-and-fraud.md).

### 2.3 Transaction-aware notifications

`internal/notification/service.go:144` exposes `NotifyTx(ctx, tx, req)`, which
writes inbox rows inside a caller-supplied transaction. Its own doc comment states
the intent: emission is atomic with the business mutation.

This means a coin grant and its "you earned 60 coins" notification can commit
together, and a rolled-back grant leaves no orphan notification. The alternative
— grant in one transaction, notify in another — produces notifications for
rewards that never landed.

**One hard constraint:** the event registry is closed and validated at boot.
`notification.ValidateRegistry()` is called from `cmd/server/main.go:173` and
calls `logger.Fatal` on any violation, with the comment *"Bad event registry
means every notification fan-out would misbehave: kill the boot."* Adding a coin
event requires both a constant in `internal/notification/registry.go` and a row in
the `Registry` map. Miss the second and the server will not start.

### 2.4 An admin key-value settings store

`internal/system/model.go:221` defines `SystemSetting{Key string (unique), Value
string}`, with `GetSystemSetting` / `SetSystemSetting` at
`internal/system/repository.go:1209-1233` and a worked consumer at
`internal/system/service.go:1056-1109` (`find_college_ad_cards`) that shows the
read-merge-defaults pattern.

This is a reasonable host for coin economy configuration — but with two caveats
documented in [02](02-architecture.md) §7: it performs **no caching** (a fresh
`SELECT` per read), and its admin route is guarded by the *broad* `roleMW`, which
includes `institution` and `scholarship_provider`. Coin economy config must not be
writable by an institution account.

### 2.5 A superadmin dashboard with an existing approval-queue pattern

There is no resource approval UI today, but the house pattern for approve/reject
already exists in the same dashboard: `PendingInstitutionsSection.tsx`,
`RejectedInstitutionsSection.tsx`, `PendingProvidersSection.tsx`,
`VerifiedProvidersSection.tsx`, registered at
`components/superadmin/client/DashboardShell.tsx:273-277, 346`.

The coin-awarded-on-approval flow is a new section built to the same shape, not a
new pattern.

---

## 3. What is missing

### 3.1 No referral system whatsoever

An exhaustive search of both repos for `referral`, `referrer`, `referred`,
`invite_code`, `inviteCode`, `invitation`, `promo`, `utm_`, and `affiliate` returns
**no matches**. All hits are false positives — `PreferredField` in the college
recommender, `Foreign Affiliated`, and a sanitizer's `RequireNoReferrerOnLinks`.

There is no `referral_code` on the user model, no attribution table, no invite
route, and no referral UI. The closest structures are `internal/analytics`
`PageVisit.Referrer` (analytics-only, never linked to signup) and
`studentdashboard.SphereInvite` (institution open-day invitations, explicitly not
a referral system).

**Do not build on `SphereInvite`.** It has a different domain, a different
lifecycle, and no referrer relationship.

### 3.2 Signup is two-phase, which complicates attribution

`auth.Service.Register` (`internal/auth/service.go:78`) creates **no database
row**. It hashes the password, generates an OTP, and stores the entire `User`
struct in an in-memory map (`internal/shared/utils/otp_store.go:20-33` — a plain
`map` with a mutex and a 10-minute TTL). The actual insert happens later in
`VerifyOTP` at `internal/auth/service.go:402`.

A referral code therefore has to be threaded through **both** phases: into
`RegisterRequest` (`internal/auth/dto.go:5`, currently five fields), through the
OTP store, and into the row that `VerifyOTP` finally creates.

**Worse, three signup paths bypass `Register` entirely**, each an independent
place to forget attribution — and each a fraud vector:

- `GoogleLoginOrRegister` — `auth/service.go:453`
- `InstitutionGoogleLoginOrRegister` — `auth/service.go:730`
- `ScholarshipProviderGoogleLoginOrRegister` — `auth/service.go:1634`

Plus `SuperadminRegister` at `:1677`. A referral system that instruments only
`Register` will silently under-count and leave an open fraud path. This is
listed as a risk in §6.

Two incidental problems in the same area, not caused by this feature but worth
noting: the OTP store is never swept except on successful verification
(`otp_store.go:43` deletes only the verified key), so repeated `/send-otp` calls
grow it unbounded; and because it is in-memory, a server restart mid-signup
destroys pending registrations silently. Neither blocks the coin system, but
`/send-otp` being unthrottled is directly relevant to fraud (see §4.2).

### 3.3 No user-facing resource upload, and no approval state machine

`internal/studyresources` has exactly one write surface, and it is superadmin-only:
`routes.go:38-47`, guarded by `authMW` + `RequireRole("superadmin","super_admin")`
from `cmd/server/main.go:599`.

The only moderation state is a boolean. From `internal/studyresources/model.go:9`:

```go
IsPublished bool `gorm:"not null;default:true;index:idx_study_resources_published"`
```

The admin UI reflects this honestly — `StudyResourcesSection.tsx:726-733` renders
only "Draft" or "Published", and grepping those two files for `approv` returns
nothing.

So "users upload, admin approves, then coins are awarded" requires **a new state
machine**, not a flag. It needs at minimum: `pending_review`, `approved`,
`rejected` (with a reason), plus reviewer identity and a review timestamp. The
`UploadedBy` column already exists (`model.go:35`) and is set from the session
user (`handler.go:336-337`), so provenance is partially there — but it is
currently never queried and carries no ownership check.

### 3.4 No phone or email verification flags

Grepping for `is_phone_verified`, `is_email_verified`, `phone_verified`, and
`email_verified` across `*.go` and `*.sql` returns **zero hits**.

Email is implicitly verified — the user row is only created after `VerifyOTP`
succeeds — but that fact is never written to a column. Phone is collected via
`UpdateProfileRequest.Phone` and **never verified by anything**.

This is a genuine blocker for fraud-sensitive coin awards. We cannot gate a
referral reward on "the invitee verified their phone", because the system does not
know whether they did. Adding a `phone_verified_at` column and a verification
flow is a prerequisite, not a nice-to-have.

### 3.5 No row-level locking anywhere in the backend

The only `FOR UPDATE` in the entire Go codebase is `FOR UPDATE SKIP LOCKED` in the
notification outbox poller (`internal/notification/repository.go:188`). There is
no `clause.Locking` usage and no optimistic version column on any model.

A coin balance decrement therefore introduces a *new* concurrency pattern into
this codebase. That is not a reason to avoid it — it is a reason to be explicit
about it, test it, and not assume the surrounding code has opinions about it.

### 3.6 No rate limiting, essentially

Exactly one rate limiter exists in 40+ backend modules, and it is a package-local
type inlined in `internal/college/handler.go:23-64`, limiting log-comparison
requests. It is not shared middleware and is not reusable as-is.

`/register`, `/send-otp`, and `/verify-otp` are **completely unthrottled**. For a
feature whose primary earn mechanic is "invite a friend", an unthrottled signup
endpoint is the single most attractive attack surface in the system.

### 3.7 Anti-abuse signals are collected but never used for decisions

`auth.UserSession` (`internal/auth/model.go:219-233`) stores `DeviceName`,
`DeviceType`, `Browser`, `IPAddress`, and `Location`, populated by
`CreateOrUpdateSession` (`auth/service.go:168-224`). Client IP is read from
`X-Forwarded-For` (`auth/handler.go:185-191`).

All of this powers an "your active sessions" display (`GetUserSessions`,
`auth/service.go:1850`). **It is never consulted for a fraud decision.** There is
no device fingerprint across accounts, no IP-reuse detection, no duplicate-account
check beyond `emailExistsAcrossTypes` (`auth/service.go:48`, called at `:79` and
`:398`).

The infrastructure to *start* collecting signals exists. The logic to act on them
does not.

---

## 4. Two pre-existing authorization holes

These are not coin-system problems. They are coin-system *blockers* in the sense
that we cannot add a paywall in front of a door that is already open.

### 4.1 Study-resource downloads are anonymous and free today

`internal/studyresources/routes.go:16` registers the download route with **no
middleware at all**:

```go
router.GET("/study-resources/:id/download", h.DownloadResource)   // no authMW
```

The only check in the handler (`handler.go:134-183`) is publication state, via
`GetPublishedResource` (`service.go:46-55`):

```go
func (s *Service) GetPublishedResource(id uint) (*StudyResource, error) {
    resource, err := s.repo.FindResourceByID(id)
    if err != nil { return nil, err }
    if !resource.IsPublished { return nil, errors.New("resource not found") }
    return resource, nil
}
```

So **any anonymous visitor can download any published file for free.** The
frontend's `if (!user)` check in
`components/studyResources/StudyResourcesPage.tsx:195-211` is a UI affordance,
not a control — and it issues a raw `window.open` with no `Authorization` header
and no service function, so it is not even enforcing anything server-side.

This is good news for planning: there is no existing gate to unwind, so the coin
gate is a net-new control rather than a modification of a working one. It is bad
news for the fact that content has been free, and the moment we add a gate,
anonymous download traffic becomes a visible regression that must be handled
deliberately.

### 4.2 The download center serves unpublished files

`internal/downloadcenter` is a **second, entirely separate** file-download domain
with a near-identical model and no shared types. It is out of scope per decision
D3, but it carries a live bug worth recording here:

`internal/downloadcenter/handler.go:234` calls `s.service.GetItem(id)`
(`service.go:40-42`), which **does not check `IsPublished`** — unlike its
`studyresources` counterpart. The route has no middleware
(`internal/downloadcenter/routes.go:12-24`).

An unpublished `DownloadItem` is therefore downloadable by ID, anonymously.
There is no test file for this module, which is the likely reason it went
unnoticed.

Also worth noting for whoever picks it up: `DownloadItem` has **no owner column
at all** (no `UploadedBy`, no `user_id` — confirmed by grep), and its
`IsPublished` defaults to `false`, the opposite of `StudyResource`. The two
domains have diverged.

### 4.3 A correction to an earlier finding in this investigation

An intermediate read of the frontend found that video "Watch" links in the
combined catalog render as a plain anchor to the stream URL with no playback token
(`StudyResourcesPage.tsx:465-472`), and inferred that this bypassed the video paywall.

**That inference was wrong, and the backend settles it.**
`internal/studyresources/stream.go:30-59` validates the `pt` token via
`utils.ParsePlaybackToken` *before* touching the database or object storage, and
returns 401 without one. So the untokenized link does not leak content — it simply
**fails**. The actual defect is a broken player: videos listed on the combined
catalog pages do not play, while the dedicated `/study-resources/video-lectures`
player works correctly because it calls `requestStudyResourcePlaybackToken` first
(`components/studyResources/videoLectures/VideoLecturePlayer.tsx:110`).

This is recorded because it changes the work item from "security hole" to "UX bug",
and because the token gate at `stream.go:30` is genuinely the right place to hang
a coin check — it is already authenticated, already per-resource, and already
fails closed.

---

## 5. Feasibility verdict

**Feasible, and the shape of the codebase is not hostile to it.** Specifically:

**Genuinely favourable:**

- No legacy coin model to unwind.
- Transaction, notification, and unique-constraint patterns all exist to copy.
- A settings store exists for economy config.
- An approval-queue UI pattern exists in the same admin dashboard.
- The video stream gate (`stream.go:30`) is already a correct, fail-closed
  authorization point that a coin check slots into cleanly.

**Genuinely unfavourable:**

- Signup is two-phase with an in-memory handoff and three Google-login
  bypasses, so referral attribution is more invasive than it first appears.
- No verification flags exist, so the strongest fraud gate is currently
  unavailable and must be built first.
- Zero rate limiting on the exact endpoints a referral economy abuses.
- No row-locking precedent, so the core concurrency primitive is new to the
  codebase and carries the highest testing burden.
- The download route is anonymous, so Phase 1 is not additive — it replaces an
  absent gate, and anonymous-access regression handling is in scope.

**No blocking unknowns remain.** Every question raised during exploration has been
answered, either by reading the code or by a chartered accountant. Two items remain
open and neither blocks the build: the coin-value question in
[05](05-economy-and-fraud.md) §2, which needs a business input (customer acquisition
cost) rather than more research, and the written reasoning behind the Nepal tax
position in [07](07-compliance-nepal.md) §2.6, which is a confirmation to obtain
rather than an open classification.

---

## 6. Risk register

Ordered by expected cost. The three that would most damage the business if
mishandled are **R4** (criminal privacy exposure, the only one with a custodial
penalty attached), **R1** (referral fraud), and **R14** (a multi-level referral
tree, which Nepal prohibits by statute rather than merely regulating). R3, R4, R13
and R19 come from the Nepal legal research in [07](07-compliance-nepal.md) and were
not visible before that work was done.

**R3 was the fourth, and it is now closed.** A Nepali chartered accountant has
confirmed that the redemption is an exempt educational service and that we are below
the VAT registration threshold, so the unit economics stand and the coin system is
not margin-negative. The row is kept rather than deleted because the question was
real, it was asked, and it was answered, and a reader of this register later needs
that history. What remains is not the classification but the two forward constraints
the answer creates, and those are tracked at **R19** because they bite on growth and
on catalogue drift, not on the classification as it stands today.

| # | Risk | Likelihood | Impact | Mitigation | Owner |
|---|---|---|---|---|---|
| R1 | **Referral fraud** — automated signup rings farm coin awards. With unthrottled signup, no verification flags, and no device tracking, the system is currently undefended. | High | High | Database-level caps and uniqueness constraints (cheap, unfakeable by a service bug) before any vendor spend. Hold-then-release. See [05](05-economy-and-fraud.md) §3. | Backend |
| R2 | **Historical data breach at launch** — turning a free public download into a paid one makes the endpoint a target, and the endpoint currently has no auth. | Medium | High | Phase 1 ships the auth gate and the coin gate together, never coin-without-auth. Add rate limiting. See [04](04-implementation-plan.md) Phase 1. | Backend |
| R3 | **VAT classification of the redemption. RESOLVED, in our favour.** A Nepali chartered accountant has confirmed that unlocking a study resource, video lecture, or mock test in exchange for coins is an exempt educational service under Schedule 1, and that the company is not required to register for VAT: turnover is below the Rs 30 lakh services threshold and no business loan over Rs 10 lakh has been taken. No coin carries output VAT, so the coins are not margin-negative and Phases 1 and 2 are unblocked. | Closed. Was High. | Closed. Was High. | No coin-economics mitigation needed. Two carry-overs, both at R19: the exemption locks us out of registration, so input VAT is never recoverable (not a regression, we are unregistered today), and the written reasoning behind the conclusion is still outstanding, so 07 §2.6 is the live list. | Finance |
| R4 | **Privacy Act 2075 s.26(1) is a criminal offence** — up to 3 years' imprisonment, Government of Nepal as plaintiff — for giving student personal data to a third party without consent. No data-protection regulator, no adequacy mechanism, no safe harbour, no breach-notification duty. A bundled "I agree to the terms" checkbox is the weak point. | Medium | Very high | Ship **no** fraud vendor at launch; schema-level fraud controls only. If a vendor is later added, granular separately-ticked consent naming vendors, data elements, countries and retention. See 07 §5.1. | Backend + Legal |
| R5 | **A future change breaks closed-loop** — a well-meaning product request ("let users gift coins to friends") silently converts points into a regulated instrument. | Medium | Very high | ADR-001, a written invariant, plus a code comment at the transfer surface and a test that fails if a transfer path appears. | Backend + Product |
| R6 | Ledger balance drifts from postings, and we cannot prove why a user holds a given balance. | Low | High | Nightly reconciliation job asserting the five invariants, not just a test suite. This failure class produced a real $60–90M shortfall at a US fintech. | Backend |
| R7 | Referral attribution loses the three Google-login paths, under-counting referrals while leaving a fraud path open. | High | Medium | Single `applyAttribution` function called from every user-creation site, with a test per path. | Backend |
| R8 | Price changes retroactively re-price historical unlocks, making entitlement ambiguous. | Medium | Medium | Snapshot `coins_paid` onto the unlock row at purchase; price config is versioned by `effective_from`. See [02](02-architecture.md) §5. | Backend |
| R9 | Coin expiry is perceived as a penalty and costs more trust than the coins are worth. | Medium | Medium | Show per-lot expiry in the UI, remind at 30/7/1 days, activity-extend earned coins. Never silently delete. See [06](06-ui-ux-spec.md). | Product |
| R10 | Upload IP/copyright exposure — students are incentivised to upload material they may not own. | Medium | High | Award only on post-approval publication, never on upload. Licensing terms, takedown process, indemnity. See [07](07-compliance-nepal.md). | Product + Legal |
| R11 | The `institution` role can rewrite coin economy config, because the existing settings admin route uses the broad role gate. | Medium | Medium | Coin config gets its own `RequireRole("superadmin","super_admin")` group, not the shared `roleMW`. | Backend |
| R12 | An unbounded balance on the user row accumulates into an unreadable liability the accountant cannot sign off. | Medium | Medium | Buckets, lots, expiry, and an explicit accounting policy decided with a CA before launch. | Finance |
| R13 | **Under-18 users.** Privacy Act 2075 s.33 requires guardian consent *and* a benefit test for under-18s. Most of the +2 / competitive-exam market is under 18, and a referral flow inherently discloses a minor's data. | High | High | Restrict the referral mechanic to 18+ accounts (self-declaration) and say so in the terms. Cheaper and more defensible than a guardian flow at scale. | Product + Backend |
| R14 | **Pyramid-scheme prohibition.** Consumer Protection Act 2075 s.16(2)(p) and s.18(e) expressly prohibit operating a "circle system, quota system, rotational system, trip system or **token system**", and "levels or series". | Low | High | Referrals stay strictly single-level, one generation. Enforce in the schema with a test, since this is a statutory prohibition and not a style preference. | Backend |
| R15 | **"Windfall gain" naming trap.** Income Tax Act 2058 defines windfall gain to include "lottery, gift, prize, baksis, award for winning", taxed under s.5/88A. | Medium | Medium | Never use prize/award/win/winner/raffle/draw/baksis/jitauri in coin marketing, terms, or notification templates. Use coins/points/credits/balance/unlock. | Product |
| R16 | **No chance mechanics.** The Lottery Act 2025 (1968) permission regime plus CPA 2075 s.16(2)(d) contest pricing. Any randomising element — lucky multiplier, scratch card, mystery reward — converts a deterministic access right into a lottery. | Low | High | Deterministic redemption only. Add a test asserting no randomising reward path exists, so an A/B experiment cannot introduce one by accident. | Backend + Product |
| R17 | **Copyright in the core product.** Copyright Act 2059 s.18 permits reproduction "for teaching and learning" only of "a small portion" and for "activities to be performed in the classroom". A commercially-exploited online past-paper bank is neither. | High | High | Licence or commission original content for paid tiers. Award coins on post-approval publication, never on upload. See 07 §5.6. | Product + Legal |
| R18 | **E-Commerce Act 2081 registration** with the Department of Commerce, Supplies and Consumer Protection, displayed on-platform. | Medium | Low | Register and display. Working precedent: existing Nepali e-commerce operators disclose it in their T&Cs. | Founder |
| R19 | **The settled VAT position degrades forward.** The R3 answer is safe on today's facts and two things can change those facts. (1) Turnover, or a business loan over Rs 10 lakh, crosses the registration threshold: registration becomes mandatory with a 30-day clock and the exempt-versus-taxable question re-opens under time pressure. (2) The coin catalogue, or a revenue line near it, drifts outside the educational perimeter: the exemption rests on a narrow reading of "educational services provided by schools and universities" and a mixed position could change the classification. Either one can put R3 back in question. | Low | High | Diarise both as growth triggers and review them quarterly, not on a date. Any business loan over Rs 10 lakh goes past the CA before it is signed. Enforce the catalogue perimeter in code and in review: a coin may only unlock a study resource, a video lecture, or a mock test. Re-confirm the exemption in writing before crossing any threshold. See 07 §2.3.1. | Finance + Product |

---

## 7. Scope boundary for Phase 1

**In scope** — `internal/studyresources` only, per decision D3:

- Coin ledger, balances, lots, and unlock entitlements
- Profile-completion earn (existing `computeProfileCompletion`)
- Referral earn, including the attribution plumbing through all four user-creation paths
- Resource-upload earn, including a new approval state machine
- The free allowance: 3 document + 1 video + 1 mock-test entitlement, 30-day expiry
- Coin gates on: document download, video playback token, mock-test submit
- Superadmin approval queue and coin-award visibility
- Student-facing wallet, unlock, and earn-progress UI

**Explicitly out of scope, deliberately:**

| Excluded | Why | Revisit when |
|---|---|---|
| `internal/downloadcenter` coin integration | D3. Separate domain, separate types, live `IsPublished` bug. | Phase 2, after the studyresources economy is validated |
| Purchasing coins with money | D1. See ADR-001. | Only via a new ADR superseding ADR-001 |
| Coin transfer between users | D1. | Never, without a new ADR |
| Cash redemption or withdrawal | D1. | Never, without a new ADR |
| Superadmin coin issuance UI | A superuser ability to mint coins is a fraud and audit risk. Grants go through named reason codes with a recorded actor. | Only with a four-eyes approval flow |
| A/B testing the economy | Tempting but premature — the funnel is not instrumented yet. | After Phase 2, once `coin_economy_daily` has data |
| Any chance-based or randomised reward mechanic | R24. Engages the Lottery Act 2025 (1968) permission regime and CPA 2075 s.16(2)(d) contest pricing. | Never, without new counsel review |
| Any third-party fraud-detection vendor | R12. Privacy Act 2075 s.26(1) makes unconsented third-party data transfer a criminal offence, with no regulator and no safe harbour. | Only with granular separately-ticked consent and a documented vendor |
