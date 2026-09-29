# 02 — Architecture

The coin ledger design, the integration points in the existing Go codebase, and
the decisions that constrain both.

> **On naming.** The product is called **StudsToken** everywhere a student or an
> admin sees it. This document says "coin" throughout because it describes the
> *internal* implementation, and the internal names are unchanged for now: the
> `internal/coins` package, the `coin_account` / `coin_journal` / `coin_posting` /
> `coin_lot` tables, and the `coin_bucket` enum. The schema below is the version
> that was executed and validated against a live PostgreSQL instance, so renaming
> it means re-verifying that work. When the product name is finalised, the internal
> rename follows. See the naming note in [README](README.md).

---

## 1. The central design decision: a ledger, not a balance column

The obvious implementation is a `coin_balance` column on the user row, debited
with `UPDATE users SET coins = coins - 50 WHERE id = ?`. It is wrong, and it is
worth being precise about why, because each reason maps to a real capability we
will need.

**It cannot answer "why does this student have 30 coins?"** A balance is a
summation with no provenance. When a user disputes an award, or we need to
reconstruct whether a clawback was already applied, there is nothing to inspect.
The fix is that a grant, a spend, and a reversal are all *records* — journal
entries with signed postings — and the balance is a cache of their sum.

**It cannot reverse a mistaken grant.** With a balance column, refunding 60
erroneously-awarded coins means either decrementing the balance (which destroys
the evidence that the grant happened) or writing a new "I took back 60 coins"
event with no link to the original. Both leave you unable to answer "was this
already refunded?" a second time. With a ledger, a reversal is a new journal
entry pointing at its original via `reversal_of`, and the original is never
modified.

**Its correctness is a coincidence.** `UPDATE ... WHERE coins >= 50` is atomic and
safe in isolation, but combined with any other read-then-write path it is not. A
double-entry structure with a `CHECK` constraint on liability accounts makes
overdraft *structurally impossible* rather than conventionally avoided.

This is not an exotic choice. Square, Stripe, Modern Treasury, and Fragment all
converged on an append-only double-entry store independently, for the same
reasons. Square's own account of migrating off a single-entry design describes the
same pain. The failure mode is not hypothetical: a US payments company operating
without reconcilable records between its own ledger and its partner banks was
assessed a **$60–90M shortfall** across 100,000+ consumers, and a related
recordkeeping failure at another firm drew Federal Reserve enforcement action in
2024.

**Shape of the store — three tables:**

| Table | Role | Mutability |
|---|---|---|
| `coin_journal` | Head — one business event, with an idempotency key and a reason code | Append-only; only `state` transitions `PENDING → POSTED\|REVERSED` |
| `coin_posting` | Legs — one signed amount per account per journal | **Strictly append-only**, enforced by trigger |
| `coin_account_balance` | Cached projection per account | Mutable, and rebuildable from postings |

Plus two tables that carry domain meaning rather than accounting meaning:
`coin_lot` (expiry) and `resource_unlock` (entitlement). §5 explains why
entitlements are deliberately *not* in the ledger.

---

## 2. Schema

PostgreSQL. `bigint` for all amounts, never `float` or `numeric` — one coin is
one unit, and floating point on a balance is indefensible in an audit. Journal
IDs are UUIDs, not sequences, so a rolled-back transaction does not leave gaps
that complicate debugging.

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;   -- gen_random_uuid() is core from PG13

-- ── enums ──────────────────────────────────────────────────────────────
-- PURCHASED is deliberately absent. Decision D1 forbids purchasing coins, and
-- adding the value later is a non-event: the ENUM gains a value, the chart of
-- accounts gains an account, and no existing row changes meaning.
CREATE TYPE coin_bucket        AS ENUM ('FREE','EARNED');
CREATE TYPE coin_account_kind   AS ENUM ('USER','SYSTEM');
CREATE TYPE coin_journal_state  AS ENUM ('PENDING','POSTED','REVERSED');
CREATE TYPE coin_entry_type     AS ENUM
  ('GRANT','SPEND','EXPIRE','REVERSAL','ADJUST');

-- ── accounts ───────────────────────────────────────────────────────────
CREATE TABLE coin_account (
  id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  kind          coin_account_kind NOT NULL,
  owner_user_id bigint,                  -- USER only
  system_type   text,                    -- SYSTEM only
  bucket        coin_bucket,             -- USER only
  created_at    timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT coin_account_shape_ck CHECK (
    (kind='USER'   AND owner_user_id IS NOT NULL AND system_type IS NULL AND bucket IS NOT NULL)
 OR (kind='SYSTEM' AND owner_user_id IS NULL     AND system_type IS NOT NULL AND bucket IS NULL))
);

-- Two partial unique indexes, NOT one UNIQUE over (owner_user_id, kind, bucket).
-- A single unique constraint collides on (NULL,'SYSTEM',NULL) for every system
-- account. This was hit and fixed during schema validation.
CREATE UNIQUE INDEX coin_account_user_bucket_uniq
  ON coin_account (owner_user_id, bucket) WHERE kind='USER';
CREATE UNIQUE INDEX coin_account_system_uniq
  ON coin_account (system_type)            WHERE kind='SYSTEM';

-- ── cached balance projection (NOT the source of truth) ────────────────
-- `liability` is true only for USER accounts. This flag is load-bearing:
-- the system accounts model the outside world and are legitimately signed
-- against us, so the anti-overdraft CHECK must not apply to them. Applying it
-- to everything rejects every single grant.
CREATE TABLE coin_account_balance (
  account_id      bigint PRIMARY KEY REFERENCES coin_account(id),
  liability       boolean    NOT NULL DEFAULT true,
  posted_balance  bigint     NOT NULL DEFAULT 0,
  reserved        bigint     NOT NULL DEFAULT 0,  -- pending referral holds
  version         bigint     NOT NULL DEFAULT 0,  -- optimistic token
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT coin_no_overdraft_ck CHECK (NOT liability OR posted_balance - reserved >= 0)
);

-- ── journal head ───────────────────────────────────────────────────────
CREATE TABLE coin_journal (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  entry_type          coin_entry_type    NOT NULL,
  state               coin_journal_state NOT NULL DEFAULT 'POSTED',
  scope               text   NOT NULL,  -- 'user', or 'system'
  idempotency_key     text   NOT NULL,
  request_fingerprint bytea  NOT NULL,  -- sha256 of the canonical request
  reason_code         text   NOT NULL,  -- PROFILE_COMPLETE, REFERRAL_QUALIFIED, RESOURCE_UNLOCK, …
  ref_type            text, ref_id bigint,
  reversal_of         uuid REFERENCES coin_journal(id),
  effective_at        timestamptz NOT NULL DEFAULT now(),
  created_at          timestamptz NOT NULL DEFAULT now(),
  created_by          text,             -- 'api', 'admin:<id>', 'cron:expiry'
  metadata            jsonb  NOT NULL DEFAULT '{}',
  CONSTRAINT coin_journal_idem_uniq UNIQUE (scope, idempotency_key),
  CONSTRAINT coin_journal_ref_ck    CHECK (ref_type IS NULL OR ref_id IS NOT NULL)
);
CREATE INDEX coin_journal_reason_idx ON coin_journal (reason_code, created_at DESC);
CREATE INDEX coin_journal_ref_idx    ON coin_journal (ref_type, ref_id) WHERE ref_type IS NOT NULL;

-- ── postings: append-only, signed, one row per leg ─────────────────────
CREATE TABLE coin_posting (
  id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  journal_id  uuid    NOT NULL REFERENCES coin_journal(id) ON DELETE RESTRICT,
  seq         smallint NOT NULL,
  account_id  bigint  NOT NULL REFERENCES coin_account(id),
  amount      bigint  NOT NULL,  -- SIGNED; SUM(amount) = 0 per journal
  account_seq bigint  NOT NULL,
  lot_id      bigint,
  created_at  timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT coin_posting_nonzero_ck   CHECK (amount <> 0),
  CONSTRAINT coin_journal_seq_uniq     UNIQUE (journal_id, seq),
  CONSTRAINT coin_account_seq_uniq     UNIQUE (account_id, account_seq)
);
CREATE INDEX coin_posting_account_idx ON coin_posting (account_id, id);
CREATE INDEX coin_posting_lot_idx     ON coin_posting (lot_id) WHERE lot_id IS NOT NULL;

-- ── lots: independently-expiring grants (the FEFO substrate) ───────────
CREATE TABLE coin_lot (
  id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  account_id bigint      NOT NULL REFERENCES coin_account(id),
  journal_id uuid        NOT NULL REFERENCES coin_journal(id),
  bucket     coin_bucket NOT NULL,
  granted    bigint      NOT NULL CHECK (granted > 0),
  consumed   bigint      NOT NULL DEFAULT 0,
  expires_at timestamptz,          -- NULL = never expires
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT coin_lot_not_overconsumed_ck CHECK (consumed >= 0 AND consumed <= granted)
);
CREATE INDEX coin_lot_open_idx ON coin_lot (account_id, expires_at NULLS LAST, id)
  WHERE consumed < granted;        -- stays small as lots close

-- ── immutability enforcement ───────────────────────────────────────────
CREATE FUNCTION coin_deny_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'coin_posting is append-only (attempted %)', TG_OP
      USING ERRCODE = '25006'; END; $$;
CREATE TRIGGER coin_posting_append_only BEFORE UPDATE OR DELETE ON coin_posting
  FOR EACH ROW EXECUTE FUNCTION coin_deny_mutation();

CREATE FUNCTION coin_journal_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'coin_journal is append-only' USING ERRCODE = '25006';
  END IF;
  IF (OLD.id, OLD.entry_type, OLD.scope, OLD.idempotency_key, OLD.request_fingerprint,
      OLD.reason_code, OLD.ref_type, OLD.ref_id, OLD.created_at, OLD.created_by)
     IS DISTINCT FROM
     (NEW.id, NEW.entry_type, NEW.scope, NEW.idempotency_key, NEW.request_fingerprint,
      NEW.reason_code, NEW.ref_type, NEW.ref_id, NEW.created_at, NEW.created_by) THEN
    RAISE EXCEPTION 'coin_journal: provenance columns are frozen' USING ERRCODE = '25006';
  END IF;
  IF OLD.state = 'POSTED' AND NEW.state <> 'POSTED' THEN
    RAISE EXCEPTION 'coin_journal: POSTED is terminal' USING ERRCODE = '25006';
  END IF;
  RETURN NEW;
END; $$;
CREATE TRIGGER coin_journal_immutable BEFORE UPDATE OR DELETE ON coin_journal
  FOR EACH ROW EXECUTE FUNCTION coin_journal_guard();
```

The journal trigger deliberately permits one narrow mutation: `PENDING` may become
`POSTED` or `REVERSED`, and `POSTED` is terminal. This is restricted immutability,
not a contradiction of it.

### 2.1 Chart of accounts

Seeded at migration time, `kind='SYSTEM'`, `liability=false`:

| `system_type` | Direction | Meaning |
|---|---|---|
| `earned_faucet` | Goes **negative** on issue | Coins the platform has issued |
| `redeemed_sink` | Goes **positive** on destroy | Coins the platform has taken back |
| `expired_burn` | Goes **positive** | Coins destroyed by expiry |

The sign asymmetry is why `liability` must exist. A faucet account that is always
negative is not an overdraft; it is the P&L side of issuance. The global
invariant is that `SUM(posted_balance)` across *all* accounts is **0**.

---

## 3. Concurrency: one advisory lock per user

Every balance mutation begins with:

```sql
SELECT pg_advisory_xact_lock(hashtextextended('coin:user:' || $1, 0));
```

This is the highest-risk code in the feature because the codebase has **no
precedent for row locking** (see [01](01-current-state-and-feasibility.md) §3.5).
The choice of mechanism:

| Option | Verdict |
|---|---|
| `pg_advisory_xact_lock` per user | **Chosen.** One lock per user means there is no lock-ordering to get wrong and therefore no deadlock. Transaction-scoped, so it releases at COMMIT/ROLLBACK with no explicit unlock. Avoids table bloat that a table-level lock flag would cause. |
| Optimistic version column + CAS | Rejected for v1. Its weak point is multi-leg FEFO spends (§4), which require CASing each bucket row in ascending `account_id` order and retrying the whole allocation on partial failure. That is a materially more complex failure mode, and mobile double-taps make parallel requests plausible. |
| `SERIALIZABLE` isolation | Rejected. Adds monitoring overhead, requires a generalized `SQLSTATE 40001` retry handler, and still does not prevent all errors that true serial execution would not have produced. `READ COMMITTED` plus an explicit lock is cheaper and more predictable. |
| `SELECT … FOR UPDATE` on the balance row | Viable, but narrower: it does not extend to the multi-row FEFO allocation across three buckets as cleanly as a user-scoped lock. |

**Validation.** The schema and both concurrency algorithms were executed against a
live PostgreSQL 18.6 instance. Twenty concurrent transactions each spending 10
coins from a 100-coin balance produced **exactly 10 successes, 10 rejections, a
final balance of exactly 0, and 10 journal rows** — with the rejected
transactions leaving zero rows behind, because the raised error rolls back the
whole transaction. A control test confirmed the lock actually serializes: a
second transaction beginning 0.5s after the first did not write until 1.5s after
the first committed.

Set `lock_timeout` and `idle_in_transaction_session_timeout` locally at the top of
every coin transaction (`SET LOCAL`) so a pathological request cannot hold a user
lock open indefinitely.

**If coins are ever transferred between two users** in one transaction, lock both
ids in ascending numeric order. Postgres is explicit that the best defense against
deadlocks is acquiring locks on multiple objects in a consistent order. Decision
D1 means this path does not exist in v1, but ADR-001 exists to stop someone
adding it casually.

---

## 4. Spend: FEFO across lots

Which bucket gets spent is a **query**, not a schema change. That is the payoff of
modelling grants as lots:

```sql
SELECT l.id, l.account_id, (l.granted - l.consumed) AS remaining
  FROM coin_lot l
  JOIN coin_account a ON a.id = l.account_id
 WHERE a.owner_user_id = $1 AND a.kind = 'USER'
   AND l.consumed < l.granted
   AND (l.expires_at IS NULL OR l.expires_at > now())
 ORDER BY l.expires_at NULLS LAST, l.id;   -- FEFO
```

**FEFO, not LIFO.** LIFO is cost-accounting convention and is wrong for consumer
credit. FEFO burns the soonest-expiring coins first and degenerates to
"never-expiring last" automatically, which is the consumer-favourable outcome: a
user never watches coins evaporate while untouched ones sit in their balance.

Validated behaviour: `FREE` expiring in 30 days (100 coins), `EARNED` expiring in
365 days (50), `PURCHASED` never expiring (25); spending 120 consumed 100 from
`FREE` and 20 from `EARNED`, leaving `PURCHASED` untouched.

**A bug worth naming.** The first implementation of this loop *overwrote* its
running total instead of accumulating it, so a spend of 120 became 145. No
database constraint caught it, because the postings still summed to zero and the
balance still matched the cache. It fails silently and would have been a direct
overcharge of user balances in production.

The mitigation is procedural and must be part of the code review checklist: the
loop accumulates (`taken += inc`) and posts the **per-lot increment** (`-inc`),
never the running total. The allocator gets property-based tests, not just example
tests — see [04](04-implementation-plan.md) Phase 1.

---

## 5. Entitlements live in the domain, not the ledger

```sql
CREATE TABLE resource_unlock (
  id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id       bigint NOT NULL,
  resource_type text   NOT NULL,   -- 'study_resource' | 'video' | 'mock_test'
  resource_id   bigint NOT NULL,
  journal_id    uuid   NOT NULL REFERENCES coin_journal(id),
  coins_paid    bigint NOT NULL CHECK (coins_paid >= 0),
  unlocked_at   timestamptz NOT NULL DEFAULT now(),
  revoked_at    timestamptz, revoke_reason text,
  CONSTRAINT resource_unlock_uniq UNIQUE (user_id, resource_type, resource_id)
);
```

Three things follow from this table that a balance column cannot provide.

**The free allowance is enforced structurally, independent of balance.** A student
who farms a thousand fraudulent accounts still cannot unlock a fourth document,
because the constraint rejects it. That caps the worst case at "they got the three
allowance unlocks" rather than "they drained the catalogue" — which is the single
most important fraud control in the system, and it costs one unique index.

**`coins_paid` is a snapshot, not a lookup.** Prices change. If the document price
goes from 40 to 60 coins, a user who bought at 40 keeps their unlock, and the
record of what they paid survives. Storing the price on the resource row and
reading it at query time makes historical purchases retroactively re-price and
turns entitlement into an argument.

**Revocation is a reversal, not a deletion.** `revoked_at` plus a
`reversal_of` journal entry, so a clawback is auditable and idempotent.

The allowance itself is *not* a coin grant. It is three entitlements — three
`resource_unlock`-shaped records with `coins_paid = 0` and a distinct table for
per-class limits, because "one video and one mock test, forever" is a rule about
entitlement shape, not a quantity of currency. See
`user_free_allowance` in [03](03-api-contract.md).

---

## 6. Once-per-lifetime awards

Profile completion is recomputed on every dashboard request and can *decrease* — a
user who deletes their address drops below 100%. Comparing "is it 100% now" against
"was it 100% before" is racy and re-awardable.

```sql
CREATE TABLE reward_grant (
  id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id     bigint NOT NULL,
  award_code  text   NOT NULL,   -- 'PROFILE_COMPLETE', 'REFERRAL_BONUS', …
  journal_id  uuid   NOT NULL REFERENCES coin_journal(id),
  granted_at  timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT reward_grant_uniq UNIQUE (user_id, award_code)
);
```

The `UNIQUE (user_id, award_code)` makes "at most once per user per award type" a
database guarantee rather than a service-layer check that a bug can bypass.
Referral bonuses key on `award_code = 'REFERRAL_BONUS:' || referral_id`, so the
same mechanic grants repeatedly for referrals while profile completion grants once.

---

## 7. Economy configuration

Config lives in `system_settings` under a `coin_economy` key, following the
existing `find_college_ad_cards` pattern at
`internal/system/service.go:1056-1109`: read → if absent or empty return defaults →
unmarshal **on top of** the defaults struct so a partial stored object still
yields sane values for omitted keys.

Two corrections to the existing pattern are required here:

1. **The existing `roleMW` is too broad.** `cmd/server/main.go:564` builds it as
   `RequireRole("admin","super_admin","scholarship_provider","scholarship-provider","Scholarship Provider","scholarship_provider_subuser","institution")`.
   Coin economy config on that gate would let an institution account rewrite
   pricing. Coin config gets its own `RequireRole("superadmin","super_admin")`
   group, matching `studyResourcesRoleMW` at `main.go:599`.
2. **The existing store does no caching.** Every read is a fresh `SELECT`. Coin
   pricing is read on every unlock attempt and every balance render, so the coin
   module gets its own in-process cache with a short TTL and explicit invalidation
   on write, rather than putting pricing on a hot uncached path.

`SetSystemSetting` is also read-then-write, not an atomic upsert
(`internal/system/repository.go:1222-1232`), so two concurrent admin writers can
race on the unique key. Acceptable for a config value that changes rarely, but the
coin wrapper should be the one place that documents it.

---

## 8. Go package layout

Follows the house layering exactly — `routes.go` / `handler.go` / `service.go` /
`repository.go` / `model.go` / `dto.go` — as used by `internal/mocktests` and
`internal/studyresources`.

```
internal/coins/
  model.go          GORM structs mirroring §2
  types.go          Reason codes, entry types, bucket helpers
  errors.go         package-level sentinels: ErrInsufficientCoins, ErrIdempotencyKeyReuse, …
  ledger.go         Grant / Spend / Reverse — the transactional core
  service.go        business rules: award eligibility, FEFO allocation, unlock
  repository.go     GORM queries + all db.Transaction() calls (repo layer only)
  service_test.go   unit tests, property tests on the FEFO allocator
  ledger_pg_test.go integration tests against real Postgres
  ensure_indexes.go partial/expression indexes AutoMigrate cannot create
  routes.go         RegisterRoutes(r, authMW, coinAdminRoleMW, h)
  handler.go        thin: bind → service → response.Success/Error
  dto.go            request/response DTOs + converters
  config.go         typed wrapper over system_settings with cache
  economy_daily.go  daily rollup for the health metrics in [05](05-economy-and-fraud.md) §5
```

**Transactions live in the repository layer**, matching 17 existing call sites
(`mocktests/repository.go:141,162,237,263`; `studyresources/repository.go:128`;
`forum/repository.go:316`). The `ledger.go` split above is an internal
decomposition of the repository layer, not a new architectural layer.

**Errors are package-level sentinels** matched with `errors.Is`, per
`internal/mocktests/service.go:13-23`. Status-code mapping is per-module in the
handler, per `mocktests/handler.go:247-258`.

**User ID extraction uses the strict form.** `currentUserID` is duplicated
byte-for-byte in `studyresources/playback.go:73` and `mocktests/handler.go:226`,
while `studyresources/handler.go:336` uses the unsafe
`userID, _ := c.Get("user_id"); uploadedBy, _ := userID.(uint)` which silently
yields `0` on a type mismatch. A wallet must adopt the strict switch-and-reject
form. This is a good candidate for extraction into `internal/shared` as part of
Phase 1, since we are adding a third copy otherwise.

---

## 9. Migration mechanics — a trap worth understanding

`cmd/migrate` is a **separate binary that nothing invokes.** It is absent from
the Makefile, the Taskfile, and the Dockerfile; `Taskfile.yml:77-85` maps
`db-migrate` to `go run ./server`. It globs `migrations/*.sql` only, so `.go`
files in that directory are invisible to it.

**The actual schema owner is `db.AutoMigrate` at `cmd/server/main.go:187-300`**,
which migrates roughly 110 model structs, followed by hand-invoked `.go`
migrations inside a `logger.Warn`-wrapped, non-fatal block at `main.go:315-350`.

Consequences for this feature:

- Adding the models to the `AutoMigrate(...)` list is what creates the tables.
  Without that, the tables do not exist.
- `AutoMigrate` **cannot create partial or expression indexes.** This codebase has
  already been bitten: `internal/notification/ensure_indexes.go:9-17` documents
  that a fresh `go run ./cmd/server` produced a server where every preferences PUT
  failed with `42P10 ON CONFLICT without matching constraint`, because the
  partial unique index only existed in a `.sql` migration.
- Our schema is unusually index-heavy, and three of its critical constraints are
  partial: `coin_account_user_bucket_uniq`, `coin_account_system_uniq`,
  `coin_lot_open_idx`, plus the reward and unlock unique constraints.

So Phase 1 ships **all three** artefacts: GORM models in the `AutoMigrate` list,
a `migrations/<date>_create_coin_ledger.go` for the SQL-only bootstrap path, and a
`coins.EnsurePostgresIndexes(db)` call in the same `logger.Warn` block at
`main.go:339-348`. Skipping the third reproduces a known outage.

---

## 10. Integration points

Four places enforce a coin check. Three already exist as authorization boundaries
and are correct places to hang it; the fourth is new.

| # | Site | File | Current state | Change |
|---|---|---|---|---|
| 1 | Video playback | `internal/studyresources/playback.go:27-69` | `authMW`-guarded, mints a 5-minute purpose-scoped token with a non-zero user binding (`playback_token.go:137-186`). Already fails closed. | Add the entitlement/coin check **before** `IssuePlaybackToken`. A student who already holds an unlock gets a token with no debit — this endpoint is re-entrant by design. |
| 2 | Document download | `internal/studyresources/handler.go:134-183` | **Anonymous, no middleware** (`routes.go:16`). Only check is `IsPublished`. | Add `authMW`, then the coin check. Gate first, then spend — never the reverse. Also fix the dead frontend helper `getStudyResourceDownloadUrl` (`services/studyResourcesApi.ts:235`, never called) and replace the raw `window.open` in `StudyResourcesPage.tsx:195-211` with a real service call. |
| 3 | Mock-test submit | `internal/mocktests/service.go:223` (`SubmitTest`) | Auth-gated (`routes.go:26`) but **unlimited attempts**, no cost, no per-user throttle. | Add the coin check. Note `MockAttempt` already exists (`model.go:60-71`) and `GetAttempt` is owner-scoped (`:311-313`), so attempt history is available. Preserve the DTO invariant that strips `is_correct` and explanations (`services/mockTestsApi.ts:210-225`) — a gate must not leak the answer key. |
| 4 | Resource approval | `internal/studyresources` — does not exist | Writes are superadmin-only (`routes.go:38-47`); only state is `IsPublished bool`. | New: user upload route, `pending_review`/`approved`/`rejected` state machine, reviewer identity, reject reason. **Coins are granted on approval-to-published, never on upload.** |

### 10.1 Referral attribution — all four user-creation paths

A single `applyAttribution` function, called from every place a `users` row is
created, so no path can be forgotten:

- `auth.Service.VerifyOTP` — `auth/service.go:402` (the real insert)
- `GoogleLoginOrRegister` — `auth/service.go:453`
- `InstitutionGoogleLoginOrRegister` — `auth/service.go:730`
- `ScholarshipProviderGoogleLoginOrRegister` — `auth/service.go:1634`

The referral code is captured at the earliest opportunity — as a query parameter
on the invite link, persisted client-side, and sent on `RegisterRequest` — and
carried through the OTP store to the `VerifyOTP` insert. A code that arrives on a
Google-login path is applied there instead. Each call site gets a test.

The referral table carries the fraud constraints as **database** invariants rather
than service checks, so a bug in Go cannot route around them:

```sql
CREATE TABLE user_referral (
  id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  referrer_user_id  bigint NOT NULL,
  referred_user_id  bigint NOT NULL,
  referral_code     text   NOT NULL,
  status            text   NOT NULL DEFAULT 'pending',  -- pending|qualified|rejected|revoked
  qualified_at      timestamptz,
  phone_hash        bytea,       -- keyed HMAC, not plaintext — see [07](07-compliance-nepal.md)
  device_hash       bytea,
  reserved_coins    bigint NOT NULL DEFAULT 0,
  created_at        timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT user_referral_uniq        UNIQUE (referred_user_id),
  CONSTRAINT user_referral_no_self     CHECK (referrer_user_id <> referred_user_id)
);
-- One successful referral per invited phone number, ever.
CREATE UNIQUE INDEX user_referral_phone_uniq  ON user_referral (phone_hash)  WHERE phone_hash IS NOT NULL;
-- One successful referral per invited device, ever.
CREATE UNIQUE INDEX user_referral_device_uniq ON user_referral (device_hash) WHERE device_hash IS NOT NULL;
```

`UNIQUE (referred_user_id)` guarantees one inviter per invitee, so a student
cannot re-enter a code to farm both sides.

**Phone and device are stored as keyed HMAC, not plaintext.** This table is
explicitly a fraud-matching table; keeping raw PII in it would make it the most
attractive target in the database and would sit badly with the Privacy Act 2018
position in [07](07-compliance-nepal.md).

---

## 11. Reconciliation — a scheduled job, not just tests

These five assertions are cheap and catch the failure mode that actually costs
money:

```sql
-- 1. every journal nets to zero
SELECT journal_id FROM coin_posting GROUP BY 1 HAVING SUM(amount) <> 0;
-- 2. global conservation
SELECT SUM(posted_balance) FROM coin_account_balance;          -- must be 0
-- 3. no user account overdrawn
SELECT 1 FROM coin_account_balance WHERE liability AND posted_balance - reserved < 0;
-- 4. cache matches postings
SELECT b.account_id FROM coin_account_balance b
 WHERE b.posted_balance <> (SELECT SUM(amount) FROM coin_posting WHERE account_id = b.account_id);
-- 5. no duplicate idempotency keys
SELECT scope, idempotency_key FROM coin_journal GROUP BY 1,2 HAVING COUNT(*) > 1;
```

**Run these nightly in production, not only in CI.** A test suite proves the
invariants held on the day the test ran. A reconciliation job proves they still
hold, and pages someone when they stop. This is the cheapest insurance in the
system and the direct lesson from the reconciliation failures cited in §1.

**Idempotency key retention.** Journal keys are retained at least 24 hours, per
Stripe's documented minimum, because a client retry window must be shorter than
the key's lifetime — otherwise a retried request creates a *new* journal instead
of replaying the old one. At 500k users, 30 days is inexpensive; prune only after
confirming no posting references the journal, or partition by month and drop
whole partitions.

---

## 12. Idempotency, honestly stated

Not exactly-once. What this design provides:

- **The effect happens exactly once** — enforced by `UNIQUE (scope,
  idempotency_key)` plus a single database transaction. This is a single-database
  guarantee, so it is genuinely strong.
- **The caller may not learn the outcome** — a timeout after COMMIT leaves the
  client uncertain. The fix is to retry with the same key, not to add more locks.

This is the standard industry framing: at-least-once delivery plus an idempotent,
deduplicated, atomically committed effect. Two operational rules follow:

1. **Never trust a client-supplied amount.** The client sends
   `{"event":"referral_qualified"}`; the server resolves the coin value from
   `reason_code` against its own config. A client that can say what a coin is
   worth is an exploit.
2. **A reused key with a different payload is an error, not a replay.** Compare
   `request_fingerprint` (sha256 of the canonical request) and reject on
   mismatch, so a key collision cannot silently return the wrong result.
