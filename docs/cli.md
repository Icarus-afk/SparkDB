# CLI Reference

## Usage

```
sparkdb [command] [flags]
```

Global flags:

```
-c, --config string   path to config file
```

## Commands

### init
Initialize a SparkDB project.

```bash
sparkdb init [flags]
```

Flags:
- `--dir` — project directory (default: current directory)
- `--port` — server port (default: 9600)
- `--gen-cert` — generate a self-signed TLS certificate
- `--gen-key` — generate an encryption key and enable encryption
- `--data-dir` — database storage directory
- `--backup-dir` — backup storage directory

Creates `config.json`, `data/`, `backups/`, and optionally TLS cert/key and encryption key.

### start
Start the database server.

```bash
sparkdb start -c config.json
```

### health
Check server health (useful for Docker HEALTHCHECK).

```bash
sparkdb health
sparkdb health --url http://localhost:9600/health
```

### shell
Interactive SQL shell (REPL).

```bash
sparkdb shell --host localhost --port 9600 --user admin --pass admin
sparkdb shell --api-key vl_...
sparkdb shell --db mydb
```

### query
Run a single SQL query and exit.

```bash
sparkdb query "SELECT * FROM users" --db main --user admin --pass admin
```

### create-db
Create a new database.

```bash
sparkdb create-db myapp
```

### create-user
Create a database user.

```bash
sparkdb create-user dev1 Str0ngPass developer
```

Roles: `admin`, `developer`, `readonly`, `auditor`

### gen-key
Generate a 32-byte hex encryption key.

```bash
sparkdb gen-key
```

### gen-cert
Generate a self-signed TLS certificate.

```bash
sparkdb gen-cert --cert sparkdb.crt --key sparkdb.key
```

### encrypt / decrypt
Encrypt or decrypt a database file with AES-256-GCM.

```bash
sparkdb encrypt --key <hex-key> file.db
sparkdb decrypt --key <hex-key> file.db.enc
```

Key can also be set via `SPARKDB_ENCRYPTION_KEY` environment variable.

### import
Import data from CSV, JSON, or SQL file.

```bash
sparkdb import data.csv
sparkdb import data.json
sparkdb import schema.sql
sparkdb import data --format csv
sparkdb import data.csv --host remote --user admin --pass secret
sparkdb import data.csv --db appdb
```

### export
Export a table to CSV or JSON.

```bash
sparkdb export users
sparkdb export users --format json --output users.json
sparkdb export users --db appdb --format csv --output users.csv
```

### backup / restore / list-backups
Create, restore, and list backups.

```bash
sparkdb backup main
sparkdb backup mydb
sparkdb list-backups
sparkdb restore main_20260509_120000.db.backup
sparkdb restore backup_file.db.backup --database mydb
```

### stop
Gracefully stop the server.

```bash
sparkdb stop --host localhost --port 9600 --user admin --pass admin
sparkdb stop --api-key vl_...
```

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

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
