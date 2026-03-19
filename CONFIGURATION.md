# Configuration Guide

## Overview

hCTF can be configured via:
- **Command-line flags**: Override all other configuration
- **Environment variables**: Override defaults
- **Defaults**: Built-in sensible defaults for local development

The precedence order is: CLI flags > environment variables > defaults.

## Command-Line Flags

All server flags live under the `serve` subcommand. Run `./hctf serve --help` to see all available flags:

```
./hctf serve [flags]

--port PORT                    Server port (default: 8090)
--db PATH                      SQLite database path (default: ./hctf.db)
--admin-email EMAIL            Initial admin email (required for first setup)
--admin-password PASSWORD      Initial admin password (required for first setup)
--motd TEXT                    Message of the Day displayed below login form
--metrics                      Enable Prometheus /metrics endpoint
--otel-otlp-endpoint ENDPOINT  OTLP exporter endpoint (e.g. localhost:4318)
--smtp-host HOST               SMTP server host
--smtp-port PORT               SMTP server port (default: 587)
--smtp-from EMAIL              SMTP from address
--smtp-user USERNAME           SMTP username
--smtp-password PASSWORD       SMTP password
--base-url URL                 Base URL for links in emails (default: http://localhost:8090)
--jwt-secret SECRET            JWT signing secret (min 32 chars, required in production)
--cors-origins ORIGINS         Comma-separated list of allowed CORS origins (empty = same-origin only)
--umami-script-url URL         Umami analytics script URL (e.g. https://umami.example.com/script.js)
--umami-website-id ID          Umami analytics website ID (from your Umami dashboard)
--submission-rate-limit INT    Max flag submissions per minute per user (default: 5, 0 = unlimited)
--dev                          Enable development mode (allows default JWT secret)
```

Run `./hctf --help` to see the full command tree including CLI subcommands.

### Examples

**Local Development:**
```bash
./hctf serve --port 3000 --admin-email admin@test.com --admin-password testpass123
```

**Production:**
```bash
./hctf serve \
  --port 8080 \
  --db /var/lib/hctf/hctf.db \
  --jwt-secret $(openssl rand -base64 32) \
  --admin-email admin@example.com \
  --admin-password "$(read -sp 'Admin password: ' pass && echo $pass)"
```

## Environment Variables

All configuration can be set via environment variables:

```bash
# Authentication
export JWT_SECRET=$(openssl rand -base64 32)

# SMTP (email / password reset)
export SMTP_HOST=smtp.example.com
export SMTP_FROM=noreply@example.com
export SMTP_USER=apikey
export SMTP_PASSWORD=secret
export BASE_URL=https://ctf.example.com

# Analytics (optional)
export UMAMI_SCRIPT_URL=https://umami.example.com/script.js
export UMAMI_WEBSITE_ID=your-website-id
```

Then run:
```bash
./hctf serve
```

## Server Configuration

### Port and Host

**Note:** Port and host can only be configured via CLI flags (`--port`, `--db` for database path).

**Examples:**

Local only (development):
```bash
./hctf serve --port 3000
```

All interfaces (production):
```bash
./hctf serve --port 8080
```

**Note:** The server listens on all interfaces (0.0.0.0) by default. Use a firewall to restrict access if needed.

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

### CORS Origins

Control which origins can make cross-origin requests to the API.

**Default:** Empty (same-origin only, most secure)

**Configuration options:**
1. CLI flag: `--cors-origins "https://example.com,https://app.example.com"`
2. Environment variable: `CORS_ORIGINS=https://example.com,https://app.example.com`

**Special values:**
- `"*"` - Allow all origins (NOT recommended for production)
- `""` (empty) - Same-origin only (default, recommended)

For most deployments, leave this empty as the web UI is served from the same origin.

**Examples:**

Same-origin only (default, secure):
```bash
./hctf serve
```

Allow specific origins:
```bash
./hctf serve --cors-origins "https://app.example.com,https://admin.example.com"
```

Allow all origins (NOT recommended for production):
```bash
./hctf serve --cors-origins "*"
```

## Database Configuration

### SQLite Database

**Environment Variable:**
- Database path defaults to `./hctf.db` (use `--db` flag to change)

**Notes:**
- Directory must exist before running the application
- Database file is created automatically on first run
- Migrations are applied automatically on startup

**Examples:**

Local development:
```bash
./hctf serve --db data/hctf.db
```

Production:
```bash
./hctf serve --db /var/lib/hctf/hctf.db
```

Docker:
```bash
./hctf serve --db /data/hctf.db
```

### Database Location Setup

For production, create the database directory with proper permissions:

```bash
sudo mkdir -p /var/lib/hctf
sudo chown 1000:1000 /var/lib/hctf
sudo chmod 750 /var/lib/hctf
```

Then run as the application user (UID 1000):
```bash
sudo -u 1000 ./hctf serve --db /var/lib/hctf/hctf.db
```

### Backup Configuration

Database backups should be handled outside the application. Example cron job:

```bash
# Daily backup at 2 AM
0 2 * * * cp /var/lib/hctf/hctf.db /backups/hctf-$(date +\%Y\%m\%d).db
```

## Security Settings

### JWT Secret Configuration

The JWT secret is **required** for production deployments. The server will refuse to start without a proper secret unless `--dev` mode is explicitly enabled.

**Configuration Methods:**

1. **Command-Line Flag:**
   ```bash
   ./hctf serve --jwt-secret "$(openssl rand -base64 32)"
   ```

2. **Environment Variable:**
   ```bash
   export JWT_SECRET="$(openssl rand -base64 32)"
   ./hctf serve
   ```

3. **Development Mode** (insecure, for local development only):
   ```bash
   ./hctf serve --dev
   ```

**Security Requirements:**
- Minimum 32 characters
- Cryptographically secure random string
- Must be identical across all server instances in a cluster
- Never commit to version control
- Rotate periodically in production

**Generate a Secure Secret:**

Using OpenSSL (recommended):
```bash
openssl rand -base64 32
```

Using Python:
```bash
python3 -c "import secrets; print(secrets.token_urlsafe(32))"
```

Using /dev/urandom:
```bash
cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1
```

**Production vs Development:**

| Mode | Command | Behavior |
|------|---------|----------|
| Production (default) | `./hctf serve --jwt-secret <secret>` | Requires valid JWT secret |
| Development | `./hctf serve --dev` | Allows default insecure secret with warning |

**Failure Modes:**

If the JWT secret is not configured in production mode:
```
ERROR: JWT secret is required. Use --dev for development, or set --jwt-secret flag, JWT_SECRET env var. The secret must be at least 32 characters.
```

If the JWT secret is too short:
```
ERROR: Invalid JWT secret: JWT secret must be at least 32 characters
```

## Authentication Configuration

### JWT Token Settings

**Token Expiration:**
- Currently hardcoded to 7 days
- Stored in HttpOnly cookies for security
- Can be modified in `internal/auth/middleware.go` if needed

### Initial Admin Setup

**Required on First Run:**
```bash
./hctf serve \
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
- Currently 7 days per JWT token
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

### Score Evolution Chart

**Current Status:** Enabled by default. Shows a line chart of top competitors' scores over time.

**Features:**
- Visual timeline showing score progression for top 20 users
- Toggle button to show/hide the chart
- Automatically switches between individual and team modes
- Dark/light theme support
- Updates every 30 seconds (synchronized with scoreboard)

**How it works:**
- Background recorder captures score snapshots every 15 minutes
- Data is stored in the `score_history` table
- 90-day retention (old data automatically cleaned up)
- Chart.js renders the visualization client-side

**Configuration:** None required — works out of the box.

### Admin Visibility in Scoreboard

**Current Status:** Admins are hidden from scoreboard by default.

**Configuration:**
- Go to **Admin Dashboard → Settings tab**
- Toggle "Show Admins in Scoreboard"
- Save settings

**Behavior:**
- When **enabled**: Admin users and teams owned by admins appear in both scoreboard and score evolution chart
- When **disabled** (default): Admin users and teams are excluded from scoreboard and chart rankings

This allows CTF organizers to participate without affecting competition rankings.

### Analytics (Umami)

hCTF has native support for [Umami](https://umami.is/) analytics. When configured, the Umami script is injected into every page and the following CTF-specific events are tracked automatically:

| Event | Trigger |
|-------|---------|
| `flag-submit` | Flag submitted (includes `correct: true/false` property) |
| `hint-unlock` | Hint unlocked |
| `competition-register` | Team registered for a competition |
| `sql-playground-run` | SQL Playground query executed |

**Configuration:**

```bash
# Via CLI flags
./hctf serve \
  --umami-script-url https://umami.example.com/script.js \
  --umami-website-id your-website-id

# Via environment variables
export UMAMI_SCRIPT_URL=https://umami.example.com/script.js
export UMAMI_WEBSITE_ID=your-website-id
./hctf serve
```

**Disabled by default** — analytics are only active when both `UMAMI_SCRIPT_URL` and `UMAMI_WEBSITE_ID` are set. Self-hosting Umami is recommended for privacy.

## Production Settings

### Security Hardening Checklist

- [ ] Set strong JWT secret with 256+ bits entropy
- [ ] Use strong admin password (12+ characters)
- [ ] Run behind HTTPS reverse proxy (nginx/Apache)
- [ ] Use firewall to restrict access to the application port if needed
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
docker build -t hctf .
docker run -d \
  --name hctf \
  -p 8080:8080 \
  -e JWT_SECRET="$(openssl rand -base64 32)" \
  -v hctf-data:/data \
  hctf \
  --db /data/hctf.db \
  --admin-email admin@example.com \
  --admin-password password123
```

## Troubleshooting Configuration

### Port Already in Use

```bash
# Check what's using the port
lsof -i :8080

# Use a different port
./hctf serve --port 3000
```

### Database File Permissions

If you get "permission denied" errors:

```bash
# Check permissions (adjust path as needed)
ls -la hctf.db
ls -la /var/lib/hctf/hctf.db

# Fix permissions (if needed)
chmod 644 hctf.db
chmod 755 ./
```

### JWT Token Issues

If users get "invalid token" errors after restart:

1. Clear browser cookies
2. Ensure `JWT_SECRET` is consistent across restarts
3. Check server logs for token validation errors

## SMTP Configuration

### Password Reset Emails

**Command-Line Flags:**
- `--smtp-host` - SMTP server hostname (e.g. smtp.gmail.com)
- `--smtp-port` - SMTP server port (default: 587)
- `--smtp-from` - From address for emails (e.g. noreply@example.com)
- `--smtp-user` - SMTP authentication username
- `--smtp-password` - SMTP authentication password
- `--base-url` - Base URL for reset links (default: http://localhost:8090)

**Environment Variables:**
- `SMTP_HOST` - SMTP server hostname
- `SMTP_FROM` - From address for emails
- `SMTP_USER` - SMTP authentication username
- `SMTP_PASSWORD` - SMTP authentication password

**Examples:**

Using Gmail SMTP:
```bash
./hctf serve \
  --smtp-host smtp.gmail.com \
  --smtp-port 587 \
  --smtp-from noreply@yourdomain.com \
  --smtp-user your-email@gmail.com \
  --smtp-password "your-app-password" \
  --base-url https://ctf.yourdomain.com
```

Using environment variables:
```bash
export SMTP_HOST=smtp.sendgrid.net
export SMTP_FROM=noreply@yourdomain.com
export SMTP_USER=apikey
export SMTP_PASSWORD="your-sendgrid-api-key"
export BASE_URL=https://ctf.yourdomain.com

./hctf serve
```

**Development Mode:**
If SMTP is not configured, password reset links are logged to the console instead of being sent via email. This is useful for local development and testing.

## OpenTelemetry Configuration

### Prometheus Metrics

**Command-Line Flag:**
- `--metrics` - Enable Prometheus /metrics endpoint

**Environment Variable:**
- `OTEL_METRICS_PROMETHEUS=true` - Enable Prometheus metrics

The `/metrics` endpoint serves metrics in Prometheus format, including:
- `http_requests_total` - Total HTTP requests
- `http_request_duration_seconds` - HTTP request duration
- `active_users` - Current active users
- `database_queries_total` - Total database queries

**Example:**
```bash
./hctf serve --metrics --port 8090
curl http://localhost:8090/metrics
```

### OTLP Export

Export traces and metrics to an OpenTelemetry Collector or compatible backend.

**Command-Line Flag:**
- `--otel-otlp-endpoint` - OTLP endpoint (e.g. localhost:4318)

**Environment Variable:**
- `OTEL_EXPORTER_OTLP_ENDPOINT` - OTLP endpoint

**Example with Jaeger:**
```bash
# Start Jaeger
docker run -d --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 16686:16686 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest

# Run hCTF with OTLP export
./hctf serve --otel-otlp-endpoint localhost:4318
```

### Stdout Exporter (Debug)

**Environment Variable:**
- `OTEL_EXPORTER_STDOUT=true` - Log traces to stdout

Useful for debugging OpenTelemetry instrumentation during development.

## Health Check Endpoints

hCTF provides Kubernetes-style health check endpoints:

### Liveness Probe

**Endpoint:** `GET /healthz`

Returns HTTP 200 when the application is running:
```json
{"status":"ok"}
```

Use this for liveness probes in Kubernetes to restart unhealthy containers.

### Readiness Probe

**Endpoint:** `GET /readyz`

Returns HTTP 200 when ready to serve traffic:
```json
{"status":"ready","checks":{"database":"ok"}}
```

Returns HTTP 503 when not ready:
```json
{"status":"not_ready","checks":{"database":"error: connection refused"}}
```

Use this for readiness probes to control traffic routing.

### Kubernetes Example

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8090
  initialDelaySeconds: 10
  periodSeconds: 30

readinessProbe:
  httpGet:
    path: /readyz
    port: 8090
  initialDelaySeconds: 5
  periodSeconds: 10
```

### Docker HEALTHCHECK

Both Dockerfiles include a `HEALTHCHECK` directive.

The production image (scratch-based) uses the built-in `healthcheck` subcommand:

```dockerfile
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/hctf", "healthcheck", "--port", "8090"]
```

The demo image (Alpine-based) uses `wget`:

```dockerfile
HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
    CMD wget -qO- http://localhost:8090/healthz || exit 1
```

You can also run the health check manually from the host:

```bash
./hctf healthcheck --port 8090
```

The command exits 0 when the server is healthy and 1 otherwise.

### Database Migration Failures

If migrations fail on startup:

```bash
# Check database integrity (adjust path as needed)
sqlite3 hctf.db "PRAGMA integrity_check;"
sqlite3 /var/lib/hctf/hctf.db "PRAGMA integrity_check;"

# For corrupted database, restore from backup
cp /backups/hctf-latest.db hctf.db
```

## Example Configurations

### Minimal Development Setup

```bash
./hctf serve \
  --port 3000 \
  --admin-email dev@example.com \
  --admin-password devpass123
```

### Standard Production Setup

```bash
./hctf serve \
  --port 8080 \
  --db /var/lib/hctf/hctf.db \
  --jwt-secret "$(openssl rand -base64 32)" \
  --admin-email admin@ctf.example.com \
  --admin-password "$(read -sp 'Password: ' p && echo $p)"
```

### Docker Production Setup

```bash
docker run -d \
  --name hctf-prod \
  --restart unless-stopped \
  -p 8090:8090 \
  -e JWT_SECRET="$(openssl rand -base64 32)" \
  -v /var/lib/hctf:/data \
  hctf:latest \
  serve --db /data/hctf.db \
  --admin-email admin@example.com \
  --admin-password securepassword123
```

Then use nginx/Apache for TLS termination and load balancing.
