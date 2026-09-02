# StudSphere Backend — QA Test Documentation

**Version:** 1.0.0  
**Go Version:** 1.26  
**Framework:** Gin + GORM  
**Database:** PostgreSQL 16 (with pgvector)  
**Last Updated:** 2026-06-22

---

## Table of Contents

1. [System Overview](#1-system-overview)
2. [Architecture](#2-architecture)
3. [Quick Setup for QA](#3-quick-setup-for-qa)
4. [Environment Configuration](#4-environment-configuration)
5. [User Roles & Access Matrix](#5-user-roles--access-matrix)
6. [Authentication Flows](#6-authentication-flows)
7. [Complete API Reference](#7-complete-api-reference)
8. [Key Data Models](#8-key-data-models)
9. [Test Scenarios by Feature](#9-test-scenarios-by-feature)
10. [Known Issues & Limitations](#10-known-issues--limitations)
11. [Troubleshooting Guide](#11-troubleshooting-guide)

---

## 1. System Overview

StudSphere is an education platform backend with **4 user roles**, **22 modules**, and **~260+ API endpoints**. It connects to a PostgreSQL database, optional MinIO file storage, optional Redis-backed email queue, and optional AI/LLM services.

### Core Capabilities

| Feature                         | Description                                                                                             |
| ------------------------------- | ------------------------------------------------------------------------------------------------------- |
| **Multi-role Auth**             | Student, Institution, Scholarship Provider, Superadmin                                                  |
| **OAuth**                       | Google login for all 4 roles                                                                            |
| **College/University Listings** | Public browse, search, filter                                                                           |
| **Scholarship Management**      | Public + provider-managed scholarships                                                                  |
| **Course Directory**            | Browse courses with filter counts                                                                       |
| **Forum**                       | Posts, comments, communities, polls                                                                     |
| **Student Dashboard**           | Messages, calendar, invites, bookmarks, notifications                                                   |
| **Institution Portal**          | Programs, entrances, events, news, QMS, admissions, counselling                                         |
| **Provider Portal**             | Scholarships, applications, interviews, written exams, results, volunteers, services, projects, gallery |
| **Superadmin Panel**            | User management, approve/reject institutions & providers, college CRUD, ads, carousels                  |
| **Counselling**                 | Booking system for students + institution management                                                    |
| **Search**                      | Hybrid vector (pgvector + OpenAI) + keyword (ILIKE) across 8 entity types                               |
| **AI Chat**                     | RAG-powered chatbot with streaming (OpenAI-compatible)                                                  |
| **Recommendations**             | Scholarship finder, college recommender                                                                 |
| **Payment**                     | eSewa integration for scholarship applications                                                          |
| **Email Queue**                 | Async email via Asynq (Redis)                                                                           |
| **Admit Cards**                 | PDF generation with Chromium                                                                            |
| **File Uploads**                | MinIO or local storage                                                                                  |

---

## 2. Architecture

```
┌──────────────────────────────┐
│         Client (Frontend)    │  React/Next.js (studsnew)
└──────────┬───────────────────┘
           │ HTTP/JSON
           ▼
┌──────────────────────────────┐
│    Gin Router + Middleware   │  JWT Auth, CORS, Logging
└──────────┬───────────────────┘
           │
     ┌─────┴─────┐
     ▼           ▼
┌────────┐ ┌──────────┐
│ Public │ │ Protected│
│ Routes │ │ Routes   │
└────────┘ └────┬─────┘
                │
         ┌──────┴──────┐
         ▼             ▼
┌─────────────────┐ ┌─────────────────┐
│  Handler Layer  │ │  AI/Chat Handler│
│ (HTTP req/res)  │ │ (SSE streaming) │
└────────┬────────┘ └────────┬────────┘
         ▼                   │
┌─────────────────┐          │
│  Service Layer  │          │
│  (Business      │          │
│   Logic)        │          │
└────────┬────────┘          │
         ▼                   │
┌─────────────────┐          │
│ Repository Layer│          │  GORM queries
│ (Data Access)   │          │
└────────┬────────┘          │
         ▼                   ▼
┌─────────────────────────────────────┐
│         PostgreSQL + pgvector       │
│  (also: SQLite for dev/testing)     │
└─────────────────────────────────────┘
```

### External Dependencies

| Service                  | Purpose                          | Optional? |
| ------------------------ | -------------------------------- | --------- |
| PostgreSQL 16 + pgvector | Primary database                 | No        |
| Redis                    | Asynq email queue                | Yes       |
| MinIO                    | File storage (S3-compatible)     | Yes       |
| OpenAI-compatible API   | Embeddings + LLM chat            | Yes       |
| SMTP Server              | Send OTP/welcome/approval emails | Yes       |
| Google OAuth             | Social login                     | Yes       |

### Module Map

```
internal/
├── admission/        # Student admissions
├── ai/               # RAG chat assistant (SSE streaming)
├── auth/             # Auth, OAuth, profile, superadmin
├── chat/             # General chat endpoint
├── college/          # College CRUD + recommender
├── counselling/      # Counselling booking
├── education/        # Courses, exams, news, events, blogs, entrances
├── emailqueue/       # Asynq email queue
├── embedding/        # OpenAI-compatible vector embeddings
├── feedback/         # User feedback & testimonials
├── forum/            # Posts, comments, communities, polls
├── institution/      # Institution portal (full CMS)
├── projectshiksha/   # Separate scholarship program
├── review/           # College reviews with ratings
├── scholarship/      # Scholarships, applications, payments
├── scholarshipprovider/ # Provider portal (full suite)
├── search/           # Hybrid vector + keyword search
├── shared/           # Config, middleware, utils, storage, seeder
├── studentdashboard/ # Student dashboard features
├── system/           # Contact, ads, carousels
├── tools/            # Recommendation tools, logo
└── university/       # University CRUD
```

---

## 3. Quick Setup for QA

### Prerequisites

- Go 1.21+
- Docker & Docker Compose (for PostgreSQL)
- curl / Postman / Bruno (for API testing)

### Step-by-Step

```bash
# 1. Clone and enter directory
cd studsback

# 2. Copy environment config
cp .env.example .env

# 3. Edit .env with minimal settings (see section 4)

# 4. Start PostgreSQL
make docker-up

# 5. Install Go dependencies
make install

# 6. Run the server
make run
```

Server starts on **http://localhost:8080**  
Swagger docs at **http://localhost:8080/docs**  
Health check: **http://localhost:8080/health**

### Test Commands

```bash
# Run all tests
make test

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Lint
make lint
```

### Default Admin Credentials

On first run, the super admin is auto-created from `.env`:

- **Email:** `admin@studsphere.com`
- **Password:** `Admin@12345`

---

## 4. Environment Configuration

### `.env` Reference

| Variable               | Default                     | Required | Notes                                         |
| ---------------------- | --------------------------- | -------- | --------------------------------------------- |
| `PORT`                 | `8080`                      | Yes      | Server port                                   |
| `GIN_MODE`             | `debug`                     | No       | `debug` or `release`                          |
| `DB_HOST`              | `localhost`                 | Yes      | PostgreSQL host                               |
| `DB_PORT`              | `5432`                      | Yes      | PostgreSQL port                               |
| `DB_USER`              | `studsphere_user`           | Yes      | DB user                                       |
| `DB_PASSWORD`          | `studsphere_pass`           | Yes      | DB password                                   |
| `DB_NAME`              | `studsphere`                | Yes      | DB name                                       |
| `DB_SSLMODE`           | `disable`                   | Yes      | `disable` for dev, `require` for cloud        |
| `JWT_SECRET`           | —                           | Yes      | **Must change in production**                 |
| `JWT_EXPIRY`           | `24h`                       | No       | Token expiry                                  |
| `SUPER_ADMIN_EMAIL`    | `admin@studsphere.com`      | No       | Auto-seeded on first run                      |
| `SUPER_ADMIN_PASSWORD` | `Admin@12345`               | No       | Auto-seeded on first run                      |
| `GOOGLE_CLIENT_ID`     | —                           | No       | Required for Google OAuth                     |
| `GOOGLE_CLIENT_SECRET` | —                           | No       | Required for Google OAuth                     |
| `GOOGLE_REDIRECT_URL`  | —                           | No       | Must match Google Cloud Console               |
| `FRONTEND_URL`         | `http://localhost:5173`     | Yes      | OAuth redirects, CORS                         |
| `COOKIE_DOMAIN`        | —                           | No       | Set in production for cross-subdomain cookies |
| `SMTP_HOST`            | `smtp.gmail.com`            | No       | Required for email sending                    |
| `SMTP_PORT`            | `587`                       | No       | SMTP port                                     |
| `SMTP_USER`            | —                           | No       | SMTP username                                 |
| `SMTP_PASS`            | —                           | No       | SMTP password / app password                  |
| `EMBEDDING_ENABLED`    | `false`                     | No       | Enable pgvector search                        |
| `EMBEDDING_API_KEY`    | —                           | No       | OpenAI API key                                |
| `EMBEDDING_BASE_URL`   | `http://localhost:8081/v1`  | No       | OpenAI-compatible endpoint                    |
| `EMBEDDING_MODEL`      | `text-embedding-3-small`    | No       | Embedding model                               |
| `VECTOR_DIMENSION`     | `1024`                      | No       | Must match model (Qwen3-Embedding-0.6B = 1024) |
| `LLM_ENABLED`          | `false`                     | No       | Enable AI chat                                |
| `LLM_BASE_URL`         | `https://openrouter.ai/api/v1` | No       | OpenAI-compatible API base URL               |
| `LLM_MODEL`            | `openai/gpt-4o-mini`          | No       | Model identifier                              |

### Minimal `.env` for QA

```ini
PORT=8080
GIN_MODE=debug
DB_HOST=localhost
DB_PORT=5432
DB_USER=studsphere_user
DB_PASSWORD=studsphere_pass
DB_NAME=studsphere
DB_SSLMODE=disable
JWT_SECRET=qa-test-secret-key-2026
JWT_EXPIRY=24h
SUPER_ADMIN_EMAIL=admin@studsphere.com
SUPER_ADMIN_PASSWORD=Admin@12345
FRONTEND_URL=http://localhost:5173
```

---

## 5. User Roles & Access Matrix

### Roles Defined

| Role                     | `user_role` in JWT             | Description                                                   |
| ------------------------ | ------------------------------ | ------------------------------------------------------------- |
| **Student**              | `student`                      | Default role. Browse, apply to scholarships, forum, dashboard |
| **Institution**          | `institution`                  | College/institute admin portal                                |
| **Scholarship Provider** | `scholarship_provider`         | Organization managing scholarships                            |
| **Sub-User**             | `scholarship_provider_subuser` | Limited-access provider staff                                 |
| **Superadmin**           | `superadmin` / `super_admin`   | Full system administration                                    |

### Access by Role

| Area                      | Student      | Institution | Provider   | Superadmin  |
| ------------------------- | ------------ | ----------- | ---------- | ----------- |
| Public endpoints          | ✅           | ✅          | ✅         | ✅          |
| Auth (register/login)     | ✅           | ✅          | ✅         | ✅          |
| Profile management        | ✅           | Via portal  | Via portal | Full access |
| College browse            | ✅           | ✅          | ✅         | ✅          |
| Education content         | ✅           | ✅          | ✅         | ✅          |
| Forum                     | ✅           | ✅          | ✅         | ✅          |
| Counselling booking       | ✅           | ✅          | —          | ✅          |
| Scholarship apply         | ✅           | ✅          | ✅         | ✅          |
| Student dashboard         | ✅           | —           | —          | —           |
| Institution portal        | —            | ✅          | —          | —           |
| Provider portal           | —            | —           | ✅         | —           |
| Superadmin panel          | —            | —           | —          | ✅          |
| Review system             | ✅           | —           | —          | —           |
| Feedback                  | ✅           | ✅          | ✅         | ✅          |
| Ad/Carousel/Inquiry admin | —            | —           | —          | ✅          |
| College/University admin  | —            | —           | —          | ✅          |
| Scholarship admin         | —            | —           | —          | ✅          |
| Project Shiksha           | Public apply | —           | —          | Admin       |

### Auth Middleware Logic

- **authMW** (`middleware.Auth`): Validates JWT from `Authorization: Bearer <token>`, cookie `token`, or `?token=` query param. Sets `user_id`, `user_email`, `user_role`, `provider_id` in context.
- **roleMW** (`middleware.RequireRole`): Checks `user_role` against allowed roles defined per route. Used for admin-only routes.

---

## 6. Authentication Flows

### Flow 1: Email/Password Registration

```
1. POST /api/v1/auth/register
   Body: { email, password, first_name, last_name, role? }
   → 201 { data: { email, requires_otp: true } }
   (User stored temporarily in memory-based OTP store)

2. POST /api/v1/auth/send-otp
   Body: { email, type: "verification" }
   → 200 { message: "OTP sent" }
   (OTP sent via SMTP, also logged to console in dev)

3. POST /api/v1/auth/verify-otp
   Body: { email, otp }
   → 200 { data: { token, user } }
   (User created in DB, JWT returned)
```

**Important:** Steps 1 and 2 are separate. The frontend calls step 1 first, then step 2 when user clicks "Verify Account". The user is not persisted to DB until step 3 succeeds. If server restarts between steps 1-3, registration data is lost.

### Flow 2: Institution / Provider Registration

Same OTP flow as above, but:

- Institution: `POST /api/v1/institutions/auth/register`
- Provider: `POST /api/v1/scholarship-providers/auth/register`
- Status set to `"pending"` — must be approved by superadmin before login
- On approval, system generates a random 12-char password and emails it

### Flow 3: Google OAuth

```
1. GET /api/v1/auth/google?redirect=<frontend_url>
   → 307 Redirect to Google consent screen

2. User approves → Google redirects to callback:
   GET /api/v1/auth/google/callback?code=...&state=...
   → 307 Redirect to frontend with ?token=JWT

3. Frontend stores JWT in localStorage
```

Same flow for institutions (`/institutions/auth/google`) and providers (`/scholarship-providers/auth/google`).

### Flow 4: Password Reset

```
1. POST /api/v1/auth/send-otp
   Body: { email, type: "password_reset" }

2. POST /api/v1/auth/reset-password
   Body: { email, otp, password }
```

### Flow 5: Provider Sub-User Login

When a provider login fails, the system falls back to checking the `provider_access_users` table for sub-user credentials.

---

## 7. Complete API Reference

All endpoints are prefixed with `/api/v1` unless otherwise noted.

### 7.1 Public Endpoints (No Auth)

#### Health

| Method | Path      | Description          |
| ------ | --------- | -------------------- |
| GET    | `/health` | Server health status |
| GET    | `/docs`   | Swagger UI redirect  |

#### Auth

| Method | Path                                          | Description                                |
| ------ | --------------------------------------------- | ------------------------------------------ |
| POST   | `/auth/register`                              | Register student                           |
| POST   | `/auth/login`                                 | Student login                              |
| POST   | `/auth/logout`                                | Clear auth cookie                          |
| POST   | `/auth/send-otp`                              | Send verification/reset OTP                |
| POST   | `/auth/verify-otp`                            | Verify OTP + create user                   |
| POST   | `/auth/reset-password`                        | Reset password with OTP                    |
| GET    | `/auth/google`                                | Google OAuth login                         |
| GET    | `/auth/google/callback`                       | Google OAuth callback                      |
| POST   | `/institutions/auth/register`                 | Register institution                       |
| POST   | `/institutions/auth/claim`                    | Claim institution                          |
| POST   | `/institutions/auth/login`                    | Institution login                          |
| POST   | `/institutions/auth/send-otp`                 | Institution OTP                            |
| POST   | `/institutions/auth/reset-password`           | Institution password reset                 |
| GET    | `/institutions/auth/google`                   | Institution Google login                   |
| GET    | `/institutions/auth/google/callback`          | Institution Google callback                |
| POST   | `/scholarship-providers/auth/register`        | Register provider                          |
| POST   | `/scholarship-providers/auth/login`           | Provider login                             |
| POST   | `/scholarship-providers/auth/send-otp`        | Provider OTP                               |
| POST   | `/scholarship-providers/auth/reset-password`  | Provider password reset                    |
| GET    | `/scholarship-providers/auth/google`          | Provider Google login                      |
| GET    | `/scholarship-providers/auth/google/callback` | Provider Google callback                   |
| POST   | `/superadmin/auth/register`                   | Superadmin register (requires access code) |
| POST   | `/superadmin/auth/login`                      | Superadmin login                           |

#### Colleges

| Method | Path                                 | Description                          |
| ------ | ------------------------------------ | ------------------------------------ |
| GET    | `/colleges`                          | List all colleges (supports filters) |
| GET    | `/colleges/filter-counts`            | Get filter facet counts              |
| GET    | `/colleges/featured`                 | Featured colleges                    |
| GET    | `/colleges/:id`                      | College detail                       |
| POST   | `/colleges/recommend`                | College recommendations              |
| GET    | `/admissions/colleges`               | Admissions college listings          |
| GET    | `/admissions/colleges/filter-counts` | Admissions filter counts             |
| GET    | `/admissions/colleges/featured`      | Admissions featured                  |
| GET    | `/admissions/colleges/:id`           | Admissions college detail            |
| POST   | `/admissions/colleges/recommend`     | Admissions recommendations           |
| GET    | `/admissions/direct`                 | Direct admissions listing            |

#### Universities

| Method | Path                          | Description         |
| ------ | ----------------------------- | ------------------- |
| GET    | `/universities`               | List universities   |
| GET    | `/universities/filter-counts` | Filter facet counts |
| GET    | `/universities/:id`           | University detail   |
| GET    | `/universities/:id/:tab`      | University tab data |

#### Education Content

| Method | Path                                    | Description                        |
| ------ | --------------------------------------- | ---------------------------------- |
| GET    | `/education/rankings`                   | Education rankings                 |
| GET    | `/education/exams`                      | List exams                         |
| GET    | `/education/exams/:id`                  | Exam detail                        |
| GET    | `/education/courses`                    | List courses                       |
| GET    | `/education/courses/filter-counts`      | Course filter counts               |
| GET    | `/education/courses/:id`                | Course detail                      |
| GET    | `/education/courses/:id/details`        | Course details extended            |
| GET    | `/education/news`                       | List news                          |
| GET    | `/education/news/filter-counts`         | News filter counts                 |
| GET    | `/education/news/:id`                   | News detail                        |
| GET    | `/education/events`                     | List events                        |
| GET    | `/education/events/filter-counts`       | Event filter counts                |
| GET    | `/education/events/:id`                 | Event detail                       |
| GET    | `/education/blogs`                      | List blogs (paginated, searchable) |
| GET    | `/education/blogs/filter-counts`        | Blog filter counts                 |
| GET    | `/education/blogs/:id`                  | Blog detail                        |
| POST   | `/education/blogs/:id/view`             | Increment blog view count          |
| GET    | `/entrances`                            | List entrance exams                |
| GET    | `/entrances/filter-counts`              | Entrance filter counts             |
| GET    | `/entrances/:id`                        | Entrance detail                    |
| GET    | `/education/reviews/college/:collegeId` | College reviews                    |

#### Scholarships

| Method | Path                                       | Description                             |
| ------ | ------------------------------------------ | --------------------------------------- |
| GET    | `/education/scholarships`                  | List scholarships                       |
| GET    | `/education/scholarships/:id`              | Scholarship detail                      |
| GET    | `/education/scholarships/:id/similar`      | Similar scholarships                    |
| GET    | `/education/scholarships/:id/exam-centers` | Exam centers for scholarship            |
| POST   | `/education/scholarships/:id/apply`        | Apply to scholarship (can be anonymous) |
| POST   | `/education/scholarships/recommend`        | Scholarship recommendations             |
| POST   | `/scholarships/upload`                     | Upload scholarship documents            |
| POST   | `/scholarships/:id/pay`                    | Initiate payment for scholarship        |
| POST   | `/scholarships/pay/:id/confirm`            | Confirm payment                         |
| POST   | `/scholarships/pay/:id/receipt`            | Upload payment receipt                  |
| POST   | `/scholarships/pay/esewa/initiate`         | eSewa payment initiation                |
| POST   | `/scholarships/pay/esewa/verify`           | eSewa payment verification              |

#### Forum

| Method | Path                        | Description       |
| ------ | --------------------------- | ----------------- |
| GET    | `/forum/posts`              | List forum posts  |
| GET    | `/forum/posts/:id/comments` | Get post comments |
| GET    | `/forum/communities`        | List communities  |

#### Search

| Method | Path                    | Description                |
| ------ | ----------------------- | -------------------------- |
| GET    | `/search`               | Search all entities        |
| GET    | `/search/vector-status` | Check vector search status |
| POST   | `/search/reindex`       | Trigger reindex (public)   |

#### System

| Method | Path                    | Description                              |
| ------ | ----------------------- | ---------------------------------------- |
| POST   | `/system/contact`       | Submit contact inquiry                   |
| GET    | `/system/ads`           | Get active ads (filter by `?page=`)      |
| POST   | `/system/ads/:id/click` | Track ad click                           |
| GET    | `/system/carousels`     | Get carousel slides (filter by `?page=`) |
| GET    | `/system/notifications` | Get public notifications                 |

#### AI & Chat

| Method | Path         | Description               |
| ------ | ------------ | ------------------------- |
| POST   | `/ai/chat`   | AI chat (SSE streaming)   |
| GET    | `/ai/models` | List available LLM models |
| POST   | `/chat`      | General chat endpoint     |

#### Institution Public

| Method | Path                                            | Description                           |
| ------ | ----------------------------------------------- | ------------------------------------- |
| GET    | `/institutions/public`                          | List institutions                     |
| GET    | `/institutions/public/filter-counts`            | Institution filter counts             |
| GET    | `/institutions/public/:id`                      | Institution detail                    |
| GET    | `/institutions/public/:id/counselling-sessions` | Available counselling sessions        |
| GET    | `/institutions/public/news`                     | Institution public news               |
| GET    | `/institutions/public/news/:id`                 | Institution public news detail        |
| GET    | `/institutions/public/events`                   | Institution public events             |
| GET    | `/institutions/public/events/:id`               | Institution public event detail       |
| GET    | `/institutions/public/scholarships`             | Institution public scholarships       |
| GET    | `/institutions/public/scholarships/:id`         | Institution public scholarship detail |
| GET    | `/admissions/published`                         | Published admissions                  |
| GET    | `/admissions/published/institutions`            | Published institution list            |
| GET    | `/admissions/published/institutions/:id`        | Published institution detail          |

#### Provider Public

| Method | Path                           | Description                  |
| ------ | ------------------------------ | ---------------------------- |
| GET    | `/public/news`                 | Provider public news         |
| GET    | `/public/news/:id`             | Provider public news detail  |
| GET    | `/public/events`               | Provider public events       |
| GET    | `/public/events/:id`           | Provider public event detail |
| GET    | `/public/blogs`                | Provider public blogs        |
| GET    | `/public/blogs/:id`            | Provider public blog detail  |
| GET    | `/public/providers/:id`        | Provider public profile      |
| GET    | `/public/volunteers`           | List volunteer opportunities |
| GET    | `/public/volunteers/:id`       | Volunteer detail             |
| POST   | `/public/volunteers/:id/apply` | Apply to volunteer           |
| GET    | `/public/results/scholarships` | List published results       |
| GET    | `/public/results/check`        | Check result by identifier   |

#### Feedback & Reviews (Public)

| Method | Path                   | Description         |
| ------ | ---------------------- | ------------------- |
| GET    | `/public/feedback`     | Get public feedback |
| POST   | `/public/testimonials` | Submit testimonial  |
| GET    | `/blogs/:id/comments`  | Blog comments       |
| POST   | `/blogs/:id/comments`  | Add blog comment    |

#### Project Shiksha

| Method | Path                                                     | Description         |
| ------ | -------------------------------------------------------- | ------------------- |
| POST   | `/project-shiksha/applications`                          | Submit application  |
| GET    | `/project-shiksha/applications/:id`                      | Get application     |
| GET    | `/project-shiksha/applications/roll-number/:roll_number` | Get by roll number  |
| GET    | `/project-shiksha/applications/admit-card/:roll_number`  | Download admit card |
| POST   | `/project-shiksha/payments`                              | Submit payment      |
| POST   | `/project-shiksha/payments/esewa/initiate`               | eSewa initiation    |
| POST   | `/project-shiksha/payments/esewa/verify`                 | eSewa verification  |

#### Tools

| Method | Path                                         | Description                 |
| ------ | -------------------------------------------- | --------------------------- |
| POST   | `/tools/scholarship-finder/recommendations`  | Scholarship recommendations |
| POST   | `/tools/college-recommender/recommendations` | College recommendations     |
| GET    | `/tools/logo/serve`                          | Serve tool logo             |

#### Misc Public

| Method | Path                       | Description                                 |
| ------ | -------------------------- | ------------------------------------------- |
| GET    | `/api/v1/proxy-image?url=` | Proxy external images (allowlisted domains) |
| GET    | `/uploads/*filepath`       | Serve uploaded files                        |

---

### 7.2 Student Protected Endpoints (authMW)

#### Profile

| Method | Path                     | Description                 |
| ------ | ------------------------ | --------------------------- |
| GET    | `/profile`               | Get own profile             |
| PUT    | `/profile`               | Update profile              |
| POST   | `/auth/change-password`  | Change password             |
| GET    | `/profile/education`     | List education entries      |
| POST   | `/profile/education`     | Add education entry         |
| PUT    | `/profile/education/:id` | Update education entry      |
| DELETE | `/profile/education/:id` | Delete education entry      |
| POST   | `/preferences`           | Save onboarding preferences |
| POST   | `/auth/profile/picture`  | Upload profile picture      |

#### Admissions

| Method | Path              | Description                  |
| ------ | ----------------- | ---------------------------- |
| POST   | `/admissions`     | Create admission application |
| GET    | `/admissions/my`  | List own admissions          |
| GET    | `/admissions/:id` | Get admission detail         |
| PUT    | `/admissions/:id` | Update admission             |
| DELETE | `/admissions/:id` | Delete admission             |

#### Counselling

| Method | Path                         | Description              |
| ------ | ---------------------------- | ------------------------ |
| POST   | `/counselling/bookings`      | Create booking           |
| GET    | `/counselling/bookings/my`   | List own bookings        |
| POST   | `/counselling/sessions/book` | Book counselling session |

#### Scholarships (Student)

| Method | Path                             | Description            |
| ------ | -------------------------------- | ---------------------- |
| GET    | `/scholarships/my-applications`  | List own applications  |
| GET    | `/scholarships/applications/:id` | Get application detail |
| PUT    | `/scholarships/applications/:id` | Update application     |
| DELETE | `/scholarships/applications/:id` | Delete application     |

#### Dashboard

| Method | Path                             | Description          |
| ------ | -------------------------------- | -------------------- |
| GET    | `/dashboard/stats`               | Dashboard statistics |
| GET    | `/dashboard/recent-applications` | Recent applications  |
| GET    | `/my-applications`               | All my applications  |

#### Messages

| Method | Path                  | Description        |
| ------ | --------------------- | ------------------ |
| GET    | `/messages`           | List conversations |
| GET    | `/messages/:id`       | Get message thread |
| POST   | `/messages`           | Send message       |
| POST   | `/messages/:id/reply` | Reply to message   |
| GET    | `/messages/contacts`  | Get contacts       |

#### Calendar

| Method | Path                   | Description  |
| ------ | ---------------------- | ------------ |
| GET    | `/calendar/events`     | List events  |
| GET    | `/calendar/events/:id` | Get event    |
| POST   | `/calendar/events`     | Create event |
| PUT    | `/calendar/events/:id` | Update event |
| DELETE | `/calendar/events/:id` | Delete event |

#### Invites

| Method | Path                   | Description    |
| ------ | ---------------------- | -------------- |
| GET    | `/invites`             | List invites   |
| GET    | `/invites/:id`         | Get invite     |
| PUT    | `/invites/:id/accept`  | Accept invite  |
| PUT    | `/invites/:id/decline` | Decline invite |
| PUT    | `/invites/:id/save`    | Save invite    |

#### Bookmarks

| Method | Path               | Description           |
| ------ | ------------------ | --------------------- |
| GET    | `/bookmarks`       | List bookmarks        |
| POST   | `/bookmarks`       | Create bookmark       |
| DELETE | `/bookmarks/:id`   | Delete bookmark       |
| GET    | `/bookmarks/:type` | Get bookmarks by type |

#### Notifications

| Method | Path                      | Description        |
| ------ | ------------------------- | ------------------ |
| GET    | `/notifications`          | List notifications |
| PUT    | `/notifications/:id/read` | Mark as read       |
| PUT    | `/notifications/read-all` | Mark all as read   |

#### Forum (Protected)

| Method | Path                          | Description        |
| ------ | ----------------------------- | ------------------ |
| POST   | `/forum/posts`                | Create post        |
| POST   | `/forum/posts/:id/like`       | Like post          |
| POST   | `/forum/posts/:id/dislike`    | Dislike post       |
| POST   | `/forum/posts/:id/save`       | Save post          |
| PUT    | `/forum/posts/:id`            | Update post        |
| DELETE | `/forum/posts/:id`            | Delete post        |
| POST   | `/forum/posts/:id/comments`   | Add comment        |
| POST   | `/forum/posts/:id/poll/vote`  | Vote in poll       |
| POST   | `/forum/upload`               | Upload forum media |
| POST   | `/forum/communities/:id/join` | Join community     |

#### Reviews (Student)

| Method | Path                             | Description         |
| ------ | -------------------------------- | ------------------- |
| POST   | `/user/reviews`                  | Submit review       |
| GET    | `/user/reviews`                  | List own reviews    |
| PUT    | `/user/reviews/:id`              | Update review       |
| DELETE | `/user/reviews/:id`              | Delete review       |
| POST   | `/user/reviews/:id/report`       | Report review       |
| POST   | `/education/reviews/:id/helpful` | Mark review helpful |

#### Feedback

| Method | Path               | Description         |
| ------ | ------------------ | ------------------- |
| POST   | `/feedback`        | Submit feedback     |
| GET    | `/feedback/status` | Get feedback status |

#### Institution Inquiry

| Method | Path                        | Description                 |
| ------ | --------------------------- | --------------------------- |
| POST   | `/institutions/:id/inquiry` | Send inquiry to institution |

---

### 7.3 Institution Portal (authMW + roleMW: institution)

#### Dashboard & Analytics

| Method | Path                     | Description        |
| ------ | ------------------------ | ------------------ |
| GET    | `/institution/dashboard` | Dashboard overview |
| GET    | `/institution/analytics` | Analytics data     |

#### Profile & Settings

| Method | Path                             | Description     |
| ------ | -------------------------------- | --------------- |
| GET    | `/institution/profile`           | Get profile     |
| PUT    | `/institution/profile`           | Update profile  |
| GET    | `/institution/media`             | List media      |
| POST   | `/institution/media`             | Upload media    |
| DELETE | `/institution/media/:id`         | Delete media    |
| GET    | `/institution/settings`          | Get settings    |
| PUT    | `/institution/settings`          | Update settings |
| PUT    | `/institution/settings/password` | Change password |

#### Programs

| Method | Path                        | Description    |
| ------ | --------------------------- | -------------- |
| GET    | `/institution/programs`     | List programs  |
| GET    | `/institution/programs/:id` | Get program    |
| POST   | `/institution/programs`     | Create program |
| PUT    | `/institution/programs/:id` | Update program |
| DELETE | `/institution/programs/:id` | Delete program |

#### Counselling

| Method | Path                                           | Description           |
| ------ | ---------------------------------------------- | --------------------- |
| GET    | `/institution/counselling/sessions`            | List sessions         |
| POST   | `/institution/counselling/sessions`            | Create session        |
| DELETE | `/institution/counselling/sessions/:id`        | Delete session        |
| GET    | `/institution/counselling/bookings`            | List bookings         |
| PUT    | `/institution/counselling/bookings/:id/status` | Update booking status |

#### Entrance Exams

| Method | Path                                    | Description             |
| ------ | --------------------------------------- | ----------------------- |
| GET    | `/institution/entrances`                | List entrances          |
| GET    | `/institution/entrances/:id`            | Get entrance            |
| POST   | `/institution/entrances`                | Create entrance         |
| PUT    | `/institution/entrances/:id`            | Update entrance         |
| DELETE | `/institution/entrances/:id`            | Delete entrance         |
| GET    | `/institution/entrances/:id/applicants` | Get entrance applicants |

#### Events

| Method | Path                      | Description  |
| ------ | ------------------------- | ------------ |
| GET    | `/institution/events`     | List events  |
| GET    | `/institution/events/:id` | Get event    |
| POST   | `/institution/events`     | Create event |
| PUT    | `/institution/events/:id` | Update event |
| DELETE | `/institution/events/:id` | Delete event |

#### News

| Method | Path                    | Description |
| ------ | ----------------------- | ----------- |
| GET    | `/institution/news`     | List news   |
| GET    | `/institution/news/:id` | Get news    |
| POST   | `/institution/news`     | Create news |
| PUT    | `/institution/news/:id` | Update news |
| DELETE | `/institution/news/:id` | Delete news |

#### Blogs

| Method | Path                     | Description |
| ------ | ------------------------ | ----------- |
| GET    | `/institution/blogs`     | List blogs  |
| GET    | `/institution/blogs/:id` | Get blog    |
| POST   | `/institution/blogs`     | Create blog |
| PUT    | `/institution/blogs/:id` | Update blog |
| DELETE | `/institution/blogs/:id` | Delete blog |

#### QMS (Quality Management)

| Method | Path                   | Description       |
| ------ | ---------------------- | ----------------- |
| GET    | `/institution/qms`     | List QMS records  |
| GET    | `/institution/qms/:id` | Get QMS record    |
| POST   | `/institution/qms`     | Create QMS record |
| PUT    | `/institution/qms/:id` | Update QMS record |
| DELETE | `/institution/qms/:id` | Delete QMS record |

#### Messaging

| Method | Path                             | Description      |
| ------ | -------------------------------- | ---------------- |
| GET    | `/institution/messages`          | List messages    |
| GET    | `/institution/messages/:id`      | Get message      |
| POST   | `/institution/messages`          | Send message     |
| GET    | `/institution/messages/students` | Student contacts |

#### Scholarships

| Method | Path                            | Description        |
| ------ | ------------------------------- | ------------------ |
| GET    | `/institution/scholarships`     | List scholarships  |
| POST   | `/institution/scholarships`     | Create scholarship |
| PUT    | `/institution/scholarships/:id` | Update scholarship |
| DELETE | `/institution/scholarships/:id` | Delete scholarship |

#### Admissions Management

| Method | Path                                 | Description             |
| ------ | ------------------------------------ | ----------------------- |
| GET    | `/institution/admissions`            | List admissions         |
| PUT    | `/institution/admissions/:id/status` | Update admission status |
| POST   | `/institution/admission-pages`       | Create admission page   |
| GET    | `/institution/admission-pages`       | List admission pages    |
| GET    | `/institution/admission-pages/:id`   | Get admission page      |
| PUT    | `/institution/admission-pages/:id`   | Update admission page   |
| DELETE | `/institution/admission-pages/:id`   | Delete admission page   |

#### Scholarship Applications

| Method | Path                                               | Description       |
| ------ | -------------------------------------------------- | ----------------- |
| GET    | `/institution/scholarship-applications`            | List applications |
| PUT    | `/institution/scholarship-applications/:id/status` | Update status     |

#### Upload

| Method | Path                  | Description |
| ------ | --------------------- | ----------- |
| POST   | `/institution/upload` | Upload file |

#### Profile Access

| Method | Path                           | Description            |
| ------ | ------------------------------ | ---------------------- |
| GET    | `/institutions/profile-access` | Get own profile access |

---

### 7.4 Scholarship Provider Portal (authMW + roleMW: scholarship_provider)

#### Dashboard & Analytics

| Method | Path                                        | Description        |
| ------ | ------------------------------------------- | ------------------ |
| GET    | `/scholarship-providers/dashboard`          | Dashboard stats    |
| GET    | `/scholarship-providers/analytics`          | Analytics          |
| GET    | `/scholarship-providers/analytics/detailed` | Detailed analytics |

#### Profile & Settings

| Method | Path                                            | Description        |
| ------ | ----------------------------------------------- | ------------------ |
| GET    | `/scholarship-providers/profile`                | Get profile        |
| PUT    | `/scholarship-providers/profile`                | Update profile     |
| PUT    | `/scholarship-providers/change-password`        | Change password    |
| PUT    | `/scholarship-providers/change-email`           | Change email       |
| GET    | `/scholarship-providers/settings`               | Get settings       |
| PUT    | `/scholarship-providers/settings`               | Update settings    |
| GET    | `/scholarship-providers/notifications`          | List notifications |
| PUT    | `/scholarship-providers/notifications/:id/read` | Mark read          |
| PUT    | `/scholarship-providers/notifications/read-all` | Mark all read      |

#### Scholarship Management

| Method | Path                                      | Description        |
| ------ | ----------------------------------------- | ------------------ |
| POST   | `/scholarship-providers/scholarships`     | Create scholarship |
| GET    | `/scholarship-providers/scholarships`     | List scholarships  |
| GET    | `/scholarship-providers/scholarships/:id` | Get scholarship    |
| PUT    | `/scholarship-providers/scholarships/:id` | Update scholarship |
| DELETE | `/scholarship-providers/scholarships/:id` | Delete scholarship |

#### Application Management

| Method | Path                                                        | Description          |
| ------ | ----------------------------------------------------------- | -------------------- |
| GET    | `/scholarship-providers/applications`                       | List applications    |
| GET    | `/scholarship-providers/applications/pending-payment`       | Pending payment      |
| GET    | `/scholarship-providers/applications/export`                | Export applications  |
| GET    | `/scholarship-providers/applications/export-filtered`       | Filtered export      |
| GET    | `/scholarship-providers/applications/:id`                   | Get application      |
| PUT    | `/scholarship-providers/applications/:id/evaluate`          | Evaluate application |
| PUT    | `/scholarship-providers/applications/:id/status`            | Update status        |
| PUT    | `/scholarship-providers/applications/:id/payment`           | Update payment       |
| PUT    | `/scholarship-providers/applications/:id/dispute-status`    | Dispute status       |
| PUT    | `/scholarship-providers/applications/:id/resend-admit-card` | Resend admit card    |

#### Interviews

| Method | Path                                    | Description        |
| ------ | --------------------------------------- | ------------------ |
| GET    | `/scholarship-providers/interviews`     | List interviews    |
| POST   | `/scholarship-providers/interviews`     | Schedule interview |
| PUT    | `/scholarship-providers/interviews/:id` | Update interview   |

#### Messaging

| Method | Path                                        | Description       |
| ------ | ------------------------------------------- | ----------------- |
| GET    | `/scholarship-providers/messages`           | List messages     |
| POST   | `/scholarship-providers/messages`           | Send message      |
| GET    | `/scholarship-providers/messages/:id`       | Get message       |
| PUT    | `/scholarship-providers/messages/:id/read`  | Mark read         |
| POST   | `/scholarship-providers/messages/from-user` | Message from user |

#### Users & Access Control

| Method | Path                                                          | Description             |
| ------ | ------------------------------------------------------------- | ----------------------- |
| GET    | `/scholarship-providers/users/:id`                            | Get user detail         |
| POST   | `/scholarship-providers/access`                               | Create access role      |
| GET    | `/scholarship-providers/access`                               | List access roles       |
| GET    | `/scholarship-providers/access/:id`                           | Get access role         |
| PUT    | `/scholarship-providers/access/:id`                           | Update access role      |
| DELETE | `/scholarship-providers/access/:id`                           | Delete access role      |
| POST   | `/scholarship-providers/auth/access-users`                    | Create sub-user         |
| GET    | `/scholarship-providers/auth/access-users`                    | List sub-users          |
| GET    | `/scholarship-providers/auth/access-users/:id`                | Get sub-user            |
| PUT    | `/scholarship-providers/auth/access-users/:id`                | Update sub-user         |
| DELETE | `/scholarship-providers/auth/access-users/:id`                | Delete sub-user         |
| PUT    | `/scholarship-providers/auth/access-users/:id/permissions`    | Update permissions      |
| PUT    | `/scholarship-providers/auth/access-users/:id/reset-password` | Reset sub-user password |

#### Content Management (News, Events, Blogs)

| Method              | Path                                  | Description |
| ------------------- | ------------------------------------- | ----------- |
| POST/GET/PUT/DELETE | `/scholarship-providers/news[/:id]`   | Full CRUD   |
| POST/GET/PUT/DELETE | `/scholarship-providers/events[/:id]` | Full CRUD   |
| POST/GET/PUT/DELETE | `/scholarship-providers/blogs[/:id]`  | Full CRUD   |

#### Services, Sectors, Projects

| Method              | Path                                    | Description |
| ------------------- | --------------------------------------- | ----------- |
| POST/GET/PUT/DELETE | `/scholarship-providers/services[/:id]` | Full CRUD   |
| POST/GET/PUT/DELETE | `/scholarship-providers/sectors[/:id]`  | Full CRUD   |
| POST/GET/PUT/DELETE | `/scholarship-providers/projects[/:id]` | Full CRUD   |

#### Gallery & Reviews

| Method              | Path                                   | Description |
| ------------------- | -------------------------------------- | ----------- |
| POST/GET/PUT/DELETE | `/scholarship-providers/gallery[/:id]` | Full CRUD   |
| POST/GET/PUT/DELETE | `/scholarship-providers/reviews[/:id]` | Full CRUD   |

#### Volunteers

| Method              | Path                                                             | Description                |
| ------------------- | ---------------------------------------------------------------- | -------------------------- |
| POST/GET/PUT/DELETE | `/scholarship-providers/volunteers[/:id]`                        | Full CRUD                  |
| PUT                 | `/scholarship-providers/volunteers/:id/toggle`                   | Toggle status              |
| GET                 | `/scholarship-providers/volunteers/:id/applications`             | Get volunteer applications |
| GET                 | `/scholarship-providers/volunteers/applications`                 | All volunteer applications |
| PUT                 | `/scholarship-providers/volunteers/applications/:id/shortlist`   | Shortlist                  |
| PUT                 | `/scholarship-providers/volunteers/applications/:id/unshortlist` | Unshortlist                |
| PUT                 | `/scholarship-providers/volunteers/applications/:id/reject`      | Reject                     |

#### Calendar Events

| Method              | Path                                           | Description |
| ------------------- | ---------------------------------------------- | ----------- |
| POST/GET/PUT/DELETE | `/scholarship-providers/calendar-events[/:id]` | Full CRUD   |

#### Results & Written Exams

| Method              | Path                                                            | Description    |
| ------------------- | --------------------------------------------------------------- | -------------- |
| POST/GET/PUT/DELETE | `/scholarship-providers/results[/:id]`                          | Full CRUD      |
| POST/GET/PUT/DELETE | `/scholarship-providers/written-exams[/:id]`                    | Full CRUD      |
| GET                 | `/scholarship-providers/written-exams/:id/results`              | Exam results   |
| GET                 | `/scholarship-providers/written-exams/:id/results/export`       | Export results |
| GET                 | `/scholarship-providers/written-exams/:id/filter-options`       | Filter options |
| POST                | `/scholarship-providers/written-exams/:id/results`              | Add result     |
| PUT                 | `/scholarship-providers/written-exams/:id/results/:resultId`    | Update result  |
| DELETE              | `/scholarship-providers/written-exams/:id/results/:resultId`    | Delete result  |
| POST                | `/scholarship-providers/written-exams/:id/results/batch-import` | Batch import   |

#### Uploads

| Method | Path                                      | Description     |
| ------ | ----------------------------------------- | --------------- |
| POST   | `/scholarship-providers/uploads`          | Upload file     |
| POST   | `/scholarship-providers/uploads/document` | Upload document |

#### Provider Payment Approval

| Method | Path                              | Description     |
| ------ | --------------------------------- | --------------- |
| POST   | `/providers/payments/:id/approve` | Approve payment |

---

### 7.5 Superadmin Endpoints (authMW + superadminOnly)

#### Dashboard & Users

| Method | Path                              | Description                |
| ------ | --------------------------------- | -------------------------- |
| GET    | `/superadmin/dashboard/stats`     | Dashboard statistics       |
| GET    | `/superadmin/users`               | List all users (paginated) |
| GET    | `/superadmin/users/:id`           | Get user detail            |
| GET    | `/superadmin/users/:id/education` | Get user education entries |
| PUT    | `/superadmin/users/:id/suspend`   | Suspend student user       |
| PUT    | `/superadmin/users/:id/reinstate` | Reinstate student user     |

#### Scholarship Provider Management

| Method | Path                            | Description             |
| ------ | ------------------------------- | ----------------------- |
| GET    | `/superadmin/pending-providers` | Pending providers       |
| GET    | `/superadmin/providers`         | Verified providers      |
| POST   | `/superadmin/providers/approve` | Approve/reject provider |

#### Institution Management

| Method | Path                                     | Description                                                |
| ------ | ---------------------------------------- | ---------------------------------------------------------- |
| GET    | `/superadmin/pending-institutions`       | Pending institutions (filter: `?type=registration\|claim`) |
| GET    | `/superadmin/institutions`               | Verified institutions                                      |
| GET    | `/superadmin/rejected-institutions`      | Rejected institutions                                      |
| POST   | `/superadmin/institutions`               | Create institution                                         |
| GET    | `/superadmin/institutions/:id`           | Get institution detail                                     |
| POST   | `/superadmin/institutions/approve`       | Approve/reject institution                                 |
| POST   | `/superadmin/institutions/claim-approve` | Approve claim request                                      |
| POST   | `/superadmin/institutions/claim-reject`  | Reject claim request                                       |
| PUT    | `/superadmin/institutions/:id`           | Update institution                                         |
| PUT    | `/superadmin/institutions/:id/access`    | Update profile access                                      |
| PUT    | `/superadmin/institutions/:id/payment`   | Record payment                                             |
| PUT    | `/superadmin/institutions/:id/verify`    | Toggle verified                                            |
| PUT    | `/superadmin/institutions/:id/feature`   | Toggle featured                                            |
| PUT    | `/superadmin/institutions/:id/suspend`   | Suspend institution                                        |
| DELETE | `/superadmin/institutions/:id`           | Delete institution                                         |
| POST   | `/superadmin/upload`                     | Upload file                                                |

#### College CRUD (admin)

| Method | Path                           | Description          |
| ------ | ------------------------------ | -------------------- |
| GET    | `/admin/colleges`              | List colleges        |
| GET    | `/admin/colleges/:id`          | Get college          |
| POST   | `/admin/colleges`              | Create college       |
| POST   | `/admin/colleges/upload-image` | Upload college image |
| PUT    | `/admin/colleges/:id`          | Update college       |
| DELETE | `/admin/colleges/:id`          | Delete college       |
| PUT    | `/admin/colleges/:id/approve`  | Approve college      |
| PUT    | `/admin/colleges/:id/featured` | Toggle featured      |

#### University CRUD (admin)

| Method | Path                      | Description       |
| ------ | ------------------------- | ----------------- |
| GET    | `/admin/universities`     | List universities |
| GET    | `/admin/universities/:id` | Get university    |
| POST   | `/admin/universities`     | Create university |
| PUT    | `/admin/universities/:id` | Update university |
| DELETE | `/admin/universities/:id` | Delete university |

#### Scholarship CRUD (admin)

| Method | Path                       | Description        |
| ------ | -------------------------- | ------------------ |
| GET    | `/admin/scholarships`      | List scholarships  |
| GET    | `/admin/scholarships/list` | List (extended)    |
| GET    | `/admin/scholarships/:id`  | Get scholarship    |
| POST   | `/admin/scholarships`      | Create scholarship |
| PUT    | `/admin/scholarships/:id`  | Update scholarship |
| DELETE | `/admin/scholarships/:id`  | Delete scholarship |

#### Scholarship Application Management (admin)

| Method | Path                                                         | Description           |
| ------ | ------------------------------------------------------------ | --------------------- |
| GET    | `/admin/scholarship-applications`                            | List all applications |
| GET    | `/admin/scholarship-applications/:id`                        | Get application       |
| PUT    | `/admin/scholarship-applications/:id/status`                 | Update status         |
| GET    | `/admin/scholarship-applications/scholarship/:scholarshipId` | By scholarship        |

#### Admission Management (admin)

| Method | Path                                   | Description     |
| ------ | -------------------------------------- | --------------- |
| GET    | `/admin/admissions`                    | List admissions |
| GET    | `/admin/admissions/:id`                | Get admission   |
| PUT    | `/admin/admissions/:id/status`         | Update status   |
| GET    | `/admin/admissions/college/:collegeId` | By college      |

#### Contact Inquiries (admin)

| Method | Path                          | Description    |
| ------ | ----------------------------- | -------------- |
| GET    | `/admin/inquiries`            | List inquiries |
| GET    | `/admin/inquiries/:id`        | Get inquiry    |
| PUT    | `/admin/inquiries/:id/status` | Update status  |
| DELETE | `/admin/inquiries/:id`        | Delete inquiry |

#### Ad Management (admin)

| Method | Path                   | Description |
| ------ | ---------------------- | ----------- |
| GET    | `/admin/ads`           | List ads    |
| GET    | `/admin/ads/:id`       | Get ad      |
| POST   | `/admin/ads`           | Create ad   |
| PUT    | `/admin/ads/:id`       | Update ad   |
| DELETE | `/admin/ads/:id`       | Delete ad   |
| POST   | `/admin/ads/:id/click` | Track click |

#### Carousel Management (admin)

| Method | Path                       | Description    |
| ------ | -------------------------- | -------------- |
| GET    | `/admin/carousels`         | List slides    |
| GET    | `/admin/carousels/:id`     | Get slide      |
| POST   | `/admin/carousels`         | Create slide   |
| PUT    | `/admin/carousels/:id`     | Update slide   |
| DELETE | `/admin/carousels/:id`     | Delete slide   |
| PUT    | `/admin/carousels/reorder` | Reorder slides |

#### Content Management (admin)

| Method | Path                        | Description       |
| ------ | --------------------------- | ----------------- |
| GET    | `/admin/blogs`              | List blogs        |
| POST   | `/admin/blogs`              | Create blog       |
| PUT    | `/admin/blogs/:id`          | Update blog       |
| DELETE | `/admin/blogs/:id`          | Delete blog       |
| POST   | `/admin/blogs/upload-image` | Upload blog image |
| GET    | `/admin/events`             | List events       |
| GET    | `/admin/events/:id`         | Get event         |
| POST   | `/admin/events`             | Create event      |
| PUT    | `/admin/events/:id`         | Update event      |
| DELETE | `/admin/events/:id`         | Delete event      |
| PUT    | `/admin/events/:id/feature` | Toggle featured   |
| GET    | `/admin/news`               | List news         |
| GET    | `/admin/news/:id`           | Get news          |
| POST   | `/admin/news`               | Create news       |
| PUT    | `/admin/news/:id`           | Update news       |
| DELETE | `/admin/news/:id`           | Delete news       |
| POST   | `/admin/news/upload-image`  | Upload news image |

#### Payment Management (admin)

| Method | Path                               | Description           |
| ------ | ---------------------------------- | --------------------- |
| POST   | `/admin/payments/verify-esewa`     | Verify eSewa payment  |
| POST   | `/admin/payments/send-admit-cards` | Send bulk admit cards |

#### Search Reindex (admin)

| Method | Path                    | Description               |
| ------ | ----------------------- | ------------------------- |
| POST   | `/admin/search/reindex` | Regenerate all embeddings |

#### Project Shiksha (admin)

| Method | Path                                             | Description       |
| ------ | ------------------------------------------------ | ----------------- |
| GET    | `/admin/project-shiksha/applications`            | List applications |
| PUT    | `/admin/project-shiksha/applications/:id/status` | Update status     |
| DELETE | `/admin/project-shiksha/applications/:id`        | Delete            |
| POST   | `/admin/project-shiksha/payments/verify`         | Verify payment    |
| GET    | `/admin/project-shiksha/stats`                   | Statistics        |

---

## 8. Key Data Models

### User Models (3 types stored in separate tables)

| Table                        | Primary Role | Fields                                                                                                                                                                                                                                                                |
| ---------------------------- | ------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `users`                      | Student      | email, password, first_name, last_name, phone, dob, gender, nationality, address, bio, google_id, image_url, role, status, preferences (JSONB)                                                                                                                        |
| `institution_users`          | Institution  | institution_name, registration_number, email, password, google_id, role, status, contact info, address fields, org_type, pan_number, website, logo/banner/about/vision/mission, college_id, claimed, verified, featured, profile_data (JSONB), profile_access (JSONB) |
| `scholarship_provider_users` | Provider     | provider_name, registration_number, email, password, google_id, role, status, contact, pan, website, logo, address, about, mission, values, founder fields, social links, brochure, banner                                                                            |

### Scholarship Models

| Table                      | Key Fields                                                                                                                     |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `scholarships`             | title, description, provider, type, amount, deadline, eligibility, exam_date, status, slug, institution_id, embedding (vector) |
| `scholarship_applications` | user_id (nullable for anonymous), scholarship_id, form_data (JSONB), status, roll_number, payment_status, created_at           |
| `payments`                 | scholarship_application_id, amount, method (esewa/bank/etc.), status, transaction_id, paid_at, approved_at                     |
| `provider_scholarships`    | provider_id, title, description, type, amount, deadline, eligibility, exam_date, status, slug, embedding (vector)              |
| `provider_applications`    | user_id (nullable), scholarship_id, provider_id, form_data, status, roll_number, payment_status, see_gpa                       |

### College Model

| Table      | Key Fields                                                                                                                                                                                                                                                   |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `colleges` | name, full_name, slug, description, location, affiliation, college_type, website, email, phone, image_url, rating, featured, verified, university_id, admission_details, courses_offered, facilities, scholarships (JSONB), social_links, embedding (vector) |

### Forum Models

| Table               | Key Fields                                                                                                  |
| ------------------- | ----------------------------------------------------------------------------------------------------------- |
| `forum_posts`       | user_id, community_id, title, content, type (discussion/question/poll), poll_data (JSONB), tags, view_count |
| `forum_comments`    | post_id, user_id, content, parent_id                                                                        |
| `forum_communities` | name, description, icon, member_count, post_count                                                           |
| `forum_votes`       | post_id, user_id, vote_type (like/dislike)                                                                  |
| `forum_saves`       | post_id, user_id                                                                                            |
| `forum_poll_votes`  | post_id, user_id, selected_option                                                                           |

---

## 9. Test Scenarios by Feature

### 9.1 Authentication

| TC# | Scenario                                   | Steps                                                                     | Expected                   |
| --- | ------------------------------------------ | ------------------------------------------------------------------------- | -------------------------- |
| A01 | Student email registration                 | POST `/auth/register` with valid data                                     | 201, requires_otp: true    |
| A02 | Duplicate email registration               | POST `/auth/register` with same email                                     | 409 error                  |
| A03 | Send verification OTP                      | POST `/auth/send-otp` with registered email                               | 200, OTP logged in console |
| A04 | Verify OTP and create user                 | POST `/auth/verify-otp` with correct OTP                                  | 200, JWT returned          |
| A05 | Verify with wrong OTP                      | POST `/auth/verify-otp` with wrong OTP                                    | 400 error                  |
| A06 | Login with correct credentials             | POST `/auth/login`                                                        | 200, JWT + user data       |
| A07 | Login with wrong password                  | POST `/auth/login` wrong password                                         | 401 error                  |
| A08 | Login for suspended user                   | Suspend user, then login                                                  | 401, "suspended" message   |
| A09 | Access protected endpoint without token    | GET `/profile` without auth header                                        | 401                        |
| A10 | Access protected endpoint with valid token | GET `/profile` with Bearer token                                          | 200, user profile          |
| A11 | Access admin endpoint as student           | GET `/admin/colleges` with student token                                  | 403                        |
| A12 | Password reset flow                        | send-otp (type=password_reset) → reset-password → login with new password | 200 all steps              |
| A13 | Google OAuth initiation                    | GET `/auth/google`                                                        | 307 redirect to Google     |
| A14 | Institution registration                   | POST `/institutions/auth/register`                                        | 201, awaits admin approval |
| A15 | Institution login before approval          | POST `/institutions/auth/login`                                           | 401, "under review"        |
| A16 | Provider registration                      | POST `/scholarship-providers/auth/register`                               | 201, awaits admin approval |
| A17 | Superadmin register with wrong access code | POST `/superadmin/auth/register` with wrong code                          | 403 error                  |

### 9.2 Student Features

| TC# | Scenario                   | Steps                                          | Expected                   |
| --- | -------------------------- | ---------------------------------------------- | -------------------------- |
| S01 | Get profile                | GET `/profile`                                 | 200, user data             |
| S02 | Update profile             | PUT `/profile` with fields                     | 200, updated data          |
| S03 | Upload profile picture     | POST `/auth/profile/picture` with image file   | 200, image_url in response |
| S04 | Change password            | PUT `/auth/change-password` with current + new | 200                        |
| S05 | Save preferences           | POST `/preferences`                            | 200                        |
| S06 | Add education entry        | POST `/profile/education`                      | 201                        |
| S07 | List education entries     | GET `/profile/education`                       | 200                        |
| S08 | Create admission           | POST `/admissions`                             | 201                        |
| S09 | List my admissions         | GET `/admissions/my`                           | 200                        |
| S10 | Create counselling booking | POST `/counselling/bookings`                   | 201                        |
| S11 | My counselling bookings    | GET `/counselling/bookings/my`                 | 200                        |
| S12 | Book counselling session   | POST `/counselling/sessions/book`              | 201                        |
| S13 | Dashboard stats            | GET `/dashboard/stats`                         | 200                        |

### 9.3 Scholarships

| TC#  | Scenario                             | Steps                                                 | Expected             |
| ---- | ------------------------------------ | ----------------------------------------------------- | -------------------- |
| SC01 | List public scholarships             | GET `/education/scholarships`                         | 200, paginated list  |
| SC02 | Get scholarship detail               | GET `/education/scholarships/:id`                     | 200                  |
| SC03 | Get similar scholarships             | GET `/education/scholarships/:id/similar`             | 200                  |
| SC04 | Apply to scholarship (authenticated) | POST `/education/scholarships/:id/apply` with auth    | 201                  |
| SC05 | Apply to scholarship (anonymous)     | POST `/education/scholarships/:id/apply` without auth | 201, user_id is null |
| SC06 | List my applications                 | GET `/scholarships/my-applications`                   | 200                  |
| SC07 | Update application                   | PUT `/scholarships/applications/:id`                  | 200                  |
| SC08 | Delete application                   | DELETE `/scholarships/applications/:id`               | 200                  |
| SC09 | Payment initiation                   | POST `/scholarships/:id/pay`                          | 200                  |
| SC10 | eSewa verify                         | POST `/scholarships/pay/esewa/verify`                 | 200                  |

### 9.4 Forum

| TC# | Scenario           | Steps                              | Expected |
| --- | ------------------ | ---------------------------------- | -------- |
| F01 | List posts         | GET `/forum/posts`                 | 200      |
| F02 | Get communities    | GET `/forum/communities`           | 200      |
| F03 | Create post        | POST `/forum/posts` with auth      | 201      |
| F04 | Update post        | PUT `/forum/posts/:id`             | 200      |
| F05 | Delete post        | DELETE `/forum/posts/:id`          | 200      |
| F06 | Like post          | POST `/forum/posts/:id/like`       | 200      |
| F07 | Dislike post       | POST `/forum/posts/:id/dislike`    | 200      |
| F08 | Save post          | POST `/forum/posts/:id/save`       | 200      |
| F09 | Add comment        | POST `/forum/posts/:id/comments`   | 201      |
| F10 | Get comments       | GET `/forum/posts/:id/comments`    | 200      |
| F11 | Join community     | POST `/forum/communities/:id/join` | 200      |
| F12 | Vote in poll       | POST `/forum/posts/:id/poll/vote`  | 200      |
| F13 | Upload forum media | POST `/forum/upload` with file     | 200      |

### 9.5 Institution Portal

| TC# | Scenario                  | Steps                                                       | Expected |
| --- | ------------------------- | ----------------------------------------------------------- | -------- |
| I01 | Register institution      | POST `/institutions/auth/register`                          | 201      |
| I02 | Approve as superadmin     | POST `/superadmin/institutions/approve`                     | 200      |
| I03 | Login after approval      | POST `/institutions/auth/login` with credentials from email | 200      |
| I04 | Dashboard                 | GET `/institution/dashboard`                                | 200      |
| I05 | Create program            | POST `/institution/programs`                                | 201      |
| I06 | List programs             | GET `/institution/programs`                                 | 200      |
| I07 | Update profile            | PUT `/institution/profile`                                  | 200      |
| I08 | Upload media              | POST `/institution/media` with file                         | 201      |
| I09 | Create entrance           | POST `/institution/entrances`                               | 201      |
| I10 | Create event              | POST `/institution/events`                                  | 201      |
| I11 | Create news               | POST `/institution/news`                                    | 201      |
| I12 | Create blog               | POST `/institution/blogs`                                   | 201      |
| I13 | Create QMS record         | POST `/institution/qms`                                     | 201      |
| I14 | Send message              | POST `/institution/messages`                                | 201      |
| I15 | List counselling bookings | GET `/institution/counselling/bookings`                     | 200      |
| I16 | Update booking status     | PUT `/institution/counselling/bookings/:id/status`          | 200      |
| I17 | Manage scholarships       | POST `/institution/scholarships`                            | 201      |
| I18 | Manage admissions         | GET `/institution/admissions`                               | 200      |
| I19 | Update admission status   | PUT `/institution/admissions/:id/status`                    | 200      |
| I20 | Admission pages CRUD      | POST/GET/PUT/DELETE `/institution/admission-pages`          | 200/201  |

### 9.6 Scholarship Provider Portal

| TC# | Scenario                  | Steps                                                                | Expected        |
| --- | ------------------------- | -------------------------------------------------------------------- | --------------- |
| P01 | Register provider         | POST `/scholarship-providers/auth/register`                          | 201             |
| P02 | Approve as superadmin     | POST `/superadmin/providers/approve`                                 | 200             |
| P03 | Login after approval      | POST `/scholarship-providers/auth/login`                             | 200             |
| P04 | Dashboard                 | GET `/scholarship-providers/dashboard`                               | 200             |
| P05 | Create scholarship        | POST `/scholarship-providers/scholarships`                           | 201             |
| P06 | List scholarships         | GET `/scholarship-providers/scholarships`                            | 200             |
| P07 | View applications         | GET `/scholarship-providers/applications`                            | 200             |
| P08 | Evaluate application      | PUT `/scholarship-providers/applications/:id/evaluate`               | 200             |
| P09 | Update application status | PUT `/scholarship-providers/applications/:id/status`                 | 200             |
| P10 | Schedule interview        | POST `/scholarship-providers/interviews`                             | 201             |
| P11 | Create news               | POST `/scholarship-providers/news`                                   | 201             |
| P12 | Create event              | POST `/scholarship-providers/events`                                 | 201             |
| P13 | Create blog               | POST `/scholarship-providers/blogs`                                  | 201             |
| P14 | Create written exam       | POST `/scholarship-providers/written-exams`                          | 201             |
| P15 | Add exam result           | POST `/scholarship-providers/written-exams/:id/results`              | 201             |
| P16 | Batch import results      | POST `/scholarship-providers/written-exams/:id/results/batch-import` | 200             |
| P17 | Create sub-user           | POST `/scholarship-providers/auth/access-users`                      | 201             |
| P18 | Sub-user login            | POST `/scholarship-providers/auth/access-login`                      | 200             |
| P19 | Volunteer management      | POST `/scholarship-providers/volunteers`                             | 201             |
| P20 | Export applications       | GET `/scholarship-providers/applications/export`                     | 200 (CSV/Excel) |

### 9.7 Superadmin

| TC#  | Scenario                   | Steps                                      | Expected              |
| ---- | -------------------------- | ------------------------------------------ | --------------------- |
| SA01 | Dashboard stats            | GET `/superadmin/dashboard/stats`          | 200                   |
| SA02 | List users                 | GET `/superadmin/users`                    | 200, paginated        |
| SA03 | Suspend user               | PUT `/superadmin/users/:id/suspend`        | 200                   |
| SA04 | Reinstate user             | PUT `/superadmin/users/:id/reinstate`      | 200                   |
| SA05 | List pending providers     | GET `/superadmin/pending-providers`        | 200                   |
| SA06 | Approve provider           | POST `/superadmin/providers/approve`       | 200, email sent       |
| SA07 | List pending institutions  | GET `/superadmin/pending-institutions`     | 200                   |
| SA08 | Approve institution        | POST `/superadmin/institutions/approve`    | 200, email sent       |
| SA09 | Verify institution         | PUT `/superadmin/institutions/:id/verify`  | 200, toggles verified |
| SA10 | Feature institution        | PUT `/superadmin/institutions/:id/feature` | 200, toggles featured |
| SA11 | Record institution payment | PUT `/superadmin/institutions/:id/payment` | 200                   |
| SA12 | Create college             | POST `/admin/colleges`                     | 201                   |
| SA13 | Create university          | POST `/admin/universities`                 | 201                   |
| SA14 | Create scholarship         | POST `/admin/scholarships`                 | 201                   |
| SA15 | Manage ads                 | POST `/admin/ads`                          | 201                   |
| SA16 | Manage carousels           | POST `/admin/carousels`                    | 201                   |
| SA17 | Reorder carousels          | PUT `/admin/carousels/reorder`             | 200                   |
| SA18 | List inquiries             | GET `/admin/inquiries`                     | 200                   |
| SA19 | Update inquiry status      | PUT `/admin/inquiries/:id/status`          | 200                   |
| SA20 | Send bulk admit cards      | POST `/admin/payments/send-admit-cards`    | 200                   |

### 9.8 Search

| TC# | Scenario        | Steps                                    | Expected                         |
| --- | --------------- | ---------------------------------------- | -------------------------------- |
| Q01 | Keyword search  | GET `/search?q=engineering`              | 200, results from all categories |
| Q02 | Category filter | GET `/search?q=engineering&cat=colleges` | 200, only colleges               |
| Q03 | Pagination      | GET `/search?q=college&page=1&limit=5`   | 200, pagination meta             |
| Q04 | Empty query     | GET `/search?q=`                         | 200, all items                   |
| Q05 | Vector status   | GET `/search/vector-status`              | 200, shows enabled state         |

### 9.9 Reviews & Feedback

| TC# | Scenario            | Steps                                       | Expected                |
| --- | ------------------- | ------------------------------------------- | ----------------------- |
| R01 | Submit review       | POST `/user/reviews` with rating            | 201                     |
| R02 | List my reviews     | GET `/user/reviews`                         | 200                     |
| R03 | Update review       | PUT `/user/reviews/:id`                     | 200                     |
| R04 | Delete review       | DELETE `/user/reviews/:id`                  | 200                     |
| R05 | Mark helpful        | POST `/education/reviews/:id/helpful`       | 200, prevents duplicate |
| R06 | Report review       | POST `/user/reviews/:id/report`             | 200                     |
| R07 | Get college reviews | GET `/education/reviews/college/:collegeId` | 200                     |
| R08 | Submit feedback     | POST `/feedback`                            | 200                     |
| R09 | Submit testimonial  | POST `/public/testimonials`                 | 200                     |

### 9.10 System

| TC# | Scenario               | Steps                                | Expected |
| --- | ---------------------- | ------------------------------------ | -------- |
| T01 | Submit contact inquiry | POST `/system/contact`               | 200      |
| T02 | Get ads by page        | GET `/system/ads?page=landing`       | 200      |
| T03 | Track ad click         | POST `/system/ads/:id/click`         | 200      |
| T04 | Get carousels by page  | GET `/system/carousels?page=landing` | 200      |
| T05 | Public notifications   | GET `/system/notifications`          | 200      |

### 9.11 Project Shiksha

| TC# | Scenario                | Steps                                                  | Expected                  |
| --- | ----------------------- | ------------------------------------------------------ | ------------------------- |
| J01 | Submit application      | POST `/project-shiksha/applications`                   | 201, roll_number returned |
| J02 | Get application by ID   | GET `/project-shiksha/applications/:id`                | 200                       |
| J03 | Get by roll number      | GET `/project-shiksha/applications/roll-number/:rn`    | 200                       |
| J04 | Get admit card          | GET `/project-shiksha/applications/admit-card/:rn`     | 200 (PDF)                 |
| J05 | eSewa initiate          | POST `/project-shiksha/payments/esewa/initiate`        | 200                       |
| J06 | Admin list applications | GET `/admin/project-shiksha/applications` (superadmin) | 200                       |
| J07 | Admin stats             | GET `/admin/project-shiksha/stats` (superadmin)        | 200                       |

### 9.12 Education Content

| TC# | Scenario             | Steps                                                    | Expected       |
| --- | -------------------- | -------------------------------------------------------- | -------------- |
| C01 | List courses         | GET `/education/courses`                                 | 200, paginated |
| C02 | Course filter counts | GET `/education/courses/filter-counts`                   | 200            |
| C03 | Course details       | GET `/education/courses/:id/details`                     | 200            |
| C04 | List exams           | GET `/education/exams`                                   | 200            |
| C05 | List news            | GET `/education/news`                                    | 200            |
| C06 | List events          | GET `/education/events`                                  | 200            |
| C07 | List blogs           | GET `/education/blogs` (with `?search=&category=&page=`) | 200            |
| C08 | Track blog view      | POST `/education/blogs/:id/view`                         | 200            |
| C09 | List entrances       | GET `/entrances`                                         | 200            |
| C10 | Get rankings         | GET `/education/rankings`                                | 200            |

### 9.13 Messaging (Student)

| TC# | Scenario           | Steps                      | Expected |
| --- | ------------------ | -------------------------- | -------- |
| M01 | Send message       | POST `/messages`           | 201      |
| M02 | List conversations | GET `/messages`            | 200      |
| M03 | Get message thread | GET `/messages/:id`        | 200      |
| M04 | Reply to message   | POST `/messages/:id/reply` | 201      |
| M05 | Get contacts       | GET `/messages/contacts`   | 200      |

### 9.14 Calendar (Student)

| TC#  | Scenario     | Steps                         | Expected |
| ---- | ------------ | ----------------------------- | -------- |
| CL01 | Create event | POST `/calendar/events`       | 201      |
| CL02 | List events  | GET `/calendar/events`        | 200      |
| CL03 | Update event | PUT `/calendar/events/:id`    | 200      |
| CL04 | Delete event | DELETE `/calendar/events/:id` | 200      |

### 9.15 Invites & Bookmarks

| TC# | Scenario          | Steps                      | Expected |
| --- | ----------------- | -------------------------- | -------- |
| B01 | Create bookmark   | POST `/bookmarks`          | 201      |
| B02 | List bookmarks    | GET `/bookmarks`           | 200      |
| B03 | Bookmarks by type | GET `/bookmarks/:type`     | 200      |
| B04 | Delete bookmark   | DELETE `/bookmarks/:id`    | 200      |
| B05 | List invites      | GET `/invites`             | 200      |
| B06 | Accept invite     | PUT `/invites/:id/accept`  | 200      |
| B07 | Decline invite    | PUT `/invites/:id/decline` | 200      |

### 9.16 AI & Chat

| TC#  | Scenario       | Steps                                           | Expected        |
| ---- | -------------- | ----------------------------------------------- | --------------- |
| AI01 | Chat with AI   | POST `/ai/chat` with `{ message, session_id? }` | 200, SSE stream |
| AI02 | List AI models | GET `/ai/models`                                | 200             |
| AI03 | General chat   | POST `/chat`                                    | 200             |

### 9.17 College/University

| TC# | Scenario                | Steps                          | Expected |
| --- | ----------------------- | ------------------------------ | -------- |
| U01 | List universities       | GET `/universities`            | 200      |
| U02 | University detail       | GET `/universities/:id`        | 200      |
| U03 | University tab data     | GET `/universities/:id/:tab`   | 200      |
| U04 | List colleges           | GET `/colleges` (with filters) | 200      |
| U05 | Featured colleges       | GET `/colleges/featured`       | 200      |
| U06 | College detail          | GET `/colleges/:id`            | 200      |
| U07 | College recommendations | POST `/colleges/recommend`     | 200      |

### 9.18 Provider Public Content

| TC#  | Scenario                | Steps                               | Expected |
| ---- | ----------------------- | ----------------------------------- | -------- |
| PC01 | Provider public profile | GET `/public/providers/:id`         | 200      |
| PC02 | List volunteers         | GET `/public/volunteers`            | 200      |
| PC03 | Apply to volunteer      | POST `/public/volunteers/:id/apply` | 201      |
| PC04 | Check result            | GET `/public/results/check`         | 200      |

---

## 10. Known Issues & Limitations

### Critical

| #   | Issue                                                | Impact                                                                               | Workaround                                                    |
| --- | ---------------------------------------------------- | ------------------------------------------------------------------------------------ | ------------------------------------------------------------- |
| 1   | OTP data stored in-memory (not Redis)                | Registration data lost on server restart between `/register` and `/verify-otp`       | Ensure server does not restart during registration flow       |
| 2   | OAuth state also in-memory                           | OAuth in-flight state lost on restart                                                | Google OAuth will fail with "Invalid state" after restart     |
| 3   | `fixMissingColumns()` grows with every schema change | Raw SQL patches in `main.go` may miss new columns or conflict with model definitions | Reset DB (`make docker-down` + volume rm) to get clean schema |

### High

| #   | Issue                                           | Impact                                                 | Workaround                           |
| --- | ----------------------------------------------- | ------------------------------------------------------ | ------------------------------------ |
| 4   | No rate limiting on auth endpoints              | Brute force possible on login/OTP/reset                | Monitor logs for suspicious activity |
| 5   | No pagination on some GET endpoints             | `/colleges`, `/universities` may return large payloads | Use query filters                    |
| 6   | SMTP failure only logged, not returned to user  | User never receives OTP but gets `200 OK`              | Check server logs for SMTP errors    |
| 7   | `SUPER2026` hardcoded as superadmin access code | Cannot be changed without recompile                    | Set via env var in future            |

### Medium

| #   | Issue                                                              | Impact                                                              | Workaround                                  |
| --- | ------------------------------------------------------------------ | ------------------------------------------------------------------- | ------------------------------------------- |
| 8   | No DB migration tool — uses `AutoMigrate` + raw SQL                | Schema changes may fail on existing data                            | Drop and recreate DB for major changes      |
| 9   | `go.mod` has unused direct dependencies                            | Slightly larger binary                                              | Run `go mod tidy`                           |
| 10  | Institution `ClaimRegister` generates password stored in OTP cache | Password visible in memory, never sent to user until admin approval | —                                           |
| 11  | Some endpoints missing from API_DOCUMENTATION.md                   | QA may miss testing certain features                                | Use the full route listing in this document |
| 12  | `cleanupStates()` goroutine has no context cancellation            | Goroutine leak on graceful shutdown                                 | Minor — only runs while server is up        |

### Low

| #   | Issue                                                                                     | Impact                                                    |
| --- | ----------------------------------------------------------------------------------------- | --------------------------------------------------------- |
| 13  | CORS sets `Access-Control-Allow-Origin` to request origin                                 | Not recommended for production (use explicit domains)     |
| 14  | Password field included in some user JSON responses (though `json:"-"` on Password field) | Check that no endpoint accidentally exposes password hash |
| 15  | SQLite alternative available in code but not fully tested                                 | May have compatibility issues                             |

---

## 11. Troubleshooting Guide

### Server Won't Start

| Symptom                                                | Cause                                | Fix                                                                                      |
| ------------------------------------------------------ | ------------------------------------ | ---------------------------------------------------------------------------------------- |
| `dial tcp 127.0.0.1:5432: connect: connection refused` | PostgreSQL not running               | `make docker-up` or start local PostgreSQL                                               |
| `pq: password authentication failed`                   | Wrong DB credentials                 | Check `.env` DB_USER, DB_PASSWORD                                                        |
| `Failed to migrate database`                           | Schema conflict or missing extension | Reset DB: `make docker-down && docker volume rm backend_postgres_data && make docker-up` |
| `port 8080 already in use`                             | Another process on port              | Change `PORT` in `.env` or kill existing process                                         |

### Authentication Issues

| Symptom                        | Cause                                                  | Fix                                                          |
| ------------------------------ | ------------------------------------------------------ | ------------------------------------------------------------ |
| `401 Authentication required`  | Missing or invalid token                               | Ensure `Authorization: Bearer <token>` header is sent        |
| `403 Insufficient permissions` | Wrong role for endpoint                                | Check JWT `user_role` claim; ensure correct role             |
| `OTP not sent`                 | SMTP not configured                                    | Check server logs — OTP is printed to console in dev mode    |
| `Invalid or expired OTP`       | Wrong OTP or timed out                                 | OTP expires in 10 minutes; check server logs for correct OTP |
| OAuth returns "Invalid state"  | Server restarted between OAuth initiation and callback | Retry OAuth flow from beginning                              |

### Data Issues

| Symptom                          | Cause                                 | Fix                                                                   |
| -------------------------------- | ------------------------------------- | --------------------------------------------------------------------- |
| College/university listing empty | No seed data or DB empty              | Run server with `EMBEDDING_ENABLED=false` — seeder runs automatically |
| Search returns no results        | Empty DB or vectors not generated     | Seed data first, then POST `/admin/search/reindex`                    |
| File upload fails                | MinIO not available                   | Files fall back to local `./uploads/` directory                       |
| Email not sending                | SMTP unreachable or wrong credentials | Check `.env` SMTP settings; use Gmail App Password                    |

### API Testing Tips

```bash
# Get health
curl http://localhost:8080/health

# Register student
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"qa@test.com","password":"Test123!","first_name":"QA","last_name":"Tester"}'

# Get OTP from server logs (or check /send-otp)
# Verify OTP (use the OTP printed in console)
curl -s -X POST http://localhost:8080/api/v1/auth/verify-otp \
  -H "Content-Type: application/json" \
  -d '{"email":"qa@test.com","otp":"123456"}'

# Save token for authenticated requests
TOKEN="<jwt-from-response>"

# Use token in requests
curl -s http://localhost:8080/api/v1/profile \
  -H "Authorization: Bearer $TOKEN"

# Login (simpler — get token directly if email already exists)
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"qa@test.com","password":"Test123!"}'
```
