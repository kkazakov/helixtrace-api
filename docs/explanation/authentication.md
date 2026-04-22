# Authentication

**Type:** Explanation

The authentication component handles user registration, login, token generation, and Bearer token validation for protected routes.

## Responsibility

- User registration with bcrypt password hashing
- User login with password verification
- Cryptographically random token generation
- Token storage in ClickHouse with 24-hour TTL
- Bearer token validation middleware for protected routes
- Email injection into request context for downstream handlers

## Source Files

- `internal/handlers/auth.go` — AuthHandler (Login, Register), generateToken
- `internal/middleware/auth.go` — AuthMiddleware
- `internal/handlers/point.go` — ContextWithEmail, EmailFromContext (lines 22-33)

## Public Interface

### AuthHandler

```go
type AuthHandler struct {
    Conn clickhouse.Conn
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request)
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request)
```

### Middleware

```go
func AuthMiddleware(conn clickhouse.Conn, next http.HandlerFunc) http.HandlerFunc
```

### Context Helpers

```go
func ContextWithEmail(ctx context.Context, email string) context.Context
func EmailFromContext(ctx context.Context) (string, bool)
```

## Internal Structure

### Registration Flow (`Register`)
1. Parse JSON body for email and password
2. Validate both fields are non-empty
3. Check for existing user via `SELECT count() FROM users FINAL WHERE email = ?`
4. Return 409 Conflict if user exists
5. Hash password with `bcrypt.GenerateFromPassword` at default cost
6. Insert user row with `active = true`, empty `access_rights`, username = email
7. Generate token and insert into tokens table with 24-hour expiry
8. Return 201 Created with token, email, username

### Login Flow (`Login`)
1. Parse JSON body for email and password
2. Validate both fields are non-empty
3. Query user from `users FINAL` table by email
4. Return 401 if user not found
5. Compare password hash with `bcrypt.CompareHashAndPassword`
6. Return 401 if password doesn't match
7. Check `user.Active` flag; return 403 if disabled
8. Generate token and insert into tokens table with 24-hour expiry
9. Return 200 OK with token, email, username

### Token Generation (`generateToken`)
Generates 32 random bytes via `crypto/rand`, encodes as hex string (64-character token).

### Auth Middleware (`AuthMiddleware`)
1. Extract `Authorization` header
2. Return 401 if header is missing
3. Strip `Bearer ` prefix; return 401 if prefix is absent
4. Query `tokens FINAL` for matching token where `expires_at > now64()`
5. Return 401 if token not found or expired
6. Inject email into request context via `ContextWithEmail`
7. Call next handler with updated context

The tokens table has a ClickHouse TTL that automatically deletes expired rows.

## Data Model

### Users table
- `email` (String) — Unique identifier, ORDER BY key
- `password_hash` (String) — bcrypt hash
- `username` (String) — Display name, defaults to email on registration
- `active` (Bool) — Account enabled flag, default true
- `access_rights` (String) — Reserved for future RBAC, currently empty
- `updated_at` (DateTime64) — Version column for ReplacingMergeTree

Engine: `ReplacingMergeTree(updated_at)`, ORDER BY `email`

### Tokens table
- `token` (String) — 64-char hex random string, ORDER BY key
- `email` (String) — Associated user email
- `created_at` (DateTime64) — Token creation time, version column
- `expires_at` (DateTime64) — Expiry time, drives TTL deletion

Engine: `ReplacingMergeTree(created_at)`, ORDER BY `token`, TTL `expires_at DELETE`

## Dependencies

### Internal
- [Database](database.md) — ClickHouse connection for queries
- [Models](models.md) — User struct for scanning query results

### External
- `golang.org/x/crypto/bcrypt` — Password hashing
- `crypto/rand` — Token generation

## Design Decisions

### Tokens in ClickHouse, not JWT
Tokens are stored as random hex strings in ClickHouse rather than using signed JWTs. This allows immediate invalidation (delete the token row) and supports multi-worker deployments without shared secret management. The trade-off is a database round-trip on every authenticated request.

### 24-hour token expiry
Tokens expire after 24 hours. ClickHouse TTL handles automatic cleanup of expired rows.

### ReplacingMergeTree for users
The users table uses `ReplacingMergeTree(updated_at)` so that updates are handled by re-inserting with a newer `updated_at`. Queries must include `FINAL` to see the latest version.

### Context-based email passing
The middleware injects the authenticated user's email into the request context. Downstream handlers extract it via `EmailFromContext(r.Context())`. This avoids passing email as a parameter through the handler chain.

## Testing

No Go tests exist. Auth endpoints are tested via the Bruno collection in `bruno/Auth/`.
