# StudSphere StudsToken System: Specification Set

Status: **specification phase, nothing built.**
Jurisdiction: **Nepal.** Last updated 2026-09-27.

A rewards-token system for StudSphere study resources. Students earn StudsTokens by
completing their profile, inviting friends, and contributing study resources that
pass admin approval. Students spend StudsTokens to unlock study resources, videos,
and mock tests. Every new account receives a one-time allowance of three document
unlocks, one video unlock, and one mock-test unlock.

---

## Naming

**The product is called StudsToken.** That is the name in every user-facing
surface, all copy decks, all notification templates, and all documentation prose.

**Internal identifiers are unchanged and stay as they are for now:** the Go package
`internal/coins`, the `coin_account` / `coin_journal` / `coin_posting` / `coin_lot`
tables, the `coin_bucket` enum, and the `coin_economy` settings key. This is
deliberate. Nothing is built yet, so a later rename is cheap, but the schema in
[02](02-architecture.md) is the version that was executed and validated against a
live PostgreSQL instance, and renaming tables and enums means re-verifying it. The
name is also provisional. If it moves again, the internal rename follows.

**One naming concern, recorded so it is a decision and not an oversight.** Consumer
Protection Act 2075 s.18(e) prohibits operating a "**token system**" in cooperation
with more than one business providing services of the same nature. Our mechanic
does not come close: it is single-level, non-transferable, and involves no
cooperating businesses, so the prohibition does not apply on the substance. But the
word "Token" now appears in the product name, which gives an optic that the facts
do not support. See [07](07-compliance-nepal.md) §5.3. If this becomes a
visibility problem, renaming is cheap and the internal identifiers make it a
one-line change.

---

## Document index

| # | Document | Contents |
|---|---|---|
| 01 | [Current state & feasibility](01-current-state-and-feasibility.md) | What exists today, what is missing, feasibility verdict, risk register |
| 02 | [Architecture](02-architecture.md) | Ledger design, validated DDL, concurrency, FEFO, integration points |
| 03 | [API contract](03-api-contract.md) | Endpoints, Go package layout, DTOs, error semantics |
| 04 | [Implementation plan](04-implementation-plan.md) | Phased delivery, migration mechanics, Go/Postgres/frontend work items |
| 05 | [Economy & fraud](05-economy-and-fraud.md) | Coin pricing, earn/sink ratios, caps, threat model, monitoring |
| 06 | [UI/UX spec](06-ui-ux-spec.md) | Coin wallet, unlock flows, earn progress, admin approval queue, copy deck |
| 07 | [Nepal compliance](07-compliance-nepal.md) | VAT 2052, Payment & Settlement Act 2075, Consumer Protection Act 2075, Privacy Act 2075, copyright, education-sector rules |
| ADR-001 | [Closed-loop earn-only coins](adr/ADR-001-closed-loop-earn-only-coins.md) | The invariant that keeps us out of the payment-instrument regime |

---

## Locked decisions

These were decided by the founder and are treated as fixed inputs. Changing any
of them invalidates downstream documents.

| # | Decision | Consequence |
|---|---|---|
| D1 | Coins are **earn-only, never purchasable** | Closed-loop, non-transferable, non-cash-outable. This is what keeps the product outside a regulated payment-instrument regime. See ADR-001. |
| D2 | Free allowance is **one-time, expiring in 30 days** | Three document + one video + one mock-test entitlement, not a coin balance. Expiring creates urgency without a recurring liability. |
| D3 | **Phase 1 covers `studyresources` only** | `internal/downloadcenter` is out of scope. Its known unpublished-file authorization bug is filed separately. |
| D4 | **10 successful referrals per user per month** | Hard cap, database-enforced. See [05](05-economy-and-fraud.md) for the resolved coin-ceiling interaction. |

## The one-paragraph version

Coins are not a number on the user row. They are an append-only double-entry
ledger with a cached balance projection, mirroring the design Square, Stripe, and
Modern Treasury all converged on independently. A grant or a spend is a journal
entry with signed postings that sum to zero; the balance is a rebuildable cache.
This matters because the obvious design (a `coin_balance` column updated with
`UPDATE users SET coins = coins - 50`) cannot answer "why does this user have
30 coins", cannot reverse a mistaken grant without a manual correction, and is
the exact failure class that produced a real $60–90M reconciliation shortfall at
a US fintech. Expiring coins and separating "which bucket gets spent first" are
modeled as grant lots, not as columns, so that changing the expiry policy is a
config change rather than a migration. Every balance mutation is serialized per
user with a single Postgres advisory lock, and idempotency is enforced by a
`UNIQUE` constraint rather than application logic, so retries and double-taps
cannot double-spend.

## How to read this set

Read 01 first. It establishes what exists and whether the idea is feasible
against the current codebase. ADR-001 is short and should be read before any
design work, because it is the constraint most likely to be accidentally violated
by a future change. 05 contains the numbers and is the only document where
figures are expected to be tuned after launch. 07 is the only document that
requires a Nepali chartered accountant or lawyer to sign off before launch; the
rest is engineering work that can begin immediately.

**07 also materially changes the plan**, and it is worth reading before treating
this set as settled. Two things move:

- **The VAT classification of the redemption is settled, and settled in our
  favour.** A Nepali chartered accountant has confirmed that unlocking a study
  resource, video lecture, or mock test in exchange for coins is an exempt
  educational service under Schedule 1, and that we sit below the VAT registration
  threshold on both triggers. No coin carries output VAT, so **the coins are not
  margin-negative and Phases 1 and 2 are unblocked.** Two constraints come with the
  answer and both are live: while we stay exempt-only we can never recover input VAT
  on anything we buy (not a regression, we are unregistered today), and the coin
  catalogue must stay entirely inside the educational perimeter or the classification
  is at risk. The reasoning behind the conclusion has not been received in writing, so
  07 §2.6 is still open.
- **The Privacy Act adds two gates that are not optional.** Unconsented third-party
  data transfer is a criminal offence (which rules out a fraud vendor at launch),
  and under-18s require guardian consent plus a benefit test (which rules out a
  referral flow open to our core audience without an 18+ gate).

These are carried in [01](01-current-state-and-feasibility.md) §6 as **R3** (VAT
classification, now answered, with the residual forward constraints tracked at
**R19**), **R4** (criminal privacy exposure) and **R13** (under-18 users); read those
rows before estimating the work. **R4 carries the only custodial penalty in the
register.**
