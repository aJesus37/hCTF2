# Docker Deployment Guide

This guide covers deploying hCTF2 using Docker and Docker Compose.

## Quick Start

**Prerequisites:**
- Docker 20.10+
- Docker Compose v2+

### Development Mode

```bash
git clone https://github.com/yourusername/hctf2.git
cd hctf2

# Start with development config
docker compose -f docker-compose.dev.yml up -d

# Open browser: http://localhost:8090
# Default credentials: admin@hctf.local / changeme
```

### Production Mode

```bash
# Start with production config
docker compose up -d

# View logs
docker compose logs -f

# Access: http://localhost:8090
```

## Configuration

### Environment Variables

Set these in `docker-compose.yml`:

```yaml
environment:
  ADMIN_EMAIL: admin@example.com
  ADMIN_PASSWORD: securepwd123
  JWT_SECRET: $(openssl rand -base64 32)
```

### Port Mapping

Change the port in `docker-compose.yml`:

```yaml
ports:
  - "3000:8080"  # Access on port 3000
```

### Database Location

Data is persisted in Docker volume:

```yaml
volumes:
  - hctf2-data:/data  # Named volume (production)
  - ./data:/data      # Local directory (development)
```

## Common Tasks

### Build and Run

```bash
# Build image
docker build -t hctf2:latest .

# Run container
docker run -d \
  --name hctf2 \
  -p 8080:8080 \
  -e ADMIN_EMAIL=admin@example.com \
  -e ADMIN_PASSWORD=password123 \
  -v hctf2-data:/data \
  hctf2:latest
```

### Manage Services

```bash
# Start services
docker compose up -d

# Stop services
docker compose stop

# Restart services
docker compose restart

# View logs
docker compose logs -f hctf2

# Remove services (keeps volumes)
docker compose down

# Remove everything (including volumes)
docker compose down -v
```

### Database Backup

```bash
# Backup from container
docker cp hctf2:/data/hctf2.db ./backup-$(date +%Y%m%d).db

# Backup from volume
docker run --rm -v hctf2-data:/data -v $(pwd):/backup \
  alpine cp /data/hctf2.db /backup/hctf2.db

# Restore
docker cp backup.db hctf2:/data/hctf2.db
docker restart hctf2
```

## Production Deployment

For production with HTTPS, use Nginx reverse proxy:

```yaml
# docker-compose.prod.yml
version: '3.8'

services:
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/nginx/ssl:ro
    depends_on:
      - hctf2

  hctf2:
    build: .
    environment:
      ADMIN_EMAIL: ${ADMIN_EMAIL}
      ADMIN_PASSWORD: ${ADMIN_PASSWORD}
      JWT_SECRET: ${JWT_SECRET}
    volumes:
      - hctf2-data:/data
```

Then run:
```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

See [OPERATIONS.md](OPERATIONS.md) for full production deployment and monitoring guide.

## Troubleshooting

### Port Already in Use

```bash
# Find what's using port 8090
lsof -i :8090

# Use different port
docker run -p 3000:8080 hctf2:latest
```

### Permission Denied

```bash
# Add user to docker group
sudo usermod -aG docker $USER

# Apply group changes
newgrp docker
```

### Database Locked

```bash
# Restart the container
docker compose restart hctf2

# Check logs
docker compose logs hctf2 | grep -i error
```

### High Memory Usage

```bash
# Check container stats
docker stats

# Restart if needed
docker compose restart hctf2
```

## Reference

**Environment variables:**
- `ADMIN_EMAIL` - Initial admin email
- `ADMIN_PASSWORD` - Initial admin password
- `JWT_SECRET` - JWT signing secret

For detailed configuration, see [CONFIGURATION.md](CONFIGURATION.md).

For production operations, see [OPERATIONS.md](OPERATIONS.md).
