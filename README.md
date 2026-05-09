# SparkDB

A lightweight, secure SQLite-powered database server with encryption, authentication, role-based access control, and an HTTP API.

## Features

- **SQLite-backed** -- full SQL support via modernc.org/sqlite (pure Go, no CGO)
- **AES-256-GCM encryption** -- at-rest database encryption with key management
- **Argon2id password hashing** -- memory-hard, timing-safe password storage
- **Multi-factor authentication** -- JWT Bearer tokens, session tokens, and API keys
- **Role-based access control** -- admin, developer, readonly, and auditor roles with granular permissions
- **Query validation** -- dangerous query detection and permission enforcement
- **TLS support** -- auto-generated self-signed certificates or custom CA-signed certs
- **Audit logging** -- all queries and administrative actions logged to the system database
- **Rate limiting** -- configurable per-client request throttling
- **Account lockout** -- brute-force protection with configurable thresholds and lockout duration
- **Backup and restore** -- on-demand and scheduled backups with retention policies
- **Prometheus metrics** -- `/metrics` endpoint for monitoring and alerting
- **Transaction support** -- atomic multi-statement execution
- **Web console** -- built-in management UI for databases, users, API keys, backups, and audit logs
- **Primary/replica replication** -- query-log-based replication between SparkDB instances

## Quick Start

```bash
# Build from source
go build -o sparkdb ./cmd/sparkdb

# Start the server
./sparkdb start

# Authenticate and get a JWT token
curl -X POST http://localhost:9600/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin"}'

# Run a query
curl -X POST http://localhost:9600/query \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"query": "SELECT 1", "database": "main"}'
```

On first run, SparkDB creates a default `admin` user with password `admin`. **Change this password immediately in production.**

## Installation

### From Source

```bash
git clone <repo-url>
cd sparkdb
go build -o sparkdb ./cmd/sparkdb
./sparkdb start
```

### Docker

```bash
# Build the image
docker build -t sparkdb .

# Create a config file (see Configuration section)
cp .env.example .env
# Edit .env to set SPARKDB_AUTH_JWT_SECRET

# Run with Docker
docker run -d \
  --name sparkdb \
  -p 9600:9600 \
  -v sparkdb-data:/data \
  -v sparkdb-backups:/backups \
  -v $(pwd)/.env:/run/secrets/.env \
  --env-file .env \
  sparkdb

# Or use docker-compose
cp .env.example .env
# Edit .env with your settings
docker compose up -d
```

## Configuration

SparkDB is configured through a JSON configuration file, environment variables, or both. Environment variables override config file values.

### Configuration File

Create `config.json` in the working directory or `/etc/sparkdb/config.json`:

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 9600,
    "allowed_origins": ["https://myapp.example.com"]
  },
  "database": {
    "data_dir": "/data",
    "wal_mode": true,
    "max_connections": 100
  },
  "auth": {
    "jwt_secret": "your-strong-random-secret-min-32-chars"
  },
  "tls": {
    "enabled": true,
    "auto_cert": true,
    "cert_file": "/etc/sparkdb/sparkdb.crt",
    "key_file": "/etc/sparkdb/sparkdb.key"
  },
  "encryption": {
    "enabled": false,
    "key": "",
    "key_file": ""
  },
  "backup": {
    "dir": "/backups",
    "schedule": "",
    "keep_count": 10
  },
  "replication": {
    "role": "standalone",
    "primary_url": "",
    "api_key": "",
    "poll_interval": 5
  }
}
```

Use `-c /path/to/config.json` to specify a custom config path.

### Environment Variables

All config values can be set via environment variables with the `SPARKDB_` prefix:

| Variable | Description |
|----------|-------------|
| `SPARKDB_AUTH_JWT_SECRET` | JWT signing secret (min 32 chars, REQUIRED in production) |
| `SPARKDB_ENCRYPTION_KEY` | 32-byte hex key for database encryption |
| `SPARKDB_SERVER_HOST` | Server bind address (default: 0.0.0.0) |
| `SPARKDB_SERVER_PORT` | Server port (default: 9600) |
| `SPARKDB_SERVER_ALLOWED_ORIGINS` | Comma-separated CORS origins |
| `SPARKDB_DATABASE_DATA_DIR` | Database storage directory |
| `SPARKDB_BACKUP_DIR` | Backup storage directory |
| `SPARKDB_TLS_ENABLED` | Enable TLS (true/false) |
| `SPARKDB_REPLICATION_ROLE` | Replication role: primary, replica, or standalone (default) |
| `SPARKDB_REPLICATION_PRIMARY_URL` | Primary URL (required for replica role) |
| `SPARKDB_REPLICATION_API_KEY` | API key for replica authentication to primary |
| `SPARKDB_REPLICATION_POLL_INTERVAL` | Replica poll interval in seconds (default: 5) |

### Docker Secrets

When running in Docker, use a `.env` file (see `.env.example`) with `--env-file .env` or the `env_file` directive in docker-compose.yml.

## Security

### Authentication

SparkDB supports three authentication methods:

1. **JWT Bearer tokens** -- short-lived tokens (default 24h TTL) signed with HMAC-SHA256
2. **Session tokens** -- server-managed sessions stored as SHA-256 hashes in the system database
3. **API keys** -- long-lived keys prefixed with `vl_`, stored as SHA-256 hashes

All passwords are hashed with Argon2id (memory: 64MB, time: 3, threads: 4).

### Rate Limiting

- Login attempts: 5 failures trigger a 15-minute account lockout (configurable)
- API requests: 60 requests per minute per client (configurable in code)

### Encryption at Rest

Database files can be encrypted with AES-256-GCM. Generate a key with:

```bash
sparkdb gen-key
```

Set the key in config (`encryption.key`) or environment (`SPARKDB_ENCRYPTION_KEY`).

### TLS

Enable TLS for encrypted connections:

```json
{
  "tls": {
    "enabled": true,
    "auto_cert": true
  }
```

Auto-generated certificates use ECDSA P-384 and are valid for 10 years. For production, replace with CA-signed certificates.

### API Key Protection

- API keys are encrypted at rest using AES-256-GCM with a key derived from the JWT secret
- Full keys are shown only once on creation
- Re-display requires entering your account password
- Keys are hashed with SHA-256 for authentication lookups

### CORS

By default, CORS allows all origins. Restrict in production by setting `server.allowed_origins`:

```json
"allowed_origins": ["https://app.example.com", "https://admin.example.com"]
```

Or via environment: `SPARKDB_SERVER_ALLOWED_ORIGINS=https://app.example.com`

### Security Headers

When running behind a reverse proxy (nginx, Caddy, Traefik), add these headers:

```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Strict-Transport-Security: max-age=31536000; includeSubDomains
Content-Security-Policy: default-src 'self'
```

## CLI Reference

```
sparkdb start              Start the database server
sparkdb shell              Interactive SQL shell (REPL)
sparkdb query <sql>        Run a single SQL query and exit
sparkdb create-db <name>   Create a new database
sparkdb create-user <username> <password> <role>  Create a user
sparkdb gen-key            Generate a 32-byte hex encryption key
sparkdb gen-cert           Generate a self-signed TLS certificate
sparkdb encrypt <file>     Encrypt a database file with AES-256-GCM
sparkdb decrypt <file>     Decrypt a database file
sparkdb import <file>      Import data from CSV, JSON, or SQL file
sparkdb export <table>     Export a table to CSV or JSON
sparkdb backup <database>  Create a backup
sparkdb restore <file>     Restore from a backup
sparkdb list-backups       List available backups
sparkdb stop               Gracefully stop the server
```

Credentials can be passed to CLI commands with `--user`, `--pass`, or `--api-key`. If omitted, the CLI prompts for credentials or uses defaults (not recommended for production).

## Shell Meta-Commands

| Command | Description |
|---------|-------------|
| `\q` | Quit the shell |
| `\?` | Show help |
| `\dt` | List all tables in current database |
| `\d <name>` | Describe table columns |
| `\use <db>` | Switch to a different database |
| `\db` | Show current database |
| `\list` | List all databases |

## Roles and Permissions

| Role | Permissions |
|------|-------------|
| `admin` | All permissions |
| `developer` | query, write, create, alter, delete |
| `readonly` | query only |
| `auditor` | audit_log access |

## API Endpoints

### Authentication

**POST /auth/login**
```json
{"username": "admin", "password": "admin"}
```
Returns `{"token": "<jwt>", "token_type": "bearer", "user": {...}}`

**POST /auth/api-keys** -- Create a new API key (admin only)
```json
{"name": "my-key"}
```
Returns `{"api_key": "vl_...", "name": "my-key"}`

**POST /auth/api-keys/{id}/reveal** -- Re-display an API key (requires password)
```json
{"password": "your-password"}
```

**GET /auth/api-keys** -- List API keys

### Query

**POST /query**
```json
{"query": "SELECT * FROM users", "database": "main"}
```

**POST /transaction**
```json
{"queries": ["INSERT INTO t (v) VALUES (1)", "SELECT * FROM t"], "database": "main"}
```

### Administration

**POST /admin/users** -- Create user (admin only)
```json
{"username": "dev1", "password": "securepass", "role": "developer"}
```

**GET /admin/users** -- List users (admin only)

**DELETE /admin/users/{id}** -- Delete user (admin only)

**PUT /admin/users/{id}/role** -- Change user role (admin only)
```json
{"role": "readonly"}
```

**GET /admin/audit-logs** -- View audit logs (admin/auditor)

### Operations

**POST /backup** -- Create a backup (admin only)
```json
{"database": "main"}
```

**DELETE /backups/{name}** -- Delete a backup (admin only)

**POST /restore** -- Restore from backup (admin only)
```json
{"backup_file": "main_20260509_120000.db.backup", "database": "main"}
```

**GET /backups** -- List backups

**GET /databases** -- List databases

**GET /stats** -- Server statistics

### Health and Monitoring

**GET /health** -- Health check

**GET /metrics** -- Prometheus metrics endpoint

### Authentication Methods

Pass authentication via HTTP header:

```
Authorization: Bearer <jwt-token>
Authorization: Session <session-token>
X-API-Key: <api-key>
```

## Import and Export

### Import

```bash
# CSV (auto-creates table from filename)
sparkdb import data.csv

# JSON array
sparkdb import data.json

# SQL script
sparkdb import schema.sql

# Specify format explicitly
sparkdb import data --format csv

# Connect to remote server
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

## Replication

SparkDB supports primary/replica replication for high availability and read scaling. Every write query executed on the primary is logged to a `replication_log` table. Replicas poll the primary and apply changes locally.

### Setup

**Primary** (default configuration):
```bash
SPARKDB_REPLICATION_ROLE=primary ./sparkdb start
```

**Replica**:
```bash
SPARKDB_REPLICATION_ROLE=replica \
  SPARKDB_REPLICATION_PRIMARY_URL=http://primary:9600 \
  SPARKDB_REPLICATION_API_KEY=vl_... \
  ./sparkdb start
```

The replica requires an API key with admin privileges on the primary. The replica polls the primary every 5 seconds (configurable via `replication.poll_interval`) for new log entries and applies them in order.

### How It Works

1. The primary logs all write queries (INSERT, UPDATE, DELETE, CREATE, ALTER, DROP) to `replication_log` with an auto-incrementing ID
2. The replica calls `GET /replication/log?since=<last_id>` to fetch new entries
3. Entries are applied sequentially to the replica's databases
4. The replica tracks progress in a `replication_state` table
5. SELECT and PRAGMA queries are not replicated

### Limitations

- Replication is asynchronous (not real-time)
- All databases on the primary are replicated to the replica
- The replica should be treated as read-only for user access
- DDL changes (CREATE/ALTER/DROP) are replicated

## Web Console

SparkDB includes a built-in web management console served at `http://localhost:9600/`. The console provides:

- Dashboard with server statistics and database storage visualization
- SQL query editor with results export (CSV, JSON)
- Database management (create, drop, view tables, export tables)
- User and role management
- API key management with password-protected reveal
- Backup management (create, restore, delete)
- Audit log viewer with search and filtering

## Development

```bash
git clone <repo-url>
cd sparkdb

# Build
go build ./...

# Run tests
go test ./...

# Run vet
go vet ./...

# Start development server
go run ./cmd/sparkdb start
```

### Project Structure

```
cmd/sparkdb/           CLI entry point
internal/
  auth/                Authentication (JWT, sessions, API keys, Argon2)
  backup/              Backup and restore
  client/              HTTP API client (used by shell, import, export)
  config/              Configuration loading and validation
  database/            SQLite manager, executor, system schema
  encryption/          AES-256-GCM cipher, TLS certificates
  format/              ASCII table formatting for CLI output
  monitor/             Runtime monitoring and Prometheus metrics
  query/               Query validation, type detection, rate limiting
  rbac/                Role-based access control
  replication/         Query-log-based primary/replica replication
  server/              HTTP server, middleware, route handlers
  web/                 Embedded web console (static assets)
pkg/api/               Shared API types
```

### Tech Stack

- Go 1.25+ (no CGO required)
- modernc.org/sqlite -- pure Go SQLite driver
- golang-jwt/jwt/v5 -- JWT authentication
- spf13/cobra -- CLI framework
- spf13/viper -- configuration management
- golang.org/x/crypto -- Argon2id password hashing

## Deployment

### Production Checklist

1. Set a strong `SPARKDB_AUTH_JWT_SECRET` (minimum 32 random characters)
2. Enable TLS with CA-signed certificates
3. Restrict CORS origins to your application domain
4. Change the default admin password immediately
5. Configure firewall rules to restrict access to port 9600
6. Set up regular backups with `backup.schedule`
7. Enable database encryption with `sparkdb gen-key`
8. Use a reverse proxy (nginx, Caddy) for additional security headers
9. Run as a non-root user (Docker does this automatically)
10. Monitor via the `/metrics` Prometheus endpoint

### Docker Production Deployment

```bash
# Generate a strong JWT secret
openssl rand -hex 32

# Create .env file
echo "SPARKDB_AUTH_JWT_SECRET=$(openssl rand -hex 32)" > .env
echo "SPARKDB_TLS_ENABLED=true" >> .env
echo "SPARKDB_ENCRYPTION_KEY=$(./sparkdb gen-key 2>&1)" >> .env

# Deploy
docker compose up -d
```

## License

MIT
