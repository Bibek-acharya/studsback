# 04 — Implementation Plan

Phased delivery, migration runbook, test strategy, and rollout. Each phase ends in
a shippable state.

Conventions from `AGENTS.md`: backend tests live beside their Go package and use
the standard `testing` package; frontend tests are Jest with `ts-jest`, named
`*.test.ts` / `*.test.tsx`, with service tests under `services/__tests__/`. Run the
narrow test first, then the relevant full suite. `go test ./...`, `go build ./...`,
`npm run lint`, `npm test -- --runInBand`.

---

## 1. Phase overview

| Phase | Scope | Ships | Est. |
|---|---|---|---|
| **0** | Pre-flight hardening | Nothing user-visible. Closes two auth holes, adds rate limiting and verification. | 3–5 d |
| **1** | Ledger core | Schema, grant/spend, reconciliation job. No user-facing change. | 5–7 d |
| **2** | Spend gates + allowance | Resources require coins or an allowance entitlement. Monetization begins. | 5–7 d |
| **3** | Earn mechanics | Profile, referral, upload-and-approve. | 7–10 d |
| **4** | Admin console + monitoring | Approval queue, economy config, health dashboard. | 4–6 d |
| **5** | Comms + tuning | Expiry notices, reminders, economy tuning from real data. | 3–5 d |

Phases 0 and 1 are the ones to get right. Everything after is comparatively
mechanical, and a mistake in Phase 1 is expensive to unwind because it is baked
into posted rows.

**Total: roughly 5–6 engineer-weeks**, excluding the Nepal legal sign-off in
[07](07-compliance-nepal.md) and excluding designer time for
[06](06-ui-ux-spec.md).

### 1.1 Parallelisation

Phases 0, 1, and the non-blocking parts of 2 are independent and can run
concurrently by different people. Phase 3 depends on Phase 1. Phase 4 depends on
Phases 1 and 3. The ledger is the critical path.

---

## 2. Phase 0 — Pre-flight hardening

No coin code. Every item here is a prerequisite for shipping Phase 2 safely, and
three of the four are latent bugs that exist today.

### 2.1 Fix the anonymous download endpoints

**`internal/studyresources/routes.go:16`** — add `authMW` to the download route.
Currently no middleware at all ([01](01-current-state-and-feasibility.md) §4.1).

**`internal/downloadcenter/handler.go:234`** — the `GetItem` lookup does not check
`IsPublished`. Out of scope for coins (D3) but the bug is live, so fix it and add
the missing test file for the module.

**Order matters:** these go out *before* the coin gate, so the fix is
"add authentication" rather than "add payment", which is a much easier thing to
explain in a changelog and to roll back.

### 2.2 Add rate limiting as shared middleware

One rate limiter exists, inlined in `internal/college/handler.go:23-64`. It is
package-local and not reusable. Promote the pattern to
`internal/shared/middleware/ratelimit.go` and apply it to:

| Endpoint | Limit | Why |
|---|---|---|
| `/register` | 5 / hour / IP | Blocks automated account creation |
| `/send-otp` | 5 / hour / IP + 3 / hour / email | Caps OTP abuse; also fixes the unbounded `otp_store` growth |
| `/verify-otp` | 10 / hour / IP | Brute force |
| `POST /api/v1/study-resources` | 5 / day / user | Stops upload spam |
| `POST /api/v1/coins/unlock` | 20 / hour / user | Bounds FEFO allocation work |

**Rate-limit on account, device, and phone — not IP alone.** Indian and Nepali
campus networks put thousands of students behind one NAT, so an IP-only limit
punishes exactly the users we want. IP is a soft signal; the account and device
identifiers are the real keys. This matches the vendor guidance that shared IPs
are a false-positive source and insufficient evidence on their own.

### 2.3 Add phone verification

`is_phone_verified` does not exist ([01](01-current-state-and-feasibility.md)
§3.4). Add `phone_verified_at timestamptz` to `auth.User` and an OTP-to-phone
flow. The OTP infrastructure already exists for email; this is the same shape
against a different identifier.

**Gate the referral award on it.** A referral reward paid because someone typed a
number is not a referral reward. This is a launch blocker for the referral earn,
not a nice-to-have.

### 2.3a Restricting referrals to 18+ accounts

Privacy Act 2075 s.33 requires guardian consent **and** a benefit test for users
under 18. Two facts collide here: most of the +2 / competitive-exam audience is
under 18, and a referral flow inherently discloses a minor's data — the referrer
learns that their invitee signed up, and the invitee's data is exposed to another
person. So the audience the referral mechanic depends on is the audience the
consent rule most constrains.

**Therefore referral earn is gated on an 18+ self-declaration**, stated in the
terms. The under-18 path returns a clear, non-punitive message rather than an
error.

The alternative — a guardian-consent flow — is **documented as rejected-for-now,
not forgotten**. It is more expensive to build, harder to operate correctly, and
raises the bar for every account at the point of signup. Revisit only if the
commercial value of under-18 referral is proven large enough to pay for that
gate. See [01](01-current-state-and-feasibility.md) R21 and
[07](07-compliance-nepal.md).

### 2.4 Extract `currentUserID` to `internal/shared`

It is duplicated byte-for-byte at `studyresources/playback.go:73` and
`mocktests/handler.go:226`, and `studyresources/handler.go:336` uses an unsafe
variant that silently yields `0`. Extract the strict form once, use it in the third
consumer, and deprecate the unsafe call.

### 2.5 Fix the broken video link in the catalog

`StudyResourcesPage.tsx:465-472` renders an untokenized stream URL, so videos
listed on the combined catalog pages return 401. Cheap fix: route through
`requestStudyResourcePlaybackToken` like
`videoLectures/VideoLecturePlayer.tsx:110` does. Do it now so the coin gate lands
on a single, correct code path.

---

## 3. Phase 1 — Ledger core

The highest-risk phase. No user-facing behaviour changes; the ledger ships dark and
is exercised by tests and the reconciliation job before any coin is granted.

### 3.1 Schema

Three artefacts, all three required ([02](02-architecture.md) §9):

1. **GORM models** in `internal/coins/model.go`, registered in the `AutoMigrate(...)`
   list at `cmd/server/main.go:187-300`. This is what actually creates the tables.
2. **`migrations/2026xxxx_create_coin_ledger.go`** following the
   `func(db *gorm.DB) error` convention, for the SQL-only bootstrap path.
   Idempotent: `CREATE TABLE IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`.
3. **`internal/coins/ensure_indexes.go`** with `EnsurePostgresIndexes(db)`, called
   from the `logger.Warn` block at `main.go:339-348`.

Artefact 3 is not optional. `AutoMigrate` cannot create partial indexes, and this
schema depends on five of them. The codebase has already shipped this exact bug
once — `internal/notification/ensure_indexes.go:9-17` documents a fresh-boot server
where every preferences PUT failed with `42P10 ON CONFLICT without matching
constraint`.

**Seed the chart of accounts** in migration 2, guarded by `ON CONFLICT DO NOTHING`:
`earned_faucet`, `redeemed_sink`, `expired_burn`.

### 3.2 Ledger core

`internal/coins/ledger.go` — `Grant`, `Spend`, `Reverse`, `Reserve`, `ReleaseReserved`.

Every one follows the same shape:

```
BEGIN
SET LOCAL lock_timeout = '3s'
SET LOCAL idle_in_transaction_session_timeout = '10s'
pg_advisory_xact_lock(hashtextextended('coin:user:'||userID, 0))
INSERT INTO coin_journal (...) ON CONFLICT (scope, idempotency_key) DO NOTHING RETURNING id
  -- no row ⇒ replay: compare fingerprint, return the original or 409
  ... FEFO allocation, postings, balance updates ...
COMMIT
```

Transactions live in `repository.go`, matching the 17 existing call sites.

### 3.3 Test strategy

This is the part that decides whether the ledger is trustworthy. Three tiers.

**Tier 1 — property tests on the FEFO allocator.** Mandatory, and the direct
response to a bug that was hit during schema validation: an allocator loop that
*overwrote* its running total instead of accumulating it, so a spend of 120
became 145. **No database constraint caught it** — the postings still summed to
zero and the cache still matched. Properties to assert:

- `sum(consumed_delta) == requested_amount`, always
- no lot is over-consumed (`consumed <= granted` holds after every spend)
- the set of touched lots is exactly the FEFO prefix of the open-lot ordering
- spending more than the total available changes nothing
- spending exactly the total available leaves all lots fully consumed
- the result is independent of the physical order rows are returned in
- never-expiring lots are touched last

**Tier 2 — concurrency tests against real PostgreSQL.** Not mocked. The
concurrency claim is the load-bearing one, so it is tested against the engine that
will actually run it. Reproduce the validation that was already performed once
during design: N concurrent spends against a known balance must produce exactly
`floor(balance/cost)` successes, a final balance of exactly the remainder, and
zero rows from rejected transactions. Also test concurrent grant-against-spend, and
idempotent replay under concurrency.

**Tier 3 — invariant tests** for the five assertions in
[02](02-architecture.md) §11, run against a freshly built fixture.

Test file placement follows the house pattern: `internal/coins/service_test.go`,
`internal/coins/ledger_test.go`, `internal/coins/ledger_pg_test.go`. A
`//go:build integration` tag on the Postgres-dependent ones, since `go test ./...`
should not require a live database.

### 3.4 Reconciliation job

A scheduled job asserting the five invariants, alerting on violation. Not a test —
a production monitor. See [02](02-architecture.md) §11 for the SQL and for why
this is the cheapest insurance in the system.

Wire it wherever cron already runs in this codebase; if nothing does, a goroutine
on a ticker in `main.go` behind a `coins` env flag is sufficient for v1.

### 3.5 Exit criteria

- `go build ./...` and `go test ./...` pass
- Property tests pass over a randomised fixture corpus
- 20-way concurrency test passes on real Postgres
- Migrations apply cleanly to a **fresh** database *and* to a database created
  only by AutoMigrate — test both paths, they are genuinely different
- The ledger can grant, spend, expire, and reverse, with a balance that always
  equals the sum of its postings
- No user-facing change is live

---

## 4. Phase 2 — Spend gates and the allowance

Monetization begins. This is the phase that changes what users can do, and the one
most likely to produce support tickets.

### 4.1 Backend

1. `user_free_allowance` — per-class entitlement counts and expiry. Not a coin
   grant; a distinct table, because "one video and one mock test, ever" is a rule
   about entitlement shape.
2. `resource_unlock` ([02](02-architecture.md) §5), with the unique constraint that
   caps fraud damage.
3. Gate + spend at the three enforcement sites ([02](02-architecture.md) §10):
   - `studyresources/playback.go:27` — video, before `IssuePlaybackToken`
   - `studyresources/handler.go:134` — document download
   - `mocktests/service.go:223` — mock-test submit

   Each checks entitlement first, then balance, and is **idempotent**: an existing
   active unlock returns 200 with `already_unlocked: true` and charges nothing.
   Mobile retries are the normal case, not the exception.

4. Mint the `coins.unlocked` notification via `NotifyTx` in the same transaction.

**Order of operations, non-negotiable: check auth → check entitlement → check
balance → spend → mint notification → commit.** A spend that succeeds without a
corresponding unlock is a charge with no delivery.

### 4.2 Frontend

Per [06](06-ui-ux-spec.md):
extract the duplicated card JSX into a `ResourceCard` component; add the coin
badge and the full state matrix; build the unlock confirmation modal on the
existing pattern at `StudyResourcesPage.tsx:503-541`; build the insufficient-funds
state from the server's `ways_to_earn` payload; wallet surface; repurpose the dead
`components/user/dashboard/sections/ResourcesSection.tsx` rather than adding a new
page. Replace the raw `window.open` with a real service call and delete the dead
`getStudyResourceDownloadUrl` helper (`services/studyResourcesApi.ts:235`).

Every new route needs `loading.tsx` and `error.tsx` per `studsnew/AGENTS.md`.

### 4.3 The anonymous-access regression

This is the thing to plan for, not discover. Documents that anyone could download
yesterday now require coins or an allowance. Expect:

- Cached search results and shared links to 401
- Bookmarks to resources to start failing
- Support volume on day one

Mitigations, in order:

1. **Generous allowance** (3 + 1 + 1) so a real student is not blocked
2. **A clear locked state**, never a raw 401 — the UI must distinguish
   "log in", "you have no coins", and "you already own this"
3. **A `Referral`-only exception is not acceptable**; do not add a bypass
4. **Analytics on 401 rate by route** before launch, so a regression is visible
   within an hour rather than a week

### 4.4 Rollout and kill switches

Every gate reads its configuration through the cached `coin_economy` config. That
gives us kill switches without a deploy:

| Switch | Effect |
|---|---|
| `gates_enabled.documents` | Revert documents to free |
| `gates_enabled.video` | Revert video to free |
| `gates_enabled.mock_tests` | Revert mock tests to free |
| `allowance.enabled` | Grant the allowance without debit |

**Ship with `gates_enabled` false**, then enable one gate at a time — documents
first, since they have the highest volume and the clearest allowance coverage.
This is the cheapest way to find out whether the ledger is correct in production
before it is load-bearing for revenue.

**An important non-invariance:** turning a gate off must **not** refund coins
already spent. A student who paid keeps the unlock. Reverting a gate stops future
spends; it does not unwind history.

---

## 5. Phase 3 — Earn mechanics

### 5.1 Profile completion

Hook the existing `computeProfileCompletion`
(`internal/studentdashboard/service.go:327`). Award in 5 instalments of 5 coins
for a total of 25, so the ledger sees repeated grants and the UX can show
progression.

Award against `reward_grant` with a unique `(user_id, award_code)`, where
`award_code` is `PROFILE_STEP:<n>` for n in 1..5. Because the function is
recomputed per request and can *decrease*, the award is driven by crossing a
threshold since the last award, not by the absolute value. A student who drops from
100% to 90% and back does not re-earn.

This runs in the `UpdateProfile` path
(`internal/auth/dto.go:77-89`), which is the only place the fields change.

### 5.2 Referral

The most invasive item, because of the four user-creation paths
([02](02-architecture.md) §10.1):

1. Generate `referral_code` on the `User` model; backfill for existing users
2. Add `ReferralCode` to `RegisterRequest`; carry it through the OTP store
3. Add `applyAttribution` and call it from `VerifyOTP` (`auth/service.go:402`),
   `GoogleLoginOrRegister` (`:453`), `InstitutionGoogleLoginOrRegister` (`:730`),
   `ScholarshipProviderGoogleLoginOrRegister` (`:1634`)
4. `/r/[code]` invite route that captures the code and persists it client-side
5. `user_referral` with the uniqueness constraints from
   [02](02-architecture.md) §10.1, plus the caps from
   [05](05-economy-and-fraud.md) §3
6. Qualification: invitee completes profile **and** verifies phone
7. `reserved` hold for 7 days, then release or clawback
8. `/referral` student surface with the pending state made explicit

**A test per user-creation path.** This is the highest-value test in the phase: a
missed path is simultaneously an under-counted referral and an open fraud route.

#### Single-level enforcement

Consumer Protection Act 2075 s.18(e) expressly names a "token system" and
s.16(2)(p) prohibits "levels or series". A multi-level referral tree is therefore
**prohibited**, not regulated — this is a pyramid-scheme prohibition, not a tax
question, so there is no compliant version of a deeper tree to negotiate toward.

Our design credits only the *direct* referrer, which makes it single-level by
construction. But that is an invariant, not an accident, and it will be broken by
a well-meaning request ("let B's invitees also thank A") the same way a transfer
path would be. Enforce it with two tests:

- A second-generation referral — C referred by B, where B was referred by A —
  credits B and **never** A.
- No code path can propagate a reward up more than one hop.

See [01](01-current-state-and-feasibility.md) R22 and
[07](07-compliance-nepal.md).

### 5.3 Upload and approval

1. `pending_review` / `approved` / `rejected` status, reviewer identity, reject
   reason on `StudyResource`
2. `POST /api/v1/study-resources` — documents only in v1 (video needs ffmpeg
   transcoding at `handler.go:270-307`; that is a separate abuse surface)
3. Admin queue built on the `PendingInstitutionsSection` pattern
4. `POST /approve` — status, publish, and 80-coin grant in one transaction, then
   `NotifyTx`
5. Rejection gives a reason to the student and awards nothing

**Coins on publication, never on upload.** Two independent reasons: paying before
moderation creates an incentive to upload material the platform cannot clear, and it
puts copyright exposure on the platform for content it has not reviewed. See
[07](07-compliance-nepal.md).

---

## 6. Phase 4 — Admin console and monitoring

- Approval queue with bulk actions and reject reasons
- Economy config editor with the validation from [03](03-api-contract.md) §3.1, and
  a config version history
- Support view `GET /admin/coins/users/:id` — balance, lots, and full journal. This
  is the tool that answers "why does this student have 30 coins", and its existence
  is the practical payoff of the ledger design
- `POST /admin/coins/adjust` — reason-coded, audited, never "set balance to N"
- `coin_economy_daily` rollup and the health dashboard from
  [05](05-economy-and-fraud.md) §5
- A public coin table page — conversion rate, what is included, expiry, and the
  treatment of unclaimed coins. This is **mandatory** under Consumer Protection
  Act 2075 s.16(2)(n), which requires a professional service provider to specify
  price, quality, venue and time. A terms-page clause is not the disclosure the
  provision contemplates; it needs to be a page a student can actually read
  before they spend.

---

## 7. Phase 5 — Comms and tuning

- Expiry sweep job: `EXPIRE` journal → `expired_burn`, batched with
  `FOR UPDATE SKIP LOCKED` so workers parallelise. Idempotent via
  `expire:lot:<id>` keys, so a second sweep finds nothing.
- Reminders at 30 / 7 / 1 days, per lot, via the notification events in
  [03](03-api-contract.md) §5
- Never silently delete an expired balance. Rotate into a low-value clarity credit
  where possible, and always tell the student.
- Tune prices, earn rates, and the referral cap against the health metrics — this
  is the first phase where the numbers should be adjusted from evidence rather than
  from the table in [05](05-economy-and-fraud.md) §2.

---

## 8. Verification

Per `AGENTS.md`: run the narrow test first, then the relevant full suite.

```bash
# Backend
cd studsback
go build ./...
go test ./internal/coins/... -v
go test ./internal/studyresources/... ./internal/mocktests/... ./internal/auth/...
go test ./...                    # full suite
gofmt -l internal/coins migrations

# Integration (needs a live database)
go test -tags integration ./internal/coins/... -run 'Concurrent|FEFO|Idempot' -v

# Frontend
cd studsnew
npm run lint
npm test -- services/__tests__/coins --runInBand
npm test -- --runInBand          # full suite
```

**Migration verification, both paths.** Apply the migrations to (a) a fresh
database and (b) a database created only by `AutoMigrate`, and confirm the indexes
exist in both. This is the specific failure the codebase has hit before.

**Pre-launch checklist:**

- [ ] `phone_verified_at` populated and the referral award gated on it
- [ ] Referral earn gated on 18+ declaration; under-18 path returns a clear,
      non-punitive message
- [ ] Single-level referral enforced by test — a second-generation referral
      credits B and never A
- [ ] Public coin table page live (conversion rate, what is included, expiry,
      unclaimed-coin treatment) per CPA 2075 s.16(2)(n)
- [ ] Reconciliation job running on a schedule and alerting
- [ ] All five invariants clean
- [ ] `gates_enabled` defaults to `false`
- [ ] Notification registry entries added to **both** the constants and the
      `Registry` map (missing the second prevents boot)
- [ ] Mock-test DTO invariant preserved — `is_correct` and explanations still
      stripped (`services/mockTestsApi.ts:210-225`)
- [ ] No "free" and no currency amount in any coin-adjacent string
- [ ] Rate limits live on `/register`, `/send-otp`, `/verify-otp`
- [ ] Nepal legal sign-off obtained ([07](07-compliance-nepal.md))

---

## 9. Rollback

| Situation | Action |
|---|---|
| Ledger defect found post-launch | `gates_enabled` → false. Everything reverts to free. **Do not refund** coins already spent. |
| Bad economy config | Revert the config version. Prices are not retroactive, so nothing rewrites. |
| Bad grant batch | Issue a `REVERSAL` journal per grant via `POST /admin/coins/adjust`. Never edit a journal. |
| Schema defect | The ledger is append-only and the postings are the truth, so the cache can always be rebuilt from them. Prefer fixing forward. |

**The kill switch is the rollback.** That is the main reason Phase 2's config is
designed to be settable at runtime rather than requiring a deploy.
