# Module Dependency Graph

**Generated:** 2026-06-22  
**Scope:** `internal/` packages only (excludes `shared/` which is used by everything)

---

## 1. Full Dependency Graph

```
                          ┌──────────────────────────────┐
                          │         USER DOMAIN          │
                          └──────────────────────────────┘

  ┌──────────────────────┐     ┌─────────────────────────┐
  │        college        │◄────│          auth            │
  │  (college listings)   │     │ (login, register, OAuth,│
  └──────────────────────┘     │  profile, superadmin)    │
                               │                          │
  ┌──────────────────────┐     │  Imports:                │
  │      institution      │◄────│   ├── college            │
  │  (institution portal) │     │   ├── institution        │
  └──────────────────────┘     │   └── scholarshipprovider │
                               │                          │
  ┌──────────────────────┐     │  Imported by:            │
  │  scholarshipprovider  │◄────│   └── studentdashboard   │
  │  (provider portal)   │     └─────────────────────────┘
  └──────┬───────────────┘                    ▲
         │                                    │
         │  ┌──────────────────────────┐     │
         ├─►│       emailqueue          │     │
         │  │  (Asynq email tasks)     │     │
         │  └──────────┬───────────────┘     │
         │             │                     │
         │  ┌──────────┴──────────────┐     │
         ├─►│       scholarship        │     │
         │  │  (public scholarships,  │──────┘
         │  │   applications, payment)│
         │  └──────┬──────────────────┘
         │         │
         │    ┌────┴──────────────┐
         │    │       system       │
         │    │  (contact, ads,   │◄────┐
         │    │   carousels)      │     │
         │    └───────────────────┘     │
         │              ▲              │
         │              │              │
         │    ┌─────────┴──────┐      │
         │    │    education    │      │
         │    │  (courses,     │      │
         │    │   exams, news, │      │
         │    │   events, blogs)│     │
         │    └────────────────┘      │
         │                            │
         │    ┌───────────────┐       │
         └───►│    tools      │       │
              │ (recommender, │       │
              │  logo)        │       │
              └───────────────┘       │
                                      │
  ┌────────────────────────┐          │
  │      embedding          │◄────┐   │
  │  (OpenAI embeddings)    │     │   │
  └────────────────────────┘     │   │
              ▲                  │   │
         ┌────┴─────┐     ┌──────┴───┴──────┐
         │  search   │     │  ai (RAG chat)  │
         │ (hybrid   │     └─────────────────┘
         │  vector + │
         │  keyword) │     ┌─────────────────┐
         └──────────┘     │  chat (generic)  │
                          └─────────────────┘

  ┌──────────────────────┐
  │   studentdashboard    │────► admission, auth, scholarship
  │  (messages, calendar, │
  │   invites, bookmarks, │
  │   notifications)      │
  └──────────────────────┘

  ┌──────────────────────┐     ┌─────────────────────────┐
  │    Leaf Packages     │     │   Independent Modules   │
  │  (no internal deps)  │     │  (no one depends on    │
  │                      │     │   them)                 │
  │  ┌────────────────┐  │     │                        │
  │  │ counselling    │  │     │  ┌──────────────┐      │
  │  │ feedback       │  │     │  │ ai           │      │
  │  │ forum          │  │     │  │ chat         │      │
  │  │ projectshiksha │  │     │  │ counselling  │      │
  │  │ review         │  │     │  │ education    │      │
  │  │ university     │  │     │  │ feedback     │      │
  │  └────────────────┘  │     │  │ forum        │      │
  └──────────────────────┘     │  │ projectshiksha│     │
                               │  │ review       │      │
                               │  │ search       │      │
                               │  │ studentdashboard│   │
                               │  │ tools        │      │
                               │  │ university   │      │
                               │  └──────────────┘      │
                               └─────────────────────────┘
```

---

## 2. Import Table

| Package               | Imports (internal)                              | Imported By                                   |
| --------------------- | ----------------------------------------------- | --------------------------------------------- |
| `admission`           | —                                               | `studentdashboard`                            |
| `ai`                  | `embedding`                                     | —                                             |
| `auth`                | `college`, `institution`, `scholarshipprovider` | `studentdashboard`                            |
| `chat`                | `embedding`                                     | —                                             |
| `college`             | —                                               | `auth`                                        |
| `counselling`         | —                                               | —                                             |
| `education`           | `system`                                        | —                                             |
| `emailqueue`          | —                                               | `scholarship`, `scholarshipprovider`, `tools` |
| `embedding`           | —                                               | `ai`, `chat`, `search`                        |
| `feedback`            | —                                               | —                                             |
| `forum`               | —                                               | —                                             |
| `institution`         | `system`                                        | `auth`                                        |
| `projectshiksha`      | —                                               | —                                             |
| `review`              | —                                               | —                                             |
| `scholarship`         | `emailqueue`, `system`                          | `scholarshipprovider`, `studentdashboard`     |
| `scholarshipprovider` | `emailqueue`, `scholarship`                     | `auth`                                        |
| `search`              | `embedding`                                     | —                                             |
| `studentdashboard`    | `admission`, `auth`, `scholarship`              | —                                             |
| `system`              | —                                               | `education`, `institution`, `scholarship`     |
| `tools`               | `emailqueue`                                    | —                                             |
| `university`          | —                                               | —                                             |

---

## 3. Shared Package Dependencies

Every module imports from `internal/shared/`. The shared sub-packages:

```
shared/
├── config/         ── loaded by ALL packages (env vars, DB connection)
├── logger/         ── loaded by main + emailqueue
├── middleware/     ── loaded by main.go only
├── response/       ── loaded by ALL handler packages
├── seeder/         ── loaded by main.go only
├── slug/           ── loaded by scholarship packages
├── storage/        ── loaded by auth, main.go (MinIO/local file storage)
└── utils/          ── loaded by auth, college, forum
    ├── jwt.go      ── loaded by middleware
    ├── email.go    ── loaded by auth (OTP, approval emails)
    ├── otp_store.go ── loaded by auth
    └── uploads.go  ── loaded by auth
```

---

## 4. Circular Dependency Risk

```
     auth ───────────► scholarshipprovider
      │                      │
      │                      │ (via SetScholarshipProviderHandler)
      │                      │
      └──────────────────────┘

  Runtime-only circular reference:
  - auth/scholarshipprovider/login fallback checks ProviderAccessUser
  - SetScholarshipProviderHandler() is called from main.go after both
    packages are initialized
  - This is a unidirectional call at init time, NOT an import cycle
  - SAFE: auth imports scholarshipprovider, but NOT vice versa
```

---

## 5. Module Groups by Dependency Level

### Level 0 — Foundation (zero internal imports)

These modules have NO dependencies on other internal packages:

```
college          counselling      feedback         forum
projectshiksha   review           university       emailqueue
embedding        system           admission
```

**Impact analysis:** Changes to these modules never break other modules (except through shared interfaces). They are safe to modify independently.

### Level 1 — Single Dependency

```
education        → system
institution      → system
scholarship      → emailqueue, system
tools            → emailqueue
chat             → embedding
ai               → embedding
search           → embedding
```

**Impact analysis:** Changes propagate to exactly one level. If `system` changes, `education`, `institution`, and `scholarship` may need updates.

### Level 2 — Multiple Dependencies

```
scholarshipprovider  → emailqueue, scholarship
studentdashboard     → admission, auth, scholarship
```

**Impact analysis:** These are the most fragile. A change to `scholarship` affects both `scholarshipprovider` AND `studentdashboard`.

### Level 3 — Hub (most depended-upon)

```
auth   → imported by studentdashboard
system → imported by education, institution, scholarship
emailqueue → imported by scholarship, scholarshipprovider, tools
embedding  → imported by ai, chat, search
```

**Impact analysis:** These 4 packages are the most critical. Breaking changes here cascade farthest.

---

## 6. Module Dependency Flow Summary

```
                            ┌──────────────────┐
                            │   studentdashboard│
                            │  (consumer)      │
                            └────────┬─────────┘
                                     │
              ┌──────────────────────┼──────────────────────┐
              ▼                      ▼                      ▼
       ┌────────────┐       ┌──────────────────┐   ┌──────────────┐
       │ admission  │       │      auth        │   │ scholarship  │
       │ (leaf)     │       │ (hub)            │   │ (hub)        │
       └────────────┘       └────────┬─────────┘   └──────┬───────┘
                                     │                    │
              ┌──────────────────────┼──────────┐         │
              ▼                      ▼          ▼         ▼
       ┌────────────┐   ┌──────────────┐  ┌────────────┐  ┌──────────────┐
       │ college   │   │ institution  │  │ scholar-   │  │  system      │
       │ (leaf)    │   │              │  │ ship-      │  │  (hub)       │
       └────────────┘   └──────┬───────┘  │ provider   │  └──────┬───────┘
                               │          └─────┬──────┘         │
                               │                │                │
                               │          ┌─────┴──────┐   ┌─────┴──────┐
                               │          │ emailqueue │   │ education  │
                               │          │ (hub)      │   │ (consumer) │
                               │          └────────────┘   └────────────┘
                               ▼
                        ┌──────────────────┐
                        │   embedding       │
                        │   (hub)           │
                        └──────┬───────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
       ┌──────────┐    ┌──────────┐    ┌──────────┐
       │  search  │    │   ai     │    │  chat    │
       │(consumer)│    │(consumer)│    │(consumer)│
       └──────────┘    └──────────┘    └──────────┘
```

---

## 7. Key Architectural Observations

1. **`system` is a hidden hub** — 3 modules depend on it. It provides contact/ads/carousel functionality that education, institution, and scholarship use for "system" data.

2. **`emailqueue` enables async operations** — 3 modules enqueue emails. It depends on Redis (Asynq) but gracefully falls back to logging when Redis is unavailable.

3. **`embedding` enables AI features** — `ai`, `chat`, and `search` all need embeddings. Without it, `search` degrades to keyword-only and `ai`/`chat` are disabled.

4. **`auth` is the most complex** — It imports 3 other modules for cross-user-type operations (login fallback to sub-user, institution profile access, college claiming).

5. **7 leaf modules** (`counselling`, `feedback`, `forum`, `projectshiksha`, `review`, `university`, `admission`) have zero internal dependencies — easiest to test and refactor.

6. **`studentdashboard` is the heaviest consumer** — it imports from 3 modules: `admission`, `auth`, `scholarship`.
