# Authentication Service Design Document

## Overview
A lightweight authentication service that provides OAuth2-based authentication for both web applications and APIs. The service will manage user authentication, JWT issuance, and basic organization management using PostgreSQL as the primary datastore.

## Core Requirements
- OAuth2 authentication (Google only for MVP)
- JWT-based API authentication with refresh tokens
- Basic organization management with owner/sub-account roles
- PostgreSQL for all data storage using sqlx
- Standard library HTTP server with Go 1.22 routing features
- Structured logging using slog

## System Architecture

### Technology Choices
- Go 1.22 for enhanced routing features
- Standard `net/http` package for HTTP server
- `sqlx` for database operations
- `goose` for database migrations
- `slog` for structured logging
- In-memory RSA key generation for JWT signing (generated at startup)

### Project Structure
```
auth-service/
├── main.go          # Application entry point and server setup
├── handlers.go      # HTTP handlers using Go 1.22 routing
├── models.go        # Data structures and types
├── database.go      # Database operations using sqlx
├── auth.go          # Authentication logic
├── middleware.go    # Security and validation middleware
├── migrations/      # Goose SQL migrations
├── go.mod          # Dependencies
└── .env            # Configuration
```

### Database Schema
```sql
CREATE TABLE organizations (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    owner_id UUID NOT NULL,
    subscription_tier VARCHAR(50) NOT NULL DEFAULT 'free',
    max_sub_accounts INT NOT NULL DEFAULT 5,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE users (
    id UUID PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    organization_id UUID REFERENCES organizations(id),
    role VARCHAR(50) NOT NULL, -- 'owner' or 'sub_account'
    permissions JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id) -- Only one active refresh token per user
);
```

### API Endpoints
```
GET /health
    - Returns service health status
    - Checks database connectivity

POST /auth/login/google
    - Initiates Google OAuth flow
    - Returns: JWT access token + refresh token

GET /auth/callback/google
    - Handles Google OAuth callback
    - Returns: JWT access token + refresh token

POST /auth/refresh
    - Refreshes access token
    - Requires: valid refresh token
    - Returns: new JWT access token

GET /auth/.well-known/jwks.json
    - Returns public key for JWT verification

POST /organizations
    - Creates new organization
    - Sets creator as owner
    - Validates input data

POST /organizations/sub-accounts
    - Creates sub-account in organization
    - Requires: owner role
    - Validates against max_sub_accounts limit
```

### JWT Structure
```json
{
  "sub": "user_id",
  "exp": 1234567890,
  "iat": 1234567890
}
```

## Implementation Details

### Security
1. Token Management:
   - JWT access tokens: 15-minute expiry
   - Refresh tokens: 7-day expiry
   - Single active refresh token per user
   - New login invalidates previous refresh tokens
   - RSA keys generated at startup

2. OAuth Configuration:
   - Initially supporting Google OAuth only
   - State parameter for CSRF protection
   - Environment variables for credentials

3. Security Headers:
   - X-Frame-Options: DENY
   - X-Content-Type-Options: nosniff
   - X-XSS-Protection: 1; mode=block
   - Configurable CORS settings

### Error Handling
- Standard Go error handling
- HTTP status codes for API responses
- Structured logging with slog for error tracking
- No custom error types for MVP
- Input validation errors returned as 400 Bad Request

### Database Access
- Direct CRUD operations using sqlx
- Simple query patterns without repository abstraction
- Goose for managing database migrations
- Connection pooling handled by sqlx defaults
- Health checks via ping

## Production Readiness

### Server Configuration
- Graceful shutdown handling
- Request timeouts:
  - Read timeout: 10s
  - Write timeout: 10s
  - Idle timeout: 60s
- Shutdown timeout: 30s
- Health check endpoint
- Signal handling (SIGTERM/SIGINT)

### Request Validation
- JSON format validation
- Input size limits
- Basic format validation:
  - Email format
  - UUID validation
  - String length limits
- Meaningful validation error messages

### Health Monitoring
- `/health` endpoint checking:
  - Database connectivity
  - Application status
- Structured logging of all health checks
- Error tracking with context

### CORS Configuration
- Configurable allowed origins
- Standard allowed methods (GET, POST, OPTIONS)
- Allowed headers for API operation
- Proper OPTIONS handling

## Development Approach
1. Use Go 1.22's native HTTP router
2. Implement core auth service focused on Google OAuth
3. Use sqlx for database operations
4. Implement basic logging with slog
5. Use goose for database migrations
6. Keep error handling simple and standard
7. Implement essential security features
8. Add basic health monitoring

## Configuration
Required environment variables:
```
DATABASE_URL=postgres://user:pass@host:5432/dbname
GOOGLE_CLIENT_ID=xxx
GOOGLE_CLIENT_SECRET=xxx
GOOGLE_REDIRECT_URL=xxx
ALLOWED_ORIGIN=xxx  # For CORS
```

## MVP Scope

### Included
- Google OAuth flow
- JWT issuance and validation with in-memory keys
- Simple organization management
- Basic permission system
- Structured logging with slog
- Database migrations with goose
- Essential security headers
- Health check endpoint
- Graceful shutdown
- Basic request validation
- CORS configuration

### Explicitly Excluded
- GitHub OAuth integration (future)
- Custom error types
- Repository pattern
- Multiple active sessions
- Advanced permission management
- Detailed metrics/monitoring
- Circuit breakers
- Complex rate limiting
- Audit logging
- Multiple deployment configurations

## Success Criteria
- Google OAuth login flow works
- JWTs are properly signed and validated
- Refresh tokens work as expected
- Basic organization management functions
- All core functionality covered by tests
- Structured logging provides adequate debugging information
- Health check endpoint properly reflects service status
- Service handles shutdown gracefully
- Basic security measures are in place and effective
