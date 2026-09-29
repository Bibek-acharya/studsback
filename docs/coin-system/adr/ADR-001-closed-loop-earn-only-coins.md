# ADR-001: Closed-loop, earn-only coins

- **Status:** Accepted
- **Date:** 2026-09-26
- **Deciders:** Founder
- **Relates to:** [01](../01-current-state-and-feasibility.md) §3, [02](../02-architecture.md), [05](../05-economy-and-fraud.md)

---

## Context

StudSphere is introducing a virtual currency ("coins") that students earn by
completing their profile, inviting friends, and contributing study resources that
pass moderation, and spend to unlock study resources, videos, and mock tests.

A virtual currency that can be bought with money, transferred between users,
redeemed for cash, or used at third parties is a materially different product from
one that cannot. It attracts a regulatory perimeter, payment-instrument
obligations, withholding-tax exposure, refund and chargeback obligations, and
consumer-protection rules that do not apply to a closed promotional balance.

The question is not "is this a good feature" — it is "which version of this feature
are we building", because the answer changes the schema, the compliance burden, and
the cost of building it.

> **Jurisdiction note.** This platform is Nepal-based; an earlier draft of this work
> assumed India and that analysis has been discarded. The Nepal research is
> complete ([07-compliance-nepal.md](../07-compliance-nepal.md)). It
> **supports** this ADR and supplies a second, independent reason for it: under the
> Payment and Settlement Act 2075 the central bank places "loyalty and redemptions"
> inside the payment-instrument perimeter, but attributes them to **licensed**
> institutions, and there is no licence class or carve-out for a non-financial
> closed-loop loyalty programme. Nepal also has no settled guidance on the point, so
> the argument is from the *absence* of a licence class rather than from an
> exemption — which is why the invariant is enforced in code rather than argued in a
> policy document.

---

## Decision

**Coins are earn-only, closed-loop, non-transferable, and non-cash-outable.**

Specifically, all four of the following hold, permanently:

1. **Not purchasable.** No pathway exists — or will be added — by which a student
   pays money to receive coins. No coin packs, no top-up, no subscription that
   grants coins, no payment-gateway integration of any kind.
2. **Not transferable.** No user-to-user transfer, gifting, forwarding, or
   conversion. A coin belongs to one account, permanently.
3. **Not redeemable for money.** No cash-out, refund, withdrawal, or conversion to
   any payment instrument, wallet, or stored value.
4. **Not usable at third parties.** Coins redeem only against StudSphere's own
   catalogue.

Additionally:

5. **No display of a cash value.** The product must never show a currency
   equivalence for a coin — not "worth NPR 40", not "NPR 2,000 in rewards", not in
   marketing copy, notification templates, or admin screens. Coins have no
   monetary value because they have no purchase or redemption path, so a value
   claim is not a convenience, it is a claim we cannot honour.
6. **No expiry by stealth.** Balances expire only with per-lot visibility in the
   product and 30/7/1-day reminders. Never silently, never via a terms-page clause
   a student has to go looking for.

---

## Why

**It is the simplest version that works.** Every alternative is strictly more
expensive to build and to operate: a payment gateway, a refund path, a
chargeback path, a reconciliation against a payment processor, withholding tax on
payouts, and a purchase-price/promotional-balance split in the ledger.

**It preserves the actual value proposition.** The coins are not a revenue
mechanism — they are a retention and contribution mechanic. Every earn action is
something we *want* a student to do: finish their profile, bring a friend, upload
a resource that helps other students. There is no version of this feature where
letting people buy coins makes students more likely to do any of those things.

**It keeps the bad case bounded.** A fraud ring against a purchased currency is a
financial loss. A fraud ring against a closed promotional balance is a
liability-integrity and brand problem — and the analysis in
[05](../05-economy-and-fraud.md) §3.1 shows the direct revenue exposure is small,
because a fraudulent claimant is overwhelmingly someone who would never have paid.

**It is the design that lets the whole thing stay simple enough to be correct.** A
ledger that only ever issues and destroys is already a non-trivial piece of
engineering. Adding value in, value out, and value transferred is three more
correctness surfaces. The cheapest way to be sure the core is right is not to
harden the core, it is to not build the other three.

---

## Consequences

### What this makes easy

- The ledger has two buckets (`FREE`, `EARNED`) and three system accounts. No
  `PURCHASED` bucket, no cash leg, no gateway reconciliation.
- The fraud defence can be almost entirely database constraints, with no vendor
  spend justified at launch.
- Expiry is a product decision rather than a liability question, because there is no
  paid balance whose expiry needs to be disclosed as a fee or a term.
- Opening a paid tier later is a *separate feature*, not a change to this one.

### What this forecloses

- Coins cannot be the primary revenue lever. If the business later needs
  coin-denominated revenue, this ADR must be superseded — deliberately, in writing,
  with the Nepal regulatory position in [07](../07-compliance-nepal.md) reviewed
  first.
- "Gift coins to a friend" is not a growth feature we can ship. It is a change to
  this ADR. It will arrive as a well-meaning request from a product manager, and
  the reason it is refused is more likely to be forgotten than the reason it was
  agreed.
- **Any chance-based or randomised reward mechanic.** A deterministic access right
  is not a lottery; adding a randomising element engages the Lottery Act 2025
  (1968) permission regime and Consumer Protection Act 2075 s.16(2)(d) contest
  pricing. This is easy to violate by accident via an A/B test, so it is a product
  constraint, not a preference.
- **Any multi-level referral tree.** Consumer Protection Act 2075 s.18(e)
  expressly names a "token system" and s.16(2)(p) prohibits "levels or series". A
  pyramid-scheme prohibition, not a tax question. Referrals stay single-level, one
  generation, enforced by test.
- **Coins ever becoming purchasable with cash.** A purchase escalates this from a
  unit of account to a prepaid instrument, which is PSP territory: NRB licence,
  mandatory paid-up capital, full float backing, KYC/AML, annual system audit, and
  a settlement-bank agreement. Budget 12–18 months and a licensed-entity
  structure. See 07 §4.

### What it requires

This ADR is only useful if violating it is *hard*, not merely documented. Four
enforcement mechanisms, all in scope for Phase 1:

1. **No transfer code path exists.** There is no `TransferCoins` function, no
   `POST /coins/transfer` route, and no service method that can move value between
   two `owner_user_id`s. The absence of the code is the control.
2. **A test that fails if one appears.** A `coins_architecture_test.go` that
   greps the package for transfer-shaped APIs and fails the build if any appear. It
   should be written to be annoying: it names the ADR in its failure message, so
   whoever trips it learns why it exists.
3. **A comment at the natural temptation point.** In the ledger, next to
   `Grant`/`Spend`, a comment stating that these are the only two ways value moves
   and that a third is a product decision, not a bug.
4. **The two-user advisory lock ordering rule is documented but unused.**
   [02](../02-architecture.md) §3 specifies the correct ordering if a transfer is
   ever added, precisely so that whoever adds it does not also introduce a
   deadlock. Writing it down now is the cheapest part of a future migration.

### What a superseding ADR must resolve

Not a checklist to complete now — a list of what a future change would have to
answer, so the cost is visible before someone commits to it:

- The Nepal regulatory position on whether a closed-loop instrument, and a
  purchasable one, falls inside the NRRA perimeter — [07](../07-compliance-nepal.md)
- Payment-instrument authorisation eligibility for a non-bank issuer
- A payment gateway, refund, and chargeback integration
- Withholding tax on coin-related payouts, if any are ever introduced
- The `PURCHASED` bucket, its accounting, its expiry disclosure, and its breakage
  treatment — a materially harder accounting question than a promotional balance
- A revised fraud posture, since a fraud ring becomes a direct financial loss

---

## Alternatives considered

| Alternative | Why not |
|---|---|
| **Purchasable coin packs from launch** | Highest cost and regulatory surface, for a revenue mechanism that is not the point of the feature. The coins are a retention mechanic |
| **Purchasable later, designed for now** | Reasonable, and it was offered. Rejected because the `PURCHASED` bucket has a real accounting cost and a real expiry-disclosure obligation, and because the schema does not foreclose adding it — `PURCHASED` can be added as an ENUM value and a chart-of-accounts row without changing the meaning of any existing row |
| **Coin gifting between users** | A retention feature people will ask for. It is the single most likely request to bypass this ADR, and it converts a promotional balance into a transferable instrument. Revisit only as a superseding ADR |
| **Cash redemption** | Converts the entire feature into a regulated instrument and creates a direct financial liability. Never |
| **No expiry at all** | Considered because expiry has measurable trust cost. Rejected because a never-expiring balance is a permanent, growing liability with no urgency, and because the 30-day allowance expiry is the activation lever. The compromise — expire the allowance hard, extend earned activity-based, never expire what was paid for — is in [05](../05-economy-and-fraud.md) §4 |
