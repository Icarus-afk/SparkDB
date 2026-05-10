# Deployment

## Production Checklist

1. Generate a strong `SPARKDB_AUTH_JWT_SECRET` (min 32 random chars)
2. Enable TLS with CA-signed certificates
3. Restrict CORS origins to your application domain
4. Change the default admin password immediately
5. Configure firewall rules to restrict access to port 9600
6. Set up regular backups with `backup.schedule`
7. Enable database encryption with `sparkdb gen-key`
8. Use a reverse proxy (nginx, Caddy) for additional security headers
9. Run as a non-root user (Docker does this automatically)
10. Monitor via the `/metrics` Prometheus endpoint

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

### systemd Service

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

Automated backups via cron schedule:

```json
{
  "backup": {
    "dir": "/backups",
    "schedule": "0 2 * * *",
    "keep_count": 30
  }
}
```

Manual backup:
```bash
sparkdb backup main
sparkdb list-backups
sparkdb restore <backup-file>
```

## Monitoring

Prometheus metrics at `/metrics` for:
- Request counts and latency
- Active connections
- Database sizes
- Error rates
- Replication lag (replica role)
