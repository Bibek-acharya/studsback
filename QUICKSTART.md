# Quick Reference - StudSphere Backend

## 🚀 Quick Start

### Option A: With Docker (PostgreSQL in container)

```bash
# 1. Copy and configure environment
cp .env.example .env

# 2. Start PostgreSQL container
make docker-up

# 3. Install dependencies (first time only)
make install

# 4. Run the application
make run

# Or do steps 2-4 at once
make dev
```

### Option B: Without Docker (local PostgreSQL)

**Prerequisites:**
- Go 1.21+
- PostgreSQL 14+
- Make (optional, for shortcuts)

```bash
# 1. Install PostgreSQL (Ubuntu/Debian)
sudo apt update
sudo apt install postgresql postgresql-contrib

# Or macOS with Homebrew
brew install postgresql

# 2. Start PostgreSQL
# Ubuntu/Debian:
sudo systemctl start postgresql
sudo systemctl enable postgresql

# macOS:
brew services start postgresql

# 3. Create database and user
sudo -u postgres psql << 'EOF'
CREATE USER studsphere_user WITH PASSWORD 'studsphere_pass';
CREATE DATABASE studsphere OWNER studsphere_user;
GRANT ALL PRIVILEGES ON DATABASE studsphere TO studsphere_user;
\q
EOF

# 4. Copy and configure environment
cp .env.example .env

# 5. Install Go dependencies
go mod download

# 6. Run the application
go run main.go
```

Server runs at: **http://localhost:8080**
API docs at: **http://localhost:8080/docs**

---

## 🔐 Environment Variables

Copy `.env.example` to `.env` and configure:

```bash
# Server Configuration
PORT=8080
GIN_MODE=debug          # debug for dev, release for prod

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=studsphere_user
DB_PASSWORD=studsphere_pass
DB_NAME=studsphere
DB_SSLMODE=disable       # use 'require' for Neon/cloud hosts

# JWT Configuration
JWT_SECRET=change-this-to-a-secure-random-string
JWT_EXPIRY=24h

# Super Admin Bootstrap (creates admin on first run)
SUPER_ADMIN_EMAIL=admin@studsphere.com
SUPER_ADMIN_PASSWORD=Admin@12345
SUPER_ADMIN_ROLE=super_admin
SUPER_ADMIN_FIRST_NAME=Super
SUPER_ADMIN_LAST_NAME=Admin

# Google OAuth Configuration
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret
GOOGLE_REDIRECT_URL=http://localhost:8080/api/v1/auth/google/callback

# Frontend URL (for OAuth callback redirects)
FRONTEND_URL=http://localhost:5173

# SMTP Email Configuration (for OTP verification)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASS=your-app-password
```

### Google OAuth Setup

1. Go to [Google Cloud Console](https://console.cloud.google.com/apis/credentials)
2. Create a new project or select existing
3. Enable **Google+ API**
4. Create **OAuth 2.0 Client ID** (Web application)
5. Add authorized redirect URIs:
   - `http://localhost:8080/api/v1/auth/google/callback` (students)
   - `http://localhost:8080/api/v1/institutions/auth/google/callback` (institutions)
   - `http://localhost:8080/api/v1/scholarship-providers/auth/google/callback` (providers)
6. Copy Client ID and Client Secret to `.env`

### SMTP Setup (Gmail)

1. Enable 2FA on your Google account
2. Generate an [App Password](https://myaccount.google.com/apppasswords)
3. Use the app password as `SMTP_PASS`

---

## 🐘 PostgreSQL Connection

**Docker:**
- Container: `studsphere_db`
- Host: `localhost`, Port: `5432`
- User: `studsphere_user`, Password: `studsphere_pass`
- Database: `studsphere`

**Local:**
- Host: `localhost`, Port: `5432`
- Same credentials as configured in `.env`

---

## 📝 Useful Commands

```bash
# Show all available commands
make help

# Start PostgreSQL (Docker)
make docker-up

# Stop PostgreSQL (Docker)
make docker-down

# View PostgreSQL logs (Docker)
make docker-logs

# Connect to database
make db-connect

# Install dependencies
make install

# Run application
make run

# Build application
make build

# Clean build artifacts
make clean
```

---

## 🔌 Database Connection

**Docker:**
```bash
sudo docker exec -it studsphere_db psql -U studsphere_user -d studsphere
```

**Local:**
```bash
psql -U studsphere_user -d studsphere
```

Inside psql:
```sql
-- List all tables
\dt

-- View users table
SELECT * FROM users;

-- Describe users table
\d users

-- Exit
\q
```

---

## 🧪 Test API Endpoints

**Health Check:**
```bash
curl http://localhost:8080/health
```

**Register Student:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "student@example.com",
    "password": "password123",
    "first_name": "John",
    "last_name": "Doe",
    "role": "student"
  }'
```

**Login:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "student@example.com",
    "password": "password123"
  }'
```

**Get Profile (Protected):**
```bash
curl http://localhost:8080/api/v1/profile \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Register Institution:**
```bash
curl -X POST http://localhost:8080/api/v1/institutions/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "institution_name": "ABC College",
    "registration_number": "REG-12345",
    "email": "admin@abccollege.edu",
    "password": "password123"
  }'
```

**Register Scholarship Provider:**
```bash
curl -X POST http://localhost:8080/api/v1/scholarship-providers/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "provider_name": "Education Foundation",
    "registration_number": "PROV-67890",
    "email": "info@edufoundation.org",
    "password": "password123"
  }'
```

**Submit Contact Inquiry:**
```bash
curl -X POST http://localhost:8080/api/v1/system/contact \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Jane Smith",
    "email": "jane@example.com",
    "subject": "Question about scholarships",
    "message": "How do I apply for scholarships?"
  }'
```

**Get Featured Colleges:**
```bash
curl http://localhost:8080/api/v1/colleges/featured
```

**Get Active Ads (for landing page):**
```bash
curl "http://localhost:8080/api/v1/system/ads?page=landing"
```

**Get Carousel Slides:**
```bash
curl "http://localhost:8080/api/v1/system/carousels?page=landing"
```

---

## 📊 API Categories

| Category | Base Path | Auth | Description |
|----------|-----------|------|-------------|
| Auth | `/api/v1/auth` | Public | Student registration, login, Google OAuth |
| Institution Auth | `/api/v1/institutions/auth` | Public | Institution registration, login, Google OAuth |
| Provider Auth | `/api/v1/scholarship-providers/auth` | Public | Provider registration, login, Google OAuth |
| Colleges | `/api/v1/colleges` | Public | College listings, featured colleges |
| Universities | `/api/v1/universities` | Public | University listings |
| Education | `/api/v1/education` | Mixed | Scholarships, courses, news, events, blogs |
| Forum | `/api/v1/forum` | Mixed | Community posts and discussions |
| Student Dashboard | `/api/v1/profile`, `/messages`, etc. | Protected | Student features |
| Institution Portal | `/api/v1/institution` | Institution | Institution management |
| Provider Portal | `/api/v1/scholarship-providers` | Provider | Scholarship provider management |
| Admin | `/api/v1/admin` | Admin | Full system administration |
| System | `/api/v1/system` | Public | Contact, ads, carousels |

---

## 🛠️ Troubleshooting

**Docker permission denied:**
```bash
sudo usermod -aG docker $USER
newgrp docker
# Or use sudo
sudo docker compose up -d
```

**Port 5432 already in use:**
```bash
sudo lsof -i :5432
# Stop local PostgreSQL if running
sudo systemctl stop postgresql
```

**Port 8080 already in use:**
```bash
sudo lsof -i :8080
# Or change PORT in .env
```

**Reset database (Docker):**
```bash
make docker-down
sudo docker volume rm backend_postgres_data
make docker-up
```

**Reset database (Local):**
```bash
sudo -u postgres dropdb studsphere
sudo -u postgres createdb studsphere -O studsphere_user
# Restart the app to re-run migrations
go run main.go
```

**PostgreSQL authentication fails (Local):**
```bash
# Check pg_hba.conf location
sudo -u postgres psql -c "SHOW hba_file;"
# Ensure md5 or scram-sha-256 auth is enabled for local connections
```

**Google OAuth not working:**
- Verify redirect URIs match exactly in Google Cloud Console
- Check `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` in `.env`
- Ensure `GOOGLE_REDIRECT_URL` points to your server

**OTP emails not sending:**
- Check SMTP credentials in `.env`
- For Gmail, ensure you're using an App Password (not regular password)
- Check logs for SMTP errors: `go run main.go` shows detailed output

---

## 📁 Project Structure

```
studsback/
├── config/              # Configuration and database connection
├── handlers/            # API request handlers
│   ├── auth.go          # Student auth + Google OAuth
│   ├── institution_auth.go
│   ├── scholarship_provider_auth.go
│   ├── college.go
│   ├── university.go
│   ├── education.go     # Scholarships, courses, news, events, blogs
│   ├── scholarship.go
│   ├── forum.go
│   ├── counselling.go
│   ├── admission.go
│   ├── tools.go         # AI recommendation tools
│   ├── institution_scholarship.go
│   ├── institution_dashboard.go
│   ├── scholarship_provider.go
│   ├── student_dashboard.go
│   └── system.go        # Contact, ads, carousels
├── middleware/          # Auth middleware (JWT, role checks)
├── models/              # Database models (GORM)
├── routes/              # Route definitions
├── utils/               # JWT, OTP, email, response helpers
├── seeds/               # Database seed data
├── docs/                # Swagger/API documentation
├── uploads/             # Uploaded media files
├── .env                 # Environment variables (gitignored)
├── .env.example         # Environment template
├── docker-compose.yml   # PostgreSQL container
├── Makefile             # Command shortcuts
├── main.go              # Application entry point
└── API_DOCUMENTATION.md # Complete API reference
```

---

## 🔑 Default Admin Credentials

On first run, the super admin is created from `.env` values:
- **Email:** `admin@studsphere.com`
- **Password:** `Admin@12345`
- **Role:** `super_admin`

Use these credentials to access admin endpoints and manage the system.
