# API Reference

## Authentication

### POST /auth/login

```json
{"username": "admin", "password": "admin"}
```

Returns:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "bearer",
  "expires_in": 86400,
  "password_change_required": false,
  "user": {"id": 1, "username": "admin", "role": "admin", "auth_type": "jwt"}
}
```

### PUT /auth/password

Change own password (authenticated).

```json
{"old_password": "current", "new_password": "NewStr0ng!"}
```

Returns `{"message": "password changed"}`. New password must be at least 8 characters with uppercase, lowercase, and digit.

### POST /auth/api-keys

Create API key (admin only).

```json
{"name": "my-key"}
```

Returns `{"api_key": "vl_...", "name": "my-key"}`. **Full key shown only once on creation.**

### POST /auth/api-keys/{id}/reveal

Re-display API key (requires account password). API keys are encrypted at rest with AES-256-GCM.

```json
{"password": "your-password"}
```

Returns `{"api_key": "vl_..."}`.

### GET /auth/api-keys

List API keys. Returns `{"api_keys": [...]}`.

### DELETE /auth/api-keys/{id}

Delete an API key (admin only). Returns `{"message": "API key deleted"}`.

## Query

### POST /query

```json
{"query": "SELECT * FROM users WHERE id = ?", "database": "main", "params": [1]}
```

Supports parameterized queries via the `params` array. Returns:

```json
{
  "columns": ["id", "name"],
  "rows": [[1, "Alice"]],
  "time": "1.2ms"
}
```

### POST /transaction

Execute multiple queries atomically.

```json
{"queries": ["INSERT INTO t (v) VALUES (1)", "SELECT * FROM t"], "database": "main"}
```

Returns `{"results": [...]}` with per-query responses.

## Administration

### POST /admin/users

Create user (admin only).

```json
{"username": "dev1", "password": "securepass", "role": "developer"}
```

Password must be at least 8 characters with uppercase, lowercase, and digit. Returns `{"id": 2, "username": "dev1", "role": "developer"}`.

### GET /admin/users

List users (admin only). Returns `{"users": [...]}` with locked_until field.

### PUT /admin/users/{id}/role

Change user role (admin only).

```json
{"role": "readonly"}
```

### PUT /admin/users/{id}/username

Change username (admin only).

```json
{"username": "new-name"}
```

### PUT /admin/users/{id}/password

Admin-update user password (admin only). Triggers `password_change_required` flag.

```json
{"password": "NewStr0ng!"}
```

### DELETE /admin/users/{id}

Delete user (admin only). Cannot delete yourself. Returns `{"message": "user deleted"}`.

### GET /admin/audit-logs

View audit logs (admin/auditor). Returns `{"logs": [...]}`.

## Operations

### POST /backup

Create a backup (admin only).

```json
{"database": "main"}
```

Returns backup info with name, size, database, and created_at.

### GET /backups

List backups. Returns `{"backups": [...]}`.

### DELETE /backups/{name}

Delete a specific backup by name (admin only). Returns `{"message": "backup deleted"}`.

### POST /restore

Restore from backup (admin only).

```json
{"backup_file": "main_20260509_120000.db.backup", "database": "main"}
```

Returns `{"message": "restore completed", "database": "main"}`.

### GET /databases

List databases. Returns `{"databases": ["main"]}`.

### GET /stats

Server statistics (admin/auditor). Returns uptime, total queries, failed logins, active connections, avg latency, P99 latency, goroutines, memory, per-database sizes.

## Health and Monitoring

### GET /health

Health check. No auth required.

```json
{"status": "ok", "checks": {"database": "ok"}}
```

Returns 200 when healthy, 503 when degraded.

### GET /metrics

Prometheus metrics endpoint. Exposes sparkdb_uptime_seconds, sparkdb_queries_total, sparkdb_failed_logins_total, sparkdb_active_connections, sparkdb_query_latency_ms, sparkdb_goroutines, sparkdb_memory_alloc_mb, sparkdb_database_size_bytes.

## Replication

### GET /replication/log?since=N&limit=500

Get replication log entries since ID N (for replica polling). Optional `limit` (1-5000, default 500). Returns `{"entries": [...]}`.

## Server Control

### POST /shutdown

Gracefully stop the server (admin only). Returns `{"message": "shutting down"}`.
