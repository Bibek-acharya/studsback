# Quick Reference - StudSphere Backend

## 🚀 Quick Start

```bash
# Start PostgreSQL
make docker-up

# Run the application
make run

# Or do both at once
make dev
```

## 🐘 PostgreSQL Connection

**Docker Container Details:**
- Container: `studsphere_db`
- Host: `localhost`
- Port: `5432`
- User: `studsphere_user`
- Password: `studsphere_pass`
- Database: `studsphere`

## 📝 Useful Commands

```bash
# Show all available commands
make help

# Start PostgreSQL
make docker-up

# Stop PostgreSQL
make docker-down

# View PostgreSQL logs
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

## 🔌 Database Connection

If using `make db-connect` isn't working, connect manually:
```bash
sudo docker exec -it studsphere_db psql -U studsphere_user -d studsphere
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

## 🧪 Test API Endpoints

**Health Check:**
```bash
curl http://localhost:8080/health
```

**Register User:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
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
    "email": "user@example.com",
    "password": "password123"
  }'
```

**Get Profile (Protected):**
```bash
# Replace YOUR_TOKEN with the token from login response
curl http://localhost:8080/api/v1/profile \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## 🛠️ Troubleshooting

**Docker permission denied:**
```bash
# Add your user to docker group
sudo usermod -aG docker $USER
newgrp docker

# Or use sudo
sudo docker compose up -d
```

**Port 5432 already in use:**
```bash
# Check what's using port 5432
sudo lsof -i :5432

# Stop local PostgreSQL if running
sudo systemctl stop postgresql
```

**Reset database:**
```bash
make docker-down
sudo docker volume rm backend_postgres_data
make docker-up
```

## 📁 Project Structure

```
backend/
├── config/          # Configuration and database
├── handlers/        # API handlers
├── middleware/      # Auth middleware
├── models/          # Data models
├── routes/          # Route definitions
├── utils/           # Utility functions
├── bin/             # Compiled binaries
├── .env             # Environment variables
├── docker-compose.yml  # PostgreSQL container
├── Makefile         # Command shortcuts
└── main.go          # Application entry
```

## 🔐 Environment Variables

Edit `.env` to configure:
- `PORT` - Server port (default: 8080)
- `GIN_MODE` - debug/release
- `DB_*` - Database credentials
- `JWT_SECRET` - JWT signing key
- `JWT_EXPIRY` - Token expiry (e.g., "24h")
