# 06 — Coin & Rewards System: UI/UX Specification

**Status** · SPEC PHASE — no code written. This document decides the design; the build follows it.
**Scope** · `studsnew/` (Next.js App Router, React, TypeScript, Tailwind v4).
**Companion docs** · 01–05 in this folder cover the earn/spend ledger, entitlement model, API and data model. This doc assumes those and specifies only what a student sees.

> **Market correction (supersedes the earlier brief):** StudSphere is a **Nepal**-federated edtech platform, not India. All copy in §10 is Nepal-appropriate English with British-leaning spelling. The repo confirms the context — `app/onboarding/page.tsx:7-25` offers `+2 Running` / `SEE Graduate` / `IOE` / `IOM` / `CEE` / `KU` / `TU` / `MBBS` / `BBA` / `CTEVT`, and the one existing pricing surface prices in **NPR** (`app/institution-zone/pricing/page.tsx:51`).
>
> **Compliance rule — unchanged and non-negotiable.** Never describe coins as "free" — say *earned*, *starter*, *included with your account*. Never render any currency equivalence for coins ("worth NPR 500", "NPR 5,000 of value", an FX rate in a tooltip). A coin cannot be bought, sold, or redeemed for cash, so any monetary value attached to it is a false or misleading claim about value.
>
> **The statutory basis for this rule is now verified — and the earlier section number was wrong.** An earlier draft of this document cited "Nepal's Consumer Protection Act, 2075 (2018), Section 18". **The Act number and year were correct:** the Consumer Protection Act 2075 (2018) is the operative statute, superseding the 2054 (1997) Act. **The section number was wrong:** the misleading-advertising prohibition is **s.16(2)(b) and s.16(2)(c)(3)**, not s.18. s.38(f) makes it an offence, and s.40(3)(b) punishes it with **2 to 5 years' imprisonment or a fine of NPR 400,000–600,000, or both**. Source: [07-compliance-nepal.md](07-compliance-nepal.md).
>
> **The rule itself is not contingent on that research.** A coin has no cash value because it cannot be purchased (ADR-001), so a value claim is unsubstantiated on the product's own terms — it is a false statement about the instrument regardless of which statute prohibits it, and it is bad practice on any reading. The citation above is now sourced; the rule was implemented on product grounds first, and stands the same either way. For the record, an even earlier draft cited India's CCPA 2019 and a ₹10 lakh penalty, which was simply the wrong country.

---

## §0 — Design language: which dialect wins

### 0.1 The three study-resource pages are *already* unified at page level

This corrects the framing in the brief. All three catalog pages share identical chrome:

| Element | Value | Evidence |
|---|---|---|
| Page ground | `min-h-[70vh] bg-gray-50 py-8` | `StudyResourcesPage.tsx:249`, `VideoLecturesPage.tsx:134`, `MockTestListPage.tsx:172` |
| Container | `mx-auto w-full max-w-350 px-4 pb-14 sm:px-0` | `:250`, `:135`, `:173` |
| Back-link | `text-xs font-bold uppercase tracking-[0.14em] text-brand-blue` | `:256`, `:139`, `:177` |
| Heading | `text-3xl font-bold text-gray-900` | `:262`, `:144`, `:182` |
| Grid | `grid-cols-1 gap-3.5 sm:grid-cols-2 xl:grid-cols-3` | `:395`, `:261` |
| Modal | `fixed inset-0 z-50 flex items-center justify-center bg-gray-900/50 p-5` → `w-full max-w-sm rounded-lg bg-white p-6` | `:505,513` |

**There is no page-level dialect conflict to resolve.** The gray / `rounded-md` / `brand-blue` language is already the shared spine.

### 0.2 The real divergence is two levels down

1. **Card radius** — document card `rounded-md border-gray-200` (`StudyResourcesPage.tsx:403`); video list card `rounded-md` but `rose` borders (`VideoLecturesPage.tsx:270-274`); mock-test card `rounded-2xl` + `cyan` (`MockTestListPage.tsx:44-46`).
2. **The player stage** — `rounded-3xl bg-slate-950` (`VideoLecturePlayer.tsx:129`). And the route files that *should* mirror the page carry the stray `bg-[#f6f8fc]` / `rounded-3xl` dialect: `app/study-resources/loading.tsx:5-8`, `app/study-resources/error.tsx:7,15`.

### 0.3 Decision

> **The `gray-*` / `rounded-md` / `brand-blue` dialect wins as the structural system for everything the coin system adds.** The `rose` / `rounded-3xl` / `slate-950` language is confined to the video player stage and video category signalling, where it is already load-bearing.

**Justification.** The coin system crosses every surface — document, video and mock-test cards, the dashboard, the site header, the admin queue. A shared object needs a shared language, and the gray/blue dialect is the one already shared by all three catalogs, the dashboard (`DashboardLayout.tsx:57,128`) and the admin table (`StudyResourcesSection.tsx:651,684`). Choosing it means the coin chrome inherits house elevation for free.

**The player keeps its dark stage — an explicit, documented exception.** Flattening `VideoLecturePlayer.tsx:129` to gray would destroy the best screen in the feature and gain nothing. Consequence: the coin affordance *inside* the stage must meet contrast on `slate-950`, which §11 verifies.

### 0.4 This supersedes the earlier "rose = unlock/earn accent" guidance

The prior recommendation reserved `rose` as the unlock/earn accent so it would read distinctly from the blue Download button. **I am changing that.** Rose is already a *category and state* signal, not a free channel — video category (`VideoLecturesPage.tsx:272,281`), locked (`VideoLecturePlayer.tsx:158,170`), mock-test submit error (`MockTestRunner.tsx:381`). If rose also meant "spend coins", a locked document, a locked video and a coin unlock would all render rose and the signal collapses.

Corrected semantic map — **every value below already exists in the repo, so no new hue is introduced**:

| Meaning | Token | Existing precedent |
|---|---|---|
| Primary user action (unlock, search, continue) | `brand-blue` | `StudyResourcesPage.tsx:256,477,534` |
| Coin amount, neutral fact | `gray-100` / `gray-700` | `:419,427` |
| Expiring, or awaiting your action in bounded time | `amber-*` | `StudyResourcesSection.tsx:727`; `PendingInstitutionsSection.tsx:232` |
| Coins settled / approved / completed | `emerald-*` | `StudyResourcesSection.tsx:728` |
| Locked, or video category | `rose-*` | `VideoLecturePlayer.tsx:158` |
| Destructive, rejection, failed fraud check | `red-*` | `StudyResourcesPage.tsx:349`; `PendingInstitutionsSection.tsx:293` |

**Blue means "press this". Amber means "the clock is running". Emerald means "this landed". Rose means "you can't have it yet".** The unlock button is *solid* `brand-blue`; the post-unlock download stays the existing *tinted* `bg-blue-50 text-brand-blue` (`:477`). This is a deliberate departure from "make the unlock button look like the download button" — the solid/tinted split signals that this one costs something, and the cost is stated before the press, in the dialog.

### 0.5 Three implementation notes that will otherwise waste effort

- **`animate-in` / `fade-in` / `zoom-in-95` are dead classes here.** They come from `tw-animate-css`, which is **not installed** (`package.json`, `app/globals.css:1-27`), yet 44 usages exist including `app/institution-zone/pricing/page.tsx:502,504`. They silently do nothing. **Do not build coin motion on them.** Working motion: core `animate-pulse` / `animate-spin`, plus house classes `.animate-shine` (`globals.css:53`) and `.shimmer-effect` (`:145`).
- **Dialogs in this feature get no entrance animation.** Neither existing dialog animates (`:503-541`; `PendingInstitutionsSection.tsx:311-335`). Matching the house beats a half-working animation.
- **One addition to `globals.css`**, documented in §2.5 — a `coin-pop` keyframe written in the style of `.animate-shine` (`globals.css:48-53`). Nothing else in the token file changes.

---

## §1 — Design principles

1. **Earning should feel like progress; spending should feel like a decision. Never like a toll.** A student leaves an earn moment with *"I did that, and it counted"* and a spend moment with *"I chose this over something else."* The difference is copy and button weight, not the amount.
2. **Never let the cost be a surprise, never let the state be a mystery.** A price that appears only at payment is a bait-and-switch. Every gated surface states its cost before the press; every locked card states why it is locked and what would change that.
3. **A deficit is information, not an accusation.** The insufficient screen is a calculator, not a scolding: the gap, then three concrete routes with real coin values. No red, no urgency language, and never a dead end.
4. **Expiry is a fact with a date, not a threat with a countdown.** `expires_at` per lot is shown in the UI, per the research. No live-ticking timers, no countdowns in modals, no "you will lose" framing. A student who finds out on day 29 is mildly annoyed; a student manipulated by a fake clock stops trusting the product.
5. **Scarcity is stated once, in one place, honestly.** The 30-day allowance is a real constraint, shown in the wallet and on affected cards — never amplified into a campaign. No interstitial, no first-login modal, no push at odd hours, no badge on every nav item.
6. **The catalogue stays fully browsable while gated.** Locked cards remain complete: title, description, course, year, size. Gating the content as well as the action would make the spend uninformed.
7. **A pending hold is never described as a loss.** Referral coins in the 7-day fraud hold are *held until a date*, with the reason stated. The student is not scolded for inviting someone.
8. **One number, always truthful, never the largest thing on screen.** The header balance is a small `text-xs` chip that looks like navigation, because it is navigation. It is not a slot-machine display.

---

## §2 — The coin wallet / balance surface

### 2.1 Where it lives — four placements, each with one job

| # | Placement | Job | Behaviour |
|---|---|---|---|
| A | **Header chip** in the right cluster of `EducationNavbar` (public) and `DashboardHeader` (`DashboardLayout.tsx:77-92`) | The always-visible number | A `Link`, **not a popover.** Click → `/user/dashboard/coins` |
| B | **Wallet section**, replacing the dead `ResourcesSection` | Full ledger + all earn routes | One page, three stacked sections |
| C | **Starter-allowance line** on gated cards | Local scarcity | One `text-[11px]` line, only when an entitlement exists |
| D | **`coin-pop` moment** on the chip | Confirmation that coins landed | 1.2s, once |

**No popover on the chip — a decision, not an omission.** A balance popover full of counters is the most manipulative shape this feature could take, and it also breaks for signed-out visitors. A chip that is a link is honest, keyboard-obvious, and works for anonymous users (`Earn coins → /login?next=…`).

### 2.2 Placement A — the header chip

Signed out, and the expiring variant (the only decorated state; the plain variant is the same markup with `border-gray-200 bg-white` and a `text-gray-700` count):

```tsx
// Signed out: href="/login", text "Earn coins", plain border-gray-200/bg-white.
<Link href="/user/dashboard/coins?focus=expiring" className="inline-flex items-center gap-1.5 rounded-md border border-amber-200 bg-amber-50 px-2.5 py-1.5 text-xs text-amber-700 transition-colors hover:bg-amber-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-500 focus-visible:ring-offset-2">
  <Coins size={14} aria-hidden="true" />
  <span className="font-bold tabular-nums text-gray-900">{fmt(128)}</span>
  <span className="h-1 w-1 rounded-full bg-amber-600" />
  <span className="font-semibold">20 expiring</span>
</Link>
```

Rules: `tabular-nums` so the number does not jitter; `new Intl.NumberFormat("en-IN")` for grouping (lakh/crore is what this audience reads, and values stay in four digits). **Never `₹`/`NPR`, never an FX rate, ever, in the chip.** The accessible name comes from `aria-label` on the container (`"Coin balance: 128. Open your coins."`); inner spans are `aria-hidden` so it is not read twice.

### 2.3 Placement B — the wallet page

Replaces `components/user/dashboard/sections/ResourcesSection.tsx` — 43 lines, 100% hardcoded mock data at `:4-11` (six fake resources, fake ratings, "Dr. Academic"/"Prof. Johnson"), on a route at `app/user/dashboard/resources/page.tsx` that **no sidebar item links to** (`Sidebar.tsx:52-124`). A dead screen; killing it costs nothing.

**Route decision — a stated deviation from "repurpose in place."** The brief suggested repurposing the existing route. Instead: move it to `app/user/dashboard/coins/`, and leave `app/user/dashboard/resources/page.tsx` as a three-line server `redirect("/user/dashboard/coins")`. The sidebar label reads "Coins", and a URL reading `/resources` that serves a wallet is a permanent, indexable lie. Four lines of redirect is cheap; a wrong URL is not. The `loading.tsx` / `error.tsx` rule is satisfied either way — `app/user/dashboard/loading.tsx` and `error.tsx` already cover nested routes — but each new route still gets its own pair for skeleton fidelity.

**One page, three stacked sections, no tabs.** Tabs hide the starter allowance below the fold, defeating the entire activation purpose. Order: (1) `StarterAllowanceCard`, (2) balance ledger `lg:col-span-2` beside the `EarnRoutes` rail `lg:col-span-1`, (3) `ReferralCard`, (4) `UploadEarningsCard`. The earn rail is **last in DOM order** so mobile stacks it below the balance.

**The ledger is the research finding, made visible.** One row per lot, sorted soonest-expiring first — the order the backend spends in, so the ledger *teaches* FEFO by being sorted that way.

```tsx
<li className="flex items-center justify-between gap-3 border-b border-gray-100 py-2.5 last:border-b-0">
  <span className="flex min-w-0 items-center gap-2 text-sm">
    <span className="font-bold tabular-nums text-gray-900">{fmt(lot.amount)}</span>
    <Coins size={13} className="text-gray-400" aria-hidden="true" /><span className="sr-only">coins</span>
    <span className="truncate text-gray-500">from {lot.reason}</span>
  </span>
  <span className="shrink-0 text-xs">
    {lot.expires_at ? <ExpiryStamp days={daysUntil(lot.expires_at)} /> : <span className="text-gray-500">Never expires</span>}
  </span>
</li>
```

`ExpiryStamp` by days remaining: `> 30` → `text-gray-500` "12 November 2026" · `8–30` → `text-gray-500` "12 November 2026 · 18 days left" · `≤ 7` → `text-amber-700 font-semibold` + `Hourglass size={12}` + "expires in 5 days". One footnote under the table, not a policy link: *"We spend the coins that expire soonest first."*

**Expired lots are not rendered.** No struck-through rows, no grey tombstones — a ledger of what you lost is a scold, and the number is already gone from the total.

### 2.4 Placement C — starter allowance, without the dark pattern

The "3 starter tokens, expiring in N days" affordance — the founder's "3 free tokens" — is the highest-risk surface in the feature. Six constraints, all load-bearing:

1. **A date and a count of what remains — never a live clock.** "Use them by 12 November", "2 documents left". No `02:14:33`, no per-second re-render.
2. **It appears only on the student's own wallet page and on cards they are already looking at.** No interstitial, no first-login modal, no navbar badge, no badge on every card.
3. **The catalogue is never gated behind it.** A locked card shows full metadata (§1.6).
4. **Reminders are 30 / 7 / 1 days, and opt-out.** One toggle in the wallet: *"Email me before coins expire."* Opting out stops the reminder; the expiry stays visible on the page. Unsolicited expiry email is spam; the page is the record.
5. **No blame grammar.** "Your starter unlocks expire on 12 November" — never "You will LOSE 3 unlocks!", never a red count-down.
6. **The free allowance is never rendered as coins.** It is per-class entitlements, not a balance, so it must not appear as "130 coins" anywhere — not in the wallet, not in a tooltip, not as "equivalent to".

### 2.5 Placement D — the `coin-pop` moment

When coins land (profile step, referral release, upload approval) the header number plays a 1.2s `coin-pop`. This is the only celebratory motion in the feature. Paired with a sonner toast (§3.5). No confetti, no particles, no `animate-bounce` — a coin counter that bounces like a mobile game reads as a game, and this is a study platform.

```css
/* app/globals.css — the one addition, in the house style of .animate-shine at :48-53 */
@keyframes coin-pop {
  0%   { transform: translateY(0) scale(1); }
  35%  { transform: translateY(-3px) scale(1.12); }
  100% { transform: translateY(0) scale(1); }
}
.animate-coin-pop { animation: coin-pop 1.2s cubic-bezier(0.22, 1, 0.36, 1) 1; }
```

---

## §3 — Resource card state matrix

### 3.1 First, a refactor this section depends on

**No `ResourceCard` component exists.** Card JSX is duplicated inline at `StudyResourcesPage.tsx:400-484` and `VideoLecturesPage.tsx:262-320`, with a third variant at `MockTestListPage.tsx:44-99`. **Extract `components/studyResources/ResourceCard.tsx` and render it from all three.** The matrix is seven states; implementing it against three copies is how it drifts.

Props: `resource`, `variant: "document" | "video" | "mock-test"` (drives radius + category accent), `access: ResourceAccess` (from `useCoinState()`), `onPrimaryAction`, `onNeedEarn`, `busy`.

```tsx
// Admin-draft state: StudyResource.is_published already documents that drafts stay
// hidden (services/studyResourcesApi.ts:31-32), so this branch is defensive — it
// covers the admin's optimistic-insert path and any stale cached page.
if (resource.is_published === false && !isAdminView) return null;
```

In the **admin** table the same state is the amber `Awaiting review` badge — see §8.

### 3.2 The seven states

| State | Price badge | Primary button | Card treatment |
|---|---|---|---|
| `starter-eligible` | emerald "Starter · 2 of 3 left" | **solid** brand-blue "Use starter unlock" | normal |
| `unlocked` | gray "Unlocked" + `Check` | `bg-gray-100` "Download" + `Check` | normal |
| `affordable` | gray "40 coins" | **solid** brand-blue "Unlock" | normal |
| `insufficient` | gray "40 coins" (still shown) | **outline** "Earn 22 more" | normal, `opacity-100` |
| `anonymous` | gray "40 coins" | **outline** "Log in to unlock" | normal |
| `unlocking` | gray "40 coins" | solid brand-blue + spinner, `disabled` | `aria-busy="true"` |
| draft | — | — | renders `null` outside admin |

Three rules the table encodes:

- **The price badge is always neutral gray.** It states a fact and does not judge affordability; the **button** carries the affordance. A green badge on affordable items is a nudge, a red one on unaffordable items is a scold. Neither belongs on a fact.
- **The insufficient button is never `disabled`.** A greyed-out button with no explanation is the failure mode of every gated product. It is a live outline button labelled with the gap, because the gap *is* the action.
- **The insufficient state is not visually diminished.** No `opacity-60`, no desaturation. The student can afford a different resource; this card is simply not it.

### 3.3 `CoinBadge` — the markup

One badge slot, highest-priority tone wins: `unlocked` → `starter` → `price`. A card showing "Unlocked" *and* "40 coins" is noise, and one showing "Starter · 2 left" *and* "40 coins" invites the wrong question (is the token worth more than the coins? — it is, and the UI should let the student spend the scarcer thing, not compare prices).

```tsx
// BASE   = "inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-[11px] font-bold"
const TONE = {
  price:     "bg-gray-100 text-gray-700",
  starter:   "bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200",
  expiring:  "bg-amber-50 text-amber-700 ring-1 ring-amber-200",
  unlocked:  "bg-gray-100 text-gray-600",
} as const;
```

Dropped into the card header row, beside the existing type pill (`StudyResourcesPage.tsx:419`). `Coins` is always `aria-hidden`; the word "coins" is always in the visible text, because the glyph alone is not an accessible name and colour alone is not a state.

```tsx
<CoinBadge tone={badge.tone}>
  {access.state === "unlocked"   ? <><Check size={12} aria-hidden="true" />Unlocked</>
   : access.state === "starter-eligible"
                              ? <><Gift size={12} aria-hidden="true" />Starter · {access.starterLeft} of {access.starterTotal} left</>
   : <><Coins size={12} aria-hidden="true" />{fmt(access.price)} coins</>}
</CoinBadge>
```

When a lot expires within 7 days, the **wallet** surfaces it (§2.2) — never the card. A price badge must not carry a countdown.

### 3.4 The action row

The footer row at `StudyResourcesPage.tsx:449-482` keeps its `flex items-center justify-between` shape; only the right-hand element changes. **The button label carries the verb only** — the price lives in the badge above — so the button never wraps below `sm`.

```tsx
const BTN = "inline-flex items-center gap-1.5 rounded-md px-3 py-2 text-xs font-bold transition-colors " +
           "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-blue focus-visible:ring-offset-2";
const solid   = `${BTN} bg-brand-blue text-white hover:bg-brand-hover disabled:cursor-not-allowed disabled:opacity-60`;
const outline = `${BTN} border border-gray-200 bg-white text-gray-700 hover:bg-gray-50`;
const inert   = `${BTN} bg-gray-100 text-gray-900 cursor-default`;
// each arm: solid|outline|inert + <Icon size={13} aria-hidden="true" /> + label
//   Gift  "Use starter unlock"  → solid   (onPrimaryAction)
//   Check "Download"            → inert   (onPrimaryAction)
//   Unlock "Unlock"             → solid   (onPrimaryAction)
//   Coins "Earn {gap} more"     → outline (onNeedEarn)
//   LogIn "Log in to unlock"    → outline (onPrimaryAction)
```

The accessible name is the button's own text — no `aria-label`, no `title`. The left-hand slot keeps the existing count (`:450-461`) untouched; since `Download` is now also a count icon, only the **unlocked** state keeps it, where it is true.

**Using a zero-cost starter token still confirms.** Scarce resources deserve a confirmation even at zero cost, or a mis-tap on a phone burns one of three. It is the positive variant of §4: one primary button, no cost table.

### 3.5 Toast: yes, introduce sonner here

`Toaster` is already mounted globally at `app/providers.tsx:15` (`position="bottom-left"`, `closeButton={false}`), and 25+ components already import `toast` from `sonner` (e.g. `components/ScholarshipProvider/ApplicationsDirectory.tsx:7`). The `studyResources` tree is the outlier, using inline red banners (`:349`) and inline error text (`MockTestRunner.tsx:381`).

**Decision: adopt sonner for coin *events* only; do not migrate existing banners in this change.** A coin event is transient, non-blocking, and needs no layout space — which is what a toast is for. A load failure is a state the page must render, which is what the existing banners are for. Migrating them is a separate cleanup with its own regression surface.

```tsx
toast.success("Unlocked. 40 coins spent, 88 left.");                                                   // grant
toast("60 coins held until 22 November.", { description: "We confirm the invitee's first action first." });  // neutral
toast.error("That unlock was not completed. Your balance has not changed.");                            // real failure only
toast.success("+80 coins added for \"Organic Chemistry Notes\".");                                       // upload approved
```

Bottom-left lands away from the card action in the bottom-right, so the student does not lose their place. The `MockTestRunner` score view must **not** use a toast — a score is the result the student came for and is rendered inline.

### 3.6 Per-surface notes

- **Video list card** (`VideoLecturesPage.tsx:262-320`) — keep `rounded-md` and the `rose` selection ring (`:272`) untouched; the badge slots into the header row at `:286`. The `Sign in to play` label at `:312` is **replaced** by the state badge, and its `text-slate-400` is **corrected** (§11, 2.56:1).
- **Mock-test card** (`MockTestListPage.tsx:90-95`) — the bare `Link` to the runner becomes `<ResourceCard variant="mock-test">`. The `cyan` accent (`:46,:91`) is **left alone**: `cyan` is the mock-test category signal, and the coin badge sits above it in the header row.
- **Mock test runner** (`MockTestRunner.tsx:387-411`) — the locked submit block gains the attempt entitlement. The copy at `:397-400` becomes false once attempts are metered and must be rewritten (§10.3). A chip appears in the runner header: `1 attempt used of 1` / `Uses 1 of your 1 mock test unlock`.

---

## §4 — Unlock confirmation

Reuses the existing modal shell **verbatim** — `fixed inset-0 z-50 flex items-center justify-center bg-gray-900/50 p-5` wrapping `w-full max-w-sm rounded-lg bg-white p-6` (`StudyResourcesPage.tsx:505,513`) — extracted into `components/ui/Modal.tsx` so all dialogs share one focus implementation (§11.1). The dialog states three things and nothing else: **cost**, **resulting balance**, **which lots are consumed**.

```tsx
<Modal open onClose title={resource.title} labelledBy={titleId} describedBy={bodyId}>
  <h2 id={titleId} className="flex items-center gap-2 text-base font-semibold text-gray-900">{resource.title}</h2>
  <dl id={bodyId} className="mt-4 space-y-2.5 text-sm">
    {/* each row: flex items-baseline justify-between gap-3 — dt is text-gray-500, dd is font-bold tabular-nums text-gray-900 */}
    <div className="flex items-baseline justify-between gap-3"><dt className="text-gray-500">Cost</dt>
      <dd className="font-bold tabular-nums text-gray-900"><Coins size={13} className="mr-1 inline align-[-2px] text-gray-400" aria-hidden="true" />{fmt(preview.cost)} coins</dd></div>
    <div className="flex items-baseline justify-between gap-3 border-t border-gray-100 pt-2.5"><dt className="text-gray-500">Your balance</dt>
      <dd className="font-bold tabular-nums text-gray-900"><span className="text-gray-400">{fmt(preview.balanceBefore)}</span>
        <ArrowRight size={13} className="mx-1.5 inline align-[-2px] text-gray-400" aria-hidden="true" />{fmt(preview.balanceAfter)}</dd></div>
  </dl>
  {/* FEFO in plain words — the only place the mechanic is taught */}
  <div className="mt-4 rounded-md bg-gray-50 p-3">
    <p className="text-xs font-semibold text-gray-700">Coins used</p>
    {preview.allocation.map((leg) => (
      <p key={leg.lotId} className="mt-1.5 flex items-baseline justify-between gap-3 text-xs text-gray-500">
        <span className="truncate">{leg.reason} · {leg.expiresAt ? `expires ${formatDate(leg.expiresAt)}` : "never expires"}</span>
        <span className="shrink-0 font-bold tabular-nums text-gray-700">{fmt(leg.amount)}</span>
      </p>
    ))}
    <p className="mt-2 text-[11px] leading-4 text-gray-400">We spend the coins that expire soonest first, so nothing is wasted.</p>
  </div>
  <div className="mt-6 flex gap-2.5">
    <button onClick={onClose} className="flex-1 rounded-md bg-gray-100 px-4 py-2.5 text-sm font-semibold text-gray-900 hover:bg-gray-200">Not now</button>
    <button onClick={confirm} disabled={busy} className="flex-1 rounded-md bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-hover disabled:opacity-60">
      {busy ? "Unlocking…" : `Unlock for ${fmt(preview.cost)} coins`}
    </button>
  </div>
</Modal>
```

- **The allocation must come from the server.** `POST /api/v1/coins/spend-preview` returns `{ cost, balanceBefore, balanceAfter, allocation[] }`. The client never computes FEFO — it does not know the lots. **If the preview fails the confirm button is not rendered**: show the error, `Try again`, `Not now`. A live confirm without a balance check is how students get charged twice.
- **On a 409 (balance changed since preview)** the dialog stays open, the numbers refresh, the allocation re-renders. Silent re-pricing is a bait-and-switch.
- **`already-unlocked` guard.** A 409 `already_unlocked` flips the card to `unlocked` and fires `toast.info("Already unlocked — nothing was spent.")`. Never charged twice, never told they were about to be.
- **The button label repeats the cost** — "Unlock for 40 coins", not "Unlock". The number is the point of the press.

### 4.1 Starter-token variant

Same shell, no `dl`, no allocation. Title "Use a starter unlock?" · body "This is 1 of your 3 document unlocks. 2 will be left after this." · primary `bg-brand-blue` "Use starter unlock" · secondary "Keep browsing". Because the resource costs nothing there is no `balanceAfter` row — showing "128 → 128 coins" would be theatre.

---

## §5 — Insufficient funds — the highest-stakes screen

This screen decides whether students think the system is fair. Four rules:

1. **No red, no warning icon, no urgency copy.** Rose and red are already locked to *destructive* and *video* (§0.4); reusing them here would make a normal moment look like a fine. The framing colour is **amber** — the house's "needs your attention in a bounded time" tone (`StudyResourcesSection.tsx:727`, `PendingInstitutionsSection.tsx:232`).
2. **Lead with arithmetic, not with the deficit.** Cost, then balance, then gap. Not "You can't afford this".
3. **Every number is real** — quoted from the same constants the earn panel uses, so the two can never disagree.
4. **Never leave a dead end.** The third action is a way *forward into the catalogue*.

Shell: the same `Modal`, one step wider — `max-w-md` — plus `max-h-[85vh] overflow-y-auto` for short viewports.

```tsx
<Modal open onClose title="You need 22 more coins" labelledBy={titleId} describedBy={bodyId} size="md">
  <h2 id={titleId} className="flex items-start gap-2 text-lg font-semibold text-gray-900">
    <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-amber-50 text-amber-700 ring-1 ring-amber-200"><Coins size={16} aria-hidden="true" /></span>
    You need {fmt(gap)} more coins
  </h2>
  <p id={bodyId} className="mt-2.5 text-sm leading-6 text-gray-500">This {noun} costs {fmt(price)} coins. Your balance is {fmt(balance)}.</p>

  {/* Ranked: the fastest route that can actually close the gap, first. */}
  <ul className="mt-4 divide-y divide-gray-100 rounded-md border border-gray-200">
    {routes.map((r) => (
      <li key={r.id} className="flex items-center gap-3 p-3">
        <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-gray-100 text-gray-700"><r.Icon size={16} aria-hidden="true" /></span>
        <div className="min-w-0 flex-1">
          <p className="flex items-baseline gap-2 text-sm font-semibold text-gray-900">{r.title}
            <span className="shrink-0 text-xs font-bold tabular-nums text-gray-500">{r.remaining !== null ? `+${fmt(r.remaining)} coins left` : `+${fmt(r.value)} coins`}</span></p>
          {r.partial > 0 && <EarnMeter done={r.partial} total={r.parts} />}
          <p className="mt-0.5 text-xs text-gray-500">{r.detail}</p>
        </div>
        <Link href={r.href} className="shrink-0 rounded-md border border-gray-200 bg-white px-3 py-1.5 text-xs font-bold text-gray-700 transition-colors hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-blue focus-visible:ring-offset-2">{r.cta}</Link>
      </li>
    ))}
  </ul>

  {/* The anti-dead-end. Always present. */}
  <Link href="/study-resources?affordable=1" className="mt-3 flex items-center gap-2 rounded-md bg-gray-50 px-3 py-2.5 text-sm font-semibold text-gray-700 transition-colors hover:bg-gray-100">
    <Filter size={14} aria-hidden="true" />Browse resources you can unlock now<ChevronRight size={14} className="ml-auto text-gray-400" aria-hidden="true" />
  </Link>
</Modal>
```

| Rank | Route | Value | Condition | Detail line |
|---|---|---|---|---|
| 1 | Complete your profile | 15 remaining of 25 | 3 of 5 steps done | `EarnMeter` 3/5 |
| 2 | Invite a friend | 60 | cap not reached | "60 coins each, up to 10 a month. Released after they complete their first action." |
| 3 | Upload a study resource | 80 | uploads allowed | "Once an admin approves and publishes it." |

Ranking is computed, not hardcoded: sort by `remaining` descending when `remaining >= gap`, else by `value` descending. A profile with 5 coins left is a better lead for a 22-coin gap than a 60-coin referral that takes a week.

**Compliance guard on this screen specifically.** The most tempting line here is "that's NPR 220 of study material". **Banned** (preamble). The gap is expressed only in coins — the only unit the student can act in.

**Forbidden copy on this screen**, recorded so it does not drift in review: "You can't afford this" · "Unlock more with…" (implies purchase) · "Only Rs. 22 away!" · "Don't miss out!" · any countdown · any currency figure · the word "free".

---

## §6 — Referral surface

**Code and link.** Six characters, upper-case alphanumeric, no `0/O` or `1/I` ambiguity. Shown in `rounded-md border border-gray-200 bg-gray-50 px-3 py-2 font-mono text-sm font-bold tracking-[0.18em] text-gray-900`, beside a copy button `bg-brand-blue … text-white hover:bg-brand-hover` with `Copy`/`Check` and a 1.6s `Copied` swap, firing `toast.success("Referral code copied.")`. The link is `https://<host>/register?ref=<CODE>`.

**Share targets, in order:** the Web Share API when `navigator.share` exists (the path most students will actually take on a phone), then **WhatsApp** (`wa.me/?text=`), **Viber** (`viber://chat?text=`), **Facebook** (`m.me/`), **Email**, then Copy link. WhatsApp and Viber are the channels this audience uses; Facebook is included because referral links are routinely shared there in Nepal. Buttons use the `outline` variant from §3.4, 32px, with `aria-label` + `title`.

**Per-referral states** — each is `icon + text + tone`, never colour alone:

| State | Badge classes | Label |
|---|---|---|
| `pending-hold` | `bg-amber-50 text-amber-700 ring-1 ring-amber-200` | `Hourglass` · "60 coins held until 22 November" |
| `qualified` | `bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200` | `Check` · "60 coins added" |
| `failed` | `bg-red-50 text-red-700 ring-1 ring-red-200` | `X` · "Not confirmed" |
| `capped` | `bg-gray-100 text-gray-600` | `Lock` · "Cap reached" |

The **pending-hold row needs the most care**:
```
● Aarav Sharma   [Hourglass] 60 coins held until 22 November
  We confirm their first completed action before releasing. If they don't
  complete one, these 60 coins are not released.
```
The second sentence is the honesty load-bearer. It tells the student exactly what the hold can cost them — the opposite of what a hold is usually used to obscure — and it makes the invite legitimate rather than scammy.

**Capped.** `7 of 10 used this month · resets 1 December` with a `Meter` at 70% — informational, not a progress bar toward a reward, because there is no reward framing; there is no reward. The **copy button is not disabled** when capped: the student may still share, a referral next month is still a referral, and hiding the code teaches them to screenshot it elsewhere.

**The invitee's 25 coins are never mentioned on the referrer's surface.** It is the invitee's award, shown in the invitee's wallet; mentioning it here would imply the referrer is recruiting for their own benefit with someone else's reward attached.

---

## §7 — Earn progress

### 7.1 Profile ladder — 5 instalments of 5

`ProfileSection.tsx:587-598` already has four tabs (Personal Details / Education History / Budget & Funding / Documents) and `app/onboarding/page.tsx:97-157` already collects level, exam preference, city and course. The five steps map onto fields that genuinely exist:

| # | Step | Coins | Deep link |
|---|---|---|---|
| 1 | Add your full name | 5 | `/user/dashboard/profile#personal` |
| 2 | Set your current level | 5 | `/user/dashboard/profile#personal` |
| 3 | Choose your courses | 5 | `/user/dashboard/profile#education` |
| 4 | Add your city | 5 | `/user/dashboard/profile#personal` |
| 5 | Add a profile photo | 5 | `/user/dashboard/profile` |

```tsx
<li className="flex items-center gap-3 py-2.5">
  <span className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-full ${done ? "bg-emerald-100 text-emerald-700" : "border border-gray-300 text-gray-400"}`}>
    {done ? <Check size={13} aria-hidden="true" /> : <span className="text-[10px] font-bold">{i + 1}</span>}
  </span>
  <span className="flex-1 text-sm text-gray-700">{step.label}</span>
  <span className="shrink-0 text-xs font-bold tabular-nums text-gray-500">+{fmt(step.coins)}</span>
  {!done && <Link href={step.href} className="shrink-0 text-xs font-bold text-brand-blue hover:text-brand-hover hover:underline">Do this</Link>}
</li>
```

Header: `15 coins left to earn` with a `Meter` at 40%. **The remaining total is the headline, not the checklist.** A ladder that says "complete your profile" without saying *which part* is a dead end; a ladder that says "15 coins left" with five deep-linked rows is a task list.

**Payout is per-step, immediate and quiet.** Each completed step fires `toast.success("+5 coins. 20 to go.")` and the header number plays `coin-pop`. Never stack toasts — if two steps complete in one session, show only the last, with the summed value. Installing five toasts is how a reward becomes an interruption.

### 7.2 Upload earnings

| State | Tone | Content |
|---|---|---|
| `pending-review` | amber `Hourglass` | `"Organic Chemistry Notes" — under review. Submitted 12 November. We usually respond within 2 working days.` No action |
| `approved` | emerald `Check` | `"Physics 2075 Past Paper" — +80 coins added on 2 November.` + `View` link |
| `rejected` | red `X` | `"Thermodynamics Notes" was not approved.` + the admin's reason, **verbatim**, in a `border-l-2 border-red-200 pl-3` quote + `Upload another` |

**The rejection reason must be shown to the uploader, not just to admins.** `RejectedInstitutionsSection.tsx:131` proves the data model already carries `rejection_reason`; today only admins read it. A student whose upload was refused with no stated reason earns nothing and learns nothing.

**"2 working days" is a commitment, so it must be one the moderation queue can meet.** If it cannot, change the copy before launch, not after.

---

## §8 — Admin approval queue

**Today there is no approval queue.** `StudyResourcesSection.tsx:726-733` renders exactly two states from a boolean `is_published`, and `upload` implies `publish` for documents (`services/studyResourcesApi.ts:130` — "omitted for documents (implicitly published)"). The coin system requires a third state and a real queue.

**Registration** — three edits in the existing patterns: `DashboardShell.tsx` ~:146 `const PendingResourcesSection = lazy(() => import("./PendingResourcesSection"));` · ~:412-419 add `{ section: "pending-resources", label: "Pending Approval" }` to the existing `Study Resources` group (`:420-427`) · ~:684 `case "pending-resources":` alongside `case "study-resources":` (`:682`).

**Component** — `components/superadmin/client/PendingResourcesSection.tsx`, cloned structurally from `PendingInstitutionsSection.tsx`: the same `superadminFetch` helper with `superadmin_token` (`:29-45`), the same `loading` / `actionLoading` / `authError` triple (`:68-69`), the same `auth_required` block (`:164-174`), the same `RefreshCw` header (`:187-191`), the same confirm-dialog pattern (`:311-335`), the same `Loader2`-in-button busy state (`:300`).

**Three deliberate changes to the house pattern, each stated:**

1. **The approve dialog names the coin consequence.** `PendingInstitutionsSection.tsx:114` already does exactly this — *"An email with login credentials will be sent."* The coin analogue: *"Approve and publish "…"? The uploader gets 80 coins and it appears in the public catalogue."* Same move, same place.
2. **Reject-with-reason is new.** `RejectedInstitutionsSection` *displays* `rejection_reason` (`:131`) but never captures one — its `handleReject` takes no reason. The coin queue adds a reject dialog with a `<textarea>` on the shared modal shell, `required`. An extension of the house pattern, and mandatory because §7.2 shows the reason to the student.
3. **The award is visible on the button, before the click.** `Approve · +80 coins` with a `Coins` glyph, `bg-emerald-600 hover:bg-emerald-700`, `Check size={16}`. Reject stays the house `bg-red-500 hover:bg-red-600` with `X` (`:293,299`). Two solid buttons, high contrast against each other, weight on approve — matching `:290-303`.

**List card** — `rounded-xl border border-gray-200 bg-white p-6 transition-all hover:shadow-md` (`:222`) with the `bg-blue-100 text-blue-600` `h-10 w-10` icon tile (`:226-228`), an `bg-amber-50 border-amber-200 text-amber-700` `Awaiting review` pill (`:232`), then the `grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3` meta grid (`:238`) carrying title, file name, type, course, year, size, **uploader name and email**, and submitted-at. The uploader is not in the house institution card because institutions are self-submitted; here it is the person being paid 80 coins, so the admin must be able to identify them without opening a file.

**After the action** — `toast.success("Approved. 80 coins added to Sujan Thapa.")`, matching the other admin tables. The row leaves the list optimistically, exactly as `PendingInstitutionsSection.tsx:139` does.

**The badge change in the existing table.** `StudyResourcesSection.tsx:724-734` goes from two states to three — `awaiting: "bg-amber-100 text-amber-700"` · `published: "bg-emerald-100 text-emerald-700"` · `rejected: "bg-red-100 text-red-700"` — and the upload form gains one line under the publish control: *"Publishing a submitted resource adds 80 coins to the uploader's balance."* The admin spends the student's coins; they are entitled to know that before they click.

---

## §9 — Empty, loading, error, responsive

| Surface | Loading | Empty | Error |
|---|---|---|---|
| Header chip | `h-7 w-16 animate-pulse rounded-md bg-gray-100` | signed out → `Earn coins` | **hidden.** A failed balance fetch must not show `0`, which would be a lie; fall back to the link with no number |
| Wallet lots | 4 × `h-11 animate-pulse rounded-md bg-gray-100` | "No coins yet" + the three earn routes inline | `rounded-md border border-red-200 bg-red-50 p-4` + `Try again` |
| Wallet, zero balance, no allowance | — | "You have no coins yet" + `Complete your profile` / `Invite a friend` / `Upload a resource` | — |
| Starter allowance, all spent | — | "Your starter unlocks are used up." + "You can still unlock anything with coins." | — |
| Starter allowance, expired | — | "Your starter unlocks expired on 12 November." **No apology, no red.** | — |
| Referral, none yet | — | "No invites yet." + share row still fully interactive | — |
| Referral, capped | — | cap meter + "Resets 1 December" | — |
| Uploads, none | — | "You have not uploaded anything." + `Upload a resource` | — |
| Profile ladder, complete | — | "Profile complete. +25 coins earned." emerald, with the date | — |
| Resource grid | reuse the existing skeleton at `StudyResourcesPage.tsx:355-380` — **`data-testid="resource-skeleton"` preserved** | reuse `:382-392` `FolderOpen` empty | reuse `:349-351` red banner + `Try again` |
| Card, price fetch fails | — | — | badge degrades to `bg-gray-100 text-gray-500` "Unlock to see price"; the grid never breaks |
| Unlock dialog, preview fails | spinner in the dialog body | — | "We could not check your balance. Nothing has been spent." + `Try again` / `Not now`. **Confirm is not rendered.** |
| Insufficient dialog | — | — | earn routes render with the `Get` link only; the gap line still shows |
| Admin queue, loading | `min-h-[400px]` centred `Loader2 h-8 w-8 animate-spin text-blue-600`, copied from `:177` | "No uploads waiting for review." + "New uploads land here first, before they appear in the catalogue." | `console.error` + rows retained, matching `:143` |
| Admin queue, 401 | the `auth_required` block at `:164-174`, verbatim | — | — |

**Responsive.** The grid stays `grid-cols-1 gap-3.5 sm:grid-cols-2 xl:grid-cols-3` (`:395`, `:261`); the badge sits in the existing header row so no column width changes. Button labels carry verbs only, never prices — that is the mobile decision, since a price in the label wraps below `sm` and doubles the footer height; the price is always in the badge above, which never wraps. The insufficient dialog is the only one wider than the house (`max-w-md` + `max-h-[85vh] overflow-y-auto`); at 360px its three earn rows stack the `Link` under the text (`flex-col items-start sm:flex-row sm:items-center`). The wallet page is `grid-cols-1 lg:grid-cols-3` with the earn rail last in DOM order. The header chip drops its `20 expiring` text below `sm` and keeps only the amber dot, with full detail on the wallet page — the chip never wraps the header. The player stage (`VideoLecturePlayer.tsx:129`) gets a fourth `StagePanel` tone for `coins-required`, reusing the existing `bg-slate-950` locked treatment at `:74` and the `rose-500` CTA at `:170`, so a video unlock reads as video.

---

## §10 — Copy deck

**House style:** plain, short, second person, present tense. British-leaning spelling — *recognise, personalise, colour, favourite, practise, organised, cancelled, enrol, fulfil, licence*. No exclamation marks in system copy. No second-person excitement. Currency appears in this product (`app/institution-zone/pricing/page.tsx:51`, "NPR 5,000") but **never** next to StudsTokens.

**Banned outright:** `free` · `no cost` · `worth NPR/₹ X` · `cash out` · `redeem for money` · `refundable` · `you will lose` · `hurry` · `limited time` (on earned StudsTokens) · `unlock more with` · `only Rs/NPR X away` · any emoji in system copy.

**Also banned:** `prize` · `award` · `win` · `winner` · `raffle` · `draw` · `baksis` · `jitauri` — the Income Tax Act 2058 windfall-gain definition ("lottery, gift, prize, baksis, award for winning", taxed under s.5/88A). Naming, not phrasing: use *StudsTokens*, *credits*, *balance*, *unlock*.

**Header chip** — `Earn StudsTokens` · `StudsToken balance: {n}. Open your StudsTokens.` (`aria-label`) · `{n} expiring` · `All starter unlocks used` · `Starter unlocks expire in {d} days`

**Resource card** — `Starter · {left} of {total} left` · `Starter unlock expires in {d} days` · `{price} StudsTokens` · `Unlocked` · `Use starter unlock` · `Unlock` · `Download` · `Earn {gap} more` · `Log in to unlock` · `Use them by {date}`

**Unlock dialog** — `Cost` · `Your balance` · `StudsTokens used` · `We spend the StudsTokens that expire soonest first, so nothing is wasted.` · `Unlock for {price} StudsTokens` · `Unlocking…` · `Not now` · `Use a starter unlock?` · `This is 1 of your 3 document unlocks. 2 will be left after this.` · `Keep browsing` · `Already unlocked — nothing was spent.` · `That unlock was not completed. Your balance has not changed.`

**Insufficient dialog** — `You need {gap} more StudsTokens` · `This {noun} costs {price} StudsTokens. Your balance is {balance}.` · `+{n} StudsTokens left` · `+{n} StudsTokens` · `Browse resources you can unlock now` · `You can still unlock other resources with the StudsTokens you have.`

**Wallet** — `StudsTokens` · `{n} StudsTokens` · `{n} starter unlocks left` · `Starter unlocks` · `3 documents · 1 video lecture · 1 mock test` · `Use them by {date}` · `StudsToken balance, by expiry` · `from referrals` · `from your uploads` · `from sign-up` · `from your profile` · `Never expires` · `expires in {d} days` · `12 November 2026` · `No StudsTokens yet` · `You have no StudsTokens yet` · `Your starter unlocks are used up.` · `Your starter unlocks expired on {date}.` · `Email me before StudsTokens expire.`

**Referral** — `Invite a friend` · `60 StudsTokens for each friend who completes their first action.` · `Copy code` · `Copied` · `Referral code copied.` · `Share` · `WhatsApp` · `Viber` · `Email` · `Copy link` · `{used} of {cap} used this month · resets {date}` · `60 StudsTokens held until {date}` · `We confirm their first completed action before releasing. If they don't complete one, these 60 StudsTokens are not released.` · `60 StudsTokens added` · `Not confirmed` · `No invites yet.` · `You've reached the monthly invite limit. It resets on {date}.` · pre-filled share message: `I'm studying on StudSphere. Join me with my code {CODE}: {LINK}`

**Earn progress** — `Complete your profile` · `{n} StudsTokens left to earn` · `Add your full name` · `Set your current level` · `Choose your courses` · `Add your city` · `Add a profile photo` · `Do this` · `+{n} StudsTokens` · `Profile complete. +25 StudsTokens earned.` · `+5 StudsTokens. {n} to go.` · `Your uploads` · `under review` · `Submitted {date}. We usually respond within 2 working days.` · `+80 StudsTokens added on {date}` · `was not approved` · `Upload another` · `View` · `{reason}`

**Admin** — `Pending Approval` · `{n} upload(s) awaiting review` · `Awaiting review` · `Approve` · `Approve · +80 StudsTokens` · `Reject` · `Reject with a reason` · `Reason for rejecting` · `Approve and publish "{title}"? The uploader gets 80 StudsTokens and it appears in the public catalogue.` · `Reason is required before rejecting.` · `Approved. 80 StudsTokens added to {name}.` · `No uploads waiting for review.` · `New uploads land here first, before they appear in the catalogue.` · `Publishing a submitted resource adds 80 StudsTokens to the uploader's balance.` · status column: `Awaiting review` · `Published` · `Rejected`

**Mock test** — `Attempts used` · `1 attempt used of 1` · `Uses 1 of your 1 mock test unlock` · `Log in to take this test` · `This test uses 1 of your 1 mock test unlock. You have 0 left — unlock another with 60 StudsTokens.` · `You can look through the questions, but you need an unlock to submit and see your score.`

**Copy that changes today** — two strings become untrue the moment StudsTokens exist and must be **edited, not left**: `MockTestRunner.tsx:397-400` *"Answer as many questions as you like — nothing is sent until you sign in and submit, so you can explore the paper first."* is still true for a signed-out visitor and **false** for a signed-in student who has spent their one attempt, so it must branch on `user`. `VideoLecturePlayer.tsx:165-166` *"The video collection stays open — you just need an account to start playback."* stays with `login-required`; the new `StudsTokens-required` state gets its own copy rather than reusing it.

---

## §11 — Accessibility

**1 · The existing modal is broken, and every coin dialog inherits the fix.** `StudyResourcesPage.tsx:510-513` sets `role="dialog"` and `aria-modal="true"` and then does nothing — no focus move, no focus trap, no `Escape`, no `aria-labelledby`. Focus stays on the card button behind the overlay and a keyboard user is trapped in the page. That is a WCAG 2.1.1 failure shipping today.

No Radix, no shadcn — so **`components/ui/Modal.tsx` is hand-rolled, ~70 lines**, built to the exact classes at `:505,513`, and shared by the login modal, the unlock dialog, the starter-token dialog, the insufficient dialog and the admin approve/reject dialogs. On open: capture `document.activeElement` and focus `[data-modal-initial]` (the primary button, not the heading, so a keyboard user can act immediately). On `keydown`: `Escape` closes, `Tab` cycles within `[data-modal-root]` and wraps at both ends. On close: **restore focus to the captured element** — the step usually missed, and the one that makes the card feel like it responded. `aria-labelledby` and `aria-describedby` are required props; no dialog ships without both. Body scroll locks while open. The backdrop click-to-close at `:507` (`e.target === e.currentTarget`) is preserved.

**2 · Contrast — measured, not assumed**, on the tokens this feature uses:

| Pair | Ratio | | Pair | Ratio |
|---|---|---|---|---|
| `brand-blue` on `white` | **8.59:1** AAA | | `amber-700` on `amber-50` | **4.84:1** AA |
| `brand-blue` on `blue-50` | **7.90:1** AAA | | `emerald-700` on `emerald-50` | **5.21:1** AA |
| `brand-hover` on `white` | **11.22:1** AAA | | `rose-700` on `rose-50` | **5.72:1** AA |
| `gray-700` on `white` | **10.31:1** AAA | | `red-700` on `red-50` | **5.91:1** AA |
| `gray-500` on `white` | **4.83:1** AA | | `slate-400` on `white` | **2.56:1** ❌ |
| | | | `gray-400` on `white` | **2.54:1** ❌ |

**Two required corrections to existing code.** `VideoLecturesPage.tsx:312` — the `Sign in to play` label is `text-slate-400` on white at **2.56:1**; change to `text-slate-500` (**4.76:1**). That line is replaced by the coin state badge anyway, but the class is used elsewhere and must be swept. And `rose-600` on `rose-50` is **4.28:1 — AA-large only**: fine as the 20px icon at `StudyResourcesPage.tsx:409` (a graphical object, threshold 3:1), but it **fails for 11px text**, so the video coin badge uses `rose-700` on `rose-50`, not the `rose-600` of `:410`. `text-gray-400` is acceptable on decorative empty-state icons (`:382`) and placeholders (`:431`) but must not carry text in the coin UI.

On the dark stage: `rose-300` on `slate-950` is **10.67:1** and `slate-400` on `slate-950` is **7.87:1**, both AAA — the new `coins-required` panel inherits the `VideoLecturePlayer.tsx:161,164` treatment unchanged.

**3 · Never colour-only.** Every state in §3.2, §6 and §7.2 carries a distinct glyph **and** distinct text: `LockKeyhole`+`Locked` · `Gift`+`Starter` · `Coins`+`{n} coins` · `Check`+`Unlocked` · `Hourglass`+`expires in 5 days` · `X`+`Not confirmed`. The coin price is **always literal text** ("40 coins"), never a glyph and never a coloured pill. Tone reinforces; it never carries meaning alone.

**4 · Motion.** All coin motion is opacity or a ≤3px translate, so `prefers-reduced-motion` needs no variant — except `coin-pop`, which scales: add `motion-reduce:animate-none` at the call site. No `animate-bounce`, no infinite loop on a balance, nothing that pulses while the student is trying to read a price.

**5 · Live regions.** The wallet total is `aria-live="polite" aria-atomic="true"` so a spend is announced. The results count at `StudyResourcesPage.tsx:302-303` already is — do not remove it. `unlocking` sets `aria-busy="true"`. Toasts are `role="status"` via sonner and need no extra region.

**6 · Touch targets.** Every new interactive element is ≥32px tall (`px-3 py-2 text-xs` on a 12px font) and the share buttons are `h-8 w-8` with `aria-label` + `title`. The card's primary button is the only tap target in the footer row, so there is no mis-tap ambiguity on a phone.

**7 · Focus-visible everywhere.** The `focus-visible:ring-2 focus-visible:ring-brand-blue focus-visible:ring-offset-2` pattern already in the repo (`:256,339,405`) is applied to all four button variants and all `Link`s in the earn panel.

---

## §12 — File-by-file change list

**New — service.** `services/coinsApi.ts` — typed client for `/api/v1/coins/*` (balance + lots, entitlements, spend preview, spend, referral code/link/status, upload status), following the `studyResourcesApi.ts` shape: exported interfaces, a namespace object, `apiRequest` from `services/api.ts:38`, `suppressAuthExpired` where a 401 is a normal outcome.

**New — coin components.** `components/coins/` — `CoinContext.tsx` (one provider holding the wallet snapshot + `refresh()`, consumed by the header chip, every card and every dialog so one spend updates all surfaces at once) · `useCoinState.ts` (the single function turning `(resource, wallet, user)` into one of the seven states in §3.2 — the matrix is implemented exactly once) · `CoinBadge.tsx` (§3.3) · `CoinActionButton.tsx` (state → button mapping, §3.4) · `CoinBalanceHeader.tsx` (§2.2, self-reads `CoinContext`, mounts in two hosts) · `WalletLots.tsx` (the per-lot `expires_at` ledger, sorted FEFO) · `StarterAllowanceCard.tsx` (the six §2.4 constraints) · `UnlockConfirmDialog.tsx` (§4) · `InsufficientFundsDialog.tsx` (§5) · `EarnRoutes.tsx` (the ranked route list, shared by the wallet rail and the insufficient dialog so the two can never disagree) · `ReferralCard.tsx` (§6) · `ProfileLadder.tsx` (§7.1) · `UploadEarningsCard.tsx` (§7.2).

**New — shared UI.** `components/ui/Modal.tsx` — hand-rolled focus-trapping, `Escape`-aware, `aria-labelledby`-required dialog on the existing classes (§11.1), replacing four hand-rolled overlays.

**New — routes and sections.** `app/user/dashboard/coins/page.tsx` (server shell) · `app/user/dashboard/coins/loading.tsx` and `error.tsx` (required by `studsnew/AGENTS.md`) · `components/user/dashboard/sections/WalletSection.tsx` (page body: three stacked sections, `lg:grid-cols-3`).

**Modified — study resources**

| File:line | Change |
|---|---|
| `components/studyResources/ResourceCard.tsx` **(new)** | Extracted from the duplicated inline JSX; renders all seven states |
| `StudyResourcesPage.tsx:400-484` | **Delete** inline card JSX; render `<ResourceCard variant="document">` |
| `StudyResourcesPage.tsx:503-541` | **Replace** the inline login modal with `components/ui/Modal.tsx` — fixes the focus trap (§11.1) while it is being touched |
| `StudyResourcesPage.tsx:195-211` | `handleDownload` branches on access: unlocked → download · affordable → unlock dialog · insufficient → gap dialog · anonymous → login |
| `StudyResourcesPage.tsx` (page root) | Wrap in `<CoinProvider>`; add the `?affordable=1` filter branch that §5's anti-dead-end link targets |
| `VideoLecturesPage.tsx:262-320` | **Delete** the duplicated card JSX; render `<ResourceCard variant="video">` |
| `VideoLecturesPage.tsx:312` | **Delete** the `Sign in to play` `text-slate-400` label (2.56:1); replaced by the coin state badge |
| `VideoLecturePlayer.tsx:156-176` | Add a fourth, `login-required`-sibling state `coins-required`, reusing the `bg-slate-950` locked panel and the `rose-500` CTA |
| `playbackState.ts` | Extend `PlaybackViewState` and `toPlaybackViewState` with the new state |
| `mockTests/MockTestListPage.tsx:90-95` | Swap the bare `Link` for `<ResourceCard variant="mock-test">`; keep the `cyan` category accent |
| `mockTests/MockTestRunner.tsx:387-411` | Attempt entitlement: the chip, the unlock path, and the `:397-400` copy fix (§10) |
| `StudyResourceFilterPanel.tsx` | Add the "Can unlock now" toggle, backed by `?affordable=1` |
| `services/studyResourcesApi.ts:18-37` | Add `access?: ResourceAccess` to `StudyResource` so the list endpoint carries per-resource state |

**Modified — dashboard and public chrome**

| File:line | Change |
|---|---|
| `components/navigation/EducationNavbar.tsx` | Mount `<CoinBalanceHeader>` in the right cluster, immediately before `NotificationBell` |
| `app/navbar-wrapper.tsx` | Drop the coin prop if the chip self-reads `CoinContext`; keep the wrapper otherwise |
| `components/user/dashboard/DashboardLayout.tsx:77-92` | Mount `<CoinBalanceHeader>` in the dashboard header |
| `components/user/dashboard/Sidebar.tsx:52-124` | Add the `Coins` nav item → `/user/dashboard/coins` |
| `app/user/dashboard/resources/page.tsx` | **Replace** with a three-line `redirect("/user/dashboard/coins")` |
| `components/user/dashboard/sections/ResourcesSection.tsx` | **Delete** — 43 lines of hardcoded mock data (`:4-11`) on an unlinked route |

**Modified — admin**

| File:line | Change |
|---|---|
| `components/superadmin/client/PendingResourcesSection.tsx` **(new)** | The approval queue, cloned from `PendingInstitutionsSection.tsx` (§8) |
| `DashboardShell.tsx` ~:146 / ~:420-427 / ~:682 | The three registration points: lazy import, nav child, `case` |
| `StudyResourcesSection.tsx:724-734` | Two-state badge → three (`awaiting` / `published` / `rejected`) |
| `StudyResourcesSection.tsx` (upload form) | One line: publishing awards the uploader 80 coins |

**Modified — tokens.** `app/globals.css` — **one addition**: `@keyframes coin-pop` + `.animate-coin-pop` (§2.5), in the style of `.animate-shine` at `:44-51`.

**Explicitly unchanged.** No new colour is added to `globals.css` — the coin is identified by the `Coins` glyph plus the literal word "coins", which is both an accessible name and a non-colour channel, and adding a fifth hue for a non-transactional counter is how a fourth dialect starts. No shadcn, no Radix, no `cn()`. The video player keeps `rounded-3xl bg-slate-950` (`VideoLecturePlayer.tsx:129`) as a documented exception. `app/study-resources/loading.tsx` and `error.tsx` get a realignment to `bg-gray-50` + `rounded-md` (they carry the stray `bg-[#f6f8fc]` / `rounded-3xl` dialect at `:5-8` and `:7,15`) — small, but it removes the last visible trace of the second language at route level. The 44 dead `animate-in` / `zoom-in-95` usages are left alone: fixing them is a separate repo-wide cleanup, and the coin feature does not depend on them (§0.5).

---

## §13 — Open items for the founder

1. **Moderation throughput.** §7.2 promises "within 2 working days" and §8 requires a human approve/reject-with-reason on every upload. If the queue is not staffed to that, the promise comes out of the copy first — the 80-coin award is otherwise an unbounded liability.
2. **Fraud-hold policy.** §6 states the hold's cost to the student plainly. Legal should confirm a 7-day hold with no partial release is defensible, and that "held" is not a representation of a deposit.
3. **The statutory citation — partially corrected, and one part of it was right.** An earlier draft of this document replaced the withdrawn India CCPA 2019 / ₹10 lakh reference with "Nepal's Consumer Protection Act 2075 (2018) §18". The **Act number and year were correct** — the Consumer Protection Act 2075 (2018) is the operative statute, superseding the 2054 (1997) Act — so the citation is now verified. The **section number was wrong**: the misleading-advertising prohibition is **s.16(2)(b) and s.16(2)(c)(3)**, not s.18. s.38(f) makes it an offence and s.40(3)(b) punishes it with **2 to 5 years' imprisonment or a fine of NPR 400,000–600,000, or both**. Source: [07-compliance-nepal.md](07-compliance-nepal.md). The "Legal Metrology Act 2074" reference remains withdrawn — it is not a Nepali statute. Nepali counsel should still confirm before launch. Do not re-add any other statute reference to this document from memory.
