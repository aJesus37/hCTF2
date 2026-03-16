# Operations Guide

## Quick Start

For detailed configuration options, see [CONFIGURATION.md](CONFIGURATION.md).

```bash
docker compose up -d
```

Open http://localhost:8090 and log in with the admin credentials you configured.

## Deployment

### Pre-Deployment Checklist

- [ ] JWT secret generated and set (`JWT_SECRET` env var)
- [ ] Admin user credentials configured
- [ ] Firewall rules configured (expose port 8090 or reverse proxy)
- [ ] Backup strategy planned
- [ ] SSL/TLS certificate ready (for reverse proxy)

### Docker Compose (recommended)

Create a `docker-compose.yml`:

```yaml
services:
  hctf:
    image: ghcr.io/ajesus37/hCTF:latest
    container_name: hctf
    ports:
      - "8090:8090"
    volumes:
      - hctf-data:/data
    command:
      - serve
      - --port
      - "8090"
      - --db
      - /data/hctf.db
      - --admin-email
      - admin@hctf.local
      - --admin-password
      - changeme
    environment:
      - JWT_SECRET=${JWT_SECRET:-change-me-in-production}
    restart: unless-stopped

volumes:
  hctf-data:
```

Start, stop, and view logs:

```bash
docker compose up -d        # start
docker compose down          # stop
docker compose logs -f       # follow logs
docker compose restart       # restart
```

### Docker Run (single command)

```bash
docker run -d \
  --name hctf \
  -p 8090:8090 \
  -v hctf-data:/data \
  -e JWT_SECRET="$(openssl rand -base64 32)" \
  ghcr.io/ajesus37/hCTF:latest \
  serve --db /data/hctf.db \
    --admin-email admin@hctf.local \
    --admin-password changeme
```

### Building the image locally

```bash
docker build -t hctf .
docker compose up -d
```

### Reverse Proxy Setup (nginx)

**Configuration file `/etc/nginx/sites-available/hctf`:**

```nginx
upstream hctf {
    server 127.0.0.1:8080;
    keepalive 32;
}

server {
    listen 80;
    server_name ctf.example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name ctf.example.com;

    ssl_certificate /etc/letsencrypt/live/ctf.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/ctf.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    client_max_body_size 10M;

    location / {
        proxy_pass http://hctf;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_redirect off;
    }
}
```

Enable it:
```bash
sudo ln -s /etc/nginx/sites-available/hctf /etc/nginx/sites-enabled/
sudo nginx -s reload
```

## Monitoring

### Health Checks

hCTF doesn't yet have a dedicated health check endpoint, but you can verify it's running:

```bash
curl -I http://localhost:8090/
# Should return 200 or 302 (redirect to login)
```

### Server Logs

```bash
docker compose logs -f         # follow all logs
docker compose logs --tail 100 # last 100 lines
docker logs -f hctf           # follow by container name
```

### Key Log Messages

- **"Database migration applied"** - Normal startup, migrations running
- **"Server listening"** - Server started successfully
- **"ERROR: database locked"** - Database concurrency issue
- **"ERROR: invalid token"** - JWT validation failure
- **"404 Not Found"** - Request to non-existent endpoint

### Metrics & Telemetry

hCTF uses **OpenTelemetry** for instrumentation. The telemetry package (`internal/telemetry/`) initializes a tracer and meter on startup.

#### Instrumented Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `http_requests_total` | Counter | Total HTTP requests, labelled by method, path, status |
| `http_request_duration_seconds` | Histogram | Request duration in seconds, labelled by method and path |
| `active_users` | UpDownCounter | Active user count (defined but not yet incremented) |
| `database_queries_total` | Counter | Total database queries |

`http_requests_total` and `http_request_duration_seconds` are recorded automatically via `telemetry.Middleware`. `active_users` and `database_queries_total` are defined but not yet incremented in the current implementation.

#### Enabling Trace Output

Set the environment variable to print traces to stdout (useful for debugging):

```bash
OTEL_EXPORTER_STDOUT=true ./hctf
```

#### Exporting to an OTEL Collector

To ship traces and metrics to a backend (Jaeger, Grafana Tempo, Datadog, etc.):

1. Run an OpenTelemetry Collector alongside hCTF
2. Configure the collector endpoint via environment variable:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://collector:4317 ./hctf
```

Note: OTLP export requires adding the OTLP exporter package to the binary. Currently only stdout export is wired up.

#### Prometheus / Grafana

The current implementation does not expose a `/metrics` Prometheus scrape endpoint. To add one:

1. Add `go.opentelemetry.io/otel/exporters/prometheus` to `go.mod`
2. Register the Prometheus exporter in `internal/telemetry/telemetry.go`
3. Expose `/metrics` route in `main.go`

Until then, monitor via log aggregation (see **Server Logs** section above).

#### Recommended Alerts

- **HTTP 5xx rate** > 1% of requests over 5 minutes
- **Request duration p99** > 2 seconds
- **Process restart** (uptime monitoring via container health check)
- **Disk space** < 1GB remaining (SQLite database growth)

## Maintenance

### Database Backups

**Manual backup:**
```bash
cp /var/lib/hctf/hctf.db /backups/hctf-$(date +%Y%m%d-%H%M%S).db
```

**Automated backup (cron):**
```bash
# Add to root crontab: sudo crontab -e
0 2 * * * cp /var/lib/hctf/hctf.db /backups/hctf-$(date +\%Y\%m\%d).db
# Keep last 30 days
30 2 * * * find /backups -name "hctf-*.db" -mtime +30 -delete
```

**Using rsync (remote backup):**
```bash
# On backup server, add to crontab:
0 3 * * * rsync -av -e ssh \
  hctf@prod-server:/var/lib/hctf/hctf.db \
  /backups/prod/hctf-$(date +\%Y\%m\%d).db
```

### Database Integrity Checks

**Weekly integrity check:**
```bash
sqlite3 /var/lib/hctf/hctf.db "PRAGMA integrity_check;"
```

**Schedule with cron:**
```bash
# Sunday at 3 AM
0 3 * * 0 sqlite3 /var/lib/hctf/hctf.db "PRAGMA integrity_check;" | \
  grep -q "ok" || echo "Database integrity check failed" | \
  mail -s "hCTF Database Alert" admin@example.com
```

### Database Migrations

Migrations run automatically on application startup. To check migration status:

1. Look for "Database migration applied" in logs
2. Verify all migrations in `internal/database/migrations/` have been applied
3. Check database schema: `sqlite3 data/hctf.db ".schema"`

### Performance Monitoring

**Database size:**
```bash
du -h /var/lib/hctf/hctf.db
# Should grow ~1MB per 10,000 submissions
```

**Database free space:**
```bash
sqlite3 /var/lib/hctf/hctf.db "PRAGMA page_count; PRAGMA page_size;"
# (page_count * page_size) = total size
```

**Optimize database:**
```bash
sqlite3 /var/lib/hctf/hctf.db "VACUUM;"
# Reclaims unused space, takes a few seconds
```

### Updating the Application

**Docker Compose:**
```bash
# 1. Backup database
docker compose exec hctf cat /data/hctf.db > /backups/hctf-pre-update.db

# 2. Pull and restart
docker compose pull
docker compose up -d

# 3. Verify
docker compose logs --tail 20
```

Migrations run automatically - no additional steps needed.

## Troubleshooting

### Application Won't Start

**Check logs:**
```bash
docker compose logs --tail 50
```

**Common issues:**

1. **Port already in use:**
   ```bash
   lsof -i :8090
   # Change port mapping in docker-compose.yml or kill the conflicting process
   ```

2. **Database permission denied:**
   The Docker image runs as UID 1000. Ensure the volume is writable by that user.

3. **Container keeps restarting:**
   ```bash
   docker compose logs hctf
   # Check for missing JWT_SECRET or other configuration errors
   ```

### High Memory Usage

**Symptoms:** Process using >1GB memory

**Causes:**
- Too many concurrent users
- Memory leak (unlikely in Go)
- Large cache

**Solutions:**
1. Restart container: `docker compose restart`
2. Monitor memory: `docker stats hctf`
3. Consider load balancing if consistently high

### Database Locked

**Symptoms:** "database is locked" errors in logs

**Causes:**
- Multiple processes accessing same database
- Long-running query holding lock
- Corrupted database

**Solutions:**

1. Check for multiple instances:
   ```bash
   ps aux | grep hctf
   # Should see exactly one process
   ```

2. Restart container:
   ```bash
   docker compose restart
   ```

3. Check database integrity:
   ```bash
   sqlite3 /var/lib/hctf/hctf.db "PRAGMA integrity_check;"
   ```

4. If corrupted, restore from backup:
   ```bash
   cp /backups/hctf-latest.db /var/lib/hctf/hctf.db
   sudo chown hctf:hctf /var/lib/hctf/hctf.db
   ```

### Users Can't Login

**Symptoms:** Login form appears but credentials rejected

**Causes:**
- Admin user not created
- Password changed but not updated
- Database corrupted

**Solutions:**

1. Verify admin user exists:
   ```bash
   sqlite3 /var/lib/hctf/hctf.db "SELECT * FROM users WHERE is_admin = 1;"
   ```

2. Recreate admin by restarting with admin flags in `docker-compose.yml`, then restart:
   ```bash
   docker compose up -d
   ```

### High CPU Usage

**Symptoms:** CPU usage >80% consistently

**Causes:**
- Many active users
- Inefficient query
- Infinite loop (unlikely)

**Solutions:**

1. Check number of requests:
   ```bash
   # Monitor with nginx: tail /var/log/nginx/access.log
   ```

2. Identify slow queries (enable in code if needed)

3. Add caching layer (future feature)

4. Scale horizontally with load balancer

## Security Maintenance

### Regular Security Tasks

**Monthly:**
- [ ] Review admin user list
- [ ] Check for unusual login patterns
- [ ] Update dependencies: `go get -u ./...` + `task build`
- [ ] Review access logs for suspicious activity

**Quarterly:**
- [ ] Rotate JWT secret (requires all users to re-login)
- [ ] Audit database for stale user accounts
- [ ] Review and update firewall rules
- [ ] Test backup restoration process

**Annually:**
- [ ] Security audit
- [ ] Update SSL certificates
- [ ] Review and update security policies
- [ ] Disaster recovery drill

### Rotating JWT Secret

**Process:**
1. Generate new secret: `openssl rand -base64 32`
2. Update environment or config
3. Restart application
4. All users will need to login again (tokens invalidated)

**Command:**
```bash
# 1. Generate new secret
export JWT_SECRET="$(openssl rand -base64 32)"

# 2. Update your .env or docker-compose.yml with the new JWT_SECRET

# 3. Restart
docker compose up -d

# 4. Verify
docker compose logs --tail 10
```

## Disaster Recovery

### Recovery from Database Corruption

```bash
# 1. Stop container
docker compose down

# 2. Restore from backup (copy into the named volume)
docker run --rm -v hctf-data:/data -v $(pwd)/backups:/backups alpine \
  cp /backups/hctf-latest.db /data/hctf.db

# 3. Start application
docker compose up -d

# 4. Verify
curl http://localhost:8090/
```

### Recovery from Lost Admin Password

Restart the container with `--admin-email` and `--admin-password` flags to recreate the admin user. The server upserts the admin on startup, so existing data is preserved.

## Performance Tuning

### Database Query Optimization

**Check for slow queries:**
```bash
# Enable query logging in Go code (future enhancement)
# Monitor with: grep "slow" /var/log/hctf.log
```

**Database optimization:**
```bash
# Ensure indexes exist (applied by migrations)
sqlite3 /var/lib/hctf/hctf.db ".indices"

# Analyze query plans
sqlite3 /var/lib/hctf/hctf.db "EXPLAIN QUERY PLAN SELECT ...;"

# Vacuum database (reclaim space)
sqlite3 /var/lib/hctf/hctf.db "VACUUM;"

# Analyze statistics
sqlite3 /var/lib/hctf/hctf.db "ANALYZE;"
```

### Connection Pool Tuning

Connection pooling is handled internally by Go's `database/sql` package.

**For high concurrency (1000+ users), consider:**
- Multiple application instances behind load balancer
- Upgrading to PostgreSQL (handles higher concurrency)
- Implementing query caching

### Memory Usage Optimization

**Baseline:** ~50MB
**Per concurrent user:** ~100KB
**Estimate for 1000 users:** ~150MB

If memory exceeds available resources:
1. Reduce number of concurrent users per instance
2. Run multiple instances
3. Implement request queuing/rate limiting (future)

## Useful Commands

```bash
# Check if running
docker compose ps

# Restart
docker compose restart

# View recent logs
docker compose logs --tail 100

# Follow logs
docker compose logs -f

# Test connectivity
curl -I http://localhost:8090/

# Backup database from volume
docker compose exec hctf cat /data/hctf.db > hctf-backup.db
```

## Getting Help

- **Application errors:** Check `docker compose logs`
- **Database issues:** Consult [CONFIGURATION.md](CONFIGURATION.md)
- **Architecture questions:** See [ARCHITECTURE.md](ARCHITECTURE.md)
- **API endpoints:** See the OpenAPI docs at `/api/openapi`
