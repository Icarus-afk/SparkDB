# Development

## Prerequisites

- Go 1.25+ (no CGO required)

## Setup

```bash
git clone <repo-url>
cd sparkdb
go build ./...
```

## Project Structure

```
cmd/sparkdb/           CLI entry point
├── main.go            Command registration and server start
├── init.go            Project initialization
├── shell.go           Interactive SQL shell (REPL)
├── export.go          Table export (CSV, JSON)
├── import.go          Data import (CSV, JSON, SQL)
├── stop.go            Graceful server shutdown
└── utils.go           Shared utilities

internal/
├── auth/              Authentication (JWT, sessions, API keys, Argon2)
├── backup/            Backup and restore
├── client/            HTTP API client
├── config/            Configuration loading and validation
├── database/          SQLite manager, executor, system schema
├── encryption/        AES-256-GCM cipher, TLS certificates
├── format/            ASCII table formatting for CLI output
├── monitor/           Runtime monitoring and Prometheus metrics
├── query/             Query validation, type detection, rate limiting
├── rbac/              Role-based access control
├── replication/       Query-log-based primary/replica replication
├── server/            HTTP server, middleware, route handlers
└── web/               Embedded web console (static assets)

pkg/api/               Shared API types
```

## Commands

```bash
go build ./...                    # Build all packages
go build -o sparkdb ./cmd/sparkdb # Build the binary
go test ./...                     # Run unit tests
./test.sh                         # Run integration test suite (44 tests)
go vet ./...                      # Static analysis
make lint                         # Vet + staticcheck
make dev                          # Build and start dev server
make fresh                        # Clean DBs and start fresh
```

## Python SDK

A full ORM lives in `sdk/python/`:

```python
from sparkdb import SparkDB, Model, fields

db = SparkDB(url="http://localhost:9600", username="admin", password="admin")

class User(Model):
    name = fields.String(max_length=100)
    email = fields.String(unique=True)
    class Meta:
        database = db

User.create_table()
User.create(name="Alice", email="alice@example.com")
users = User.where(name="Alice").all()
```

See `sdk/python/README.md` for full docs.

## Tech Stack

- **Go 1.25+** — no CGO required
- **modernc.org/sqlite** — pure Go SQLite driver
- **golang-jwt/jwt/v5** — JWT authentication
- **spf13/cobra** — CLI framework
- **spf13/viper** — configuration management
- **golang.org/x/crypto** — Argon2id password hashing

## Web Console

The built-in web console is served at `/` and provides:
- Dashboard with server statistics and database storage visualization
- SQL query editor with results export (CSV, JSON)
- Database management (create, drop, view tables, export tables)
- User and role management
- API key management with password-protected reveal
- Backup management (create, restore, delete)
- Audit log viewer with search and filtering

## Testing

```bash
# Unit tests
go test ./...

# Integration tests
./test.sh
```

The test suite includes:
- Authentication (login, JWT, sessions, API keys)
- Query execution and transaction support
- Role-based access control
- Rate limiting and account lockout
- Backup and restore
- Import and export
- Replication (primary/replica)
- Stress testing

## Docker

```bash
make docker-build   # Build Docker image
make docker-run     # Start Docker Compose services
make docker-stop    # Stop Docker Compose services
```

The Docker image uses Alpine, runs as a non-root user, and includes security hardening.
