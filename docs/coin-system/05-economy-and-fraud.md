# 05 — Economy & Fraud

The numbers, the threat model, and the monitoring. This is the only document whose
figures are expected to change after launch — treat §2 as a starting hypothesis, not
a specification.

**Provenance is marked throughout.** `SOURCED` means a retrieved primary or
professional source supports it. `DERIVED` means our own reasoning from the business
inputs. Anything `UNVERIFIED` was not retrieved and should not be relied on.

---

## 1. Design constraints the numbers must satisfy

1. A student who engages seriously for a month should be able to unlock a
   meaningful amount of content — the feature must feel worth returning for.
2. A student who does nothing should not be able to unlock the catalogue. Faucet
   and sink must reach approximate balance.
3. **The free allowance must dominate early experience.** It is the acquisition
   hook. Coins are the retention mechanic, not the conversion mechanic.
4. The cheapest paid acquisition must pay back within roughly one unlock. This is
   what makes the whole loop profitable, and it is the constraint that actually
   determines coin value (§2).
5. Total unredeemed balance must stay inside a ceiling the balance sheet can carry
   (§3.4).

---

## 2. Pricing

### 2.1 Prices and earn rates

| Item | Coins | Provenance |
|---|---|---|
| Document unlock | 40 | Our pricing. A document is cheap to serve. |
| Video unlock | 90 | Our pricing. Highest production cost; also the asset students care about most. |
| Mock-test unlock | 60 | Our pricing. Highest-intent signal — mock tests are what students come to StudSphere for. |
| Profile completion | 25 total, 5 × 5 instalments | Our pricing. |
| Referral — referrer | 60 | Our pricing. Equal to one mock test, so the value is immediately legible. |
| Referral — invitee | 25 | Our pricing. Enough to feel real without being a second income. |
| Resource approved | 80 | Our pricing. The highest earn, because it is the most expensive for the student to produce. |
| Free allowance | 3 documents + 1 video + 1 mock, 30 days | Decision D2. |

Prices are ordered by **cost to serve**: documents cheapest, video most expensive.
The one constraint that overrides this is cannibalisation — if a paid tier ever
exists, a coin unlock must cost less than the cash price, or we are giving away
revenue we could have booked.

### 2.2 Coin value — the number that is not yet set

**We cannot set this without your customer acquisition cost.** The derivation
matters more than the value, so here it is in full:

> A student is only worth acquiring if the lifetime value they generate exceeds
> CAC. The referral and profile mechanics exist to reduce effective CAC. If one
> coin is worth `V` and a mock test unlocks for 60 coins, one unlock is worth
> `60 × V`. That unlock must be worth a meaningful fraction of CAC — say 30–50% —
> or acquisition is structurally unprofitable, and a student who never converts is
> a net loss.

So: **`V ≈ CAC × 0.3 ÷ 60` for a single mock-test unlock to be worth 30% of CAC.**

Worked, in NPR, purely to show the shape:

| If CAC is | 1 coin ≈ | 60-coin mock test ≈ | 40-coin document ≈ | 90-coin video ≈ |
|---|---|---|---|---|
| NPR 200 | 1.00 | 60 | 40 | 90 |
| NPR 400 | 2.00 | 120 | 80 | 180 |
| NPR 800 | 4.00 | 240 | 160 | 360 |

**This is a placeholder framework, not a recommendation.** It is arithmetic on an
unknown input. Tell us your real blended CAC and the table collapses to one row.

**What coin value is actually for.** It is a *unit of account*, not money. Since
coins cannot be purchased (D1), `V` exists only so we can reason about whether the
economy is balanced — it must never appear in the product. Displaying "1 coin =
NPR 4" is a misleading value claim on an instrument that has no cash redemption
path, and it is the single easiest way to create a consumer-protection problem. See
[07](07-compliance-nepal.md).

### 2.3 Faucet and sink balance

`SOURCED` — Unity's game-economy guide names inflation rate, currency velocity,
**sink-to-faucet ratio**, days-of-currency-on-hand, payer conversion, and ARPPU as
the six metrics to instrument, and describes the target as approximate balance
rather than hoarding or deflation. It gives **no** numeric target.

`DERIVED` — our operating target is a **monthly sink:faucet ratio of 0.9–1.1** at
steady state. Instrument and tune it. Do not hardcode it into an alert threshold on
day one; a first month's data is not a trend.

Illustrative steady state per 1,000 monthly active students:

| Faucet | Coins | Sink | Coins |
|---|---|---|---|
**These figures are entirely assumption-dominated, and the assumptions are the
finding.** Every line rests on a conversion rate we have not measured, because the
funnel is not instrumented yet. The first version of this table assumed 5
referrals per user per month and 2 uploads per user per month, which put the faucet
at 9.4× the sink. Those are not realistic participation rates — 5 referrals per user
per month sits near the 10/month cap for *every* student simultaneously, and no
referral programme achieves that. The corrected table below is the defensible range.

A more honest statement of the same arithmetic: **a referral at 60 coins is 1.5× a
mock-test unlock, so referrals dominate issuance the moment they work**, while
profile completion at 25 is worth less than a single document. The faucet is
therefore structurally dependent on the referral rate, which is the one number we
have no basis to forecast. That asymmetry is the argument for the referral cap, and
it is also why the faucet cannot be projected today.

| Faucet driver | Low case | High case |
|---|---|---|
| Profile completions (35% × 1,000 × 25) | 8,750 | 8,750 |
| Referrals qualified (3% of users × 1.2 each × 60) | 2,160 | (25% × 4 each) = 60,000 |
| Uploads approved (2% of users × 80) | 1,600 | (10% × 2) = 16,000 |
| **Total faucet / month** | **~12,500** | **~84,750** |

| Sink driver | Low case | High case |
|---|---|---|
| Document unlocks (paid) | 300 × 40 = 12,000 | 1,000 × 40 = 40,000 |
| Video unlocks | 50 × 90 = 4,500 | 250 × 90 = 22,500 |
| Mock-test unlocks | 100 × 60 = 6,000 | 500 × 60 = 30,000 |
| **Total sink / month** | **~22,500** | **~92,500** |

The two scenarios bracket 0.55:1 and 0.92:1. **The plausible range straddles
balance**, which is the opposite conclusion from the first table — and it is the
reason the reward value should stay at **60 for launch** rather than be pre-cut on
the strength of a table we know is wrong.

**Recommendation: launch at 60, instrument, then tune.** Pre-cutting to 30 based on
an incorrect model would have cost half the referral reward for no reason. Instead,
make 30 the first candidate adjustment, contingent on the measured sink:faucet
ratio sitting above 1.2 after the first full month.

Three levers, in the order we would pull them if the measured ratio demands it:

1. **Cut the referral reward** from 60 to 20–30. Cheapest, least damaging, and the
   highest-leverage lever precisely because referrals dominate the faucet.
2. **Lower the monthly referral cap** from 10.
3. **Raise prices.** Least preferred — it directly harms the student.

The reward value is a config value ([03](03-api-contract.md) §3.1), so any of these
is a runtime change with no deploy.

### 2.4 Resolving a conflict in the referral caps

Decision D4 was chosen as "10 successful referrals per user per month, 200
coins/month lifetime". **As stated those two limits contradict each other**, and
the contradiction is worth being explicit about rather than quietly resolving.

At 60 coins per referral, 10 referrals is 600 coins. A 200-coin ceiling therefore
binds at **3.3 referrals**, so the count cap of 10 is unreachable and is decorative.

**Resolution: the count cap is primary; the coin ceiling is derived from it.**

- `monthly_cap = 10` successful referrals per user per calendar month
- `lifetime_coin_cap = 10 × monthly_cap × referral_reward`, evaluated against a
  lifetime total

With the launch value of 60 that is 600 coins per month and 6,000 per year. The cap
must be stored as a **count** and a **derived coin total**, and both
checked in the same transaction as the award. Whichever binds first is fine; what
matters is that the intent is a referral cap and not a coin cap that happens to
make the count cap a lie.

**One further rule that is not a cap and matters more than either:** at most one
successful referral per invited phone number, ever, and at most one per invited
device, ever. Enforced as database unique indexes
([02](02-architecture.md) §10.1), not application checks. A cap bounds loss; these
two prevent the same person being counted repeatedly, which is the actual fraud
mechanism.

---

## 3. Fraud

### 3.1 The economics invert the usual assumption

The most useful finding in the fraud research, and the one most likely to save
money: **in a non-purchasable closed-loop system, fraud is a
liability-integrity and brand problem, not a cash-bleed problem.**

The reasoning: a fraudulent coin recipient is overwhelmingly someone who would
never have paid. So the revenue "lost" to a fraudulent unlock is
`coins × redemption_rate × would_have_paid`. The last term is tiny.

Worked, on a 1,000-MAU base with a 20% lifetime redemption rate:

| Scenario | Coins issued | Revenue at risk (at 3% would-have-paid, NPR 399 mock test) |
|---|---|---|
| 5% fraud rate, capped | 23,400 | ~NPR 5,600 |
| 100% fraud rate, uncapped | 468,750 | ~NPR 112,000 |

Fraud 10× over the target costs roughly **NPR 5,600/month** in direct revenue risk.
`SOURCED` work on device-fingerprint pricing puts detection at roughly NPR 0.05–3
per event for a fingerprint call plus NPR 2–8 for a phone/email reputation lookup.
At 10,000 flagged signups/month that is **NPR 20,000–110,000/month** — *more than
the revenue at risk.*

**Therefore: do not buy a fraud vendor at launch.** The ROI does not justify it for
the coin programme alone, and buying detection before you need it is how you end up
with a permanent vendor line item justified by a problem you no longer have.

This conclusion inverts with one change. If coins ever unlock something worth
NPR 999, or become purchasable, the economics reverse entirely and full device
intelligence plus consortium data becomes obviously worth it. Revisit at that point,
not before.

**Nepal law independently reaches the same conclusion, and for a stronger reason.**
Privacy Act 2075 s.26(1) makes giving student personal data to a third party
without consent a criminal offence — with no data-protection regulator, no adequacy
mechanism, no safe harbour, and no breach-notification duty to lean on. So a
bundled "I agree to the terms" checkbox is not a cheap substitute for the
schema-level controls recommended above; it is the weak point in an otherwise
defensible fraud posture. See [07](07-compliance-nepal.md).

### 3.2 Threat model

Adapted from vendor production documentation, not blogspam. Severity assumes
launch-scale traffic.

| # | Technique | Sev | Mitigation | Cost |
|---|---|---|---|---|
| 1 | **Fake-account farm** — scripted signups, device farms, emulated devices | Critical | One device per account (unique index); per-phone and per-device daily caps; disposable-email and email-age heuristics | Free |
| 2 | **SIM farm / number recycling** — bulk Nepali numbers at low cost make OTP meaningless | Critical | Phone verification gate on referral awards (`phone_verified_at`); recycled-number age signal; line-type check | Free + OTP cost |
| 3 | **Self-referral** — A creates B, refers A→B; or a 3-cycle A→B→C→A | High | `CHECK (referrer <> referred)`; `UNIQUE (referred_user_id)`; one successful referral per phone and per device | Free |
| 4 | **VPN / proxy rotation** — defeats IP heuristics | High | Rate-limit on account and device, **not IP**; IP as a soft signal only. Campus NAT makes IP-only limits actively harmful | Free |
| 5 | **Phone/email reputation** — one throwaway address, N accounts | High | Email-domain blocklist; email-age; normalised-phone hashing. Individually weak, useful in combination | Low |
| 6 | **Ring / graph** — 200 accounts where no single one trips a velocity rule | Critical | `user_referral` uniqueness constraints make most rings structurally impossible; detect the rest from shared hashed attributes | Free |
| 7 | **Incentivised ad/SMS networks** — a paid operator with a portfolio of identities | High | Treat any referral arriving within seconds of an ad click as suspect. `SOURCED`: Amazon's structural anti-stacking rule is a 24-hour click window that closes on order or a competing click — copy the window, not the payout | Free |
| 8 | **Identity mismatch** — one real student, four accounts on one device | High | One device per account. `SOURCED`: the cheapest single control available, and it kills this class outright | Free |
| 9 | **Qualified-action gaming** — auto-complete a profile, click one button, forever | High | Require depth, not clicks: real document upload or a resource that passes moderation. The upload earn already satisfies this by construction | Free |
| 10 | **Account takeover** — attacker logs in and spends a real student's balance | Medium | TOTP already exists on `auth.User` (`model.go:42-43`) — offer it, do not force it. Never transfer coins (D1), which removes the laundering destination | Free |

**Seven of the ten are prevented by database constraints and existing session
data.** That is the shape of the recommendation: schema first, vendor never
initially, revisit when the economics change.

### 3.3 Hold-then-release

`SOURCED` — every credible production referral system holds rewards in a pending
state until the referred party qualifies. GrowSurf states it explicitly; Rivo gates
advocate reward on a qualifying order and runs fraud checks at invite submission;
Rewardful flags for review and withholds commission with automatic refund handling;
Adyen models available funds as pending + reserve + deposit.

`DERIVED` — our two-stage model, which is why `reserved` exists on the balance row:

```
referral qualified  → reserved += 60    (student holds it, cannot spend it)
hold window (7 d)   → if clean: reserved -= 60, posted += 60
                    → if fraud:  REVERSAL journal, reserved -= 60
```

`SOURCED` — Stripe holds a reserve for an additional 3 business days and releases to
zero at 180 days, and advises holding funds "only when there's a clear purpose and a
commitment to transfer them or pay them out when an event occurs or a precondition
is satisfied." That is exactly this case: a stated precondition, a defined payout
event.

`DERIVED` — **7-day hold, 180-day clawback window.** The hold is short because SEON's
production signal is that a recycled number shows up in a 24–72 hour window; the
clawback window is long because Amazon's dormancy precedent runs in months and
Stripe's release-to-zero is 180 days. A student who is falsely accused can be
reinstated within the window rather than being left permanently debited.

**Available = posted − reserved.** A student with 200 posted and 150 reserved can
only spend 50. Verified during schema validation: a 60-coin spend against that
state is correctly refused.

### 3.4 Liability ceiling

Outstanding balance must stay inside a multiple of the monthly earn rate, or the
ledger becomes a dumping ground and the balance sheet becomes unreadable.

> **Outstanding balance ≤ 3 × monthly faucet**, alerting at 2×, hard-blocking new
> issuance at 3×.

At the illustrative 1,000-MAU faucet of 468,750 coins/month that is a ceiling of
~1.4M outstanding coins. This is the single most useful operational number in the
system and it is a hard block, not an alert — an alert nobody acts on is not a
control.

### 3.5 Do not do

| Anti-pattern | Why |
|---|---|
| Reward on sign-up | The single most-abused design in the category. Reward on a **qualified action**, held. |
| Primary rate limit on IP | Defeated by a SIM farm, and it punishes campus NAT — i.e. our actual users |
| `if (user.coins >= cost)` in the client | That is UX, not authorization. The debit is a server transaction that re-reads under lock |
| Client-supplied amounts | The client must never be able to say what a coin is worth. Resolve from `reason_code` |
| Plaintext phone/email in a fraud table | Keyed HMAC. That table is the juiciest target in the database and the worst thing to hold in plaintext |
| Manual balance edits | Use a reason-coded `ADJUST` journal. A balance you cannot explain is a balance you cannot defend |
| Silent expiry | A single T&C line is the pattern users and regulators punish hardest. Per-lot display plus reminders |
| Calling coins "free" | Say "earned". See [07](07-compliance-nepal.md) |
| Paying uploaders before moderation | Creates the incentive to upload uncleared material and puts IP exposure on the platform |
| Calling a coin a "prize", "award", "win", "raffle", or "draw" | "Windfall gain" under Income Tax Act 2058 is defined to include "lottery, gift, prize, baksis, award for winning" and is taxed under s.5/88A. A naming choice, not a copy preference. |
| Any randomising reward (lucky multiplier, scratch card, mystery reward) | Engages the Lottery Act 2025 (1968) permission regime and CPA 2075 s.16(2)(d) contest pricing. Deterministic redemption only. |

---

## 4. Expiry policy

`SOURCED` — loyalty-programme research converges that expiry is a **trust tax, not
a revenue lever**: points going unused is a symptom of low perceived value, and
customers who feel penalised disengage rather than accelerating. Activity-based
expiry (any qualifying action resets the clock) outperforms fixed calendar expiry.
Typical windows run 12–24 months.

⚠️ **Confidence: medium.** These are loyalty-retail and SaaS figures, vendor-
compiled and secondary. They are not edtech figures and not Nepali. The *direction*
is well supported; the specific percentages should not be quoted as if they were.

`DERIVED` — our policy, split by bucket because the sources genuinely conflict
between urgency and trust:

| Bucket | Policy | Reasoning |
|---|---|---|
| **Allowance** | 30 days, hard | It is a trial. Carries no expectation of permanence, and expiring is the urgency lever that makes it drive activation. Now that the redemption is confirmed exempt there is no VAT consequence to expiry, which removes the last reason to be timid about it |
| **Earned** | 12 months, extended to 18 on any qualifying action, 2 reminders | The frustration data is about *earned* points vanishing, so this is where trust is lost. Activity-extension means an active student is never punished |
| **Purchased** | Does not exist | D1. If D1 ever changes, this bucket must never expire — see ADR-001 |

Three mitigations, near-free, that the UI must implement: show `expires_at` per lot
rather than in a terms page; remind at 30/7/1 days; never delete a balance without
telling the student.

---

## 5. Instrumentation

`SOURCED` — Unity names six health metrics explicitly. Instrument all six from day
one in a `coin_economy_daily` rollup. **The dashboard is the actual deliverable; the
target numbers are not.**

| Metric | Why it matters | Alert |
|---|---|---|
| **Sink:faucet ratio** | Detects both deflation (hoarding) and inflation (worthless coins) | Outside 0.7–1.3 monthly |
| **Inflation rate** | Are coins becoming worthless? | Rising trend |
| **Currency velocity** | Coins earned ÷ coins spent — separates earners from spenders | Sustained < 0.4 means coins are accruing, not circulating |
| **Days of currency on hand** | How long the current balance lasts at the current spend rate | Sudden rise means hoarding |
| **Payer conversion** | Share of students who spend anything | Falling after an earn-rate change means the change was wrong |
| **ARPPU** | Average revenue per paying user — only meaningful once a paid tier exists | Defer |

**Plus three feature-specific metrics** the generic list does not cover:

1. **Referral qualification rate** — invited → qualified. A drop means either fraud
   or a broken qualification rule. These need to be distinguishable, so track
   *rejected* and *never qualified* separately.
2. **Allowance redemption curve** — what share of the 3+1+1 is actually used, and
   how fast. A low number means the allowance is too generous or the catalogue is
   not attractive enough.
3. **Fraud ratio** — `DERIVED` target: referral issuance ≤ 8% of total issuance.
   Treat as a ratio monitored weekly, not an incident discovered later.

**Operational queries worth having from day one:** cohort retention by
first-unlock-week, the median time from signup to first unlock (the activation
metric this whole feature is judged on), and the distribution of time-to-first-unlock
by acquisition source, which will show you whether the referral cohort activates
faster than the organic one.

---

## 6. What is unverified

Do not cite these onward:

- **No platform case studies were retrieved.** Nothing in this document is based on
  observed data from Duolingo, Steam, Stack Overflow, or any comparable earn-rate or
  expiry policy. Any intuition about those is unvalidated here.
- **Game-economy primary literature (Hamaker, Carter, Playnomics, GameAnalytics) was
  not retrieved.** The faucet/sink framing and the six metrics come from Unity's
  published guide, which is a vendor document, not peer-reviewed work.
- **The loyalty-expiry statistics** are vendor-compiled and retail/SaaS-derived.
  Directionally useful, not transferable to Nepali edtech.
- **The CAC → coin-value derivation in §2.2 is arithmetic on an input we do not
  have.** It is a framework, not a number.
- **The §2.3 faucet table is illustrative and almost certainly too large.** It is
  an argument for the referral cap, not a forecast.
- **The redemption's VAT classification is answered and settled in our favour.** The
  chartered accountant has confirmed the unlock is an exempt educational service
  and that we sit below both registration triggers, so coins are not
  margin-negative. Two constraints carry forward: we can never claim input VAT
  credit while the business is exempt-only, and the coin catalogue must stay
  inside the educational perimeter. See
  [07](07-compliance-nepal.md) §2.3.
