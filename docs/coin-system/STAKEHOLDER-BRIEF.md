# Coin system: what we are proposing, and what we need from you

Prepared by the engineering and product team · 26 September 2026 · 12 minute read

This is a decision document, not a technical spec. The full specs are in
`docs/coin-system/` and linked at the end. Everything you need to make a call is
in here.

---

## The short version

We want students to earn **StudsTokens** and spend them on study resources. Complete your
profile, invite a friend, upload a study resource that an admin approves, earn
StudsTokens. Spend StudsTokens to unlock a past paper, a video lecture, or a mock test. Every new
account gets three free resource unlocks, one video, and one mock test.

We think this is worth building. It turns three things we want students to do
anyway (finish their profile, bring friends, share resources) into a reason to come
back, and it creates a reason to hold an account rather than browse anonymously.

Two things needed a professional opinion before we started. One has come back and
it is good. The other has not, and it is the bigger of the two. The full detail is
below, but the short version is:

1. **Our accountant has answered the VAT question, and it is good news.**
   Unlocking a study resource, video, or mock test in exchange for StudsTokens is an
   exempt educational service, and we are below the threshold that would require us
   to register for VAT. So the StudsTokens do not carry a tax charge, the feature is not
   margin-negative, and phases 1 and 2 are unblocked. Two conditions come with the
   answer, both recorded below.
2. **Our past papers may not be ours to sell.** This is a bigger exposure than the
   VAT question ever was, and it is not new. It needs a lawyer, and it is the one
   thing still capable of changing the plan on its own.

Neither needs engineering. The lawyer can start Monday, and phase 0 should start
now regardless of both.

---

## What the feature does

**Students earn StudsTokens four ways**

| Action | StudsTokens | When they get them |
|---|---|---|
| Complete their profile | 25 (in five steps of 5) | Immediately, as each part is filled in |
| Invite a friend | 60 for them, 25 for the friend | Only after the friend completes their own profile and verifies their phone, then held 7 days |
| Upload a study resource | 80 | Only after an admin approves and publishes it |
| Refer a friend | Capped at 10 successful referrals a month | |

**Students spend StudsTokens three ways**

| What they unlock | StudsTokens |
|---|---|
| Study resource (past paper, notes, model questions, syllabus) | 40 |
| Video lecture | 90 |
| Mock test | 60 |

**New accounts get a starter pack:** 3 resource unlocks, 1 video, 1 mock test, which
expires after 30 days. We are calling them "starter unlocks" rather than "free
resources" for a legal reason explained further down.

**The maths that matters:** one referral (60 StudsTokens) buys one mock test. One
completed upload (80 StudsTokens) buys two mock tests. A student who does nothing gets
three starter unlocks and stops. A student who engages gets a real library.

---

## Why we are fairly confident this is buildable

We spent the exploration phase reading the actual codebase rather than designing in
the abstract. A few things came out of it that matter for your confidence.

**There is no half-built coin system to untangle.** No coin, wallet, credit or
referral code exists anywhere in either application. That sounds obvious, but it is
genuinely good news: nothing to migrate, no existing balances to reconcile, no
legacy behaviour to preserve. We design it correctly once.

**The hard part of the feature is well understood and already proven.** The real
technical risk is making sure a student cannot spend the same coin twice if they
double-tap a button or a request times out. We built a working version of the
design against a real database and tested it with twenty simultaneous competing
requests. Exactly ten succeeded, ten were rejected, and the balance landed on
exactly zero. That specific problem is solved.

**The parts we would normally worry about already have a home.** Your team has an
admin dashboard with a working "pending approval" pattern for colleges and
providers. The new resource-approval queue copies it. Your notification system can
send a message inside the same database transaction that grants the StudsTokens, so a
student never gets told they earned something they did not.

**We found two live security problems while reading, and you should know about them
regardless of whether we build this.** Study resource downloads currently have no
authentication at all. Anyone, including someone who has never signed in, can
download any published file today. Separately, the download centre has a bug that
lets unpublished files be downloaded by guessing the URL. Neither is caused by the
coin system, but both need fixing, and the first one becomes more serious the
moment content is worth money.

---

## Three findings that need your attention

These came out of the Nepal-specific legal research. Two of them changed our
design. One of them has now been answered, and it unblocks the build.

### 1. The VAT question, answered, and the answer is good news

**Our chartered accountant has confirmed two things: unlocking a study resource,
video lecture, or mock test in exchange for StudsTokens is an exempt educational service
under Schedule 1, and we are not required to register for VAT.** We asked for this
because the previous read of the statute was the opposite, and it was the one answer
that could have made the whole feature uneconomic.

Why it mattered so much. If unlocking a resource were a taxable service at 13% and
we sat below the registration threshold, we would pay 13% on the value of content we
would otherwise have given away free, with no input credit to set it against,
because an unregistered business cannot claim one. Every coin would have been a
small loss. That is the multiple-lakh annual question this project turned on.

It is not. No coin carries a tax charge, so the StudsTokens are not margin-negative and
**phases 1 and 2 are unblocked.** The accountant also confirmed we are below the
registration threshold on both triggers that would oblige us to register: our
turnover against the Rs 30 lakh services threshold, and the Rs 10 lakh business
loan trigger.

**Two conditions come with the answer, and both are real.**

**We can never recover input VAT.** In Nepal, a person doing only exempt
transactions is not permitted to register for VAT at all. Because our service is
exempt, that is us. So the VAT on anything we buy in future, cloud hosting, tools,
vendors, equipment, is never recoverable. To be clear about what this is and is not:
it is not a new problem, and it is not a regression. We are unregistered today, so
we have no credit today and we are giving up nothing we currently have. It is a
standing cost of the arrangement that made the StudsTokens work.

**The coin catalogue has to stay educational.** The exemption is based on a narrow
reading. Schedule 1 Group 6(c) covers "educational services provided by schools
and universities", and we are neither. The reading holds while everything the coin
system unlocks is educational, and only then. So a coin must unlock a study
resource, a video lecture, or a mock test, and nothing else. If we ever add a
non-educational reward next to the StudsTokens, or a non-educational revenue line beside
them, we risk a mixed position that could change the classification and put this
answer back in question. **This is now a design constraint on the catalogue, not
just a legal opinion.** We will enforce it in the schema and in review.

**Two things we still want in writing.** The answer is settled but the reasoning
behind it has not reached us, and for an exemption this narrow we want the reasoning
on the record, not only the conclusion. Separately, the Finance Act 2083 (effective
July 2026) replaced Schedule 1 in full and our research could not obtain the
current gazetted text, so we need to know which version the accountant worked from.
Both are confirmations, not a blocking question.

### 2. Privacy: a criminal offence, and the consent quality is all that stands between us and it

Nepal's Privacy Act 2075 makes it an offence to pass a student's personal data to a
third party without their consent. The penalty is up to three years'
imprisonment, and the Government of Nepal prosecutes, not the student.

We would be collecting name, phone, email, date of birth, address, and their
education history. All of that is personal information under the Act.

Nepal does not require data to be stored locally, so sending data abroad is
permitted. But there is no data protection authority, no regulator to ask, no
adequacy mechanism, and no safe harbour. The consent is the only protection.

The practical consequence for us: **do not buy a fraud-detection vendor at launch.**
We had planned to evaluate third-party fraud tools, and the economics never
supported it anyway. This removes the temptation entirely, and we can do fraud
prevention with database rules we own.

The trap for later: if we ever add such a vendor, a single "I agree to the terms"
checkbox is not good enough. We would need separate, specific consent naming the
vendors, the data, the countries, and the retention period.

### 3. Most of our students are under 18

Section 33 of the Privacy Act requires guardian consent and a benefit test before
disclosing a minor's personal information. Much of the +2 and competitive exam
audience is under 18, and a referral inherently discloses one student's details to
another.

We are recommending we restrict the referral reward to accounts that declare they
are 18 or over. This is cheap to build and defensible. The alternative, a
guardian-consent flow, is a significant product project of its own and we do not
think it is worth it at this stage.

This also removed something from the design: Nepal's consumer protection law
expressly prohibits a "token system" and "levels or series" operated with more than
one other person. A multi-level referral tree is prohibited by statute here, not
merely regulated. Our design credits only the person who directly invited, which is
single-level by construction, but we are adding a test so nobody can quietly change
it later.

---

## A fourth thing, separate from this feature, that is larger than all of it

**We may not have the right to sell past papers and model questions commercially.**

Nepal's Copyright Act allows reproduction "for teaching and learning", but only of
"a small portion" of a work, and only for "activities to be performed in the
classroom". A commercially run online bank of past papers is neither. The author
retains the exclusive right to reproduce and to sell.

This is not a coin system issue. It is about the core product. Our research found
no Nepali case law and no NEB or HEB position on who owns the copyright in
question-syllabus papers, and the legal position is genuinely unsettled. With the
VAT question answered, **this is now the largest unresolved legal exposure in the
project**, and the only one still capable of changing the answer on its own.

For context on why this is live rather than theoretical: "NEB notes, past papers,
model questions" is an established commercial category in Nepal. It appears to
operate without licences, which tells you it is happening, not that it is safe.

**Recommendation:** commission original questions for anything we charge for, and
get a lawyer's view on the existing bank before we scale. The upload feature
actually makes this better, not worse. Because StudsTokens are only awarded after an
admin approves a resource, we control what enters the library. We would be creating
a moderation function rather than adding to a content problem.

---

## What it costs

**Engineering: 5 to 6 engineer-weeks.** Broken into six phases, and the first
phase is deliberately useful on its own:

| Phase | What | Time |
|---|---|---|
| 0 | Fix the two security holes, add rate limiting, add phone verification | 3 to 5 days |
| 1 | Build and test the coin ledger (no user sees anything) | 5 to 7 days |
| 2 | Put the payment gates on resources, ship the starter pack | 5 to 7 days |
| 3 | Profile earning, referrals, user uploads with approval | 7 to 10 days |
| 4 | Admin screens and the economy dashboard | 4 to 6 days |
| 5 | Expiry notices and reminders, then tuning from real data | 3 to 5 days |

Phase 0 is worth doing whatever you decide about the coin system. It closes two
live security holes and adds the rate limiting that any referral-driven feature
will need.

**Money:** no new vendors, no payment gateway, no fraud-detection subscription, by
design. The one meaningful cost is professional time: the accountant to put the tax
position and its reasoning in writing, and a lawyer for the copyright question.

**What we are giving up:** resources stay free to anonymous visitors until Phase 2
ships. That is the point of the feature, but expect a visible change in how
downloads convert. We are shipping it with the gates switched off by default and
turning them on one at a time, so it can be reverted without a deploy.

---

## What we are deliberately not doing

**Not letting students buy StudsTokens with money.** This is the decision that keeps the
whole thing simple. If StudsTokens can be bought, they become a prepaid payment
instrument, which in Nepal means a central bank licence, incorporation, mandatory
paid-up capital, full float backing, KYC/AML, an annual audit, and a settlement
bank agreement. Our research puts that at 12 to 18 months and a licensed corporate
structure.

There is no version of this feature where letting people buy StudsTokens makes them more
likely to finish their profile, invite a friend, or upload something useful. We
would be adding regulatory surface to a retention mechanic.

**Not letting students transfer StudsTokens to each other.** Same reason, plus the
consumer law point above.

**Not letting StudsTokens expire silently.** StudsTokens we give for referrals expire after a
year, extended if the student stays active. The starter pack expires in 30 days. We
show the expiry date on each balance, remind students 30, 7 and 1 days ahead, and
never delete a balance without telling them. Loyalty research is consistent that
surprise expiry drives people away rather than bringing them back.

**Not randomising anything.** No lucky multipliers, no scratch cards, no mystery
rewards. In Nepal a chance element turns this into a lottery, which has its own
permission regime. This is the kind of thing that gets added by accident during an
A/B test, so we are putting a test in place to stop it.

---

## Decisions we need from you

1. **Do we proceed now that the VAT question is answered?** Our recommendation is
   yes, and not conditionally. Phases 1 and 2 are unblocked, so the plan starts
   immediately rather than waiting on the accountant. The two things the answer
   leaves live, keeping the catalogue educational and watching the registration
   threshold, are conditions on what we build rather than gates on whether we
   build. Phase 0 is worth doing whatever you decide.
2. **Referral rewards limited to 18+ accounts?** Our recommendation is yes. It is
   cheap, defensible, and keeps us out of guardian-consent work.
3. **Do you want the lawyer on copyright now, or after launch?** Our recommendation
   is now, but scoped: one question, is our past-paper bank something we can sell.
   It is now the largest unresolved legal exposure in the project, it is larger than
   the VAT question ever was, and it is not going away on its own.
4. **What is our blended customer acquisition cost?** We cannot set the coin value
   without it. Every price in the spec is a placeholder until we have this number.
   The rule is simple: one mock test unlock should be worth roughly a third of what
   it costs us to acquire a student. If acquiring a student costs Rs 400, a coin is
   worth about Rs 2, and a mock test at 60 StudsTokens is worth Rs 120.

---

## Confirmations to get from the accountant

The blocking question is answered. What is left is to get the answer on the record
and to close two loose ends, none of which changes the decision. Copy these.

1. Please put **the conclusion in writing, with the reasoning behind it.** We hold
   the conclusion that the redemption is an exempt educational service, and the
   exemption depends on a narrow reading of "educational services provided by schools
   and universities", so we need the reasoning on the record.
2. Which version of Schedule 1 did you work from: the **current gazetted text as
   replaced by the Finance Act 2083**, or a pre-amendment reprint? Our research could
   not obtain the current gazette.
3. Confirm that **issuing a coin is not itself a separate supply**, but purely part
   of the consideration for the exempt redemption. Our own analysis reached that
   conclusion and the exemption depends on it.
4. Does the **expiry of unredeemed StudsTokens** create a taxable supply? We plan to expire
   some balances deliberately. With no VAT to owe this is now mainly an accounting
   question, so we will route the treatment to the auditor as well.
5. Does the **3% Education Equity Fee** apply to us, or only to schools, colleges
   and universities directly?
6. **Monitorable, not a question:** we are told we can never register for VAT while
   we stay exempt-only, so input VAT on purchases is never recoverable. Please
   confirm that is right, and confirm what happens to the position if turnover
   reaches the Rs 30 lakh threshold or we take a business loan over Rs 10 lakh.

**And the question for the lawyer, in one line:** can we commercially sell a bank
of past examination question papers in Nepal, given that the Copyright Act's
teaching exception covers only a small portion of a work and only classroom use?

---

## Timeline

| When | What |
|---|---|
| This week | Phase 0 starts. Security fixes, rate limiting, phone verification. Lawyer engaged. |
| Within 2 weeks | Lawyer responds. The accountant returns the written reasoning and the two confirmations, in parallel and without blocking the build. |
| Weeks 3 to 8 | Phases 1 to 5. Ledger first and tested hard before any money is involved. |
| Week 9 | Soft launch to a slice of users, gates switched on one at a time. |
| Week 12 | First economy review against real numbers, then tune prices. |

The VAT gate is gone, so this is now a routine six-week build. The one thing that
could still change the plan is the copyright question, and that sits outside this
feature. **If a growth trigger fires, the exemption comes back into question before
we act on it**: turnover near the Rs 30 lakh threshold, or any business loan over
Rs 10 lakh, goes to the accountant first.

---

## The honest summary

The engineering is in better shape than we expected. The ledger design is
validated, the codebase cooperates, and phase 0 is worth doing on its own merits.

The VAT question is answered, and it went our way. That was the one answer that
could have made the feature uneconomic, and it did not, so phases 1 and 2 are
unblocked and we can build without waiting on anyone. Two conditions come with it:
we can never recover input VAT while we stay exempt-only, which costs us nothing we
have today, and the coin catalogue has to stay entirely educational, which is a
design rule we will enforce rather than a legal opinion we hope holds.

**The copyright question is the larger exposure and it is still open.** It is
bigger than the feature we are proposing, it predates us, and it needs a lawyer
this month.

What I would not do is treat the VAT answer as permission to relax. The exemption
holds because we are exempt-only, and the two ways to stop being exempt-only are
turning over more and selling something that is not educational. Both are cheap to
avoid now and expensive to unwind later.

---

**Full technical specs:** `docs/coin-system/`

| Document | For |
|---|---|
| `01` Current state and feasibility | Anyone assessing whether this is doable |
| `02` Architecture | Engineers building the ledger |
| `03` API contract | Frontend and backend engineers |
| `04` Implementation plan | Engineering planning and tracking |
| `05` Economy and fraud | Product and finance, now and at launch |
| `06` UI/UX specification | Designers and frontend engineers |
| `07` Nepal compliance | Legal and finance. Needs a chartered accountant to sign off |
| `ADR-001` Closed-loop StudsTokens | Everyone. It is the constraint most likely to be broken by accident |

Questions on anything above, come back to us. If a decision here is not obvious
from the document, that is our failure to explain it, not a reason to guess.
