# 03 — API Contract

Endpoints, DTOs, and error semantics for the coin system. Conventions follow
`internal/mocktests` and `internal/studyresources` exactly: handlers bind and
delegate, services hold business logic, repositories hold transactions, and all
responses go through `response.Success` / `response.Error` with the standard
`{success, message, data, error}` envelope (`internal/shared/response/response.go:5-25`).

---

## 1. Route registration

```go
// cmd/server/main.go
coinAdminRoleMW := middleware.RequireRole("superadmin", "super_admin")
coins.RegisterRoutes(router, authMW, coinAdminRoleMW, coinHandler)
```

`coinAdminRoleMW` is **separate from the shared `roleMW`**. The shared gate at
`main.go:564` includes `institution` and `scholarship_provider`, which must not be
able to read or change coin pricing. This mirrors `studyResourcesRoleMW`
(`main.go:599`).

Note that `admin` is a phantom role in this codebase — it appears in allow-lists
but is never assigned to any user — so it is deliberately omitted from the allow
list. Both `superadmin` and `super_admin` spellings are included because
`RequireRole` (`internal/shared/middleware/auth.go:135-155`) does exact string
comparison after lowercasing, and both spellings exist in the codebase.

---

## 2. Student-facing endpoints

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/v1/coins/balance` | required | Wallet summary |
| GET | `/api/v1/coins/transactions` | required | Paginated ledger history |
| GET | `/api/v1/coins/allowance` | required | Free allowance status |
| POST | `/api/v1/coins/unlock` | required | Spend coins to unlock a resource |
| GET | `/api/v1/referrals` | required | My code, link, and stats |
| POST | `/api/v1/referral/validate` | optional | Validate a code pre-registration |
| GET | `/api/v1/referrals/mine` | required | My referrals and their status |
| GET | `/api/v1/study-resources/mine` | required | My uploads and approval status |
| POST | `/api/v1/study-resources` | required | Submit a resource for review |

### 2.1 `GET /api/v1/coins/balance`

The hot path — read on every wallet render and every unlock attempt. Serves from
the cached projection, never from `SUM()` over postings.

```jsonc
// 200
{
  "success": true,
  "data": {
    "total_available": 145,          // SUM(posted_balance - reserved) across user accounts
    "total_reserved": 0,             // coins promised but not yet issued; see below
    "buckets": [
      { "bucket": "FREE",   "balance": 40, "expires_at": "2026-10-26T00:00:00Z", "lot_count": 1 },
      { "bucket": "EARNED", "balance": 105,"expires_at": null,              "lot_count": 3 }
    ],
    "spend_order": [                 // FEFO, so the UI can explain what gets spent first
      { "bucket": "FREE",   "coins": 40,  "expires_at": "2026-10-26T00:00:00Z" },
      { "bucket": "EARNED", "coins": 105,"expires_at": null }
    ],
    "allowance": {
      "granted_at": "2026-09-26T04:12:00Z",
      "expires_at": "2026-10-26T04:12:00Z",
      "document_unlocks": 3,  "document_used": 1,
      "video_unlocks": 1,     "video_used": 0,
      "mock_test_unlocks": 1, "mock_test_used": 0
    }
  }
}
```

`expires_at` is surfaced per bucket, not as a global terms link. A student must be
able to see *which* coins expire and when without reading a T&C page.

`total_reserved` is currently **always 0**. The referral mechanic no longer
reserves from the referrer — coins are credited directly and the 7-day wait lives
in the referral row's state machine — so nothing in the earn path writes
`reserved`. The field stays because `Reserve`/`ReleaseReserved` are general
ledger primitives, and a reservation is the right shape for a future mechanic that
promises coins before issuing them. It is not a referral figure.

### 2.2 `GET /api/v1/coins/transactions`

Cursor-paginated over `coin_journal`, newest first, scoped to the authenticated
user. Reads from the journal and postings; the UI shows reason, amount, and
resulting balance.

```jsonc
{
  "success": true,
  "data": {
    "items": [
      {
        "journal_id": "9f2c…",
        "entry_type": "SPEND",
        "reason_code": "RESOURCE_UNLOCK",
        "amount": 40,                       // signed, from the user's perspective
        "balance_after": 105,
        "ref": { "type": "study_resource", "id": 812 },
        "description": "Unlocked: GATE 2024 Question Paper",
        "reversed": false,
        "created_at": "2026-09-26T05:02:11Z"
      }
    ],
    "next_cursor": "eyJjcmVhdGVkX2F0IjoiMjAyNi0w…"
  }
}
```

**Reversals appear in this history as their own entries**, referencing the
original. Nothing is edited or hidden, so a student who disputes a clawback can
see both the original grant and its reversal.

### 2.3 `POST /api/v1/coins/unlock`

The single write path for spending. Documented as the reference for how every
other coin mutation behaves.

```jsonc
// request
{
  "resource_type": "study_resource",   // study_resource | video | mock_test
  "resource_id": 812
}

// 200
{
  "success": true,
  "data": {
    "unlocked": true,
    "already_unlocked": false,          // true ⇒ idempotent no-op, nothing charged
    "coins_paid": 40,
    "balance_after": 105,
    "spent_from": [ { "bucket": "FREE", "coins": 40, "expires_at": "2026-10-26T00:00:00Z" } ],
    "used_allowance": false
  }
}
```

**Server-side rules — the client supplies none of these:**

- The price is resolved from `reason_code`-keyed config, never from the request.
  A client that can name the cost is an exploit.
- The allowance is checked **before** the balance. A student with 0 coins and an
  unused document allowance unlocks successfully with `coins_paid: 0` and
  `used_allowance: true`.
- The `Idempotency-Key` header is **required**. Without it, `400`.

| Status | Error | When |
|---|---|---|
| 400 | `INVALID_IDEMPOTENCY_KEY` | header missing or malformed |
| 200 | `ALREADY_OWNED` | not an error. An active unlock exists, so the endpoint returns the success body with `already_unlocked: true` and `coins_paid: 0`. A **403 is wrong here**: it is a client-error status, so the frontend would render a failure for a successful outcome, and a mobile retry, which is the normal case rather than the exception, would surface to the student as a failure. |
| 404 | `RESOURCE_NOT_FOUND` | unknown id, or not published, or wrong type |
| 402 | `INSUFFICIENT_COINS` | body carries the gap and the earning routes |
| 409 | `IDEMPOTENCY_KEY_REUSE` | key reused with a different payload |
| 423 | `ALLOWANCE_EXPIRED` | allowance lapsed and the student is not covered by a purchase |

`INSUFFICIENT_COINS` is the highest-traffic error in the feature, so its body is a
designed object, not a string:

```jsonc
{
  "success": false,
  "error": {
    "code": "INSUFFICIENT_COINS",
    "message": "You need 40 more coins for this resource.",
    "data": {
      "required": 40, "available": 25, "shortfall": 15,
      "expires_in_days": 12,
      "ways_to_earn": [
        { "code": "REFERRAL",  "label": "Invite a friend",  "potential": 60 },
        { "code": "PROFILE",   "label": "Complete your profile", "potential": 15 },
        { "code": "UPLOAD",    "label": "Upload a study resource", "potential": 80 }
      ]
    }
  }
}
```

`ways_to_earn` is computed server-side from the student's actual remaining
eligibility — a completed profile does not appear. It is the same payload the UI
needs to render the motivating empty state, so the two cannot drift.

### 2.4 `GET /api/v1/referrals`

Owner-scoped: every query filters on the caller's own id, and no parameter can
name another account. Unauthenticated is `401`; any database failure is `500`.
There is no `403` and no `404` — a caller with no referrals gets zeros, which is
a true answer rather than a missing resource.

> **Corrected against the implementation.** This section previously specified
> `GET /api/v1/referral/me` and a `coins_pending` figure. The path is now
> `/api/v1/referrals` (one path per resource), and `coins_pending` is **gone**:
> a referral payout no longer reserves from the referrer, so there is no reserved
> balance to report, and a "pending" figure pointing at a column nothing writes
> would be the exact disagreement the old sentence existed to prevent.

```jsonc
{
  "success": true,
  "message": "Referral summary",
  "data": {
    "referral_code": "STU-7K2M9Q",                    // null before the backfill reaches the account
    "referral_link": "https://studsphere.com/r/STU-7K2M9Q",   // null if no base URL is configured
    "qualification_available": false,                 // see below
    "qualification_notice": "Referral StudsTokens cannot be added yet. …",
    "referral_hold_days": 7,                          // so the UI renders the same date this body computes
    "stats": {
      "invited": 14, "qualified": 9, "pending": 3, "expired": 1, "rejected": 1,
      "coins_earned_total": 540,
      "this_month_qualified": 4,
      "monthly_cap_remaining": 6,
      "lifetime_cap_remaining": 60
    },
    "referrals": [                                    // the caller's own rows, newest first
      {
        "referral_id": 913,
        "status": "pending",
        "referred_kind": "user",
        "referred_at": "2026-09-26T06:00:00Z",
        "eligible_at": "2026-10-03T06:00:00Z",       // referred_at + referral_hold_days
        "state": "waiting_for_window",
        "awarded_coins": 0
      },
      {
        "referral_id": 907,
        "status": "qualified",
        "referred_kind": "user",
        "referred_at": "2026-09-14T06:00:00Z",
        "eligible_at": "2026-09-21T06:00:00Z",
        "qualified_at": "2026-09-20T11:30:00Z",       // met §5.2's bar
        "settled_at": "2026-09-22T02:00:00Z",         // coins granted — a separate fact, may be days later
        "state": "settled",
        "awarded_coins": 60
      }
    ]
  }
}
```

**`coins_pending` is replaced by per-referral `status` + `eligible_at`.** "Three
referrals have not paid yet" is answerable by counting rows the page is already
rendering, and it cannot disagree with anything because there is no second number
to disagree with. Do not reintroduce an aggregate.

`stats.pending` counts **only** rows in status `pending` — those that have not
qualified and may still pay. Terminal rows are counted in `expired` and `rejected`
instead, so `invited == qualified + pending + expired + rejected` holds and no row
that can never pay is ever reported as one that might.

Two subtleties a client must not get wrong:

- **`qualified` counts `clawedback` too.** A referral that qualified and was then
  reversed on a fraud finding *did* qualify. Rendering "0 qualified" to a student
  who was genuinely credited and then had it taken back is worse than showing no
  number at all, so clawedback is folded into the qualified count and reported as
  a separate per-row `state`. `coins_earned_total` sums `awarded_coins` and so
  still includes a clawed-back lot — which is why it is a separate number from the
  wallet balance rather than derived from it.
- **`qualified` is not "paid".** It is a status, and a qualified row is still
  waiting for its window to close. Only `settled_at` says the coins moved.

The cap figures come from `referral_cap_slot` — the table the settlement is
refused by — not from counting referral rows, because a cap that counts something
other than what it pays is decorative. `lifetime_cap_remaining` is the derived
ceiling from 05 §2.4, not a counter.

Per-row `state` is derived, not stored, and is what the page should render. It
exists because a raw `status` cannot tell "come back on the 3rd" from "your friend
has not finished yet", and those are different things for a student to do.

| `status` | `state` | Meaning |
|---|---|---|
| `pending`, window still open | `waiting_for_window` | The fraud window is still running. Nothing to do. |
| `pending`, window closed | `waiting_for_invitee` | The invitee has not finished. Not the student's fault (06 §7). |
| `qualified` | `settled` | Coins granted. |
| `expired` | `expired` | Terminal, never pays. |
| `clawedback` | `clawedback` | Paid, then reversed on a fraud finding. |
| `rejected` | `rejected` | Disqualified before payout. |

A qualified row is emitted as `settled` even in the same run that qualified it, so
a client must use `settled_at` — not `state` — to decide whether coins have moved.

`reason` is set only for a terminal row and is never a penalty sentence (06 §7).

Two further states exist as **frontend-only** concepts and are never emitted by
this endpoint: `unverifiable` (render when `qualification_available` is false) and
`settling` (qualified, window still running — currently unreachable, since the
qualification pass settles a row in the same transaction that qualifies it, so
there is no observable moment in between).

**`qualification_available` is `false` in every deployment today.** It is derived
from the same `PhoneVerification` port the qualification pass refuses on when it
is nil, so the page cannot claim referrals are earnable while the pass pays none.
It is the one field here that is about the server rather than the student, and it
exists so the page can say *why* nothing has paid instead of spinning forever.

`referral_link` is `null` rather than a bare `/r/CODE`: a relative link is
meaningless in a copied string or an email, and a link that silently omits its
host ships broken on every host that is not localhost.

The invitee's identity is **not** exposed — no name, email, phone, or referred
user id (07 §5.2). A sequential id plus `referred_kind` is the most a referral
discloses about the other person. `expires_at` is not currently emitted for a
referral row; the per-lot expiry dates live on the wallet endpoint, which reads
the caller's own lots.

### 2.5 `POST /api/v1/referral/validate`

Optional auth — this runs on the signup form before an account exists. Used by
the UI to show "invited by X" and to catch typos client-side. **Purely
advisory:** the authoritative check happens at user creation, and this endpoint
must never be trusted to grant anything.

```jsonc
// request  { "code": "STU-7K2M9Q" }
// 200      { "success": true, "data": { "valid": true, "referrer_first_name": "Aarav" } }
// 200      { "success": true, "data": { "valid": false, "reason": "NOT_FOUND" } }
```

Reasons: `NOT_FOUND`, `SELF`, `SUSPENDED`, `CAP_REACHED`. Returning a reason
distinguishes a typo from a self-referral from a capped referrer, which the
signup form needs to say something useful.

### 2.6 `POST /api/v1/study-resources` — user upload

New public write surface. The existing write route
(`internal/studyresources/routes.go:42`) stays superadmin-only; this is a separate
`authMW`-only route that creates a row in `pending_review`.

Documents only in v1 — video upload requires ffmpeg transcoding
(`handler.go:270-307`, HTTP 503 if absent) and would multiply the abuse surface
for no launch benefit. Videos stay admin-uploaded.

```jsonc
// multipart/form-data: title, description, resource_type, course, year, file
// 201
{
  "success": true,
  "data": {
    "id": 913, "status": "pending_review", "uploaded_at": "2026-09-26T06:00:00Z",
    "coins_awardable": 80, "coins_note": "Awarded when an admin approves and publishes this resource."
  }
}
```

The existing validation applies unchanged — 16-extension whitelist and a hard 20MB
cap (`types.go:177-182`, `handler.go:28`).

---

## 3. Admin endpoints

All under `/api/v1/admin/coins/`, all on `coinAdminRoleMW`.

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/v1/admin/coins/economy` | Read economy config |
| PUT | `/api/v1/admin/coins/economy` | Update economy config |
| GET | `/api/v1/admin/coins/economy-daily` | Health metrics time series |
| GET | `/api/v1/admin/coins/users/:id` | Support view: balance, lots, journal |
| POST | `/api/v1/admin/coins/adjust` | Manual signed adjustment, reason-coded |
| GET | `/api/v1/admin/study-resources/pending` | Approval queue |
| POST | `/api/v1/admin/study-resources/:id/approve` | Approve, publish, award |
| POST | `/api/v1/admin/study-resources/:id/reject` | Reject with a reason |

### 3.1 `PUT /api/v1/admin/coins/economy`

```jsonc
{
  "prices": { "study_resource": 40, "video": 90, "mock_test": 60 },
  "awards": {
    "profile_complete": 25, "profile_instalment": 5, "profile_instalments": 5,
    "referral_referrer": 60, "referral_referred": 25,
    "resource_approved": 80
  },
  "allowance": {
    "document_unlocks": 3, "video_unlocks": 1, "mock_test_unlocks": 1,
    "expires_in_days": 30
  },
  "expiry": { "free_days": 30, "earned_days": 365, "activity_extend_days": 180 },
  "referral": { "monthly_cap": 10, "lifetime_coin_cap": 600, "hold_days": 7 },
  "clawback_window_days": 180
}
```

**Validation on write**, because a misconfigured economy is a support incident:

- all amounts are non-negative integers, `bigint`-safe
- `allowance.document_unlocks` ≥ 0, and the per-class values are consistent
  (a non-zero `video_unlocks` requires `prices.video` > 0)
- `expiry.free_days` > 0; `earned_days` > 0
- `referral.hold_days` ≥ 0
- **prices are never applied retroactively.** Changing `prices.study_resource`
  from 40 to 60 affects only unlocks created after the change, because
  `resource_unlock.coins_paid` is a snapshot ([02](02-architecture.md) §5).

Every write appends a `coin_economy_config_version` row with the actor and
timestamp, so a pricing change is reconstructible. `ValidateRegistry`-style boot
fatalism is not appropriate here — a bad price should fail the request, not the
server.

### 3.2 `POST /api/v1/admin/coins/adjust`

A signed correction, because sometimes the ledger is right and reality is wrong
(a duplicate award, a bad clawback). It writes an `ADJUST` journal with
`created_by = 'admin:<id>'` and a mandatory `reason_code`.

**Deliberately not a "set balance" endpoint.** There is no way to make a balance
be a number; adjustments move it. A superadmin cannot mint arbitrary coins
without a recorded reason, which is the audit trail an accountant needs and the
control that stops a compromised admin session from creating unlimited value.

Request includes `Idempotency-Key`; a replayed adjustment returns the original
journal rather than applying twice.

### 3.3 Approval → award

`POST /api/v1/admin/study-resources/:id/approve` performs three things in **one
transaction**: sets `status = 'approved'`, sets `is_published = true`, and grants
80 `EARNED` coins to `uploaded_by`, then emits the notification via
`notification.NotifyTx` in that same transaction.

All three or none. An approved-but-unoffered resource and an awarded-but-unpublished
resource are both support tickets.

Coins are granted **on publication, never on upload** — see [05](05-economy-and-fraud.md)
§4 for the IP and incentive reasons.

---

## 4. Error semantics

Package-level sentinels in `internal/coins/errors.go`, matched with `errors.Is`,
per the house style at `internal/mocktests/service.go:13-23`.

```go
var (
    ErrInsufficientCoins  = errors.New("insufficient coins")
    ErrIdempotencyKeyReuse = errors.New("idempotency key reused with a different payload")
    ErrAccountFrozen       = errors.New("coin account is frozen")
    ErrNotFound            = errors.New("not found")
    ErrAccountExists       = errors.New("account already exists")
    ErrCapReached          = errors.New("referral cap reached")
    ErrSelfReferral        = errors.New("self-referral is not allowed")
    ErrImmutable           = errors.New("entry is immutable")
    ErrInvalidArgument     = errors.New("invalid argument")
)
```

Status mapping lives in the handler, per `mocktests/handler.go:247-258`:

```go
func statusForError(err error) int {
    switch {
    case errors.Is(err, ErrInsufficientCoins):   return http.StatusPaymentRequired
    case errors.Is(err, ErrIdempotencyKeyReuse): return http.StatusConflict
    case errors.Is(err, ErrNotFound):            return http.StatusNotFound
    case errors.Is(err, ErrAccountExists):       return http.StatusConflict
    case errors.Is(err, ErrCapReached):          return http.StatusConflict
    case errors.Is(err, ErrSelfReferral):        return http.StatusBadRequest
    case errors.Is(err, ErrInvalidArgument):     return http.StatusBadRequest
    case errors.Is(err, ErrAccountFrozen):       return http.StatusLocked
    default:                                     return http.StatusInternalServerError
    }
}
```

`402 Payment Required` for insufficient coins is deliberate and carries the
earning payload — it is a designed state in the funnel, not a failure.

---

## 5. Notification events

The registry is closed and boot-validated (`notification.ValidateRegistry()` at
`cmd/server/main.go:173` calls `logger.Fatal` on any violation). Adding a
constant without a `Registry` row prevents the server from starting.

| Constant | Key | Recipient | Transactional |
|---|---|---|---|
| `EventCoinsCredited` | `coins.credited` | explicit user | no |
| `EventCoinsDebited` | `coins.debited` | explicit user | no |
| `EventReferralQualified` | `referral.qualified` | referrer | no |
| `EventReferralReleased` | `referral.released` | referrer | no |
| `EventReferralRevoked` | `referral.revoked` | referrer | no |
| `EventAllowanceExpiring` | `allowance.expiring` | explicit user | no |
| `EventAllowanceExpired` | `allowance.expired` | explicit user | no |
| `EventResourceApproved` | `resource.approved` | uploader | no |
| `EventResourceRejected` | `resource.rejected` | uploader | no |

The six reminder/sweep events fire from a scheduled job, not from a request, and
each is emitted inside the same transaction as the state change that caused it —
so a rolled-back expiry never produces an "your coins expired" notification.

Title and body are Go `text/template` rendered from the event `Data` map
(`notification/registry.go:239`). Templates are data, so a `{{.amount}}` is
interpolated, never evaluated.

**Copy constraint for all templates:** never render "free", never render a
currency amount. A coin has no cash value because it cannot be purchased (D1), so
a value claim is a misleading-advertisement exposure. See
[07](07-compliance-nepal.md).
