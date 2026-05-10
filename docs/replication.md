# Replication

SparkDB supports primary/replica replication for high availability and read scaling. Every write query executed on the primary is logged to a `replication_log` table. Replicas poll the primary and apply changes locally.

## Setup

### Primary

```bash
SPARKDB_REPLICATION_ROLE=primary ./sparkdb start
```

Or in config:
```json
{
  "replication": {
    "role": "primary"
  }
}
```

### Replica

```bash
SPARKDB_REPLICATION_ROLE=replica \
  SPARKDB_REPLICATION_PRIMARY_URL=http://primary:9600 \
  SPARKDB_REPLICATION_API_KEY=vl_... \
  ./sparkdb start
```

Or in config:
```json
{
  "replication": {
    "role": "replica",
    "primary_url": "http://primary:9600",
    "api_key": "vl_...",
    "poll_interval": 5
  }
}
```

The replica requires an API key with admin privileges on the primary.

## How It Works

1. The primary logs all write queries (INSERT, UPDATE, DELETE, CREATE, ALTER, DROP) to `replication_log` with an auto-incrementing ID
2. The replica calls `GET /replication/log?since=<last_id>` to fetch new entries
3. Entries are applied sequentially to the replica's databases
4. The replica tracks progress in a `replication_state` table
5. SELECT and PRAGMA queries are not replicated

## Architecture

```
┌─────────────┐     poll /replication/log     ┌─────────────┐
│   Primary   │ ◄──────────────────────────    │   Replica   │
│             │     (every poll_interval s)    │             │
│ Write Q   replication_log ─────────────────► Apply Q      │
│             │     query-log entries          │             │
└─────────────┘                               └─────────────┘
```

## Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `replication.role` | `standalone` | `primary`, `replica`, or `standalone` |
| `replication.primary_url` | `""` | Primary server URL (required for replica) |
| `replication.api_key` | `""` | Admin API key on the primary (required for replica) |
| `replication.poll_interval` | `5` | Seconds between poll requests |

## Limitations

- Replication is asynchronous (not real-time)
- All databases on the primary are replicated to the replica
- The replica should be treated as read-only for user access
- DDL changes (CREATE/ALTER/DROP) are replicated
- No conflict resolution (primary-only writes assumed)
