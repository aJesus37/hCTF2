# Configuration Guide

## Overview

hCTF2 can be configured via:
- **Command-line flags**: Override all other configuration
- **Environment variables**: Override defaults
- **Defaults**: Built-in sensible defaults for local development

The precedence order is: CLI flags > environment variables > defaults.

## Command-Line Flags

Run `./hctf2 --help` to see all available flags:

```
--port PORT                    Server port (default: 8080)
--host HOST                    Server host (default: 0.0.0.0)
--database-path PATH           SQLite database path (default: data/hctf2.db)
--jwt-secret SECRET            JWT signing secret (auto-generated if not set)
--admin-email EMAIL            Initial admin email (required for first setup)
--admin-password PASSWORD      Initial admin password (required for first setup)
```

### Examples

**Local Development:**
```bash
./hctf2 --port 3000 --admin-email admin@test.com --admin-password testpass123
```

**Production:**
```bash
./hctf2 \
  --port 8080 \
  --host 0.0.0.0 \
  --database-path /var/lib/hctf2/hctf2.db \
  --jwt-secret $(openssl rand -base64 32) \
  --admin-email admin@example.com \
  --admin-password "$(read -sp 'Admin password: ' pass && echo $pass)"
```

## Environment Variables

All configuration can be set via environment variables:

```bash
# Server
export PORT=8080
export HOST=0.0.0.0

# Database
export DATABASE_PATH=/var/lib/hctf2/hctf2.db

# Authentication
export JWT_SECRET=$(openssl rand -base64 32)
export ADMIN_EMAIL=admin@example.com
export ADMIN_PASSWORD=securepassword123
```

Then run:
```bash
./hctf2
```

## Server Configuration

### Port and Host

**Environment Variables:**
- `PORT` - Server listening port (default: 8080)
- `HOST` - Server listening address (default: 0.0.0.0)

**Examples:**

Local only (development):
```bash
./hctf2 --host 127.0.0.1 --port 3000
```

All interfaces (production):
```bash
./hctf2 --host 0.0.0.0 --port 8080
```

### TLS/HTTPS

**Current Status:** Not yet implemented. The application currently serves over HTTP only.

**For Production:** Use a reverse proxy (nginx, Apache) to handle TLS termination.

Example nginx configuration:
```nginx
server {
    listen 443 ssl http2;
    server_name ctf.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### CORS Settings

**Current Status:** CORS is not explicitly configured. The application serves from a single origin.

## Database Configuration

### SQLite Database

**Environment Variable:**
- `DATABASE_PATH` - Path to SQLite database file (default: data/hctf2.db)

**Notes:**
- Directory must exist before running the application
- Database file is created automatically on first run
- Migrations are applied automatically on startup

**Examples:**

Local development:
```bash
./hctf2 --database-path data/hctf2.db
```

Production:
```bash
./hctf2 --database-path /var/lib/hctf2/hctf2.db
```

Docker:
```bash
./hctf2 --database-path /data/hctf2.db
```

### Database Location Setup

For production, create the database directory with proper permissions:

```bash
sudo mkdir -p /var/lib/hctf2
sudo chown ctf2:ctf2 /var/lib/hctf2
sudo chmod 750 /var/lib/hctf2
```

Then run as the `ctf2` user:
```bash
sudo -u ctf2 ./hctf2 --database-path /var/lib/hctf2/hctf2.db
```

### Backup Configuration

Database backups should be handled outside the application. Example cron job:

```bash
# Daily backup at 2 AM
0 2 * * * cp /var/lib/hctf2/hctf2.db /backups/hctf2-$(date +\%Y\%m\%d).db
```

## Authentication Configuration

### JWT Configuration

**Environment Variable:**
- `JWT_SECRET` - Secret key for signing JWT tokens

**Security Notes:**
- Must be a cryptographically secure random string
- Should be at least 32 bytes (256 bits)
- Never commit to version control
- Must be the same across all server instances

**Generate JWT Secret:**

Using OpenSSL:
```bash
openssl rand -base64 32
```

Using Go:
```bash
go run -c 'package main; import ("crypto/rand"; "encoding/base64"; "fmt"; "io"); func main() { b := make([]byte, 32); io.ReadFull(rand.Reader, b); fmt.Println(base64.StdEncoding.EncodeToString(b)) }'
```

**Token Expiration:**
- Currently hardcoded to 24 hours
- Can be modified in `internal/auth/jwt.go` if needed

### Initial Admin Setup

**Required on First Run:**
```bash
./hctf2 \
  --admin-email admin@example.com \
  --admin-password "securepassword123"
```

These flags set up the initial administrator account. Subsequent runs don't require these flags.

**Security Notes:**
- Admin password should be strong (12+ characters, mixed case, numbers, symbols)
- Change the admin password immediately after first login
- Create additional admin accounts via the admin panel if needed

### Session Management

**Session Duration:**
- Currently 24 hours per JWT token
- Users must re-login after token expiration

**HttpOnly Cookies:**
- JWT tokens stored in HttpOnly cookies
- Prevents JavaScript access to tokens
- Reduces XSS attack surface

## Feature Configuration

### SQL Playground

**Current Status:** Enabled by default. DuckDB (WebAssembly) runs client-side.

**Configuration:** No configuration needed. Works out of the box.

**Limitations:**
- Uses DuckDB WASM (no file I/O)
- Queries run entirely in the browser
- No persistent state between sessions

### Challenge Visibility

**Current Status:** All published challenges are visible to all authenticated users.

**Filtering Options:**
- Category filter (client-side)
- Difficulty filter (client-side)
- Search by name/description (client-side)

### Scoreboard

**Modes:**
- **Individual**: Shows user scores
- **Team**: Shows team scores (if user is on a team)

**Visibility:** Public (visible to all users)

## Production Settings

### Security Hardening Checklist

- [ ] Set strong JWT secret with 256+ bits entropy
- [ ] Use strong admin password (12+ characters)
- [ ] Run behind HTTPS reverse proxy (nginx/Apache)
- [ ] Set `Host: 0.0.0.0` to listen on all interfaces (or specific IP)
- [ ] Use firewall to restrict access to port 8080 if needed
- [ ] Enable database backups (daily minimum)
- [ ] Monitor disk space for database growth
- [ ] Configure log rotation
- [ ] Keep Go runtime updated

### Performance Tuning

**Database:**
- SQLite is suitable for up to ~1000 concurrent users
- For larger deployments, consider PostgreSQL migration
- Enable WAL mode (Write-Ahead Logging) for better concurrency

**Memory:**
- Baseline memory usage: ~50MB
- Add ~100KB per concurrent user
- Estimate for 1000 users: ~150MB

**CPU:**
- Single Go process scales to ~4 CPU cores
- For more, run multiple processes behind a load balancer

### Docker Configuration

See [DOCKER.md](DOCKER.md) for complete Docker setup.

**Quick Start:**
```bash
docker build -t hctf2 .
docker run -d \
  --name hctf2 \
  -p 8080:8080 \
  -e ADMIN_EMAIL=admin@example.com \
  -e ADMIN_PASSWORD=password123 \
  -v hctf2-data:/data \
  hctf2
```

## Troubleshooting Configuration

### Port Already in Use

```bash
# Check what's using the port
lsof -i :8080

# Use a different port
./hctf2 --port 3000
```

### Database File Permissions

If you get "permission denied" errors:

```bash
# Check permissions
ls -la data/hctf2.db

# Fix permissions (if needed)
chmod 644 data/hctf2.db
chmod 755 data/
```

### JWT Token Issues

If users get "invalid token" errors after restart:

1. Clear browser cookies
2. Ensure `JWT_SECRET` is consistent across restarts
3. Check server logs for token validation errors

### Database Migration Failures

If migrations fail on startup:

```bash
# Check database integrity
sqlite3 data/hctf2.db "PRAGMA integrity_check;"

# For corrupted database, restore from backup
cp /backups/hctf2-latest.db data/hctf2.db
```

## Example Configurations

### Minimal Development Setup

```bash
./hctf2 \
  --port 3000 \
  --admin-email dev@example.com \
  --admin-password devpass123
```

### Standard Production Setup

```bash
./hctf2 \
  --port 8080 \
  --host 0.0.0.0 \
  --database-path /var/lib/hctf2/hctf2.db \
  --jwt-secret "$(openssl rand -base64 32)" \
  --admin-email admin@ctf.example.com \
  --admin-password "$(read -sp 'Password: ' p && echo $p)"
```

### Docker Production Setup

```bash
docker run -d \
  --name hctf2-prod \
  --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -e JWT_SECRET="$(openssl rand -base64 32)" \
  -e ADMIN_EMAIL=admin@example.com \
  -e ADMIN_PASSWORD=securepassword123 \
  -v /var/lib/hctf2:/data \
  hctf2:latest
```

Then use nginx/Apache for TLS termination and load balancing.
