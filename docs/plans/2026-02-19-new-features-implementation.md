# New Features Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement 7 improvements from new-features.md: team profile link, comprehensive config, monitoring docs, SVG logo, README rewrite + file cleanup, and OpenAPI auto-generation via swaggo.

**Architecture:** Items 1–3 are isolated low-risk changes (template, config, docs). Item 4 (logo) adds static SVG assets embedded into the binary. Item 5 (README) rewrites the front door and deletes 8 redundant markdown files. Item 6 (swaggo) is the largest: installs a CLI tool, adds ~35 handler annotations across 8 files, and replaces the manually maintained OpenAPI YAML.

**Tech Stack:** Go 1.24, Chi router, HTMX, swaggo/swag v1.x, go embed, Taskfile

---

## Task 1: Team name as clickable link in profile

> `UserStats` already has `TeamID *string` — only the template needs updating.

**Files:**
- Modify: `internal/views/templates/profile.html` (lines ~9–13)

**Step 1: Edit the template**

Find this block in `profile.html`:
```html
{{if .Stats.TeamName}}
<p class="text-gray-500 dark:text-gray-400">
    Team: <span class="text-blue-400">{{.Stats.TeamName}}</span>
</p>
{{end}}
```

Replace with:
```html
{{if .Stats.TeamName}}
<p class="text-gray-500 dark:text-gray-400">
    Team:
    {{if .Stats.TeamID}}
    <a href="/teams/{{deref .Stats.TeamID}}/profile" class="text-blue-400 hover:underline">{{deref .Stats.TeamName}}</a>
    {{else}}
    <span class="text-blue-400">{{deref .Stats.TeamName}}</span>
    {{end}}
</p>
{{end}}
```

> Note: `TeamName` and `TeamID` are `*string` — use `deref` template function if it exists, or check how other templates dereference pointer fields. Look at `teams.html` for examples. If no `deref` helper exists, use `{{if .Stats.TeamID}}<a href="/teams/{{.Stats.TeamID}}/profile"...>{{.Stats.TeamName}}</a>{{end}}` — Go templates dereference pointer values automatically when printing.

**Step 2: Rebuild and verify**

```bash
task rebuild
./hctf2 --port 8090 --admin-email admin@hctf.local --admin-password changeme
# Navigate to a profile page for a user who is in a team
# Verify the team name is a clickable link
```

**Step 3: Commit**
```bash
git add internal/views/templates/profile.html
git commit -m "feat(profile): make team name a clickable link to team page"
```

---

## Task 2: Comprehensive config.example.yaml

**Files:**
- Modify: `config.example.yaml`

**Step 1: Rewrite the file**

Replace the entire content of `config.example.yaml` with:

```yaml
# hCTF2 Configuration File
#
# This file documents all available configuration options.
# Copy this file to config.yaml and adjust as needed.
#
# Configuration precedence (highest to lowest):
#   1. CLI flags (--port, --db, etc.)
#   2. Environment variables (PORT, DATABASE_PATH, etc.)
#   3. This config file
#   4. Built-in defaults (shown in comments below)

server:
  # Port the HTTP server listens on
  # CLI: --port | Env: PORT | Default: 8090
  port: 8090

  # Interface the server binds to. Use 0.0.0.0 for all interfaces,
  # 127.0.0.1 to restrict to localhost only.
  # CLI: --host | Env: HOST | Default: 0.0.0.0
  host: 0.0.0.0

database:
  # Path to the SQLite database file.
  # The directory must exist. Relative paths are resolved from the working directory.
  # CLI: --db | Env: DATABASE_PATH | Default: ./hctf2.db
  path: ./hctf2.db

auth:
  # Secret key used to sign JWT tokens.
  # REQUIRED in production — use a long random string.
  # Generate one: openssl rand -base64 32
  # CLI: --jwt-secret | Env: JWT_SECRET | Default: (auto-generated, changes on restart)
  jwt_secret: "change-this-to-a-random-secret-in-production"

  # How long JWT sessions remain valid.
  # Format: Go duration string (e.g. 24h, 7d, 168h).
  # Default: 168h (7 days)
  session_duration: 168h

admin:
  # Email address for the initial admin account.
  # Only used on first startup when no admin exists.
  # CLI: --admin-email | Env: ADMIN_EMAIL | Default: (none)
  email: admin@example.com

  # Password for the initial admin account.
  # Only used on first startup. Change via UI after first login.
  # CLI: --admin-password | Env: ADMIN_PASSWORD | Default: (none)
  password: "changeme"

# Message of the Day — displayed below the login form.
# Supports plain text. Leave empty to hide.
# CLI: --motd | Env: MOTD | Default: (none)
motd: ""

telemetry:
  # Service name used in OpenTelemetry traces.
  # Default: hctf2
  service_name: hctf2

  # Service version reported in telemetry.
  # Default: (binary version)
  service_version: "0.5.0"

  # Deployment environment tag (e.g. production, staging, development).
  # Env: ENVIRONMENT | Default: (none)
  environment: production

  # Set to true to print OpenTelemetry traces to stdout.
  # Useful for debugging. Not recommended in production.
  # Env: OTEL_EXPORTER_STDOUT | Default: false
  enable_stdout_exporter: false
```

**Step 2: Verify the file looks clean**

```bash
cat config.example.yaml
```

**Step 3: Commit**
```bash
git add config.example.yaml
git commit -m "docs(config): expand config.example.yaml with all options and defaults"
```

---

## Task 3: Metrics/monitoring documentation in OPERATIONS.md

> OPERATIONS.md already has a "## Monitoring" section with a stub "### Metrics (Future)" that says metrics aren't exposed. The telemetry package actually implements OpenTelemetry with 4 counters/histograms and a middleware. Update the docs to reflect reality.

**Files:**
- Modify: `OPERATIONS.md`

**Step 1: Find the outdated section**

Look for `### Metrics (Future)` in OPERATIONS.md. It currently says:
```
Currently, hCTF2 doesn't expose Prometheus metrics...
```

**Step 2: Replace with accurate documentation**

Replace the `### Metrics (Future)` section with:

```markdown
### Metrics & Telemetry

hCTF2 uses **OpenTelemetry** for instrumentation. The telemetry package (`internal/telemetry/`) initializes a tracer and meter on startup.

#### Instrumented Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `http_requests_total` | Counter | Total HTTP requests, labelled by method, path, status |
| `http_request_duration_seconds` | Histogram | Request duration in seconds, labelled by method and path |
| `active_users` | UpDownCounter | Active user count |
| `database_queries_total` | Counter | Total database queries |

These metrics are recorded automatically via the `telemetry.Middleware` applied to every route.

#### Enabling Trace Output

Set the environment variable to print traces to stdout (useful for debugging):

```bash
OTEL_EXPORTER_STDOUT=true ./hctf2
```

#### Exporting to an OTEL Collector

To ship traces and metrics to a backend (Jaeger, Grafana Tempo, Datadog, etc.):

1. Run an OpenTelemetry Collector alongside hCTF2
2. Configure the collector endpoint via environment variable:
   ```bash
   OTEL_EXPORTER_OTLP_ENDPOINT=http://collector:4317 ./hctf2
   ```
   > Note: OTLP export requires adding the OTLP exporter package to the binary. Currently only stdout export is wired up.

#### Prometheus / Grafana

The current implementation does not expose a `/metrics` Prometheus scrape endpoint. To add one:

1. Add `go.opentelemetry.io/otel/exporters/prometheus` to `go.mod`
2. Register the Prometheus exporter in `internal/telemetry/telemetry.go`
3. Expose `/metrics` route in `main.go`

Until then, monitor via log aggregation (see **Server Logs** section above).

#### Recommended Alerts

- **HTTP 5xx rate** > 1% of requests over 5 minutes
- **Request duration p99** > 2 seconds
- **Process restart** (uptime monitoring via systemd or container health check)
- **Disk space** < 1GB remaining (SQLite database growth)
```

**Step 3: Commit**
```bash
git add OPERATIONS.md
git commit -m "docs(operations): document OpenTelemetry metrics and monitoring setup"
```

---

## Task 4: SVG logo and favicon

> Use the canvas-design skill to create the logo. The design: flag motif + terminal bracket `>_`, purple `#A55EEA`, dark background. Two variants: full logo (icon + text) and icon-only favicon.

**Files:**
- Create: `internal/views/static/logo.svg`
- Create: `internal/views/static/favicon.svg`
- Modify: `internal/views/templates/base.html` (favicon link)

**Step 1: Invoke the canvas-design skill**

Use `Skill("canvas-design")` to create two SVG files:

1. `internal/views/static/logo.svg` — Full logo: terminal `>_` bracket icon left of "hCTF2" text, purple `#A55EEA`, transparent background, roughly 200×50px viewBox
2. `internal/views/static/favicon.svg` — Icon only: the `>_` glyph in a rounded square, purple on dark (`#1a1a2e`) background, 32×32px viewBox

**Step 2: Update the favicon link in base.html**

Find the existing favicon `<link>` tag in `internal/views/templates/base.html`. If none exists, add one inside `<head>`:

```html
<link rel="icon" type="image/svg+xml" href="/static/favicon.svg">
```

**Step 3: Rebuild and check**

```bash
task rebuild
./hctf2 --port 8090 --admin-email admin@hctf.local --admin-password changeme
# Open http://localhost:8090 in browser
# Verify favicon appears in browser tab
# Verify logo.svg loads at http://localhost:8090/static/logo.svg
```

**Step 4: Commit**
```bash
git add internal/views/static/logo.svg internal/views/static/favicon.svg
git add internal/views/templates/base.html
git commit -m "feat(ui): add SVG logo and favicon"
```

---

## Task 5: README rewrite + file cleanup

> This task rewrites README.md for a self-hosted audience and deletes 8 redundant files. Do the deletions first so the README can absorb their essential content.

**Files to delete:**
- `INSTALL.md` — absorbed into README Installation section
- `QUICKSTART.md` — absorbed into README Quick Start section
- `API.md` — replaced by auto-generated OpenAPI spec at `/api/openapi`
- `FEATURES_IMPLEMENTATION.md` — internal history, no user value
- `IDEA.md` — internal planning doc
- `IMPLEMENTATION_SUMMARY.md` — internal history
- `IMPROVEMENTS_AND_ROADMAP.md` — superseded
- `DOCKER.md` — absorbed into README Self-hosting section

**Files to modify:**
- `README.md` — full rewrite

**Step 1: Delete the redundant files**

```bash
git rm INSTALL.md QUICKSTART.md API.md FEATURES_IMPLEMENTATION.md IDEA.md IMPLEMENTATION_SUMMARY.md IMPROVEMENTS_AND_ROADMAP.md DOCKER.md
```

**Step 2: Rewrite README.md**

Replace the entire content with the following structure (write the actual content, don't use placeholders):

```markdown
# hCTF2

![hCTF2 logo](internal/views/static/logo.svg)

[![License: MIT](https://img.shields.io/badge/License-MIT-purple.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://go.dev)

A self-hosted CTF (Capture The Flag) platform. Single binary, no dependencies, runs anywhere Go does.

---

## Features

- **Single binary** — embed all assets, templates, and migrations; copy one file and run
- **Auto-migrations** — schema upgrades apply automatically on startup, no data loss
- **Challenge management** — categories, difficulties, flag masking, point hints
- **Team play** — create teams, invite-link based joining, team scoreboard
- **SQL Playground** — per-challenge DuckDB WASM sandbox for SQL-style CTF challenges
- **Dark/light theme** — persistent toggle, no flash of unstyled content
- **Admin panel** — full CRUD for challenges, questions, hints, categories, users
- **OpenAPI docs** — browsable Swagger UI at `/api/openapi`

---

## Quick Start

```bash
docker compose up -d
```

Open http://localhost:8090 — default credentials: `admin@hctf.local` / `changeme`

<details>
<summary>Full docker-compose.yml</summary>

```yaml
services:
  hctf2:
    image: ghcr.io/yourusername/hctf2:latest
    ports:
      - "8090:8090"
    volumes:
      - ./data:/data
    environment:
      DATABASE_PATH: /data/hctf2.db
      JWT_SECRET: change-this
      ADMIN_EMAIL: admin@hctf.local
      ADMIN_PASSWORD: changeme
    restart: unless-stopped
```
</details>

---

## Installation

### Option 1: Binary (fastest)

Download the latest binary from [Releases](https://github.com/yourusername/hctf2/releases):

```bash
curl -L https://github.com/yourusername/hctf2/releases/latest/download/hctf2-linux-amd64 -o hctf2
chmod +x hctf2
./hctf2 --admin-email admin@example.com --admin-password yourpassword
```

### Option 2: Build from source

Requires Go 1.24+ and [Task](https://taskfile.dev):

```bash
git clone https://github.com/yourusername/hctf2.git
cd hctf2
task deps
task build
./hctf2 --admin-email admin@example.com --admin-password yourpassword
```

---

## Configuration

All options can be set via CLI flags, environment variables, or `config.yaml`. See [`config.example.yaml`](config.example.yaml) for a fully annotated example.

| Option | Flag | Env var | Default |
|--------|------|---------|---------|
| Port | `--port` | `PORT` | `8090` |
| Host | `--host` | `HOST` | `0.0.0.0` |
| Database | `--db` | `DATABASE_PATH` | `./hctf2.db` |
| JWT secret | `--jwt-secret` | `JWT_SECRET` | auto-generated |
| Admin email | `--admin-email` | `ADMIN_EMAIL` | — |
| Admin password | `--admin-password` | `ADMIN_PASSWORD` | — |
| Message of the Day | `--motd` | `MOTD` | — |

---

## Self-Hosting

### Volumes

The only persistent data is the SQLite database file:

```bash
# Mount a local directory for the database
docker run -v ./data:/data -e DATABASE_PATH=/data/hctf2.db hctf2
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
cp hctf2.db hctf2.db.backup-$(date +%Y%m%d)
```

### Upgrading

Replace the binary and restart — migrations run automatically:

```bash
systemctl stop hctf2
cp hctf2-new hctf2
systemctl start hctf2
```

No manual migration steps needed. The platform tracks applied migrations and only runs new ones.

---

## Security

- Passwords hashed with bcrypt (cost 12)
- JWT tokens stored in HttpOnly cookies
- All SQL queries use parameterized statements
- No telemetry or analytics by default
- Admin routes protected by role middleware

---

## Documentation

| Doc | Contents |
|-----|----------|
| [CONFIGURATION.md](CONFIGURATION.md) | All config options in detail |
| [ARCHITECTURE.md](ARCHITECTURE.md) | How the codebase is structured |
| [OPERATIONS.md](OPERATIONS.md) | Deployment, monitoring, backup, troubleshooting |
| [SQL_PLAYGROUND.md](SQL_PLAYGROUND.md) | DuckDB WASM SQL challenge mode |
| [TESTING.md](TESTING.md) | Running and writing tests |
| [KNOWN_ISSUES.md](KNOWN_ISSUES.md) | Known bugs and workarounds |

---

## Contributing

Issues and PRs welcome. To run locally:

```bash
task deps
task run  # starts on :8090 with a default admin
task test  # run tests
```

---

## License

MIT — see [LICENSE](LICENSE).

---

## How this was built

The architecture, database schema, and core backend were designed and implemented before any AI assistance. We use AI tools for specific, scoped tasks — drafting boilerplate, suggesting refactors, writing docs — and every change is reviewed and usually rewritten by a human maintainer. This is not autonomous "generate and ship" code.
```

**Step 3: Rebuild and verify**

```bash
task rebuild
# Verify the server still starts
./hctf2 --port 8090 --admin-email admin@hctf.local --admin-password changeme &
sleep 1 && curl -s http://localhost:8090 | grep -c "hCTF" && kill %1
```

**Step 4: Commit**
```bash
git add README.md
git commit -m "docs(readme): rewrite for self-hosted audience, remove 8 redundant docs"
```

---

## Task 6: OpenAPI auto-generation with swaggo/swag

> This is the largest task. It installs the swag CLI, adds a `task generate-openapi` command, adds general info annotations to `main.go`, and annotates every HTTP handler.

### Task 6a: Install swaggo and add Taskfile task

**Step 1: Install swag CLI**

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

Verify:
```bash
swag --version
# Expected: swag version v1.x.x
```

**Step 2: Add task to Taskfile.yml**

Add this task to `Taskfile.yml` before the `help` task:

```yaml
generate-openapi:
  desc: Generate OpenAPI/Swagger spec from code annotations
  cmds:
    - swag init -g main.go -o docs/ --outputTypes yaml
    - mv docs/swagger.yaml docs/openapi.yaml
    - echo "OpenAPI spec generated at docs/openapi.yaml"
```

**Step 3: Add general info block to main.go**

Add these comment annotations directly above the `package main` declaration at the top of `main.go`:

```go
// @title hCTF2 API
// @version 0.5.0
// @description Self-hosted CTF platform API. Most endpoints require authentication via JWT cookie.
// @termsOfService http://example.com/terms/

// @contact.name hCTF2 Support
// @contact.url https://github.com/yourusername/hctf2/issues

// @license.name MIT

// @host localhost:8090
// @BasePath /api

// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name auth_token

// @tag.name Auth
// @tag.description Authentication endpoints
// @tag.name Challenges
// @tag.description Challenge and question management
// @tag.name Teams
// @tag.description Team management
// @tag.name Hints
// @tag.description Hint viewing and unlocking
// @tag.name Scoreboard
// @tag.description Scoreboard data
// @tag.name Admin
// @tag.description Admin-only CRUD operations
// @tag.name SQL
// @tag.description SQL Playground snapshot
```

**Step 4: Commit setup**
```bash
git add Taskfile.yml main.go
git commit -m "build(openapi): add swaggo task and main.go info block"
```

---

### Task 6b: Annotate auth handlers

**Files:**
- Modify: `internal/handlers/auth.go`

Add annotations above each handler function. Here are all annotations:

```go
// Register godoc
// @Summary Register a new user
// @Tags Auth
// @Accept json,multipart/form-data
// @Produce json
// @Param body body object{email=string,name=string,password=string} false "Registration data"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Router /auth/register [post]
func (h *AuthHandler) Register(...)

// Login godoc
// @Summary Log in and receive a session cookie
// @Tags Auth
// @Accept json,multipart/form-data
// @Produce json
// @Param body body object{email=string,password=string} false "Login credentials"
// @Success 200 {object} object{message=string,user=object}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Router /auth/login [post]
func (h *AuthHandler) Login(...)

// Logout godoc
// @Summary Log out and clear session cookie
// @Tags Auth
// @Security CookieAuth
// @Success 200 {object} object{message=string}
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(...)

// ForgotPassword godoc
// @Summary Request a password reset token
// @Tags Auth
// @Accept json,multipart/form-data
// @Param body body object{email=string} false "Email address"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{error=string}
// @Router /auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(...)

// ResetPassword godoc
// @Summary Reset password using a token
// @Tags Auth
// @Accept json,multipart/form-data
// @Param body body object{token=string,password=string} false "Reset data"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{error=string}
// @Router /auth/reset-password [post]
func (h *AuthHandler) ResetPassword(...)
```

**Step after annotating:**
```bash
task generate-openapi
# Expected: docs/openapi.yaml updated, no errors
git add internal/handlers/auth.go docs/openapi.yaml
git commit -m "docs(openapi): annotate auth handlers"
```

---

### Task 6c: Annotate challenge handlers

**Files:**
- Modify: `internal/handlers/challenges.go`

```go
// ListChallenges godoc
// @Summary List all challenges
// @Tags Challenges
// @Produce json
// @Success 200 {array} models.Challenge
// @Router /challenges [get]

// GetChallenge godoc
// @Summary Get a single challenge with its questions
// @Tags Challenges
// @Produce json
// @Param id path string true "Challenge ID"
// @Success 200 {object} models.Challenge
// @Failure 404 {object} object{error=string}
// @Router /challenges/{id} [get]

// SubmitFlag godoc
// @Summary Submit a flag for a question
// @Tags Challenges
// @Security CookieAuth
// @Accept json,multipart/form-data
// @Param id path string true "Question ID"
// @Param body body object{flag=string} false "Flag submission"
// @Success 200 {object} object{correct=bool,message=string}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Router /questions/{id}/submit [post]

// CreateChallenge godoc
// @Summary Create a new challenge (admin)
// @Tags Admin
// @Security CookieAuth
// @Accept json,multipart/form-data
// @Param body body object{name=string,description=string,category=string,difficulty=string} false "Challenge data"
// @Success 200 {object} models.Challenge
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /admin/challenges [post]

// UpdateChallenge godoc
// @Summary Update a challenge (admin)
// @Tags Admin
// @Security CookieAuth
// @Accept json,multipart/form-data
// @Param id path string true "Challenge ID"
// @Success 200 {object} models.Challenge
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /admin/challenges/{id} [put]

// DeleteChallenge godoc
// @Summary Delete a challenge (admin)
// @Tags Admin
// @Security CookieAuth
// @Param id path string true "Challenge ID"
// @Success 200 {object} object{message=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /admin/challenges/{id} [delete]

// CreateQuestion godoc
// @Summary Create a question for a challenge (admin)
// @Tags Admin
// @Security CookieAuth
// @Accept json,multipart/form-data
// @Param body body object{challenge_id=string,name=string,description=string,flag=string,points=int} false "Question data"
// @Success 200 {object} models.Question
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /admin/questions [post]

// UpdateQuestion godoc
// @Summary Update a question (admin)
// @Tags Admin
// @Security CookieAuth
// @Param id path string true "Question ID"
// @Success 200 {object} models.Question
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /admin/questions/{id} [put]

// DeleteQuestion godoc
// @Summary Delete a question (admin)
// @Tags Admin
// @Security CookieAuth
// @Param id path string true "Question ID"
// @Success 200 {object} object{message=string}
// @Failure 403 {object} object{error=string}
// @Router /admin/questions/{id} [delete]

// CreateHint godoc
// @Summary Create a hint for a question (admin)
// @Tags Admin
// @Security CookieAuth
// @Accept json,multipart/form-data
// @Param body body object{question_id=string,content=string,cost=int,order=int} false "Hint data"
// @Success 200 {object} models.Hint
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /admin/hints [post]

// UpdateHint godoc
// @Summary Update a hint (admin)
// @Tags Admin
// @Security CookieAuth
// @Param id path string true "Hint ID"
// @Success 200 {object} models.Hint
// @Failure 403 {object} object{error=string}
// @Router /admin/hints/{id} [put]

// DeleteHint godoc
// @Summary Delete a hint (admin)
// @Tags Admin
// @Security CookieAuth
// @Param id path string true "Hint ID"
// @Success 200 {object} object{message=string}
// @Failure 403 {object} object{error=string}
// @Router /admin/hints/{id} [delete]
```

**Step after annotating:**
```bash
task generate-openapi
git add internal/handlers/challenges.go docs/openapi.yaml
git commit -m "docs(openapi): annotate challenge and admin hint/question handlers"
```

---

### Task 6d: Annotate team, hint, scoreboard, profile, settings, and SQL handlers

**Files:**
- Modify: `internal/handlers/teams.go`, `hints.go`, `scoreboard.go`, `profile.go`, `settings.go`, `sql.go`

Annotations (add above each function):

**teams.go:**
```go
// CreateTeam godoc
// @Summary Create a new team
// @Tags Teams
// @Security CookieAuth
// @Accept json,multipart/form-data
// @Param body body object{name=string} false "Team name"
// @Success 200 {object} models.Team
// @Failure 400 {object} object{error=string}
// @Router /teams [post]

// JoinTeam godoc
// @Summary Join a team using an invite code
// @Tags Teams
// @Security CookieAuth
// @Param invite_id path string true "Invite code"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /teams/join/{invite_id} [post]

// LeaveTeam godoc
// @Summary Leave your current team
// @Tags Teams
// @Security CookieAuth
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{error=string}
// @Router /teams/leave [post]

// TransferOwnership godoc
// @Summary Transfer team ownership to another member
// @Tags Teams
// @Security CookieAuth
// @Param body body object{new_owner_id=string} false "New owner"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /teams/transfer-ownership [post]

// DisbandTeam godoc
// @Summary Disband the team (owner only)
// @Tags Teams
// @Security CookieAuth
// @Success 200 {object} object{message=string}
// @Failure 403 {object} object{error=string}
// @Router /teams/disband [post]

// ListTeams godoc
// @Summary List all teams
// @Tags Teams
// @Produce json
// @Success 200 {array} models.Team
// @Router /teams [get]

// GetTeam godoc
// @Summary Get a team by ID
// @Tags Teams
// @Param id path string true "Team ID"
// @Success 200 {object} models.Team
// @Failure 404 {object} object{error=string}
// @Router /teams/{id} [get]

// GetTeamScoreboard godoc
// @Summary Get team scoreboard
// @Tags Scoreboard
// @Produce json
// @Success 200 {array} object
// @Router /teams/scoreboard [get]

// RegenerateInviteCode godoc
// @Summary Regenerate the team invite code (owner only)
// @Tags Teams
// @Security CookieAuth
// @Success 200 {object} object{invite_id=string}
// @Failure 403 {object} object{error=string}
// @Router /teams/regenerate-invite [post]

// UpdateInvitePermission godoc
// @Summary Update who can see the invite code
// @Tags Teams
// @Security CookieAuth
// @Param body body object{permission=string} false "owner_only or all_members"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{error=string}
// @Router /teams/invite-permission [post]
```

**hints.go:**
```go
// UnlockHint godoc
// @Summary Unlock a hint (deducts points)
// @Tags Hints
// @Security CookieAuth
// @Param id path string true "Hint ID"
// @Success 200 {object} object{content=string}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Router /hints/{id}/unlock [post]

// GetHints godoc
// @Summary Get hints for a question (shows locked/unlocked state)
// @Tags Hints
// @Param questionId path string true "Question ID"
// @Success 200 {array} models.Hint
// @Router /questions/{questionId}/hints [get]
```

**scoreboard.go:**
```go
// GetScoreboard godoc
// @Summary Get individual user scoreboard
// @Tags Scoreboard
// @Produce json
// @Success 200 {array} object
// @Router /scoreboard [get]
```

**sql.go:**
```go
// GetSnapshot godoc
// @Summary Get a read-only SQL snapshot of the database
// @Tags SQL
// @Security CookieAuth
// @Produce json
// @Success 200 {object} object
// @Failure 500 {object} object{error=string}
// @Router /sql/snapshot [get]
```

**settings.go (admin):**
```go
// CreateCategory godoc
// @Summary Create a challenge category (admin)
// @Tags Admin
// @Security CookieAuth
// @Success 200 {object} models.CategoryOption
// @Router /admin/categories [post]

// UpdateCategory godoc
// @Summary Update a challenge category (admin)
// @Tags Admin
// @Security CookieAuth
// @Param id path string true "Category ID"
// @Success 200 {object} models.CategoryOption
// @Router /admin/categories/{id} [put]

// DeleteCategory godoc
// @Summary Delete a challenge category (admin)
// @Tags Admin
// @Security CookieAuth
// @Param id path string true "Category ID"
// @Success 200 {object} object{message=string}
// @Router /admin/categories/{id} [delete]

// CreateDifficulty godoc
// @Summary Create a difficulty level (admin)
// @Tags Admin
// @Security CookieAuth
// @Success 200 {object} models.DifficultyOption
// @Router /admin/difficulties [post]

// UpdateDifficulty godoc
// @Summary Update a difficulty level (admin)
// @Tags Admin
// @Security CookieAuth
// @Param id path string true "Difficulty ID"
// @Success 200 {object} models.DifficultyOption
// @Router /admin/difficulties/{id} [put]

// DeleteDifficulty godoc
// @Summary Delete a difficulty level (admin)
// @Tags Admin
// @Security CookieAuth
// @Param id path string true "Difficulty ID"
// @Success 200 {object} object{message=string}
// @Router /admin/difficulties/{id} [delete]

// GetCustomCode godoc
// @Summary Get custom HTML/JS injection code (admin)
// @Tags Admin
// @Security CookieAuth
// @Param page query string false "Page name (e.g. scoreboard, challenges)"
// @Success 200 {object} object{code=string}
// @Router /admin/custom-code [get]

// UpdateCustomCode godoc
// @Summary Update custom HTML/JS injection code (admin)
// @Tags Admin
// @Security CookieAuth
// @Success 200 {object} object{message=string}
// @Router /admin/custom-code [put]

// ListUsers godoc
// @Summary List all users (admin)
// @Tags Admin
// @Security CookieAuth
// @Success 200 {array} models.User
// @Router /admin/users [get]

// UpdateUserAdmin godoc
// @Summary Promote or demote a user to/from admin (admin)
// @Tags Admin
// @Security CookieAuth
// @Param id path string true "User ID"
// @Param body body object{is_admin=bool} false "Admin flag"
// @Success 200 {object} object{message=string}
// @Router /admin/users/{id}/admin [put]

// DeleteUser godoc
// @Summary Delete a user account (admin)
// @Tags Admin
// @Security CookieAuth
// @Param id path string true "User ID"
// @Success 200 {object} object{message=string}
// @Router /admin/users/{id} [delete]
```

**Step after annotating all files:**
```bash
task generate-openapi
# Expected: docs/openapi.yaml updated with all endpoints, no swag errors
git add internal/handlers/teams.go internal/handlers/hints.go
git add internal/handlers/scoreboard.go internal/handlers/sql.go
git add internal/handlers/settings.go docs/openapi.yaml
git commit -m "docs(openapi): annotate remaining handlers (teams, hints, scoreboard, settings, sql)"
```

---

### Task 6e: Update CLAUDE.md and remove old manual spec

**Files:**
- Modify: `CLAUDE.md`
- Delete: the old `docs/openapi.yaml` section instructions (replace with new ones)

**Step 1: Update CLAUDE.md**

Find the `### OpenAPI Specification` section in CLAUDE.md. Replace its content:

```markdown
### OpenAPI Specification

**Location**: `docs/openapi.yaml` (auto-generated — do NOT edit by hand)

**Generation**: Run `task generate-openapi` after any API change.

**How it works**: swaggo/swag reads `// @Summary`, `// @Router`, etc. annotations from handler comments and generates `docs/openapi.yaml`.

**CRITICAL**: After adding or modifying any HTTP handler:
1. Add/update swag annotations in the handler file
2. Run `task generate-openapi`
3. Commit the updated `docs/openapi.yaml`

**Access**: Served at `/api/openapi.yaml` and browsable at `/api/openapi`

**Annotation format:**
```go
// FunctionName godoc
// @Summary One-line description
// @Tags TagName
// @Security CookieAuth
// @Param paramName path/query/body type required "description"
// @Success 200 {object} ResponseType
// @Failure 400 {object} object{error=string}
// @Router /path [method]
func (h *Handler) FunctionName(w http.ResponseWriter, r *http.Request) {
```
```

**Step 2: Commit**
```bash
git add CLAUDE.md
git commit -m "docs(claude): update OpenAPI instructions to reference swaggo workflow"
```

---

## Final verification

After all tasks are complete:

```bash
task rebuild
./hctf2 --port 8090 --admin-email admin@hctf.local --admin-password changeme &
sleep 2

# Check OpenAPI spec is served
curl -s http://localhost:8090/api/openapi.yaml | head -20

# Check Swagger UI loads
curl -s http://localhost:8090/api/openapi | grep -c "swagger"

# Check logo SVG is served
curl -s http://localhost:8090/static/favicon.svg | grep -c "svg"

# Kill test server
kill %1
```

Then run:
```bash
task test
```

Expected: all tests pass, no build errors.
