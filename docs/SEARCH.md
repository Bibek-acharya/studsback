# Search Engine — Architecture & Operations

## Overview

Hybrid vector + keyword search across 7 entity types powered by **pgvector** (PostgreSQL) and **OpenAI embeddings**. Graceful fallback to pure keyword search when vectors are unavailable.

## Setup Guide

### Prerequisites

| Dependency | Version | Notes |
|-----------|---------|-------|
| Go | 1.21+ | `go version` |
| Node.js | 18+ | For frontend |
| Docker | 24+ | For PostgreSQL with pgvector |
| OpenAI API key | — | `sk-...` key for embeddings |

### 1. Start PostgreSQL with pgvector

```bash
cd studsback
docker compose up -d
```

This starts PostgreSQL 16 with the `vector` extension pre-installed (`pgvector/pgvector:pg16` image).

### 2. Backend Configuration

Create `.env` from the template and configure:

```bash
cp .env.example .env
```

Required edits in `.env`:

```ini
# PostgreSQL (defaults match docker-compose.yml)
DB_HOST=localhost
DB_PORT=5432
DB_USER=studsphere_user
DB_PASSWORD=studsphere_pass
DB_NAME=studsphere
DB_SSLMODE=disable

# Vector Search (optional — skip for keyword-only)
EMBEDDING_ENABLED=true
EMBEDDING_API_KEY=sk-your-openai-api-key-here
EMBEDDING_MODEL=text-embedding-3-small
VECTOR_DIMENSION=1536
EMBEDDING_BATCH_SIZE=20

# JWT (for admin reindex endpoint)
JWT_SECRET=your-secret-key-change-this-in-production
```

### 3. Start Backend

```bash
cd studsback
go run ./cmd/server/
```

The server:
1. Connects to PostgreSQL
2. Runs auto-migrations (creates tables + `embedding vector(1536)` columns)
3. Enables the `vector` extension (`CREATE EXTENSION IF NOT EXISTS vector`)
4. Creates IVFFlat indexes for fast vector search
5. Seeds sample data
6. Listens on `:8080`

**Verify the server is live:**

```bash
curl http://localhost:8080/health
# → {"status":"ok","message":"Server is running"}
```

### 4. Configure Frontend

```bash
cd studsnew
```

Ensure `.env.local` (or `.env`) contains:

```bash
NEXT_PUBLIC_API_URL=http://localhost:8080
```

Start the frontend:

```bash
npm install   # first time only
npm run dev
```

### 5. Verify Search Works (Keyword Mode)

Search the frontend from your browser at `http://localhost:5173/search?q=engineering` or test the API directly:

```bash
curl "http://localhost:8080/api/v1/search?q=engineering"
```

You should get results from the seeded data, using keyword (ILIKE) search.

### 6. Enable Vector Search (Optional)

Vector search requires embeddings to be generated. After seeding:

```bash
# Check current vector status
curl http://localhost:8080/api/v1/search/vector-status
# → total_vectors: 0

# Generate embeddings for all unembedded rows
# Requires admin JWT token (grab from server logs or seed with known credentials)
TOKEN="<admin-jwt-token>"

curl -X POST http://localhost:8080/api/v1/admin/search/reindex \
  -H "Authorization: Bearer $TOKEN"
```

Progress is logged server-side. Re-run the search:

```bash
curl "http://localhost:8080/api/v1/search?q=engineering"
```

The response will now include `"vector": {"enabled": true, ...}`, confirming hybrid vector+keyword search is active.

**Total setup time:** ~10 minutes.

### Quick Start (tl;dr)

```bash
# Terminal 1: Database
cd studsback && docker compose up -d

# Terminal 2: Backend
cd studsback && cp .env.example .env
# edit .env → set EMBEDDING_ENABLED=false (or add API key)
go run ./cmd/server/

# Terminal 3: Frontend
cd studsnew && npm run dev

# Test
curl "http://localhost:8080/api/v1/search?q=college"
```

### Switching Search Modes

| Mode | `.env` Config | Search Method |
|------|--------------|---------------|
| **Keyword-only** | `EMBEDDING_ENABLED=false` | ILIKE queries (no external deps) |
| **Hybrid** | `EMBEDDING_ENABLED=true` + `EMBEDDING_API_KEY=sk-...` | Vector cosine + ILIKE |
| **Vector-only** | (block keyword via `?cat=` only) | Pure semantic search within category |

The system degrades gracefully: if the embedding API is unreachable, it transparently falls back to keyword search.

## Architecture

```
User Query
    │
    ▼
┌──────────────────┐
│  SearchBar UI    │  React component → /search?q=...
│  (Frontend)      │
└────────┬─────────┘
         │ fetch
         ▼
┌──────────────────┐
│  GET /api/v1/    │  Go Gin handler
│  search          │
└────────┬─────────┘
         │
    ┌────┴────┐
    ▼         ▼
Vector    Keyword
Search    Fallback
    │         │
    │    ┌────┴────┐
    ▼    ▼         ▼
┌──────────┐  ┌──────────┐
│ pgvector │  │  ILIKE   │
│ <=>      │  │  queries │
│ cosine   │  │  (GORM)  │
└──────────┘  └──────────┘
    │              │
    └──────┬───────┘
           ▼
    ┌──────────────┐
    │  Merged +    │
    │  Paginated   │
    │  Results     │
    └──────────────┘
```

## Searchable Tables

| Entity | DB Table | Module | Vector Column |
|--------|----------|--------|---------------|
| Colleges | `colleges` | `internal/college/` | `embedding vector(1536)` |
| Courses | `courses` | `internal/education/` | `embedding vector(1536)` |
| Exams | `exams` | `internal/education/` | `embedding vector(1536)` |
| Scholarships | `scholarships` | `internal/scholarship/` | `embedding vector(1536)` |
| News | `news` | `internal/education/` | `embedding vector(1536)` |
| Events | `events` | `internal/education/` | `embedding vector(1536)` |
| Blogs | `blogs` | `internal/education/` | `embedding vector(1536)` |

## API Endpoints

### Search (Public)

```
GET /api/v1/search?q=<query>&cat=<category>&page=<page>&limit=<limit>
```

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `q` | string | `""` | Search query. If empty, returns all items unfiltered |
| `cat` | string | auto-detect | Category filter: `colleges`, `courses`, `exams`, `scholarships`, `news`, `events`, `blogs` |
| `page` | int | `1` | Page number (1-indexed) |
| `limit` | int | `12` | Items per page (max: 50) |

Category is auto-detected from the query when `cat` is omitted:
| Keyword | Category |
|---------|----------|
| college, colleges | `colleges` |
| course, courses | `courses` |
| exam, exams | `exams` |
| scholarship, scholarships | `scholarships` |
| news | `news` |
| event, events | `events` |
| blog, blogs | `blogs` |

**Response:**

```json
{
  "success": true,
  "message": "Search results retrieved successfully",
  "data": {
    "items": [
      {
        "id": 1,
        "type": "college",
        "title": "St. Xavier's College",
        "description": "A premier institution...",
        "image": "/uploads/colleges/...",
        "featured": true,
        "verified": true,
        "rating": 4.7,
        "institutionType": "Private",
        "location": "Kathmandu",
        "university": "Tribhuvan University",
        "website": "sxc.edu.np",
        "slug": "st-xaviers",
        "tags": []
      }
    ],
    "category": {
      "title": "Colleges",
      "description": "Explore college listings...",
      "related": ["engineering", "medical", "business"],
      "tabs": ["Discover", "Engineering", "Medical"],
      "key": "colleges"
    },
    "categoryKey": "colleges",
    "meta": {
      "page": 1,
      "limit": 12,
      "total": 42,
      "pages": 4
    },
    "vector": {
      "enabled": true,
      "provider": "openai",
      "model": "text-embedding-3-small",
      "dimensions": 1536
    }
  }
}
```

### Vector Status (Public)

```
GET /api/v1/search/vector-status
```

Check if vector search is operational.

```json
{
  "success": true,
  "data": {
    "embedding_enabled": true,
    "pgvector_ready": true,
    "model": "text-embedding-3-small",
    "dimensions": 1536,
    "is_postgresql": true,
    "total_vectors": 142
  }
}
```

### Reindex Embeddings (Admin)

```
POST /api/v1/admin/search/reindex
Authorization: Bearer <admin_token>
```

Regenerates embeddings for all rows where `embedding IS NULL`. Runs asynchronously — returns `202 Accepted` immediately. Progress is logged to the server log.

```json
{
  "success": true,
  "message": "Embedding reindex started in background. Check server logs for progress."
}
```

## Search Algorithm

### Vector Mode (default when available)

```
1. Embed query → [0.023, -0.041, ..., 0.012]  (1536-dim float32)
2. For each table matching the category:
     SELECT ..., 1 - (embedding <=> query_vec) AS vector_score
     FROM table
     WHERE embedding IS NOT NULL
       AND keyword_filter_matches(query)
     ORDER BY vector_score DESC
     LIMIT 30
3. Merge results from all tables
4. Apply pagination
```

The `<=>` operator computes **cosine distance**. `1 - distance` gives **cosine similarity** (0 to 1, higher = more similar).

### Fallback Mode (when vectors unavailable)

Sequential ILIKE queries per table with `LOWER(column) LIKE LOWER('%query%')` across title, description, location, and key entity fields.

## Configuration

### Environment Variables (`.env`)

```bash
# Embedding / Vector Search
EMBEDDING_ENABLED=false          # Set true to enable vector search
EMBEDDING_API_KEY=sk-...         # OpenAI API key
EMBEDDING_MODEL=text-embedding-3-small  # OpenAI model ID
VECTOR_DIMENSION=1536            # Must match model output dimensions
EMBEDDING_BATCH_SIZE=20          # Rows per API call during reindex
```

### Infrastructure

**Docker image** must be `pgvector/pgvector:pg16` (includes the `vector` extension).

The extension is auto-enabled at startup:
```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

IVFFlat indexes are auto-created after migration:
```sql
CREATE INDEX IF NOT EXISTS idx_<table>_embedding
ON <table> USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 10);
```

`lists = 10` is appropriate for small datasets (<10K rows). Tune via `lists = 4 * sqrt(rows)` for production.

## Embedding Content Strategy

Each entity is indexed using concatenated text fields with ` | ` separator:

| Table | Fields Embedded |
|-------|----------------|
| `colleges` | name, full_name, description, location, affiliation, college_type |
| `courses` | title, short_title, description, field, level, affiliation |
| `exams` | title, description, board, type, university |
| `scholarships` | title, description, provider, location, scholarship_type |
| `news` | title, excerpt, content, category, source |
| `events` | title, description, excerpt, category, location |
| `blogs` | title, excerpt, content, category, author |

Each input is truncated to 8000 characters (OpenAI token limit for `text-embedding-3-small`).

## Operations

### Provisioning Vector Search

```bash
# 1. Enable in .env
echo "EMBEDDING_ENABLED=true" >> .env
echo "EMBEDDING_API_KEY=sk-your-key" >> .env

# 2. Restart server (auto-creates pgvector extension + indexes)
./main

# 3. Trigger reindex
curl -X POST http://localhost:8080/api/v1/admin/search/reindex \
  -H "Authorization: Bearer <admin_token>"
```

### Monitoring

Check `GET /api/v1/search/vector-status` for:
- `embedding_enabled` — Is the embedding service configured?
- `pgvector_ready` — Is the pgvector extension loaded?
- `total_vectors` — How many rows have embeddings?
- `is_postgresql` — Is PostgreSQL active (not SQLite fallback)?

### Reindexing

Reindex is **idempotent** — it only updates rows where `embedding IS NULL`. To force a full reindex:

```sql
UPDATE colleges SET embedding = NULL;
UPDATE courses SET embedding = NULL;
UPDATE exams SET embedding = NULL;
UPDATE scholarships SET embedding = NULL;
UPDATE news SET embedding = NULL;
UPDATE events SET embedding = NULL;
UPDATE blogs SET embedding = NULL;
```

Then call `POST /api/v1/admin/search/reindex`.

## Code Map

```
studsback/
├── internal/
│   ├── search/                    # Search module
│   │   ├── model.go               # DTOs, category metadata, keyword matching
│   │   ├── service.go             # Hybrid search (vector + keyword)
│   │   ├── handler.go             # HTTP handlers (search, reindex, status)
│   │   └── routes.go              # Route registration
│   ├── embedding/                 # Embedding service
│   │   ├── model.go               # Embeddable interface
│   │   └── service.go             # OpenAI API client, batch reindex
│   ├── college/model.go           # +Embedding field
│   ├── education/model.go         # +Embedding field (Course, Exam, News, Event, Blog)
│   ├── scholarship/model.go       # +Embedding field
│   └── shared/config/
│       ├── config.go              # EMBEDDING_* env vars
│       └── database.go            # pgvector extension + index creation
├── docker-compose.yml             # pgvector/pgvector:pg16 image
└── cmd/server/main.go             # Register vector index creation
```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Search works but `vector.enabled` is `false` | Embedding not configured | Set `EMBEDDING_ENABLED=true` and `EMBEDDING_API_KEY` in `.env` |
| Vector search returns no items | No embeddings generated yet | Run `POST /api/v1/admin/search/reindex` |
| `pgvector_ready: false` | PostgreSQL image missing extension | Use `pgvector/pgvector:pg16` image |
| `is_postgresql: false` | SQLite fallback active | Check PostgreSQL connection in `.env` |
| OpenAI API errors in logs | Invalid API key or rate limit | Verify `EMBEDDING_API_KEY` and check OpenAI usage dashboard |
| Slow vector search | No index or bad index parameters | Verify IVFFlat indexes exist; tune `lists` parameter |
