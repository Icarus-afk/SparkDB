# SparkDB

A lightweight, secure SQLite-powered database server with encryption, authentication, role-based access control, and an HTTP API.

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

On first run, SparkDB creates a default `admin` user with password `admin`. **Change this immediately in production.**

## Features

- **SQLite-backed** — full SQL via modernc.org/sqlite (pure Go, no CGO)
- **AES-256-GCM encryption** — at-rest database encryption
- **Argon2id password hashing** — memory-hard, timing-safe
- **Multi-factor auth** — JWT tokens, session tokens, and API keys
- **Role-based access control** — admin, developer, readonly, auditor
- **TLS support** — auto-generated or custom certificates
- **Audit logging** — all queries and actions logged
- **Rate limiting & account lockout** — brute-force protection
- **Backup and restore** — on-demand and scheduled
- **Prometheus metrics** — `/metrics` endpoint
- **Web console** — built-in management UI
- **Primary/replica replication** — query-log-based

## Documentation

| Topic | Description |
|-------|-------------|
| [Installation](docs/installation.md) | Build, init, Docker, Makefile targets |
| [Configuration](docs/configuration.md) | Config file, env vars, Docker secrets |
| [Authentication](docs/authentication.md) | JWT, sessions, API keys, user management |
| [CLI Reference](docs/cli.md) | All commands, flags, shell meta-commands |
| [API Reference](docs/api.md) | REST endpoints, request/response formats |
| [Replication](docs/replication.md) | Primary/replica setup, architecture |
| [Security](docs/security.md) | Encryption, rate limiting, query validation, CORS |
| [Deployment](docs/deployment.md) | Production checklist, Docker, systemd, reverse proxy |
| [Development](docs/development.md) | Project structure, tech stack, testing |

## License

MIT
