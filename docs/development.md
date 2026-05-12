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
├── commands/
│   ├── admin.go       Create-db and create-user commands
│   ├── backup.go      Backup, restore, list-backups commands
│   ├── crypto.go      Gen-key, gen-cert, encrypt, decrypt commands
│   ├── export.go      Table export (CSV, JSON)
│   ├── health.go      Health check command
│   ├── import.go      Data import (CSV, JSON, SQL)
│   ├── root.go        Root command definition
│   ├── shell.go       Interactive SQL shell (REPL)
│   ├── start.go       Server start command
│   ├── stop.go        Graceful server shutdown
│   └── utils.go       Shared utilities

internal/
├── auth/              Authentication (JWT, sessions, API keys, Argon2, rate limiting)
├── backup/            Backup and restore
├── client/            HTTP API client
├── config/            Configuration loading and validation
├── database/          SQLite manager, executor, system schema, migrations
├── encryption/        AES-256-GCM cipher, TLS certificates
├── format/            ASCII table formatting for CLI output
├── monitor/           Runtime monitoring and Prometheus metrics (P99 latency)
├── query/             Query validation, type detection, rate limiting
├── rbac/              Role-based access control
├── replication/       Query-log-based primary/replica replication
├── server/            HTTP server, middleware, route handlers
└── web/               Embedded web console (static assets via embed.FS)

pkg/api/               Shared API types
```

## Commands

```bash
go build ./...                    # Build all packages
go build -o sparkdb ./cmd/sparkdb # Build the binary
go test ./...                     # Run unit tests (200+ tests across all packages)
./test.sh                         # Run integration test suite
go vet ./...                      # Static analysis
make lint                         # Vet + staticcheck
make dev                          # Build and start dev server
make fresh                        # Clean DBs and start fresh
```

## Tech Stack

- **Go 1.25+** — no CGO required
- **modernc.org/sqlite** — pure Go SQLite driver
- **golang-jwt/jwt/v5** — JWT authentication
- **spf13/cobra** — CLI framework
- **spf13/viper** — configuration management
- **golang.org/x/crypto** — Argon2id password hashing

## Web Console

The built-in web console is served at `/` and provides:

- **Setup Wizard** — First-run guided setup to change default admin credentials with password strength meter
- **Dashboard** — Server statistics with database storage visualization cards
- **SQL Query Editor** — Interactive query editor with results export (CSV, JSON), supports parameterized queries
- **Schema Visualizer** — Visual database browser with schema cards showing table columns, types, and inline data editing
- **Database Management** — Create, drop, view tables, export tables
- **User and Role Management** — Create, edit, delete users; change roles and passwords
- **API Key Management** — Create, list, delete API keys with password-protected reveal
- **Backup Management** — Create, restore, and delete backups
- **Audit Log Viewer** — Searchable audit log with filtering and timeline
- **Responsive Sidebar** — Mobile-friendly navigation with collapsible sidebar

UI is built with vanilla JavaScript and CSS, using SVG icons and Inter font. No external JavaScript dependencies.

## Testing

```bash
# Unit tests (200+ tests)
go test ./...

# Integration tests
./test.sh
```

The test suite covers:
- Authentication (login, JWT, sessions, API keys, rate limiting, account lockout)
- Query execution and transaction support with parameterized queries
- Role-based access control (all roles and permissions)
- Password strength validation
- Backup and restore with encryption
- Import and export (CSV, JSON, SQL)
- Replication (primary/replica)
- Configuration loading and validation
- Encryption (AES-256-GCM, TLS cert generation)
- Server endpoints and middleware
- Web console static serving
- Stress testing with concurrent queries

## Docker

```bash
make docker-build   # Build Docker image (Alpine, non-root, hardened)
make docker-run     # Start Docker Compose services
make docker-stop    # Stop Docker Compose services
```

The Docker image uses Alpine, runs as a non-root user, includes a HEALTHCHECK, drops all capabilities, and uses a read-only root filesystem.
