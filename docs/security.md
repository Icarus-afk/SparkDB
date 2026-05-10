# Security

## Authentication

SparkDB uses Argon2id for password hashing (memory: 64MB, time: 3, threads: 4).

Three authentication methods:
- **JWT Bearer tokens** — HMAC-SHA256 signed, 24h TTL
- **Session tokens** — SHA-256 hashed, server-managed
- **API keys** — prefixed with `vl_`, SHA-256 hashed for storage

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
- Re-display requires entering account password
- Keys are hashed with SHA-256 for authentication lookups

## Rate Limiting

- Login attempts: 5 failures trigger account lockout (15 min, configurable)
- API requests: rate limited per client (configurable)

## Query Validation

Dangerous queries are blocked for all roles:
- `DROP TABLE`
- `DROP DATABASE`
- `ALTER TABLE`
- `DELETE FROM sqlite_master`

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
| `admin` | All permissions |
| `developer` | query, write, create, alter, delete |
| `readonly` | query only |
| `auditor` | audit_log access |

## Account Lockout

Brute-force protection locks accounts after configurable failed login attempts. Lockout duration is configurable.
