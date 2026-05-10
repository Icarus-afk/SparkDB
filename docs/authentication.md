# Authentication

SparkDB supports three authentication methods:

1. **JWT Bearer tokens** — short-lived tokens (default 24h TTL) signed with HMAC-SHA256
2. **Session tokens** — server-managed sessions stored as SHA-256 hashes
3. **API keys** — long-lived keys prefixed with `vl_`, stored as SHA-256 hashes

All passwords are hashed with Argon2id (memory: 64MB, time: 3, threads: 4).

## JWT Authentication

Login with username/password to receive a JWT token:

```bash
curl -X POST http://localhost:9600/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin"}'
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "bearer",
  "expires_in": 86400,
  "user": {
    "id": 1,
    "username": "admin",
    "role": "admin"
  }
}
```

Use the token in subsequent requests:

```bash
curl -X POST http://localhost:9600/query \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"query": "SELECT 1", "database": "main"}'
```

## Session Authentication

Login returns a session token that can be used with the `Session` auth scheme:

```
Authorization: Session <session-token>
```

Session tokens are managed server-side and can be revoked.

## API Keys

Create an API key (admin only):

```bash
curl -X POST http://localhost:9600/auth/api-keys \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-key"}'
```

Response:
```json
{
  "api_key": "vl_a1b2c3d4e5f6...",
  "name": "my-key",
  "id": 1
}
```

**Full key is shown only once on creation.** Re-display requires account password:

```bash
curl -X POST http://localhost:9600/auth/api-keys/1/reveal \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"password": "your-password"}'
```

Use API keys with the `X-API-Key` header:

```bash
curl -X POST http://localhost:9600/query \
  -H "X-API-Key: vl_a1b2c3d4e5f6..." \
  -H "Content-Type: application/json" \
  -d '{"query": "SELECT 1", "database": "main"}'
```

List API keys:

```bash
curl -X GET http://localhost:9600/auth/api-keys \
  -H "Authorization: Bearer <token>"
```

## Request Headers

| Scheme | Header |
|--------|--------|
| JWT | `Authorization: Bearer <token>` |
| Session | `Authorization: Session <token>` |
| API Key | `X-API-Key: <key>` |

## User Management

Create a user (admin only):

```bash
curl -X POST http://localhost:9600/admin/users \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"username": "dev1", "password": "securepass", "role": "developer"}'
```

List users (admin only):

```bash
curl -X GET http://localhost:9600/admin/users \
  -H "Authorization: Bearer <token>"
```

Change user role (admin only):

```bash
curl -X PUT http://localhost:9600/admin/users/2/role \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"role": "readonly"}'
```

Delete user (admin only):

```bash
curl -X DELETE http://localhost:9600/admin/users/2 \
  -H "Authorization: Bearer <token>"
```

## CLI Credentials

CLI commands accept credentials via flags:

```bash
sparkdb shell --user admin --pass admin
sparkdb query "SELECT 1" --user admin --pass admin
sparkdb shell --api-key vl_...
sparkdb stop --user admin --pass admin
```
