# hCTF2

A modern, beautiful CTF (Capture The Flag) platform built with Go, featuring a unique SQL query interface for exploring challenge data.

## Features

- **Beautiful Dark UI** - Modern interface built with Tailwind CSS
- **Answer Masks** - Shows flag format without revealing the answer (e.g., `FLAG{********************}`)
- **SQL Query Interface** - Explore CTF data using SQL queries (powered by DuckDB WASM)
- **Individual & Team Scoring** - Compete solo or as a team
- **Admin Panel** - Easy challenge and question management
- **Single Binary** - No external dependencies, easy deployment
- **Pure Go** - No CGO required, uses modernc.org/sqlite

## Tech Stack

**Backend:**
- Go 1.24+ with Chi router
- SQLite (via modernc.org/sqlite)
- JWT authentication
- Embedded templates and migrations

**Frontend:**
- HTMX 2.x for interactivity
- Tailwind CSS via CDN
- Alpine.js for client-side reactivity
- DuckDB WASM for SQL queries

## Quick Start

### Prerequisites

**Option 1: Native (Go)**
- Go 1.24 or higher
- Task (https://taskfile.dev) - Install: `go install github.com/go-task/task/v3/cmd/task@latest`

**Option 2: Docker (Recommended for quick setup)**
- Docker 20.10+
- Docker Compose v2+

### Installation

**Option 1: Docker (Quick Start)**

```bash
# Clone the repository
git clone https://github.com/yourusername/hctf2.git
cd hctf2

# Start with docker compose
docker compose -f docker-compose.dev.yml up -d

# Access: http://localhost:8090
# Admin: admin@hctf.local / changeme
```

**Option 2: Native Go**

```bash
# Clone the repository
git clone https://github.com/yourusername/hctf2.git
cd hctf2

# Install dependencies
task deps

# Run the server (creates admin user)
task run
```

The server will start on http://localhost:8090

**Default admin credentials:**
- Email: admin@hctf.local
- Password: changeme

### Building

```bash
# Build single binary
task build

# Run the binary
./hctf2 --port 8090 --admin-email admin@example.com --admin-password yourpassword
```

### Configuration

For detailed configuration options, see [CONFIGURATION.md](CONFIGURATION.md).

**Quick options:**
```bash
./hctf2 [options]

Options:
  --port int                Server port (default 8090)
  --host string            Server host (default 0.0.0.0)
  --database-path string   Database path (default data/hctf2.db)
  --jwt-secret string      JWT signing secret
  --admin-email string     Admin email for first-time setup
  --admin-password string  Admin password for first-time setup
```

## Usage

### Creating Challenges

1. Login as admin
2. Navigate to `/admin`
3. Create a challenge with category, difficulty, and description
4. Add questions with flags and point values
5. Flag masks are auto-generated (e.g., `FLAG{secret}` → `FLAG{******}`)

### SQL Playground

The SQL Playground allows users to query CTF data using standard SQL:

```sql
-- Top 10 users by points
SELECT u.name, SUM(q.points) as total_points
FROM users u
JOIN submissions s ON u.id = s.user_id
JOIN questions q ON s.question_id = q.id
WHERE s.is_correct = 1
GROUP BY u.id, u.name
ORDER BY total_points DESC
LIMIT 10;
```

All queries run client-side in the browser using DuckDB WASM, ensuring:
- Zero server load
- No SQL injection risk
- Full SQL feature set (CTEs, window functions, etc.)

## Database Schema

### Core Tables

- **users** - User accounts and authentication
- **teams** - Team collaboration
- **challenges** - Challenge containers
- **questions** - Individual flags within challenges
- **submissions** - Answer attempts
- **hints** - Optional hints for questions
- **hint_unlocks** - Track hint usage

## Documentation

- [QUICKSTART.md](QUICKSTART.md) - Get up and running in 5 minutes
- [CONFIGURATION.md](CONFIGURATION.md) - Configuration options and examples
- [OPERATIONS.md](OPERATIONS.md) - Production deployment and maintenance
- [KNOWN_ISSUES.md](KNOWN_ISSUES.md) - Known bugs and workarounds
- [ARCHITECTURE.md](ARCHITECTURE.md) - Technical architecture and design
- [API.md](API.md) - API endpoint reference
- [DOCKER.md](DOCKER.md) - Docker deployment guide
- [CLAUDE.md](CLAUDE.md) - Instructions for AI assistants

## Development

```bash
# Run in development mode
task run

# Run tests
task test

# Format code
task fmt
```

## Deployment

For production deployment steps, see [OPERATIONS.md](OPERATIONS.md).

### Docker (Recommended)

See [DOCKER.md](DOCKER.md) for comprehensive Docker deployment guide.

```bash
# Development
docker compose -f docker-compose.dev.yml up -d

# Production
docker compose up -d
```

### Native Deployment

```bash
# Build
task build

# Run
./hctf2 --port 8080 --admin-email admin@example.com --admin-password password
```

See [OPERATIONS.md](OPERATIONS.md) for systemd service setup and full deployment guide.

## Project Structure

```
hctf2/
├── cmd/server/          # Entry point
├── internal/
│   ├── auth/           # Authentication & middleware
│   ├── database/       # Database layer
│   ├── handlers/       # HTTP handlers
│   ├── models/         # Data models
│   └── views/          # Templates & static files
├── migrations/         # SQL migrations
├── Taskfile.yml
└── README.md
```

## Roadmap

### Phase 1 (MVP) ✅
- User authentication
- Challenge browsing
- Flag submission with masks
- Scoreboard
- Admin panel
- SQL query interface

### Phase 2
- Hints system
- Team management
- File uploads
- Markdown support

### Phase 3
- Dynamic scoring
- Regex flag validation
- Challenge dependencies
- Import/export challenges

## License

MIT License - see LICENSE file for details

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## Credits

Built with love for the CTF community.
