# SparkDB

A lightweight, secure SQLite-powered database server with encryption, authentication, role-based access control, HTTP API, and a full-featured web console.

```bash
# Initialize a project and start
sparkdb init && sparkdb start
```

## Quick Start

```bash
go build -o sparkdb ./cmd/sparkdb
./sparkdb init
./sparkdb start

# Log in and run a query
curl -X POST http://localhost:9600/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin"}'
```

On first run, SparkDB creates a default `admin` user with password `admin`. **Change this immediately in production.** The web console shows a setup wizard on first login to help with this.

## Features

- **SQLite-backed** — full SQL via modernc.org/sqlite (pure Go, no CGO)
- **AES-256-GCM encryption** — at-rest database encryption
- **Argon2id password hashing** — memory-hard, timing-safe
- **Password strength validation** — min 8 chars, uppercase, lowercase, digit
- **Multi-factor auth** — JWT tokens, session tokens, and API keys
- **Encrypted API keys** — stored as AES-256-GCM ciphertext at rest
- **Role-based access control** — admin, developer, readonly, auditor
- **TLS support** — auto-generated or custom certificates
- **Audit logging** — all queries and actions logged (async, non-blocking)
- **Rate limiting & account lockout** — brute-force protection
- **Backup and restore** — on-demand and scheduled with automatic pruning
- **Prometheus metrics** — `/metrics` endpoint with P99 latency
- **Web console** — built-in management UI with setup wizard, schema visualizer, inline data editing, and query editor
- **Primary/replica replication** — query-log-based
- **Context-aware query execution** — cancellation and timeout support
- **Parameterized queries** — safe interpolation via `params` field

## Documentation

| Topic | Description |
|-------|-------------|
| [Installation](docs/installation.md) | Build, init, Docker, Makefile targets |
| [Configuration](docs/configuration.md) | Config file, env vars, Docker secrets |
| [Authentication](docs/authentication.md) | JWT, sessions, API keys, user management, password policies |
| [CLI Reference](docs/cli.md) | All commands, flags, shell meta-commands |
| [API Reference](docs/api.md) | REST endpoints, request/response formats |
| [Replication](docs/replication.md) | Primary/replica setup, architecture |
| [Security](docs/security.md) | Encryption, rate limiting, query validation, CORS, password policies |
| [Deployment](docs/deployment.md) | Production checklist, Docker, systemd, reverse proxy |
| [Development](docs/development.md) | Project structure, tech stack, testing, web console |

## License

MIT
