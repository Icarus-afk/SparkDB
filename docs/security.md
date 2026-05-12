# Security

## Authentication

SparkDB uses Argon2id for password hashing (memory: 64MB, time: 3, threads: 4).

Three authentication methods:
- **JWT Bearer tokens** — HMAC-SHA256 signed, 24h TTL
- **Session tokens** — SHA-256 hashed, server-managed
- **API keys** — prefixed with `vl_`, SHA-256 hashed for storage

## Password Strength Policy

All passwords are validated on creation and update:

- Minimum 8 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one digit

When an admin resets a user's password, the user is flagged with `password_change_required`. The login response includes this flag so the UI can prompt for a password change.

## Encryption at Rest

Database files can be encrypted with AES-256-GCM:

```bash
# Generate a key
sparkdb gen-key

# Enable in config
SPARKDB_ENCRYPTION_KEY=<hex-key> ./sparkdb start
```

Encrypted files use `.enc` extension. Stale `.dec` files are cleaned on startup.

## API Key Protection

- API keys are encrypted at rest using AES-256-GCM with a key derived from the JWT secret
- Full keys shown only once on creation
- Re-display requires entering account password (verify password before decrypting)
- Keys are hashed with SHA-256 for authentication lookups
- Encrypted ciphertext is stored in the `encrypted_key` column for secure re-display

## Rate Limiting

Two-tier rate limiting:
- **User-level** — 60 requests per minute per authenticated user
- **IP-level** — 100 requests per minute per IP address
- Login attempts: 5 failures trigger account lockout (15 min)

Excessive requests return HTTP 429.

## Request Body Size Limit

All requests are limited to 1MB to prevent large payload attacks.

## Query Validation

Dangerous queries are blocked for all roles:
- `DROP TABLE`
- `DROP DATABASE`
- `ALTER TABLE`
- `DELETE FROM sqlite_master`

Query types are identified and checked against the user's RBAC permissions before execution.

## TLS

Enable TLS for encrypted connections:

```json
{
  "tls": {
    "enabled": true,
    "auto_cert": true
  }
}
```

Auto-generated certificates use ECDSA P-384 (valid 10 years). For production, replace with CA-signed certificates.

## CORS

Default allows all origins. Restrict in production:

```json
{
  "server": {
    "allowed_origins": ["https://app.example.com"]
  }
}
```

Or via env: `SPARKDB_SERVER_ALLOWED_ORIGINS=https://app.example.com`

## Security Headers

When behind a reverse proxy:

```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Strict-Transport-Security: max-age=31536000; includeSubDomains
Content-Security-Policy: default-src 'self'
```

## Roles and Permissions

| Role | Permissions |
|------|-------------|
| `admin` | All permissions (query, write, create, alter, drop, delete, create_user, backup, restore, audit_log) |
| `developer` | query, write, create, alter, delete |
| `readonly` | query only |
| `auditor` | audit_log access |

## Account Lockout

Brute-force protection locks accounts after configurable failed login attempts (default: 5). Lockout duration is configurable (default: 15 minutes). Login attempts are tracked per-username.

## Middleware Stack

The server applies the following middleware in order:

1. **Logging** — all requests are logged with method, path, status, and duration
2. **Panic Recovery** — panics are caught and returned as 500 errors
3. **Body Size Limit** — 1MB max request body
4. **CORS** — configurable allowed origins
5. **Rate Limiting** — per-user and per-IP limits
6. **Authentication** — JWT, session, or API key validation
