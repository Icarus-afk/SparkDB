# API Reference

## Authentication

### POST /auth/login
```json
{"username": "admin", "password": "admin"}
```
Returns `{"token": "<jwt>", "token_type": "bearer", "expires_in": 86400, "user": {...}}`

### POST /auth/api-keys
Create API key (admin only).
```json
{"name": "my-key"}
```
Returns `{"api_key": "vl_...", "name": "my-key", "id": 1}`

### POST /auth/api-keys/{id}/reveal
Re-display API key (requires password).
```json
{"password": "your-password"}
```

### GET /auth/api-keys
List API keys.

## Query

### POST /query
```json
{"query": "SELECT * FROM users", "database": "main", "params": []}
```

### POST /transaction
Execute multiple queries atomically.
```json
{"queries": ["INSERT INTO t (v) VALUES (1)", "SELECT * FROM t"], "database": "main"}
```

## Administration

### POST /admin/users
Create user (admin only).
```json
{"username": "dev1", "password": "securepass", "role": "developer"}
```

### GET /admin/users
List users (admin only).

### DELETE /admin/users/{id}
Delete user (admin only).

### PUT /admin/users/{id}/role
Change user role (admin only).
```json
{"role": "readonly"}
```

### GET /admin/audit-logs
View audit logs (admin/auditor). Supports `?search=` and `?limit=` params.

## Operations

### POST /backup
Create a backup (admin only).
```json
{"database": "main"}
```

### DELETE /backups/{name}
Delete a backup (admin only).

### POST /restore
Restore from backup (admin only).
```json
{"backup_file": "main_20260509_120000.db.backup", "database": "main"}
```

### GET /backups
List backups.

### GET /databases
List databases.

### GET /stats
Server statistics.

## Health and Monitoring

### GET /health
Health check. Returns `{"status": "ok", "uptime": "1h2m3s", "databases": 3}`.

### GET /metrics
Prometheus metrics endpoint.

## Replication

### GET /replication/log?since=N
Get replication log entries since ID N (for replica polling).

## Server Control

### POST /shutdown
Gracefully stop the server (admin only).
