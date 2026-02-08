# Docker Deployment Guide for hCTF2

This guide covers deploying hCTF2 using Docker and Docker Compose.

## Quick Start

### Prerequisites

- Docker 20.10+ (https://docs.docker.com/get-docker/)
- Docker Compose v2+ (usually included with Docker Desktop)

### Option 1: Docker Compose (Recommended)

```bash
# Clone the repository
git clone https://github.com/yourusername/hctf2.git
cd hctf2

# Start with docker-compose
docker-compose up -d

# View logs
docker-compose logs -f

# Access the platform
# Open browser: http://localhost:8090
```

**Default Setup:**
- Port: 8090
- Database: Persisted in Docker volume `hctf2-data`
- No admin user created automatically (use API to create first user)

### Option 2: Development Mode

Use the development compose file with admin user pre-configured:

```bash
# Start in development mode
docker-compose -f docker-compose.dev.yml up -d

# Access with default admin credentials
# Email: admin@hctf.local
# Password: changeme
```

### Option 3: Docker Run (Manual)

```bash
# Build the image
docker build -t hctf2:latest .

# Run the container
docker run -d \
  --name hctf2 \
  -p 8090:8090 \
  -v hctf2-data:/app/data \
  hctf2:latest \
  --port 8090 \
  --db /app/data/hctf2.db \
  --admin-email admin@hctf.local \
  --admin-password changeme

# View logs
docker logs -f hctf2
```

## Configuration

### Port Mapping

Change the host port by editing `docker-compose.yml`:

```yaml
ports:
  - "3000:8090"  # Access on port 3000
```

### Admin User Creation

To create an admin user on first run, edit `docker-compose.yml`:

```yaml
command:
  - --port
  - "8090"
  - --db
  - /app/data/hctf2.db
  - --admin-email
  - your-email@example.com  # Change this
  - --admin-password
  - your-secure-password     # Change this
```

### Environment Variables

You can also use environment variables:

```yaml
environment:
  - ADMIN_EMAIL=admin@example.com
  - ADMIN_PASSWORD=securepassword
```

Then update the command:

```yaml
command:
  - --port
  - "8090"
  - --db
  - /app/data/hctf2.db
  - --admin-email
  - ${ADMIN_EMAIL}
  - --admin-password
  - ${ADMIN_PASSWORD}
```

## Data Persistence

### Docker Volume (Production)

By default, the database is stored in a Docker volume:

```bash
# List volumes
docker volume ls

# Inspect volume
docker volume inspect hctf2-data

# Backup database
docker cp hctf2:/app/data/hctf2.db ./backup-$(date +%Y%m%d).db

# Restore database
docker cp ./backup.db hctf2:/app/data/hctf2.db
docker restart hctf2
```

### Local Directory (Development)

In `docker-compose.dev.yml`, data is stored in `./data/`:

```yaml
volumes:
  - ./data:/app/data  # Local directory mount
```

This makes it easy to backup and inspect:

```bash
# Backup
cp data/hctf2.db backup-$(date +%Y%m%d).db

# View with sqlite3
sqlite3 data/hctf2.db "SELECT * FROM users;"
```

## Management Commands

### Start/Stop/Restart

```bash
# Start
docker-compose up -d

# Stop
docker-compose stop

# Restart
docker-compose restart

# Remove (keeps volumes)
docker-compose down

# Remove everything (including volumes)
docker-compose down -v
```

### View Logs

```bash
# Follow logs
docker-compose logs -f

# Last 100 lines
docker-compose logs --tail=100

# Logs for specific service
docker-compose logs hctf2
```

### Shell Access

```bash
# Open shell in container
docker-compose exec hctf2 sh

# Run commands
docker-compose exec hctf2 ls -la /app/data
```

### Rebuild

After code changes:

```bash
# Rebuild and restart
docker-compose up -d --build

# Force rebuild
docker-compose build --no-cache
docker-compose up -d
```

## Production Deployment

### With Nginx Reverse Proxy

Create a docker-compose override file:

```yaml
# docker-compose.prod.yml
version: '3.8'

services:
  nginx:
    image: nginx:alpine
    container_name: hctf2-nginx
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/nginx/ssl:ro
    depends_on:
      - hctf2
    networks:
      - hctf2-network
```

Example `nginx.conf`:

```nginx
events {
    worker_connections 1024;
}

http {
    upstream hctf2 {
        server hctf2:8090;
    }

    server {
        listen 80;
        server_name ctf.example.com;

        location / {
            proxy_pass http://hctf2;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }
    }
}
```

Deploy:

```bash
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

### With SSL (Let's Encrypt)

Use the Nginx + Certbot approach:

```bash
# Install certbot
docker-compose run --rm certbot certonly --webroot -w /var/www/certbot \
  -d ctf.example.com

# Update nginx.conf to use SSL
# Restart
docker-compose restart nginx
```

## Monitoring

### Health Checks

The container includes a health check:

```bash
# Check container health
docker-compose ps

# Manual health check
docker exec hctf2 wget --spider http://localhost:8090/
```

### Resource Usage

```bash
# View stats
docker stats hctf2

# Limit resources in docker-compose.yml
services:
  hctf2:
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 512M
        reservations:
          memory: 256M
```

## Troubleshooting

### Container Won't Start

```bash
# Check logs
docker-compose logs hctf2

# Check if port is available
sudo lsof -i :8090

# Try different port
docker-compose down
# Edit docker-compose.yml port to 3000:8090
docker-compose up -d
```

### Database Issues

```bash
# Reset database (WARNING: deletes all data)
docker-compose down -v
docker-compose up -d

# Backup before reset
docker cp hctf2:/app/data/hctf2.db ./backup.db
```

### Cannot Access Platform

```bash
# Check if container is running
docker ps | grep hctf2

# Check container IP
docker inspect hctf2 | grep IPAddress

# Test from container
docker exec hctf2 wget -O- http://localhost:8090

# Check firewall
sudo ufw status
```

### Build Failures

```bash
# Clean build
docker-compose down
docker system prune -a
docker-compose build --no-cache
docker-compose up -d
```

## Scaling

### Multiple Instances

For high traffic, run multiple instances behind a load balancer:

```yaml
# docker-compose.scale.yml
version: '3.8'

services:
  hctf2:
    deploy:
      replicas: 3
    # Use shared PostgreSQL instead of SQLite
```

Note: SQLite is not suitable for horizontal scaling. Consider PostgreSQL for production.

## Updates

### Update to Latest Version

```bash
# Pull latest code
git pull origin main

# Rebuild and restart
docker-compose up -d --build

# Check version
docker-compose logs hctf2 | grep "Server starting"
```

### Rollback

```bash
# Use specific git commit
git checkout <commit-hash>
docker-compose up -d --build

# Or use specific image tag
docker pull hctf2:v0.1.0
docker tag hctf2:v0.1.0 hctf2:latest
docker-compose up -d
```

## Security Best Practices

1. **Change Default Credentials**
   ```bash
   # Don't use admin@hctf.local / changeme in production
   ```

2. **Use Secrets Management**
   ```yaml
   # docker-compose.yml
   secrets:
     admin_password:
       file: ./secrets/admin_password.txt
   ```

3. **Run as Non-Root**
   - Already configured in Dockerfile (user: hctf)

4. **Limit Privileges**
   ```yaml
   security_opt:
     - no-new-privileges:true
   cap_drop:
     - ALL
   ```

5. **Network Isolation**
   ```yaml
   networks:
     hctf2-network:
       driver: bridge
       internal: true  # No external access
   ```

6. **Regular Backups**
   ```bash
   # Automated backup script
   0 2 * * * docker cp hctf2:/app/data/hctf2.db /backups/hctf2-$(date +\%Y\%m\%d).db
   ```

## FAQ

### Q: Can I use PostgreSQL instead of SQLite?
A: Currently hCTF2 only supports SQLite. PostgreSQL support is planned for Phase 3.

### Q: How do I migrate from one server to another?
A: Copy the database file (`hctf2.db`) and start the container with the same database.

### Q: Can I run this on ARM (Apple Silicon)?
A: Yes! Docker will automatically build for your architecture.

### Q: How do I enable HTTPS?
A: Use a reverse proxy (nginx/caddy) with Let's Encrypt certificates.

### Q: What's the resource usage?
A: Typically 50-100MB RAM, <1% CPU when idle. Increases with concurrent users.

## Additional Resources

- **Docker Docs**: https://docs.docker.com/
- **Docker Compose**: https://docs.docker.com/compose/
- **hCTF2 Docs**: README.md, INSTALL.md
- **Issues**: https://github.com/yourusername/hctf2/issues
