# Configuration

SparkDB is configured through a JSON file, environment variables, or both. Environment variables override config file values.

## Configuration File

By default, SparkDB looks for `config.json` in the current directory or `/etc/sparkdb/config.json`. Use `-c` to specify a custom path:

```bash
sparkdb start -c /path/to/config.json
```

### Full Example

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 9600,
    "read_only": false,
    "allowed_origins": ["https://app.example.com"]
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

## Environment Variables

All config values can be set via environment variables with the `SPARKDB_` prefix (dots become underscores):

| Variable | Description |
|----------|-------------|
| `SPARKDB_AUTH_JWT_SECRET` | JWT signing secret (min 32 chars, **REQUIRED in production**) |
| `SPARKDB_ENCRYPTION_KEY` | 32-byte hex key for database encryption |
| `SPARKDB_SERVER_HOST` | Server bind address (default: `0.0.0.0`) |
| `SPARKDB_SERVER_PORT` | Server port (default: `9600`) |
| `SPARKDB_SERVER_READ_ONLY` | Run in read-only mode |
| `SPARKDB_SERVER_ALLOWED_ORIGINS` | Comma-separated CORS origins |
| `SPARKDB_DATABASE_DATA_DIR` | Database storage directory |
| `SPARKDB_DATABASE_WAL_MODE` | Enable WAL mode (true/false) |
| `SPARKDB_DATABASE_MAX_CONNECTIONS` | Max concurrent connections |
| `SPARKDB_BACKUP_DIR` | Backup storage directory |
| `SPARKDB_BACKUP_SCHEDULE` | Cron schedule for automated backups |
| `SPARKDB_BACKUP_KEEP_COUNT` | Number of backups to retain |
| `SPARKDB_TLS_ENABLED` | Enable TLS (true/false) |
| `SPARKDB_TLS_AUTO_CERT` | Auto-generate self-signed cert on startup |
| `SPARKDB_TLS_CERT_FILE` | Path to TLS certificate |
| `SPARKDB_TLS_KEY_FILE` | Path to TLS private key |
| `SPARKDB_ENCRYPTION_ENABLED` | Enable database encryption |
| `SPARKDB_ENCRYPTION_KEY_FILE` | Path to file containing encryption key |
| `SPARKDB_REPLICATION_ROLE` | `primary`, `replica`, or `standalone` |
| `SPARKDB_REPLICATION_PRIMARY_URL` | Primary URL for replica role |
| `SPARKDB_REPLICATION_API_KEY` | API key for replica authentication |
| `SPARKDB_REPLICATION_POLL_INTERVAL` | Poll interval in seconds (default: 5) |

## Configuration Precedence

1. Environment variables (highest precedence)
2. Config file values
3. Defaults

## Docker Secrets

In Docker, use a `.env` file with `--env-file` or docker-compose `env_file`:

```bash
SPARKDB_AUTH_JWT_SECRET=$(openssl rand -hex 32)
SPARKDB_TLS_ENABLED=true
```
