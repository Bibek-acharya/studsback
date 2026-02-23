# StudSphere Backend

A Go-based backend API for StudSphere using Gin framework with JWT authentication.

## Features

- ✅ User Registration & Login
- ✅ JWT-based Authentication
- ✅ Password Hashing with bcrypt
- ✅ Role-based Access Control (Student, Teacher, Admin)
- ✅ PostgreSQL Database with GORM
- ✅ RESTful API Design
- ✅ Environment-based Configuration

## Project Structure

```
backend/
├── config/          # Configuration files
│   ├── config.go    # Application configuration
│   └── database.go  # Database connection
├── handlers/        # Request handlers
│   └── auth.go      # Authentication handlers
├── middleware/      # Middleware functions
│   └── auth.go      # Auth middleware
├── models/          # Data models
│   └── user.go      # User model
├── routes/          # Route definitions
│   └── routes.go    # API routes
├── utils/           # Utility functions
│   ├── jwt.go       # JWT utilities
│   └── response.go  # Response helpers
├── .env.example     # Example environment variables
├── .gitignore       # Git ignore file
└── main.go          # Application entry point
```

## Prerequisites

- Go 1.21 or higher
- PostgreSQL 13 or higher

## Installation

1. Clone the repository and navigate to the backend directory

2. Install dependencies:
```bash
go mod download
```

3. Create a `.env` file from the example:
```bash
cp .env.example .env
```

4. Update the `.env` file with your database credentials and JWT secret

5. Make sure PostgreSQL is running and create the database:
```sql
CREATE DATABASE studsphere;
```

## Running the Application

```bash
go run main.go
```

The server will start on `http://localhost:8080` (or the port specified in your `.env` file)

## API Endpoints

### Public Endpoints

#### Health Check
```
GET /health
```

#### Register
```
POST /api/v1/auth/register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123",
  "first_name": "John",
  "last_name": "Doe",
  "role": "student"  // optional: student (default), teacher, admin
}
```

#### Login
```
POST /api/v1/auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}
```

### Protected Endpoints (Requires Authentication)

Add the JWT token to the Authorization header:
```
Authorization: Bearer <your-jwt-token>
```

#### Get Profile
```
GET /api/v1/profile
```

#### Admin Routes (Requires Admin Role)
```
GET /api/v1/admin/users
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| PORT | Server port | 8080 |
| GIN_MODE | Gin mode (debug/release) | debug |
| DB_HOST | Database host | localhost |
| DB_PORT | Database port | 5432 |
| DB_USER | Database user | postgres |
| DB_PASSWORD | Database password | - |
| DB_NAME | Database name | studsphere |
| JWT_SECRET | JWT signing secret | your-secret-key |
| JWT_EXPIRY | JWT token expiry duration | 24h |

## Database Schema

### Users Table
- `id` (Primary Key)
- `email` (Unique)
- `password` (Hashed)
- `first_name`
- `last_name`
- `role` (student/teacher/admin)
- `created_at`
- `updated_at`
- `deleted_at` (Soft delete)

## Development

### Running with hot reload
Install Air for hot reloading:
```bash
go install github.com/cosmtrek/air@latest
air
```

### Building for production
```bash
go build -o bin/server main.go
./bin/server
```

## Security Notes

- Always use strong JWT secrets in production
- Change default database credentials
- Use HTTPS in production
- Keep dependencies updated
- Never commit `.env` file to version control

## Tech Stack

- **Framework**: Gin
- **Database**: PostgreSQL
- **ORM**: GORM
- **Authentication**: JWT (golang-jwt/jwt)
- **Password Hashing**: bcrypt
- **Environment Config**: godotenv

## License

MIT
