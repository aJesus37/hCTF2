# hCTF2 Installation Guide

## Prerequisites

- **Go 1.24+** - Download from https://go.dev/dl/
- **Task** - Install from https://taskfile.dev or run: `go install github.com/go-task/task/v3/cmd/task@latest`
- **Git** (for cloning)

## Installation

### Option 1: Quick Setup (Recommended)

```bash
# Clone the repository
git clone https://github.com/yourusername/hctf2.git
cd hctf2

# Run setup script
chmod +x setup.sh
./setup.sh

# Start the server
./hctf2 --port 8090 --admin-email admin@hctf.local --admin-password changeme
```

### Option 2: Manual Setup

```bash
# Clone the repository
git clone https://github.com/yourusername/hctf2.git
cd hctf2

# Download dependencies
task deps

# Build the application
task build

# Run the server
./hctf2 --port 8090 --admin-email admin@hctf.local --admin-password changeme
```

### Option 3: Development Mode

```bash
# Clone and setup
git clone https://github.com/yourusername/hctf2.git
cd hctf2

# Install dependencies
task deps

# Run directly (without building)
task run
```

## First Run

When you start the server for the first time:

1. The database will be automatically created at `./hctf2.db`
2. Migrations will run automatically
3. If you provide `--admin-email` and `--admin-password`, an admin account will be created
4. The server will start on the specified port (default: 8090)

**Access the application:**
- Homepage: http://localhost:8090
- Challenges: http://localhost:8090/challenges
- SQL Playground: http://localhost:8090/sql
- Admin Panel: http://localhost:8090/admin (requires admin login)

## Creating Your First Challenge

1. Login with your admin credentials
2. Navigate to http://localhost:8090/admin
3. Click "Create Challenge"
4. Fill in the details:
   - Name: "My First Challenge"
   - Description: "Welcome to hCTF2!"
   - Category: "misc"
   - Difficulty: "easy"
   - Visible: Yes
5. Click "Add Question" to add a flag:
   - Name: "Question 1"
   - Description: "Find the hidden flag"
   - Flag: "FLAG{welcome_to_hctf2}"
   - Points: 100
   - Case Sensitive: No
6. The flag mask will auto-generate as `FLAG{******************}`

## Configuration

For detailed configuration options, see [CONFIGURATION.md](CONFIGURATION.md).

**Quick options:**
```bash
./hctf2 [options]

Options:
  --port int                Server port (default: 8090)
  --host string            Server host (default: 0.0.0.0)
  --database-path string   Database path (default: data/hctf2.db)
  --jwt-secret string      JWT signing secret
  --admin-email string     Admin email for first-time setup
  --admin-password string  Admin password for first-time setup
```

**Environment variables:**
```bash
export PORT=8090
export HOST=0.0.0.0
export DATABASE_PATH=data/hctf2.db
export JWT_SECRET=your-secret
export ADMIN_EMAIL=admin@example.com
export ADMIN_PASSWORD=changeme

./hctf2
```

## Production Deployment

For complete production setup and operations, see [OPERATIONS.md](OPERATIONS.md).

**Quick systemd setup:**

```bash
# Build
task build

# Create system user
sudo useradd -r hctf2

# Create data directory
sudo mkdir -p /var/lib/hctf2
sudo chown hctf2:hctf2 /var/lib/hctf2

# Copy binary
sudo cp hctf2 /usr/local/bin/

# Create systemd service (see OPERATIONS.md)
# Then enable and start:
sudo systemctl daemon-reload
sudo systemctl enable hctf2
sudo systemctl start hctf2
```

For Nginx, Docker, and detailed configuration, see [OPERATIONS.md](OPERATIONS.md).

### Build Optimized Binary

```bash
# Build optimized binary
task build-prod

# The binary will be at: hctf2-linux-amd64
```

### Systemd Service

Create `/etc/systemd/system/hctf2.service`:

```ini
[Unit]
Description=hCTF2 Platform
After=network.target

[Service]
Type=simple
User=hctf
WorkingDirectory=/opt/hctf2
ExecStart=/opt/hctf2/hctf2 --port 8090 --db /var/lib/hctf2/hctf2.db
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable hctf2
sudo systemctl start hctf2
```

### Nginx Reverse Proxy

```nginx
server {
    listen 80;
    server_name ctf.example.com;

    location / {
        proxy_pass http://localhost:8090;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Docker Deployment

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o hctf2 cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/hctf2 .
EXPOSE 8090
CMD ["./hctf2", "--port", "8090"]
```

Build and run:

```bash
docker build -t hctf2 .
docker run -p 8090:8090 -v $(pwd)/data:/root hctf2
```

## Troubleshooting

### Port Already in Use

If port 8090 is already in use, specify a different port:

```bash
./hctf2 --port 3000
```

### Database Locked

If you get a "database is locked" error:
1. Make sure only one instance of hctf2 is running
2. Check file permissions on the database file
3. Delete `.db-shm` and `.db-wal` files if they exist

### Cannot Create Admin User

If the admin user creation fails:
1. The user may already exist - try logging in
2. Delete the database and restart: `rm hctf2.db && ./hctf2 ...`

### Template Errors

If you see template errors:
1. Make sure all template files are present in `internal/views/templates/`
2. Rebuild the application: `make clean && make build`

## Updating

To update to the latest version:

```bash
git pull origin main
task clean
task build
./hctf2 --port 8090
```

Migrations will run automatically on startup.

## Backup

To backup your CTF data:

```bash
# Backup database
cp hctf2.db hctf2.db.backup

# Or use SQLite backup
sqlite3 hctf2.db ".backup hctf2.db.backup"
```

## Security Recommendations

1. **Change default admin password** immediately after first login
2. **Use a strong JWT secret** in production
3. **Use HTTPS** with a reverse proxy (nginx/caddy)
4. **Regular backups** of the database
5. **Keep Go updated** to latest stable version
6. **Monitor logs** for suspicious activity

## Getting Help

- **Documentation**: See README.md
- **Issues**: https://github.com/yourusername/hctf2/issues
- **Discussions**: https://github.com/yourusername/hctf2/discussions
