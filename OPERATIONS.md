# Operations Guide

## Quick Start

For detailed configuration options, see [CONFIGURATION.md](CONFIGURATION.md).

**First Time:**
```bash
./hctf2 --admin-email admin@test.com --admin-password password123
```

**Subsequent Runs:**
```bash
./hctf2
```

## Deployment

### Pre-Deployment Checklist

- [ ] Binary built with `task build` or `task build-prod`
- [ ] Database created and migrations applied
- [ ] JWT secret generated and set
- [ ] Admin user created
- [ ] Firewall rules configured
- [ ] Backup strategy planned
- [ ] Log rotation configured
- [ ] SSL/TLS certificate ready (for reverse proxy)
- [ ] Monitoring alerts set up

### Initial Setup Checklist

1. **Create system user:**
   ```bash
   sudo useradd -r -s /bin/false hctf2
   ```

2. **Create data directory:**
   ```bash
   sudo mkdir -p /var/lib/hctf2
   sudo chown hctf2:hctf2 /var/lib/hctf2
   sudo chmod 700 /var/lib/hctf2
   ```

3. **Place binary:**
   ```bash
   sudo cp hctf2 /usr/local/bin/
   sudo chmod 755 /usr/local/bin/hctf2
   ```

4. **Create systemd service:**
   ```bash
   sudo tee /etc/systemd/system/hctf2.service <<EOF
   [Unit]
   Description=hCTF2 CTF Platform
   After=network.target

   [Service]
   Type=simple
   User=hctf2
   WorkingDirectory=/var/lib/hctf2
   ExecStart=/usr/local/bin/hctf2 \\
     --database-path /var/lib/hctf2/hctf2.db \\
     --admin-email admin@example.com \\
     --admin-password \${ADMIN_PASSWORD}
   Restart=on-failure
   RestartSec=10
   StandardOutput=journal
   StandardError=journal

   [Install]
   WantedBy=multi-user.target
   EOF
   ```

5. **Enable and start:**
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable hctf2
   sudo systemctl start hctf2
   ```

6. **Verify running:**
   ```bash
   sudo systemctl status hctf2
   curl http://localhost:8080
   ```

### Docker Deployment

See [DOCKER.md](DOCKER.md) for complete Docker setup including production Dockerfile and docker-compose.

**Quick deployment:**
```bash
docker build -t hctf2 .
docker run -d \
  -p 8080:8080 \
  -v hctf2-data:/data \
  -e ADMIN_EMAIL=admin@example.com \
  -e ADMIN_PASSWORD=password123 \
  hctf2
```

### Reverse Proxy Setup (nginx)

**Configuration file `/etc/nginx/sites-available/hctf2`:**

```nginx
upstream hctf2 {
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
        proxy_pass http://hctf2;
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
sudo ln -s /etc/nginx/sites-available/hctf2 /etc/nginx/sites-enabled/
sudo systemctl restart nginx
```

## Monitoring

### Health Checks

hCTF2 doesn't yet have a dedicated health check endpoint, but you can verify it's running:

```bash
curl -I http://localhost:8080/
# Should return 200 or 302 (redirect to login)
```

### Server Logs

**Systemd logs:**
```bash
sudo journalctl -u hctf2 -f
# -f = follow (tail)
# -n 100 = last 100 lines
# --since "1 hour ago" = filter by time
```

**Docker logs:**
```bash
docker logs -f hctf2
# -f = follow
```

### Key Log Messages

- **"Database migration applied"** - Normal startup, migrations running
- **"Server listening"** - Server started successfully
- **"ERROR: database locked"** - Database concurrency issue
- **"ERROR: invalid token"** - JWT validation failure
- **"404 Not Found"** - Request to non-existent endpoint

### Metrics (Future)

Currently, hCTF2 doesn't expose Prometheus metrics, but the following could be monitored:
- HTTP request count
- Average request duration
- Database query performance
- JWT validation failures
- Active user sessions

## Maintenance

### Database Backups

**Manual backup:**
```bash
cp /var/lib/hctf2/hctf2.db /backups/hctf2-$(date +%Y%m%d-%H%M%S).db
```

**Automated backup (cron):**
```bash
# Add to root crontab: sudo crontab -e
0 2 * * * cp /var/lib/hctf2/hctf2.db /backups/hctf2-$(date +\%Y\%m\%d).db
# Keep last 30 days
30 2 * * * find /backups -name "hctf2-*.db" -mtime +30 -delete
```

**Using rsync (remote backup):**
```bash
# On backup server, add to crontab:
0 3 * * * rsync -av -e ssh \
  hctf2@prod-server:/var/lib/hctf2/hctf2.db \
  /backups/prod/hctf2-$(date +\%Y\%m\%d).db
```

### Database Integrity Checks

**Weekly integrity check:**
```bash
sqlite3 /var/lib/hctf2/hctf2.db "PRAGMA integrity_check;"
```

**Schedule with cron:**
```bash
# Sunday at 3 AM
0 3 * * 0 sqlite3 /var/lib/hctf2/hctf2.db "PRAGMA integrity_check;" | \
  grep -q "ok" || echo "Database integrity check failed" | \
  mail -s "hCTF2 Database Alert" admin@example.com
```

### Database Migrations

Migrations run automatically on application startup. To check migration status:

1. Look for "Database migration applied" in logs
2. Verify all migrations in `internal/database/migrations/` have been applied
3. Check database schema: `sqlite3 data/hctf2.db ".schema"`

### Performance Monitoring

**Database size:**
```bash
du -h /var/lib/hctf2/hctf2.db
# Should grow ~1MB per 10,000 submissions
```

**Database free space:**
```bash
sqlite3 /var/lib/hctf2/hctf2.db "PRAGMA page_count; PRAGMA page_size;"
# (page_count * page_size) = total size
```

**Optimize database:**
```bash
sqlite3 /var/lib/hctf2/hctf2.db "VACUUM;"
# Reclaims unused space, takes a few seconds
```

### Updating the Application

**Process:**
1. Download new binary
2. Stop application: `sudo systemctl stop hctf2`
3. Backup database: `cp /var/lib/hctf2/hctf2.db /backups/hctf2-pre-update.db`
4. Replace binary: `sudo cp hctf2-new /usr/local/bin/hctf2`
5. Start application: `sudo systemctl start hctf2`
6. Verify running: `sudo systemctl status hctf2`
7. Check logs: `sudo journalctl -u hctf2 -n 20`

Migrations run automatically - no additional steps needed.

## Troubleshooting

### Application Won't Start

**Check logs:**
```bash
sudo journalctl -u hctf2 -n 50
```

**Common issues:**

1. **Port already in use:**
   ```bash
   lsof -i :8080
   # Change port or kill process using it
   ```

2. **Database permission denied:**
   ```bash
   ls -la /var/lib/hctf2/
   # Should be owned by hctf2:hctf2
   # Fix: sudo chown -R hctf2:hctf2 /var/lib/hctf2
   ```

3. **Missing data directory:**
   ```bash
   mkdir -p /var/lib/hctf2
   chown hctf2:hctf2 /var/lib/hctf2
   ```

### High Memory Usage

**Symptoms:** Process using >1GB memory

**Causes:**
- Too many concurrent users
- Memory leak (unlikely in Go)
- Large cache

**Solutions:**
1. Restart application: `sudo systemctl restart hctf2`
2. Monitor memory over time: `top -p $(pgrep -f hctf2)`
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
   ps aux | grep hctf2
   # Should see exactly one process
   ```

2. Restart application:
   ```bash
   sudo systemctl restart hctf2
   ```

3. Check database integrity:
   ```bash
   sqlite3 /var/lib/hctf2/hctf2.db "PRAGMA integrity_check;"
   ```

4. If corrupted, restore from backup:
   ```bash
   cp /backups/hctf2-latest.db /var/lib/hctf2/hctf2.db
   sudo chown hctf2:hctf2 /var/lib/hctf2/hctf2.db
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
   sqlite3 /var/lib/hctf2/hctf2.db "SELECT * FROM users WHERE is_admin = 1;"
   ```

2. Create new admin if needed:
   ```bash
   sudo systemctl stop hctf2
   sudo -u hctf2 /usr/local/bin/hctf2 \
     --database-path /var/lib/hctf2/hctf2.db \
     --admin-email admin@example.com \
     --admin-password newpassword
   sudo systemctl start hctf2
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
# Stop service
sudo systemctl stop hctf2

# Backup current database
cp /var/lib/hctf2/hctf2.db /backups/hctf2-pre-rotation.db

# Update systemd service with new JWT_SECRET
sudo systemctl edit hctf2
# In [Service] section, update or add:
# Environment="JWT_SECRET=new-secret-here"

# Restart
sudo systemctl daemon-reload
sudo systemctl start hctf2

# Verify
sudo systemctl status hctf2
```

## Disaster Recovery

### Recovery from Database Corruption

```bash
# 1. Stop application
sudo systemctl stop hctf2

# 2. Restore from backup
cp /backups/hctf2-latest.db /var/lib/hctf2/hctf2.db
sudo chown hctf2:hctf2 /var/lib/hctf2/hctf2.db
chmod 644 /var/lib/hctf2/hctf2.db

# 3. Verify integrity
sqlite3 /var/lib/hctf2/hctf2.db "PRAGMA integrity_check;"

# 4. Start application
sudo systemctl start hctf2

# 5. Verify
curl http://localhost:8080/
```

### Recovery from Lost Admin Password

```bash
# Stop application
sudo systemctl stop hctf2

# Delete admin account
sqlite3 /var/lib/hctf2/hctf2.db \
  "DELETE FROM users WHERE email = 'admin@example.com';"

# Start application and recreate admin
sudo systemctl start hctf2

# Run with admin setup flags
sudo -u hctf2 /usr/local/bin/hctf2 \
  --database-path /var/lib/hctf2/hctf2.db \
  --admin-email admin@example.com \
  --admin-password newpassword
```

## Performance Tuning

### Database Query Optimization

**Check for slow queries:**
```bash
# Enable query logging in Go code (future enhancement)
# Monitor with: grep "slow" /var/log/hctf2.log
```

**Database optimization:**
```bash
# Ensure indexes exist (applied by migrations)
sqlite3 /var/lib/hctf2/hctf2.db ".indices"

# Analyze query plans
sqlite3 /var/lib/hctf2/hctf2.db "EXPLAIN QUERY PLAN SELECT ...;"

# Vacuum database (reclaim space)
sqlite3 /var/lib/hctf2/hctf2.db "VACUUM;"

# Analyze statistics
sqlite3 /var/lib/hctf2/hctf2.db "ANALYZE;"
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
pgrep -f hctf2

# Restart application
sudo systemctl restart hctf2

# View recent logs
sudo journalctl -u hctf2 -n 100

# Check database size
du -h /var/lib/hctf2/hctf2.db

# Create backup
cp /var/lib/hctf2/hctf2.db /backups/hctf2-$(date +%Y%m%d).db

# Test connectivity
curl -I http://localhost:8080/

# Query database
sqlite3 /var/lib/hctf2/hctf2.db "SELECT COUNT(*) as users FROM users;"
```

## Getting Help

- **Application errors:** Check `/var/log/` or systemd logs
- **Database issues:** Consult [CONFIGURATION.md](CONFIGURATION.md)
- **Architecture questions:** See [ARCHITECTURE.md](ARCHITECTURE.md)
- **API endpoints:** See [API.md](API.md)
