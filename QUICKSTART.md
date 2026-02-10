# hCTF2 Quick Start Guide

## 5-Minute Setup

### Step 1: Install Go
```bash
# Check if Go is installed
go version

# If not, download from https://go.dev/dl/
```

### Step 2: Clone and Build
```bash
git clone https://github.com/yourusername/hctf2.git
cd hctf2
task deps
task build
```

### Step 3: Start Server
```bash
./hctf2 --port 8090 --admin-email admin@hctf.local --admin-password changeme
```

### Step 4: Access Platform
- Open browser: http://localhost:8090
- Login: admin@hctf.local / changeme
- Change password in settings

## Quick Commands

```bash
# Build
task build

# Run (creates admin)
task run

# Run dev (no admin setup)
task run-dev

# Clean
task clean

# Test
task test
```

## Creating Your First Challenge

1. Login as admin
2. Go to `/admin` (not yet implemented - use API)
3. Use the API to create a challenge:

```bash
# Register a user (for testing)
curl -X POST http://localhost:8090/api/auth/register \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "email=test@example.com&password=password123&name=Test User"

# Login as admin
curl -X POST http://localhost:8090/api/auth/login \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "email=admin@hctf.local&password=changeme" \
  -c cookies.txt

# Create a challenge
curl -X POST http://localhost:8090/api/admin/challenges \
  -H "Content-Type: application/json" \
  -b cookies.txt \
  -d '{
    "name": "Welcome Challenge",
    "description": "Your first CTF challenge!",
    "category": "misc",
    "difficulty": "easy",
    "visible": true
  }'

# Create a question (replace CHALLENGE_ID)
curl -X POST http://localhost:8090/api/admin/questions \
  -H "Content-Type: application/json" \
  -b cookies.txt \
  -d '{
    "challenge_id": "CHALLENGE_ID",
    "name": "Find the Flag",
    "description": "The flag is: FLAG{welcome_to_hctf2}",
    "flag": "FLAG{welcome_to_hctf2}",
    "case_sensitive": false,
    "points": 100
  }'
```

## Testing Flag Submission

```bash
# Submit a flag
curl -X POST http://localhost:8090/api/questions/QUESTION_ID/submit \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -b cookies.txt \
  -d "flag=FLAG{welcome_to_hctf2}"
```

## Using SQL Playground

1. Go to http://localhost:8090/sql
2. Try these queries:

```sql
-- View all challenges
SELECT * FROM challenges;

-- Top users
SELECT u.name, SUM(q.points) as points
FROM users u
JOIN submissions s ON u.id = s.user_id
JOIN questions q ON s.question_id = q.id
WHERE s.is_correct = 1
GROUP BY u.id
ORDER BY points DESC;

-- Challenge difficulty distribution
SELECT difficulty, COUNT(*) as count
FROM challenges
GROUP BY difficulty;
```

## Common Issues

### "Port already in use"
```bash
# Use different port
./hctf2 --port 3000
```

### "Database is locked"
```bash
# Only one instance can run at a time
pkill hctf2
./hctf2 --port 8090
```

### "Admin already exists"
```bash
# Skip admin creation
./hctf2 --port 8090
# Or delete database
rm hctf2.db && ./hctf2 --port 8090 --admin-email admin@hctf.local --admin-password changeme
```

## Project Structure

```
hctf2/
├── cmd/server/main.go          # Entry point
├── internal/
│   ├── auth/                   # Authentication
│   ├── database/               # Database layer
│   ├── handlers/               # HTTP handlers
│   ├── models/                 # Data models
│   └── views/                  # Templates & static files
├── migrations/                 # SQL migrations
├── Taskfile.yml                # Build commands
└── README.md                   # Documentation
```

## Next Steps

1. **Customize**: Edit templates in `internal/views/templates/`
2. **Add Challenges**: Use the API or build an admin UI
3. **Configure**: Copy `config.example.yaml` to `config.yaml`
4. **Deploy**: See INSTALL.md for production deployment
5. **Contribute**: See ARCHITECTURE.md to understand the codebase

## Resources

- **Configuration Guide**: [CONFIGURATION.md](CONFIGURATION.md)
- **Full Installation Guide**: [INSTALL.md](INSTALL.md)
- **Architecture Overview**: [ARCHITECTURE.md](ARCHITECTURE.md)
- **Main Documentation**: [README.md](README.md)
- **API Reference**: [API.md](API.md)
- **Operations Guide**: [OPERATIONS.md](OPERATIONS.md)

## Getting Help

- Open an issue: https://github.com/yourusername/hctf2/issues
- Discussions: https://github.com/yourusername/hctf2/discussions

## Tips

- Use `task run` for development (auto-creates admin)
- Database is at `./hctf2.db` (SQLite file)
- Logs go to stdout (use systemd to redirect)
- JWT secret should be changed in production
- Backup database regularly: `cp hctf2.db backup.db`
