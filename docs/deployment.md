# Deployment

## Production Checklist

1. Generate a strong `SPARKDB_AUTH_JWT_SECRET` (min 32 random chars)
2. Enable TLS with CA-signed certificates
3. Restrict CORS origins to your application domain
4. Change the default admin password immediately (or via setup wizard)
5. Configure firewall rules to restrict access to port 9600
6. Set up regular backups with `backup.schedule` and `backup.keep_count`
7. Enable database encryption with `sparkdb gen-key`
8. Use a reverse proxy (nginx, Caddy) for additional security headers
9. Run as a non-root user (Docker does this automatically)
10. Monitor via the `/metrics` Prometheus endpoint
11. Enable rate limiting to prevent brute-force attacks
12. Set `server.allowed_origins` to restrict CORS in production

## Docker Production Deployment

```bash
# Generate a strong JWT secret
openssl rand -hex 32

# Create .env file
echo "SPARKDB_AUTH_JWT_SECRET=$(openssl rand -hex 32)" > .env
echo "SPARKDB_TLS_ENABLED=true" >> .env
echo "SPARKDB_ENCRYPTION_KEY=$(sparkdb gen-key 2>&1)" >> .env

# Deploy
docker compose up -d
```

The Docker Compose setup includes:
- Read-only root filesystem with writable tmpfs for `/tmp`
- Drop all Linux capabilities (`no-new-privileges:true`, `cap_drop: ALL`)
- Non-root user inside the container
- HEALTHCHECK using `sparkdb health` command (interval: 30s)
- Persistent volumes for data and backups

## Reverse Proxy

### nginx

```nginx
server {
    listen 443 ssl;
    server_name db.example.com;

    ssl_certificate /etc/ssl/certs/sparkdb.crt;
    ssl_certificate_key /etc/ssl/private/sparkdb.key;

    location / {
        proxy_pass http://127.0.0.1:9600;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    add_header X-Content-Type-Options nosniff;
    add_header X-Frame-Options DENY;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains";
    add_header Content-Security-Policy "default-src 'self'";
}
```

## systemd Service

```ini
[Unit]
Description=SparkDB Database Server
After=network.target

[Service]
Type=simple
User=sparkdb
Group=sparkdb
ExecStart=/usr/local/bin/sparkdb start -c /etc/sparkdb/config.json
Restart=always
RestartSec=5
EnvironmentFile=/etc/sparkdb/env
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

## Backups

Automated backups via scheduled interval:

```json
{
  "backup": {
    "dir": "/backups",
    "schedule": "24h",
    "keep_count": 30
  }
}
```

The scheduler also automatically prunes old backups when `keep_count` is set.

Manual backup:
```bash
sparkdb backup main
sparkdb list-backups
sparkdb restore <backup-file> --database main
```

## Monitoring

Prometheus metrics at `/metrics` for:
- Request counts and latency (including P99)
- Active connections
- Database sizes
- Error rates
- Failed logins
- Replication lag (replica role)
- Goroutine count and memory usage

Health check endpoint at `/health` (no auth required):
```bash
curl http://localhost:9600/health
{"status":"ok","checks":{"database":"ok"}}
```
