# SparkDB

A lightweight, secure SQLite-powered database server with encryption, authentication, RBAC, and HTTP API.

## Features

- **SQLite-backed** — full SQL via modernc.org/sqlite (pure Go, no CGO)
- **AES-256-GCM encryption** — at-rest database encryption
- **Argon2id hashing** — secure password storage
- **Multi-auth** — JWT, session tokens, and API keys
- **RBAC** — admin, developer, readonly, and auditor roles
- **Query validation** — dangerous query detection and permission enforcement
- **TLS support** — auto-generated self-signed certificates
- **Audit logging** — all queries and actions logged to system database
- **Rate limiting** — configurable per-client request limiting
- **Backup & restore** — on-demand and scheduled backups
- **Prometheus metrics** — `/metrics` endpoint for monitoring
- **Transaction support** — atomic multi-statement execution

## Quick Start

```bash
# Build
go build -o sparkdb ./cmd/sparkdb

# Start the server (creates admin/admin on first run)
./sparkdb start

# Login (get a JWT token)
curl -X POST http://localhost:9600/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin"}'

# Run a query
curl -X POST http://localhost:9600/query \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"query": "SELECT 1"}'
```

## CLI Reference

```
sparkdb start              Start the database server
sparkdb shell              Interactive SQL shell (REPL)
sparkdb create-db <name>   Create a new database
sparkdb create-user <username> <password> <role>  Create a user
sparkdb gen-key            Generate a 32-byte hex encryption key
sparkdb gen-cert           Generate a self-signed TLS certificate
sparkdb encrypt <file>     Encrypt a database file with AES-256-GCM
sparkdb decrypt <file>     Decrypt a database file
sparkdb import <file>      Import data from CSV, JSON, or SQL file
sparkdb export <table>     Export a table to CSV or JSON
sparkdb backup [database]  Create a backup
sparkdb restore <file>     Restore from a backup
sparkdb list-backups       List available backups
```

### Roles

| Role | Permissions |
|------|------------|
| `admin` | all |
| `developer` | query, write, create, alter, delete |
| `readonly` | query |
| `auditor` | audit_log |

### Shell Meta-Commands

| Command | Description |
|---------|-------------|
| `\q` | quit the shell |
| `\?` | show help |
| `\dt` | list all tables |
| `\d <name>` | describe table columns |
| `\db` | list databases |

### Import

```bash
# CSV (auto-creates table)
sparkdb import data.csv

# JSON array
sparkdb import data.json

# SQL script
sparkdb import schema.sql

# Specify formats explicitly
sparkdb import data --format csv
sparkdb import data --format json

# Connect to a different server
sparkdb import data.csv --host 192.168.1.100 --port 9600 --user admin --pass secret
```

### Export

```bash
# Export table to CSV (stdout)
sparkdb export users

# Export to JSON file
sparkdb export users --format json --output users.json

# Export from a specific database
sparkdb export users --db appdb --format csv --output users.csv
```

## API Endpoints

### Authentication

**POST /auth/login**
```json
{"username": "admin", "password": "admin"}
```
Returns `{"token": "<jwt>", "token_type": "bearer", "user": {...}}`

**POST /auth/api-keys**
```json
{"name": "my-key"}
```
Returns `{"api_key": "vl_...", "name": "my-key"}`

### Query

**POST /query**
```json
{"query": "SELECT * FROM users", "database": "main"}
```

**POST /transaction**
```json
{"queries": ["INSERT INTO t (v) VALUES (1)", "SELECT * FROM t"], "database": "main"}
```

### Admin

**POST /admin/users** — create user (admin only)
```json
{"username": "dev1", "password": "securepass", "role": "developer"}
```

**GET /admin/users** — list users (admin only)

**GET /admin/audit-logs** — view audit logs (admin/auditor)

### Operations

**POST /backup** — create backup
```json
{"database": "main"}
```

**POST /restore** — restore from backup
```json
{"backup_file": "backups/main_20260509_120000.db.backup", "database": "main"}
```

**GET /backups** — list backups

### Monitoring

**GET /stats** — runtime statistics (admin/auditor)

**GET /metrics** — Prometheus metrics

**GET /health** — health check

### Auth Methods

Pass authentication via header:

```
Authorization: Bearer <jwt-token>
Authorization: Session <session-token>
X-API-Key: <api-key>
```

## Configuration

Create `config.json` in the working directory or `/etc/sparkdb/config.json`:

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 9600
  },
  "database": {
    "data_dir": ".",
    "wal_mode": true,
    "max_connections": 100
  },
  "auth": {
    "jwt_secret": "your-secret"
  },
  "tls": {
    "enabled": false,
    "auto_cert": true,
    "cert_file": "sparkdb.crt",
    "key_file": "sparkdb.key"
  },
  "encryption": {
    "enabled": false,
    "key": "",
    "key_file": ""
  },
  "backup": {
    "dir": "backups",
    "schedule": "",
    "keep_count": 10
  }
}
```

Use `-c /path/to/config.json` to specify a custom config path.

Environment variables: `SPARKDB_ENCRYPTION_KEY` for the encryption key.

## Development

```bash
git clone <repo>
cd sparkdb

# Build
go build ./...

# Run tests
go test ./...

# Run vet
go vet ./...

# Start dev server
go run ./cmd/sparkdb start
```

Default admin credentials on first run: `admin` / `admin`

### Project Structure

```
cmd/sparkdb/           CLI entry point
internal/
  auth/                Authentication (JWT, sessions, API keys, Argon2)
  backup/              Backup and restore
  client/              HTTP API client (used by shell and import/export)
  config/              Configuration loading
  database/            SQLite manager, executor, system schema
  encryption/          AES-256-GCM cipher, TLS certificates
  format/              ASCII table formatting for CLI output
  monitor/             Runtime monitoring and metrics
  query/               Query validation and rate limiting
  rbac/                Role-based access control
  server/              HTTP server, middleware, routes
pkg/api/               Shared API types
```

### Tech Stack

- **Go 1.25+** — no CGO required
- **modernc.org/sqlite** — pure Go SQLite
- **golang-jwt/jwt/v5** — JWT tokens
- **spf13/cobra** — CLI framework
- **spf13/viper** — configuration
- **golang.org/x/crypto** — Argon2id

## License

MIT
