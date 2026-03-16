# Heavens CTF 2 (hCTF)

![hCTF logo](internal/views/static/logo.svg)

[![CI](https://github.com/ajesus37/hCTF/actions/workflows/ci.yml/badge.svg)](https://github.com/ajesus37/hCTF/actions/workflows/ci.yml)
[![Release](https://github.com/ajesus37/hCTF/actions/workflows/release.yml/badge.svg)](https://github.com/ajesus37/hCTF/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-purple.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://go.dev)

**Heavens CTF 2** (**hCTF**) is a self-hosted CTF (Capture The Flag) platform. Single binary, no dependencies, runs anywhere Go does.

---

## Features

- **Single binary** — embeds all assets, templates, and migrations; copy one file and run
- **Full CLI** — cobra subcommands for admins and participants; charmbracelet TUI with tables, markdown, interactive browser
- **Auto-migrations** — schema upgrades apply automatically on startup, no data loss
- **Challenge management** — categories, difficulties, flag masking, point hints, file attachments
- **Team play** — create teams, invite-link based joining, team scoreboard
- **Competition lifecycle** — time-bounded competitions with draft→registration→running→ended transitions, per-competition scoreboards, scoreboard blackout
- **Live submission feed** — global `/submissions` page and per-competition feed; admin sees all attempts with flag text
- **Dynamic scoring** — points decay based on solve count
- **Score evolution chart** — visual timeline of top competitors' scores using Chart.js
- **SQL Playground** — per-challenge DuckDB WASM sandbox for SQL-style CTF challenges
- **Dark/light theme** — persistent toggle, no flash of unstyled content
- **Admin panel** — full CRUD for challenges, questions, hints, categories, users, competitions
- **Import/Export** — JSON backup and restore for challenges
- **Rate limiting** — per-user flag submission throttling
- **CTFtime export** — scoreboard in CTFtime.org JSON format
- **OpenAPI docs** — browsable Swagger UI at `/api/openapi`

---

## Quick Start

```bash
docker compose up -d
```

Open http://localhost:8090 — default credentials: `admin@hctf.local` / `changeme`

<details>
<summary>docker-compose.yml example</summary>

```yaml
services:
  hctf:
    image: ghcr.io/ajesus37/hCTF:latest
    ports:
      - "8090:8090"
    volumes:
      - ./data:/data
    environment:
      JWT_SECRET: ${JWT_SECRET:-change-me-in-production}
    command: >
      --db /data/hctf.db
      --admin-email admin@hctf.local
      --admin-password changeme
    restart: unless-stopped
```
</details>

---

## Installation

### Option 1: Binary (fastest)

Download the latest binary from [Releases](https://github.com/ajesus37/hCTF/releases):

```bash
curl -L https://github.com/ajesus37/hCTF/releases/latest/download/hctf-linux-amd64 -o hctf
chmod +x hctf
./hctf serve --admin-email admin@example.com --admin-password yourpassword
```

### Option 2: Build from source

Requires Go 1.24+ and [Task](https://taskfile.dev):

```bash
git clone https://github.com/ajesus37/hCTF.git
cd hctf
./setup.sh   # checks requirements and downloads dependencies
task build
./hctf serve --admin-email admin@example.com --admin-password yourpassword
```

---

## Configuration

All server options are set via flags on `hctf serve`. See [CONFIGURATION.md](CONFIGURATION.md) for a fully annotated reference.

| Option | Flag | Default |
|--------|------|---------|
| Port | `--port` | `8090` |
| Database | `--db` | `./hctf.db` |
| Admin email | `--admin-email` | — |
| Admin password | `--admin-password` | — |
| Message of the Day | `--motd` | — |

### CLI

The binary doubles as a full-featured CLI client for a running server:

```bash
# Authenticate
hctf login --server http://localhost:8090 --email admin@example.com --password yourpassword

# Browse challenges interactively (TTY)
hctf challenge browse

# Submit a flag
hctf flag submit <question-id> FLAG{...}

# Live submission feed (auto-refreshes every 5s)
hctf submissions --watch

# View your profile
hctf user profile

# Admin: create a challenge (interactive on TTY, or via flags)
hctf challenge create --title "My Challenge" --category Web --difficulty Easy --points 200
hctf challenge export --output backup.json
hctf challenge import backup.json

# Admin: manage competitions
hctf competition create "CTF 2026"
hctf competition start <id>
hctf competition scoreboard <id>

# Admin: manage users
hctf user list
hctf user promote <user-id>

# Admin: export/import full platform config (challenges, competitions, settings)
hctf config export -o backup.yaml
hctf config import backup.yaml

# JSON output for scripting
hctf --json challenge list | jq '.[] | .title'
```

All create/update commands prompt interactively on TTY. Pass `--quiet` to suppress output (returns ID only), `--json` for machine-readable output.

Run `hctf --help` for the full command tree.

---

## Self-Hosting

### Docker (recommended)

```bash
docker run -d \
  -p 8090:8090 \
  -v hctf-data:/data \
  -e JWT_SECRET="$(openssl rand -base64 32)" \
  ghcr.io/ajesus37/hCTF \
  serve --db /data/hctf.db --admin-email admin@hctf.local --admin-password changeme
```

Or use Docker Compose (see `docker-compose.yml` in the repo):

```bash
docker compose up -d
```

### Reverse Proxy (Caddy)

```
ctf.example.com {
    reverse_proxy localhost:8090
}
```

<details>
<summary>Nginx config</summary>

```nginx
server {
    server_name ctf.example.com;
    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```
</details>

### Backup

```bash
# Docker volume
docker compose exec hctf cat /data/hctf.db > hctf.db.backup-$(date +%Y%m%d)

# Or if using a bind mount
cp ./data/hctf.db ./data/hctf.db.backup-$(date +%Y%m%d)
```

### Upgrading

Pull the latest image and restart — migrations run automatically:

```bash
docker compose pull
docker compose up -d
```

No manual migration steps needed.

### Advanced: bare-metal binary

If you prefer running the binary directly without Docker:

```bash
./hctf serve --db /var/lib/hctf/hctf.db \
  --admin-email admin@example.com \
  --admin-password yourpassword \
  --jwt-secret "$(openssl rand -base64 32)"
```

Use your preferred process manager (e.g. supervisord, runit) to keep it running.

---

## Security

- Passwords hashed with bcrypt (cost 12)
- JWT tokens stored in HttpOnly cookies
- All SQL queries use parameterized statements
- No telemetry or analytics by default
- Admin routes protected by role middleware

### JWT Secret Configuration

**Production (required):**
```bash
./hctf serve --jwt-secret "$(openssl rand -base64 32)"
```

Or via environment variable:
```bash
export JWT_SECRET="$(openssl rand -base64 32)"
./hctf serve
```

**Development (insecure):**
```bash
./hctf serve --dev  # Allows default JWT secret with warning
```

The server will refuse to start in production mode without a proper JWT secret (minimum 32 characters). See [CONFIGURATION.md](CONFIGURATION.md) for details.


---

## Documentation

| Doc | Contents |
|-----|----------|
| [CONFIGURATION.md](CONFIGURATION.md) | All config options in detail |
| [ARCHITECTURE.md](ARCHITECTURE.md) | How the codebase is structured |
| [OPERATIONS.md](OPERATIONS.md) | Deployment, monitoring, backup, troubleshooting |
| [SQL_PLAYGROUND.md](SQL_PLAYGROUND.md) | DuckDB WASM SQL challenge mode |
| [TESTING.md](TESTING.md) | Running and writing tests |

---

## Contributing

Issues and PRs welcome. To run locally:

```bash
task deps
task run   # starts on :8090 with a default admin
task test  # run tests
```

---

## License

MIT — see [LICENSE](LICENSE).

---

## How this was built

The architecture, database schema, and core backend were designed and implemented before any AI assistance. We use AI tools for specific, scoped tasks — drafting boilerplate, suggesting refactors, writing docs — and every change is reviewed and usually rewritten by a human maintainer (aside from the frontend. While the architecture was human-chosen, this is not a core skill for me, so I let the AI implement, but reviewed and validated everything). This is not autonomous "generate and ship" code.
