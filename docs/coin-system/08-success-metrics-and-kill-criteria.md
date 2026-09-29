# 08: Success metrics and kill criteria

This document answers one question: **how will we know whether the coin feature
worked, and at what point do we change the numbers or stop?**

It is written for a decision, not for a dashboard. Every threshold below is one
you can approve or reject today. Where a number is not yet known it is marked `TBC`
rather than guessed, and that is deliberate. A plausible-looking invented number
is worse than an acknowledged gap. A gap gets closed. A guess gets quoted back to
us in a board meeting six months later. The engineering detail is in
[05](05-economy-and-fraud.md) and [04](04-implementation-plan.md), and nothing here
contradicts them.

---

## 1. The one number that matters most

Time from signup to a student's first unlock, and the share of new students who
unlock anything at all in their first week. Both halves matter. The first tells
us the feature works immediately. The second tells us whether it works for enough
people to be a business.

| Measure | What it tells us | What good looks like |
|---|---|---|
| Time to first unlock | Median hours from account created to the first unlock of anything | Hours, not days. A student who does not unlock in their first session has not understood what the coins are for. |
| First-week activation | Share of new accounts unlocking at least one thing within 7 days | A meaningful share, rising month to month. This is the headline number, and it is a percentage of signups, not of coins. |

**Coins issued is not a success metric.** It is a supply number. If coins go out
and nothing comes back, we have built a faucet, not a loop. Coins issued without
unlocks is a failure signal, and it should be read as one even while it grows.
The three numbers that would let a broken economy hide behind a big figure are
total coins issued, coins earned per user, and average wallet balance. We report
those for diagnosis. We do not celebrate them.

One further measurement from day one, because it tests the premise rather than
the tuning: split time-to-first-unlock by where the account came from. If
referred accounts unlock faster than organic ones, the referral mechanic is doing
its job. If not, we have a channel that costs us coins and returns nothing, and
that is a different decision.

---

## 2. The metric table

Eleven metrics, instrumented from launch in a daily rollup. The first six are the
standard health checks for an in-app currency, the next three are specific to what
we built, and the last two are activation.

### 2.1 Health of the currency itself

| Metric | In one sentence | Where it should land | What bad looks like |
|---|---|---|---|
| Sink-to-faucet ratio | Of every coin we hand out, how many get spent | 0.9 to 1.1 a month at steady state | Under 0.7: students hoard and coins feel like a debt they are owed. Over 1.3: students earn faster than they can spend and prices are too cheap. |
| Inflation rate | How fast the total coin supply is growing | Flat to slightly falling once settled | Any sustained rise. Coins only mean something while the pile of them is not growing faster than the things they buy. |
| Currency velocity | Coins earned divided by coins spent, so it separates people who spend from people who collect | Around 1.0. Sustained below 0.4 is the alert. | A floor of 0.4. Below it coins are piling onto balances instead of circulating. |
| Days of currency on hand | How many days the average balance would last at today's spend rate | Steady, roughly stable month to month | A sudden rise means hoarding. A sudden fall means students cannot save up for anything. |
| Payer conversion | Share of active students who spend at least one coin | Rising, and never falling after a change we made | A drop that follows an earn-rate or price change. It means the change was wrong and should be reverted. |
| ARPPU | Average revenue per paying user | Deferred. There is no paid tier, so there is no revenue and no payer. | Not applicable. Reporting a figure here would mean inventing one. Revisit only if a paid tier is ever approved. |

### 2.2 Specific to what we built, plus activation

| Metric | In one sentence | Where it should land | What bad looks like |
|---|---|---|---|
| Referral qualification rate | Of friends invited, what share qualified by completing a profile and verifying a phone | 3% to 25% of invitees. The range is wide on purpose, we have no measurement of it. | Under 3% means the referral programme is mostly noise. Over 25% means we are paying for something that is not real. Rejected and never-qualified invites are tracked separately, because a drop in one means a fraud pattern and a drop in the other means a broken rule. |
| Starter-pack redemption curve | Of the 3 resources, 1 video and 1 mock test we give every new account, what share gets used and how fast | Most of the allowance used inside 30 days, with the video and mock test used before the documents | Very low use means the pack is too generous or the catalogue is not attractive enough. Everything used at once means it is too small to show what the catalogue really offers. |
| Fraud ratio | Referral coins that turn out to be fraudulent, as a share of all coins issued | At or under 8% of total issuance | Over 8% held for two consecutive weeks means the database controls are not enough, and we choose between adding more and stopping referral pay. |
| Time to first unlock | Median hours from signup to the first unlock of anything | Hours, not days | A median in days, or a long tail where most students never unlock at all |
| First-week activation | Share of new accounts unlocking anything within 7 days | `TBC`. The method for setting it is in §6.3. | No defensible number yet. What counts as bad is a rate that does not improve after two full months of real data and two rounds of fixable causes. |

---

## 3. Kill and re-price criteria

**This is the section that matters most.** Every trigger below is a decision we
make in advance and in writing, so nobody has to argue it later while the numbers
are still moving.

### 3.1 The number we are still missing

Blended customer acquisition cost: `TBC`.

We cannot set the coin's real-world value without it. The rule is fixed and
arithmetically simple, and it is the only thing standing between this feature and
a profitability argument.

> **One coin is worth roughly (blended CAC x 0.3) divided by 60.**
>
> The 0.3 is because one mock-test unlock should be worth about 30% of what it
> costs to acquire a student. The 60 is the coin price of a mock test. A student
> who never pays us back is a net loss, so the unlock has to carry real weight.

| If blended CAC is | 1 coin is about | A 60-coin mock test is about |
|---|---|---|
| NPR 200 | NPR 1 | NPR 60 |
| NPR 400 | NPR 2 | NPR 120 |
| NPR 800 | NPR 4 | NPR 240 |

The table collapses to one row the moment you tell us your blended CAC. It is
arithmetic on an input we do not have, not a recommendation.

Three things change once we have it, and nothing else in the economy does:

1. Whether a paid unlock is profitable at all. This is the whole test.
2. What a referral is worth relative to acquisition. A 60-coin referral is worth
   roughly 30% of CAC, so one that costs us money and returns nothing is a loss,
   and one that returns a paying student is close to break-even on acquisition alone.
3. The per-unlock revenue threshold, which comes straight from it.

One note on how we will use it. Coin value is a unit of account for our own
reasoning, and it must never appear in the product. Displaying "1 coin = NPR 4" on
an instrument with no cash redemption path is a misleading value claim and the
easiest way we have to create a consumer-protection problem. See [07](07-compliance-nepal.md).

### 3.2 When the economy runs too hot

Students are earning faster than they can spend. Coins become meaningless,
prices feel too cheap, and the sense that a coin means something is lost.

| If this happens | We do this | Who decides |
|---|---|---|
| Monthly sink-to-faucet ratio above 1.3 for two consecutive months | Cut the referral reward from 60 to 30. This is lever one, and it is the right one, because a referral at 60 coins is 1.5x a whole mock-test unlock, so referrals dominate issuance the moment they work. | Product. Configuration change, no deploy. |
| Still above 1.3 after the referral reward is cut | Lower the monthly referral cap from 10 successful referrals to 5 | Product |
| Still above 1.3 after both | Raise prices. Last resort, and it goes to founders, because it directly harms the student we are trying to serve. | Founders |
| Outstanding balance above 2x the monthly faucet | Alert. Freeze the referral reward at its current value, raise no earn rate, review weekly until it is back inside 2x. | Product |
| Outstanding balance above 3x the monthly faucet | Hard block on new issuance. Earning stops for everyone until it is back under the line. The block lives in the database, not in a dashboard, because a warning nobody acts on is not a control. | Automatic. No meeting required. |

Hold onto this when inflation hits: we cut what we give away, not what students pay.
Price rises are the last lever, not the first, and the only one a student feels as a loss.

### 3.3 When the economy runs too cold

Students hoard. They earn, they do not spend, and the coins feel like a debt the
platform owes them. This is the more dangerous of the two failure modes, because
nothing looks broken. Coins are issued, students are retained, and the product
feels like it is working while the catalogue stays shut.

| If this happens | We do this | Who decides |
|---|---|---|
| Monthly sink-to-faucet ratio below 0.7 for two consecutive months | Tighten the earn rates, starting with upload and profile, so there is less to collect and more to spend. | Product |
| Currency velocity sustained below 0.4 for a month | Same response, plus a direct fix: the first thing a new student meets should be something they can unlock, not a wallet. Reduce the starter pack so the first unlock comes from earning. | Product |
| Days of currency on hand rises sharply | Same response. A balance that grows and is not spent is the clearest single symptom of hoarding. | Product |
| Starter-pack redemption very low | The allowance is too generous, or the catalogue is not attractive enough. Test the second hypothesis first, because raising earn rates into a catalogue nobody wants to use makes the balance problem worse. | Product |
| Outstanding balance above 2x the monthly faucet | Same ceiling as §3.2. It applies in both directions. | Automatic at 3x |

**Do not respond to deflation by cutting prices.** That gives away value and
makes the liability worse. The fix is to give students something worth spending
on, or to stop giving them coins they do not want.

### 3.4 When fraud is the problem

| If this happens | We do this | Who decides |
|---|---|---|
| Referral fraud above 8% of total issuance for two consecutive weeks | Tighten the controls before touching the reward. Drop the cap from 10 to 5 and require a verified phone plus a completed profile to qualify. | Product and Backend |
| Still above 8% after that | Suspend the referral earn entirely and leave profile and upload running. The economy still balances without referrals. It just gets less interesting. | Founders, on the team's recommendation |
| One device or phone number claims many referrals | Automatic reversal. Already built in. No review, no decision. | Automatic |

We are not buying a fraud-detection vendor to fix any of this, and the maths is
against it. Fraud running 10x over target costs roughly NPR 5,600 a month in direct
revenue risk, while detection tooling at our signup volume would cost NPR 20,000 to
110,000 a month. We would pay more to catch the problem than the problem costs, and
most of it is preventable with database rules we own. See
[05](05-economy-and-fraud.md) §3.1.

### 3.5 What is not re-priceable, ever

Three changes are off the table, and they are not tuning decisions. Each turns a
closed-loop loyalty mechanic into a regulated instrument. If anyone proposes one of
them, the answer is no, and it is not a judgement call for this review.

| Change | Why it is out |
|---|---|
| Selling coins for money | The defining feature of a prepaid instrument. In Nepal that means a central bank licence, incorporation, paid-up capital, full float backing, KYC/AML, an annual audit and a settlement-bank agreement. The research puts it at 12 to 18 months and a licensed corporate structure. See [07](07-compliance-nepal.md) §4.1. |
| Students transferring coins to each other | Same reason, plus a consumer-law prohibition. |
| A friend's friend earning anything | Multi-level referral trees are prohibited by statute in Nepal, not merely regulated. There is no compliant version of a deeper tree to negotiate toward. |

---

## 4. Honest stop criteria

Tuning is for a working feature with the wrong numbers. This section is about a
feature that is not working, where the fix is not a smaller number.

**We will stop, not tune, if any one of these is true.**

1. **Activation on first unlock stays low after two full months of real data and
   the causes we can fix have been fixed.** Two months is the minimum. By then we
   will have closed the catalogue gaps, smoothed the confusing moments and fixed
   the broken video links. If the share has not moved after all of that, no price
   change will make the feature land.

2. **The liability ceiling is breached repeatedly and cannot be brought back by
   tuning.** If outstanding balance keeps passing 3x the monthly faucet and the
   hard block does not hold, we do not have an economy, we have an accounting
   liability our accountant cannot sign off.

3. **Referral fraud proves resistant to the database controls.** If the only
   remaining fix is a third-party fraud vendor, we have already decided not to buy
   one, because Nepal's Privacy Act makes passing student data to a third party
   without specific consent a criminal offence, with no regulator and no safe
   harbour. The referral earn switches off and the rest of the feature carries on
   without it.

4. **Turnover crosses the VAT registration threshold and we have not re-examined
   the exemption.** The redemption is exempt, which is why the coins work and why
   the unit economics stand. That answer is a position on today's facts, and it
   lasts only while we stay exempt-only. The moment turnover reaches the threshold,
   registration becomes mandatory with a 30-day clock to comply, and the
   exempt-versus-taxable question re-opens under time pressure. The trigger is
   crossing the threshold, the action is getting the exemption re-confirmed in
   writing first, and the review happens quarterly rather than on a date we can
   pick. A business loan over Rs 10 lakh triggers it as well, and that one can
   arrive without warning. If we cannot re-confirm in time, **stop and re-plan**
   rather than register into a classification we have not tested. See
   [07](07-compliance-nepal.md) §2.3.1.

5. **The coin catalogue drifts outside the educational perimeter.** The exemption
   rests on a narrow reading of "educational services provided by schools and
   universities", and it holds only while everything a coin unlocks is educational.
   Adding a non-educational reward to the coin catalogue, or a non-educational
   revenue line next to it, risks a mixed position that could change the
   classification and put the entire economy back in question after we have built
   on it. **A coin that unlocks something other than a study resource, a video
   lecture, or a mock test is a stop, not a feature request.** This is enforced in
   the schema, so the question is whether anyone routes around it. See
   [07](07-compliance-nepal.md) §2.3.1.

6. **The coin mechanic is the only thing holding signups up.** If downloads and
   daily actives are flat or falling while coin balances grow, we are paying for
   conversion and not getting it, and there is a cheaper way to buy the same
   retention.

**We will not stop because:** the economy needs a price change, the referral
qualification rate is lower than hoped, or the first month of data looks
uncomfortable. Those are exactly what the levers in §3 are for.

What we would do instead, in most cases, is turn the gates off. Content returns to
free, coins already spent stay spent, nothing is refunded, nothing is rewritten. The
cost of being wrong is small, and that asymmetry is why we are willing to launch.

---

## 5. Review cadence

| When | What is reviewed | Who | What it is for |
|---|---|---|---|
| Weekly, from soft launch | Fraud ratio, balance against the 3x ceiling, referral qualification rate, support volume, failed access attempts on gated resources by route | Product and Backend | Catching a fraud run, a liability problem, or the anonymous-access regression while they are still small |
| Month 1, soft launch | Nothing is tuned. The numbers are recorded and left alone. | Product | Establishing the baseline |
| Month 2, first real review | All eleven metrics in §2 against their targets. The first re-price decision is made here. | Product, with founders | The first and most important tuning decision |
| Month 3 onwards | The same review monthly, until three consecutive months are inside the healthy range | Product | Confirming a change held rather than just moved |
| Quarterly | Full metric review, fraud assumptions, whether the referral mechanic earns its cost, and the two VAT growth triggers in §4 (distance to the registration threshold, and whether the catalogue is still entirely educational) | Founders | The stop-or-continue decision in §4 |
| On any breach | Immediately, not at the next scheduled review | Whoever found it | A 3x breach is a block, not a meeting |

### 5.1 How much data before the numbers mean anything

**The first week's numbers are not a trend, and the first month's numbers are also
not a trend.** We will say this at every review so nobody mistakes noise for signal.

| What we need first | Why |
|---|---|
| One full calendar month before any re-price | Weekly figures swing with the academic calendar. A month is the shortest window not dominated by which week we happened to look at. |
| Two consecutive months on the same side of a threshold | One month out of range is ordinary variation. Two in a row is a pattern. |
| A meaningful number of new accounts per month | With a handful of students a week, one extra referral moves the fraud ratio by several points and nothing can be concluded. Sample size gets reported next to the rate at every review. |
| Enough unlocks to compare spend against earn | A ratio computed on a few hundred coins is noise, and we will not re-price off it. |

Two rules override all of the above. A number we cannot explain is not a basis for
a decision, so every re-price ships with a one-line explanation of what changed
and why we believe it. And the targets in §2 are operating targets we set, not
sourced benchmarks. Nobody outside this team should treat them as industry
standards, because that data does not exist for a Nepali closed-loop student economy.

---

## 6. What we deliberately do not measure

Three things are out of scope on purpose, and all three are decisions, not gaps.

### 6.1 ARPPU, until there is a paid tier

ARPPU is average revenue per paying user. We have no revenue and no payer, so
there is nothing to compute, and reporting a figure would mean inventing one. The
moment a paid tier is ever approved, this metric returns with a real number attached
and with the per-unlock revenue thresholds in §3.1 already derived from CAC. Until
then the honest position is that the feature has no revenue line, and success is
measured entirely on whether students use what we built.

### 6.2 A/B testing economy parameters, not yet

**We are not running experiments on earn rates, prices, or referral rewards.** Two
reasons. First, the funnel is not instrumented yet, so an experiment measures
noise, and a noise-driven price change is a change nobody can justify when it
fails. Second, there is a specific hazard. A randomised reward, a lucky multiplier,
a scratch card, a mystery bonus: those are the default content of a growth
experiment, and in Nepal a chance element converts a deterministic access right
into a lottery, which carries its own permission regime and its own consumer-law
exposure. There is a test in the codebase that fails if a randomising reward path
is ever added, so an experiment cannot introduce one by accident. Experiments on
economy parameters become available after Phase 2, once the daily economy rollup
has data behind it.

### 6.3 The one target we refuse to guess

**There is no defensible number for the first-week activation target, and we are
not going to invent one.**

Why not. Our resources have been free and ungated since they were published, so
we have never measured what share of the people who download a resource go on to
create an account. There is no baseline, and the projection in
[05](05-economy-and-fraud.md) §2.3 already showed how badly a forecast misses
when it rests on a participation rate nobody has measured.

The method, instead of a number:

1. Ship the gates to a slice of users. Measure first-week activation for that
   cohort, and time from signup to first unlock, as a distribution rather than a
   single median.
2. Fix the causes we can identify from that data. A low unlock rate has several
   possible explanations, confusing flow, a catalogue gap, a broken video link, a
   genuinely uninteresting proposition, and they need different responses.
3. Re-measure, and set the target off the improved number, not the original.
4. Only then does a fall below that target count as a stop-criterion trigger.

What a reasonable target looks like, as a range rather than a point. Anchor it on
the download-to-account relationship, which is itself unknown and is part of what
the first cohort measures. Setting an activation target against a conversion rate
we do not have is a guess with extra steps. What can be said is that first-unlock
rate should sit well above the account conversion rate among downloaders, because
signing up and unlocking are different acts of intent. A soft-launch cohort in the
low tens of percent is unremarkable. A cohort in the single digits, sustained, is
the §4 stop signal.

**You are not approving an activation target today.** You are approving a method and
a commitment to come back with a target once the data exists. If you want a number
committed to in advance we will push back, because that would be committing to a guess.

---

## 7. The two placeholders in one place

Both are unknown, both are owned outside engineering, and both are on the critical path.

| Unknown | Status | What it unblocks | Who has it |
|---|---|---|---|
| Blended customer acquisition cost | `TBC` | Coin value, the profitability test, and the per-unlock revenue thresholds | Founders and marketing |
| First-week activation target | `TBC`, method agreed in §6.3 | Whether activation counts as success or as a stop signal | Set after the first soft-launch cohort |

Neither stops the build. Both stop us saying with confidence that the feature is working.

---

## 8. What we are asking for

Four decisions, all of which can be made before the build finishes.

1. **Approve the kill and re-price triggers in §3 as written**, including the
   hard block at 3x and the fact that price rises are the last lever rather than
   the first. The value of these triggers is that they are agreed before the
   numbers are uncomfortable.
2. **Approve the stop criteria in §4**, specifically the two-month activation
   test, the growth-threshold re-examination trigger, and the educational
   perimeter constraint on the coin catalogue.
3. Give us the blended CAC, or tell us when you will have it. Every figure in
   §3.1 is waiting on it.
4. Accept that no activation target is being set today, and that the method in
   §6.3 is what you are approving instead.

With all four agreed, the feature ships with the gates off, turns on one at a time,
and comes back to you at week 12 with real numbers for the first time. That review
is where this document gets replaced by data.
