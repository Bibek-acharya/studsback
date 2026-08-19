# Performance & Load Testing Guide

---

## 1. System Profile

| Aspect              | Detail                                                                           |
| ------------------- | -------------------------------------------------------------------------------- |
| **Framework**       | Gin (sync HTTP, no async event loop)                                             |
| **Database**        | PostgreSQL 16                                                                    |
| **ORM**             | GORM (reflection-based, adds ~10-20% overhead vs raw SQL)                        |
| **Auth**            | JWT (stateless, no session store needed)                                         |
| **File Storage**    | MinIO (S3-compatible, HTTP-based) or local disk                                  |
| **Background Jobs** | Asynq (Redis-backed, goroutine-based workers)                                    |
| **AI/LLM**          | External HTTP call to Ollama/OpenAI                                              |
| **Embedding**       | External HTTP call to OpenAI-compatible API                                      |
| **PDF Generation**  | Chromedp (headless Chromium, heavy per invocation)                               |
| **Server Model**    | One OS thread per request (Go goroutine, but DB queries are blocking via lib/pq) |
| **Build**           | CGO_ENABLED=1 (includes SQLite driver, increases binary size)                    |

### Performance Characteristics

| Operation                   | Typical Latency | Bottleneck               |
| --------------------------- | --------------- | ------------------------ |
| Static route                | <1ms            | None                     |
| Auth (JWT validate)         | <5ms            | CPU (HMAC-SHA256)        |
| Auth (bcrypt verify)        | ~100ms          | CPU (cost factor)        |
| Auth (bcrypt hash)          | ~200ms          | CPU (cost factor)        |
| Simple GET list (no joins)  | 5-20ms          | DB query                 |
| Complex GET with joins      | 20-200ms        | DB query + data size     |
| Search (keyword only)       | 10-50ms         | DB ILIKE scans           |
| Search (vector)             | 50-200ms        | Embedding API + pgvector |
| File upload (local)         | 5-50ms          | Disk I/O                 |
| File upload (MinIO)         | 50-500ms        | Network + MinIO server   |
| Email send (async, enqueue) | <5ms            | Redis push               |
| Email send (sync, SMTP)     | 500ms-5s        | SMTP server              |
| AI Chat (streaming)         | 1-30s           | LLM inference            |
| PDF generation              | 1-10s           | Chromium headless        |
| Embedding generation        | 200-2000ms      | OpenAI API               |
| Bulk search reindex         | 30s-30min       | API call per batch       |

---

## 2. Endpoint Performance Classification

### Tier 1: <10ms Expected (High Throughput)

```
GET  /health
GET  /api/v1/proxy-image
GET  /uploads/*filepath
POST /api/v1/auth/logout
GET  /api/v1/forum/communities
GET  /api/v1/system/ads
GET  /api/v1/system/carousels
GET  /api/v1/system/notifications
POST /api/v1/system/ads/:id/click
GET  /api/v1/search/vector-status
GET  /api/v1/education/rankings
```

**Load target:** 500-1000 RPS per instance

### Tier 2: 10-100ms (Normal)

```
GET  /api/v1/colleges (with pagination)
GET  /api/v1/colleges/featured
GET  /api/v1/colleges/:id
GET  /api/v1/universities
GET  /api/v1/universities/:id
GET  /api/v1/education/scholarships
GET  /api/v1/education/courses
GET  /api/v1/education/exams
GET  /api/v1/education/news
GET  /api/v1/education/events
GET  /api/v1/education/blogs
GET  /api/v1/forum/posts
GET  /api/v1/profile (authed)
GET  /api/v1/search
GET  /api/v1/messages
GET  /api/v1/notifications
GET  /api/v1/messages/contacts
GET  /api/v1/calendar/events
GET  /api/v1/bookmarks
GET  /api/v1/invites
GET  /api/v1/dashboard/stats
GET  /api/v1/institution/dashboard
GET  /api/v1/scholarship-providers/dashboard
GET  /api/v1/studentdashboard (all GET endpoints)
GET  /api/v1/admissions/my
```

**Load target:** 200-500 RPS per instance

### Tier 3: 100-500ms (I/O Heavy)

```
POST /api/v1/education/scholarships/:id/apply  (INSERT + roll_number seq)
POST /api/v1/forum/posts                        (INSERT + count update)
POST /api/v1/forum/posts/:id/like               (INSERT/UPDATE + aggregation)
POST /api/v1/admissions                          (INSERT)
POST /api/v1/counselling/bookings               (INSERT)
POST /api/v1/admissions/:id/pay                 (eSewa API call)
POST /api/v1/colleges/recommend
POST /api/v1/tools/scholarship-finder/recommendations
POST /api/v1/tools/college-recommender/recommendations
POST /api/v1/auth/login                         (bcrypt verify)
POST /api/v1/profile/education                  (INSERT)
PUT  /api/v1/profile                            (UPDATE)
POST /api/v1/auth/register                      (bcrypt hash)
POST /api/v1/auth/send-otp                      (in-memory + optional SMTP enqueue)
POST /api/v1/forum/upload                       (file write)
```

**Load target:** 50-200 RPS per instance

### Tier 4: 500ms-5s (Very Heavy)

```
POST /api/v1/auth/send-otp  (if SMTP sync)      SMTP network call
POST /api/v1/admin/search/reindex               Batch embedding API calls
POST /api/v1/admin/payments/send-admit-cards    Chromium PDF × N + SMTP × N
POST /api/v1/ai/chat                            LLM streaming API call
POST /api/v1/chat                               LLM API call
POST /api/v1/scholarship-providers/applications/export  Large data + CSV generation
POST /api/v1/scholarship-providers/written-exams/:id/results/batch-import  Bulk INSERT
```

**Load target:** 5-20 RPS per instance

---

## 3. Bottleneck Analysis

### 3.1 Database

| Bottleneck                          | Cause                                     | Mitigation                                                 |
| ----------------------------------- | ----------------------------------------- | ---------------------------------------------------------- |
| `ILIKE` on text columns             | Full table scan                           | Add `pg_trgm` GIN indexes on searched columns              |
| `ORDER BY vector_score`             | pgvector index only approximate (IVFFlat) | Tune `lists` parameter; switch to HNSW for better accuracy |
| JSONB field queries                 | No index on JSONB paths                   | Add `GIN` indexes on frequently-queried JSONB paths        |
| Soft-delete (`deleted_at IS NULL`)  | GORM adds this to every query             | Composite indexes on (deleted_at, other_filters)           |
| GORM N+1 queries                    | Missing preload on relations              | Check handlers for repeated single-query loops in services |
| Sequence generation (`roll_number`) | `CREATE SEQUENCE` with default cache      | Already correct; insert pattern matters                    |
| Lock contention on frequent UPDATEs | Forum upvote counts, blog view counts     | Use atomics or background aggregation for counters         |

**Example: Missing indexes to check from your codebase**

```sql
-- These SHOULD exist but may not be auto-created by GORM:
CREATE INDEX IF NOT EXISTS idx_forum_posts_community_id ON forum_posts(community_id);
CREATE INDEX IF NOT EXISTS idx_forum_comments_post_id ON forum_comments(post_id);
CREATE INDEX IF NOT EXISTS idx_scholarship_applications_user_id ON scholarship_applications(user_id);
CREATE INDEX IF NOT EXISTS idx_provider_applications_scholarship_id ON provider_applications(scholarship_id);
CREATE INDEX IF NOT EXISTS idx_colleges_university_id ON colleges(university_id);
CREATE INDEX IF NOT EXISTS idx_institution_programs_institution_id ON institution_programs(institution_id);

-- Trigram indexes for keyword search (if not using pgvector):
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_colleges_name_trgm ON colleges USING gin (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_colleges_location_trgm ON colleges USING gin (location gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_scholarships_title_trgm ON scholarships USING gin (title gin_trgm_ops);
```

### 3.2 Authentication

| Bottleneck             | Cause                    | Mitigation                                                          |
| ---------------------- | ------------------------ | ------------------------------------------------------------------- |
| bcrypt verify (~100ms) | Cost factor 10 (default) | Acceptable for login; cannot reduce without reducing security       |
| bcrypt hash (~200ms)   | Registration             | Acceptable; happens once per user                                   |
| JWT decode             | Base64 + HMAC            | ~1μs, negligible                                                    |
| OTP store              | In-memory map with mutex | Bottleneck under high concurrent registrations. Replace with Redis. |
| OAuth state store      | Same as OTP              | Same fix                                                            |

### 3.3 File Operations

| Bottleneck        | Cause                               | Mitigation                                                      |
| ----------------- | ----------------------------------- | --------------------------------------------------------------- |
| PDF from Chromium | Spawns headless browser per task    | Pool Chromium instances or use a dedicated PDF service          |
| MinIO upload      | Network I/O                         | Use local disk for dev; ensure MinIO is on same network in prod |
| Proxy image       | Fetch + buffer full image in memory | Stream response instead of `io.ReadAll`                         |

### 3.4 External API Calls

| API                         | Typical Latency | Risk                                         |
| --------------------------- | --------------- | -------------------------------------------- |
| Google OAuth token exchange | 200-500ms       | Depends on Google API availability           |
| OpenAI Embedding            | 200-2000ms      | Rate limits, cost, network latency           |
| LLM Chat                    | 1-30s           | Blocks the HTTP goroutine; use SSE streaming |
| eSewa payment verify        | 500-3000ms      | External service dependency                  |
| SMTP email                  | 500ms-5s        | Fallback to async queue; never block request |

---

## 4. Load Testing Scenarios

### 4.1 Scenario: Public Browse (Peak Traffic)

**Simulates:** Students browsing colleges, scholarships, courses during admission season.

```
Concurrency: 100 virtual users
Duration: 5 min ramp-up, 10 min steady

User journey (repeat):
  1. GET /health                        [warmup]
  2. GET /colleges?page=1&limit=12      [list]
  3. GET /colleges/featured             [featured]
  4. GET /colleges/:id                  [detail]
  5. GET /education/scholarships        [list]
  6. GET /education/scholarships/:id    [detail]
  7. GET /education/courses             [list]
  8. GET /education/news                [list]
  9. GET /search?q=engineering          [search]
 10. Wait 5-15s (think time)
```

**Expected metrics:**

- Throughput: ~300-500 RPS (8 endpoints × 100 users / 2s avg think time)
- P95 latency: <200ms
- Error rate: <0.1%
- DB CPU: <50%

### 4.2 Scenario: Registration Spike

**Simulates:** Exam result day — mass student registrations.

```
Concurrency: 50 virtual users
Duration: 30 min

User journey:
  1. POST /auth/register         {unique email per user}
  2. POST /auth/send-otp         {email}
  3. Wait 1-3s (simulating email read)
  4. POST /auth/verify-otp       {email, otp}
  5. POST /preferences           {onboarding data}
```

**Expected metrics:**

- Throughput: ~15-25 registrations/second
- P95 latency: <500ms for register, <200ms for verify
- Bottleneck: bcrypt hash on register (~200ms each)
- Memory: OTP store grows with active registrations

### 4.3 Scenario: Scholarship Application Flood

**Simulates:** Deadline day — mass applications submitted.

```
Concurrency: 80 virtual users
Duration: 15 min

User journey:
  1. GET /education/scholarships
  2. GET /education/scholarships/:id
  3. POST /education/scholarships/:id/apply  {full application data}
  4. POST /scholarships/pay/esewa/initiate   {if payment required}
```

**Expected metrics:**

- Throughput: ~20-30 applications/second
- P95 latency: <1s per application
- DB: High INSERT pressure on `scholarship_applications`
- Sequence: Roll number sequence generation

### 4.4 Scenario: Forum Engagement

**Simulates:** Active discussion day.

```
Concurrency: 60 virtual users
Duration: 20 min

User journey:
  1. GET /forum/posts
  2. GET /forum/communities
  3. POST /forum/posts                    [20% of users]
  4. POST /forum/posts/:id/like           [30% of users]
  5. POST /forum/posts/:id/comments       [20% of users]
  6. GET /forum/posts/:id/comments
```

**Expected metrics:**

- Read throughput: ~300 RPS
- Write throughput: ~30 RPS
- P95 latency: <100ms reads, <300ms writes
- Contention: Vote toggles (unique index upsert)

### 4.5 Scenario: Search Stress

**Simulates:** Heavy search usage.

```
Concurrency: 40 virtual users
Duration: 10 min

User journey:
  1. GET /search?q=engineering
  2. GET /search?q=science&cat=colleges
  3. GET /search?q=medicine&cat=scholarships
  4. GET /search?q=computer&page=2
  5. GET /search?q= (empty — returns all)
```

**Expected metrics:**

- Throughput: ~200 RPS
- P95 latency: <500ms (vector mode), <200ms (keyword mode)
- Vector search: Embedding API call adds 200-2000ms if enabled
- DB: pgvector index scan vs sequential scan

### 4.6 Scenario: Admit Card Mass Generation

**Simulates:** Bulk admit card generation before exam.

```
Single admin request:
  POST /admin/payments/send-admit-cards
  { scholarship_id: X, app_ids: [1..1000] }

Internal processing:
  For each of 1000 applications:
    1. Render HTML template
    2. Launch Chromium → generate PDF
    3. Upload PDF to MinIO
    4. Send email with PDF attachment
```

**Expected metrics:**

- Total time: 30-90 minutes (1-5s per application in parallel)
- Each PDF: 1-5s Chromium rendering
- Workers: Asynq concurrency limited by available Chromium processes
- Memory: ~100-300MB per Chromium instance

---

## 5. Recommended Load Testing Tools

| Tool             | Best For                           | Command                                                                                             |
| ---------------- | ---------------------------------- | --------------------------------------------------------------------------------------------------- |
| **k6** (Grafana) | Scriptable, JS-based, good metrics | `k6 run --vus 100 --duration 5m script.js`                                                          |
| **hey**          | Simple GET/POST flood              | `hey -n 10000 -c 100 http://localhost:8080/colleges`                                                |
| **wrk**          | HTTP benchmark, low overhead       | `wrk -t12 -c100 -d30s http://localhost:8080/health`                                                 |
| **vegeta**       | Attack/plot pipeline               | `echo "GET http://localhost:8080/health" \| vegeta attack -rate=500 -duration=30s \| vegeta report` |
| **autocannon**   | Node.js, real-time output          | `npx autocannon -c 100 -d 30 http://localhost:8080/api/v1/colleges`                                 |

### k6 Example Script: Public Browse

```javascript
import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  stages: [
    { duration: "2m", target: 50 },
    { duration: "5m", target: 100 },
    { duration: "2m", target: 0 },
  ],
  thresholds: {
    http_req_duration: ["p(95)<500", "p(99)<2000"],
    http_req_failed: ["rate<0.01"],
  },
};

export default function () {
  const base = "http://localhost:8080/api/v1";

  // Browse colleges
  let res = http.get(`${base}/colleges?page=1&limit=12`);
  check(res, { "colleges OK": (r) => r.status === 200 });

  // View featured
  res = http.get(`${base}/colleges/featured`);
  check(res, { "featured OK": (r) => r.status === 200 });

  // Search
  res = http.get(`${base}/search?q=engineering`);
  check(res, { "search OK": (r) => r.status === 200 });

  // List scholarships
  res = http.get(`${base}/education/scholarships`);
  check(res, { "scholarships OK": (r) => r.status === 200 });

  // Health check
  res = http.get("http://localhost:8080/health");
  check(res, { "health OK": (r) => r.status === 200 });

  sleep(Math.random() * 5 + 2); // 2-7s think time
}
```

---

## 6. Monitoring Checklist

### Before Load Test

- [ ] Database connection pool size configured (`DB_POOL_MAX_OPEN` / `DB_POOL_MAX_IDLE` in config)
- [ ] GORM logging set to warn/error only during load (not debug)
- [ ] PostgreSQL `max_connections` >= pool size
- [ ] JWT secret rotated for test (not production secret)
- [ ] OTP/SMTP sending rate-limited or mocked (don't send real emails)
- [ ] External APIs (Google OAuth, OpenAI, eSewa) mocked or stubbed
- [ ] File storage (MinIO) available and responsive
- [ ] Redis running for Asynq queue (if testing email features)
- [ ] Server resource limits known (CPU cores, RAM, disk IOPS)

### During Load Test

- [ ] Server CPU utilization < 80%
- [ ] Server memory: no OOM (watch Go heap, especially Chromium)
- [ ] DB CPU < 70%, no slow queries (>1s in pg_stat_activity)
- [ ] DB connection count below `max_connections` - 10
- [ ] No connection pool exhaustion (`timeout: connection refused`)
- [ ] API error rate < 1%
- [ ] P95 latency within target
- [ ] No goroutine leak (watch `runtime.NumGoroutine`)

### After Load Test

- [ ] Verify no data corruption (spot-check test records)
- [ ] Check DB bloat (VACUUM needed?)
- [ ] Review slow query log
- [ ] Check server logs for warnings/errors
- [ ] Reset any test data created during the run

---

## 7. Configuration for Performance

### Database Connection Pool

```go
// In internal/shared/config/database.go
sqlDB, _ := db.DB()
sqlDB.SetMaxOpenConns(25)        // Default: 0 (unlimited) — SET THIS
sqlDB.SetMaxIdleConns(10)        // Default: 2
sqlDB.SetConnMaxLifetime(5 * time.Minute)
sqlDB.SetConnMaxIdleTime(1 * time.Minute)
```

**Recommendation for load testing:**

- `MaxOpenConns`: Start at `25`, increase to `50` if needed
- `MaxIdleConns`: `10` (keeps connections warm)
- Tune based on PostgreSQL `max_connections` and server CPU cores

### GORM Session

```go
// Disable default debug logging during load
db = db.Session(&gorm.Session{
  PrepareStmt: true,   // Cache prepared statements
  Logger: logger.Default.LogMode(logger.Warn), // Only log warnings
})
```

### pgvector

```sql
-- For production datasets >10K rows, use HNSW instead of IVFFlat:
CREATE INDEX ON colleges USING hnsw (embedding vector_cosine_ops);

-- Tune IVFFlat lists: lists = sqrt(rows) * 4
-- For 100K rows: CREATE INDEX ... WITH (lists = 400);
```

### Rate Limiting

The backend currently has **no rate limiting**. For production:

```go
// Example using gin-contrib/limiter
import "github.com/ulule/limiter/v3/drivers/middleware/gin"

// Rate limit auth endpoints to 5 req/s per IP
rate := limiter.Rate{
  Period: 1 * time.Second,
  Limit:  5,
}
router.Use(ginlimiter.NewMiddleware(limiter.NewRateLimiter(
  memory.NewStore(), rate,
)))
```

### Caching Strategy

| Data                     | Strategy        | TTL    | Notes                      |
| ------------------------ | --------------- | ------ | -------------------------- |
| College/University lists | In-memory cache | 5 min  | Rarely changes             |
| Featured colleges        | In-memory cache | 10 min | Admin toggles infrequently |
| Carousel slides          | In-memory cache | 10 min | Admin controlled           |
| Ads                      | In-memory cache | 5 min  | Page-filtered              |
| Public notifications     | In-memory cache | 5 min  | Admin controlled           |
| Search results           | Query cache     | None   | Too dynamic                |
| User profile             | No cache        | —      | Per-user data              |
| Forum posts              | No cache        | —      | Highly dynamic (votes)     |

---

## 8. k6 Reference Test Suites

### Auth Stress (brute force simulation)

```javascript
import http from "k6/http";
import { check } from "k6";

export const options = {
  scenarios: {
    login_attempts: {
      executor: "constant-arrival-rate",
      rate: 50,
      timeUnit: "1s",
      duration: "1m",
      preAllocatedVUs: 20,
    },
  },
};

const BASE = "http://localhost:8080/api/v1";

export default function () {
  const payload = JSON.stringify({
    email: `user${__VU}@test.com`,
    password: "wrong-password",
  });

  const res = http.post(`${BASE}/auth/login`, payload, {
    headers: { "Content-Type": "application/json" },
  });

  check(res, {
    "expected 401": (r) => r.status === 401,
    "response time < 500ms": (r) => r.timings.duration < 500,
  });
}
```

### Database Write Pressure

```javascript
import http from "k6/http";
import { check } from "k6";

export const options = {
  stages: [
    { duration: "1m", target: 10 },
    { duration: "3m", target: 30 },
    { duration: "1m", target: 0 },
  ],
};

const BASE = "http://localhost:8080/api/v1";
const TOKEN = __ENV.TOKEN; // Pre-acquired JWT

export default function () {
  const payload = JSON.stringify({
    title: `Load test post ${__VU}-${Date.now()}`,
    content: "Performance testing content ".repeat(20),
    category: "discussion",
  });

  const res = http.post(`${BASE}/forum/posts`, payload, {
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${TOKEN}`,
    },
  });

  check(res, { "post created": (r) => r.status === 201 });
}
```

---

## 9. Resource Estimation

### Per Instance (4 CPU, 8GB RAM)

| Component        | Estimated Capacity  | Notes                           |
| ---------------- | ------------------- | ------------------------------- |
| API requests     | 500-1000 RPS mixed  | Depends on endpoint mix         |
| Concurrent users | 200-500             | With 5s think time              |
| DB connections   | 25-50               | Pooled                          |
| PostgreSQL       | Handles 300-500 RPS | On same host or dedicated       |
| Chromium (PDF)   | 1-2 concurrent      | Heavy; prefer dedicated service |
| Asynq workers    | 5-10                | For email queue                 |

### Scaling Bottlenecks

1. **Database CPU** — first to saturate under heavy read load
2. **bcrypt hashing** — register/login CPU-bound
3. **Chromium PDF generation** — memory + CPU heavy
4. **File upload bandwidth** — network I/O limit
5. **pgvector indexing** — write-heavy workloads degrade IVFFlat index quality

---

## 10. Quick Load Test Recipes

```bash
# Quick health check — 10K requests, 100 concurrent
wrk -t10 -c100 -d30s http://localhost:8080/health

# Auth login — POST test
hey -n 5000 -c 50 -m POST \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"test123"}' \
  http://localhost:8080/api/v1/auth/login

# College listing
wrk -t8 -c50 -d30s http://localhost:8080/api/v1/colleges?page=1

# Search query
wrk -t8 -c50 -d30s \
  "http://localhost:8080/api/v1/search?q=engineering&cat=colleges"

# Scholarship apply (requires valid scholarship ID)
hey -n 1000 -c 20 -m POST \
  -H "Content-Type: application/json" \
  -d '{"full_name":"Test User","gender":"Male","phone_number":"9800000000","email":"test@test.com","see_gpa":"3.5","school_name":"Test School","school_province":"1","school_district":"Kathmandu","school_municipality":"Kathmandu","guardian_name":"Guardian","guardian_phone":"9800000001","status":"pending"}' \
  http://localhost:8080/api/v1/education/scholarships/1/apply

# Mix test with vegeta
echo "GET http://localhost:8080/api/v1/colleges" | vegeta attack -rate=100 -duration=60s | vegeta report
echo "GET http://localhost:8080/api/v1/education/scholarships" | vegeta attack -rate=100 -duration=60s | vegeta report
echo "GET http://localhost:8080/api/v1/search?q=engineering" | vegeta attack -rate=50 -duration=60s | vegeta report
```
