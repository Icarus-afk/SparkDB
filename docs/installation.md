# Installation

## Quick Start

```bash
# Initialize a project (creates config, secrets, directories)
sparkdb init

# Start the server
sparkdb start

# Log in
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

## From Source

**Requirements:** Go 1.25+ (no CGO required)

```bash
git clone <repo-url>
cd sparkdb

# Build the binary
go build -o sparkdb ./cmd/sparkdb

# (Optional) Install system-wide
sudo install -m 755 sparkdb /usr/local/bin/sparkdb

# Initialize and start
sparkdb init
sparkdb start
```

## Using `sparkdb init`

The `init` command sets up a new SparkDB project:

```bash
# Default setup (config.json, data/, backups/ in current dir)
sparkdb init

# Specify project directory
sparkdb init --dir /opt/sparkdb

# Enable TLS with auto-generated certificate
sparkdb init --gen-cert

# Enable database encryption
sparkdb init --gen-key

# Custom port
sparkdb init --port 8080

# All options
sparkdb init --dir /opt/sparkdb --port 9600 --gen-cert --gen-key \
  --data-dir /var/sparkdb/data --backup-dir /var/sparkdb/backups
```

This creates:
- `config.json` with sensible defaults and a random JWT secret
- `data/` directory for database files
- `backups/` directory for backup files
- TLS certificate/key (with `--gen-cert`, generated on first start)
- Encryption key in config (with `--gen-key`; use env var in production)

## Docker

```bash
# Build the image
docker build -t sparkdb .

# Create config
sparkdb init --dir ./sparkdb-prod

# Run
docker run -d \
  --name sparkdb \
  -p 9600:9600 \
  -v $(pwd)/sparkdb-prod/config.json:/etc/sparkdb/config.json \
  -v sparkdb-data:/data \
  -v sparkdb-backups:/backups \
  sparkdb

# Or use docker-compose
docker compose up -d
```

## Makefile Targets

```bash
make build        # Build the binary
make install      # Build and install to /usr/local/bin
make run          # Build and start
make dev          # Build and start (with info message)
make fresh        # Clean databases and start fresh
make test         # Run tests
make docker-build # Build Docker image
```
