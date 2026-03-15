# hCTF2 Architecture

## Overview

hCTF2 is a modern CTF platform built with simplicity and performance in mind. The architecture follows a traditional server-side rendering pattern with progressive enhancement via HTMX.

## Tech Stack

### Backend
- **Language**: Go 1.24+
- **Router**: Chi (lightweight, composable)
- **Database**: SQLite via modernc.org/sqlite (pure Go, no CGO)
- **Authentication**: JWT with bcrypt password hashing
- **Migrations**: golang-migrate with embedded SQL files
- **Template Engine**: Go's html/template

### Frontend
- **HTML**: Server-rendered templates
- **CSS**: Tailwind CSS (via CDN, no build step)
- **JS**: HTMX 2.x for interactivity, Alpine.js for client-side state
- **SQL Engine**: DuckDB WASM for client-side SQL queries

## Architecture Patterns

### Single Binary Deployment

All assets are embedded using Go's `embed` directive:
- HTML templates
- SQL migrations
- Static files (CSS, JS, images)

This creates a truly portable single binary with zero external dependencies.

### Server-Side Rendering

Unlike modern SPA frameworks, hCTF2 uses server-side rendering for several reasons:
1. **Simplicity**: No complex build toolchain
2. **SEO**: All content is immediately visible
3. **Performance**: Faster initial page load
4. **Developer Experience**: Templates are easier to maintain

### Progressive Enhancement with HTMX

HTMX adds modern UX without heavy JavaScript:
- Form submissions without page refresh
- Live scoreboard updates
- Dynamic content loading
- Smooth transitions

Example:
```html
<form hx-post="/api/questions/{id}/submit"
      hx-target="#result"
      hx-swap="innerHTML">
  <input type="text" name="flag">
  <button type="submit">Submit</button>
</form>
```

## CLI Architecture

The binary serves a dual role: HTTP server and CLI client.

### Entry Point

`main.go` calls `cmd.Execute(version)`. Cobra dispatches to either `serve` (server) or a CLI subcommand.

### Server (`cmd/serve.go`)

All existing server logic lives here. Flags registered with `serveCmd.Flags()` mirror the old flat flags exactly — the only user-visible change is `hctf2 serve --port 8090` instead of `hctf2 --port 8090`.

### CLI Client

CLI subcommands in `cmd/` use `internal/client/` to communicate with a running server via its REST API. No direct database access from the CLI.

```
cmd/challenge.go → internal/client/challenges.go → GET /api/challenges
cmd/user.go      → internal/client/users.go      → GET /api/admin/users
```

### Auth Flow (CLI)

```
hctf2 login --email ... --password ...
      ↓
POST /api/auth/login (form-encoded)
      ↓
JWT extracted from Set-Cookie response header
      ↓
Stored in ~/.config/hctf2/config.yaml
      ↓
Subsequent commands: Cookie: auth_token=<jwt> on every request
```

### Output Strategy

TTY detection at startup determines output mode:
- **TTY**: lipgloss tables, glamour markdown, huh forms, bubbletea browser
- **Pipe / `--json`**: raw JSON to stdout
- **`--quiet`**: minimal (IDs only on create, "ok" on success)

Errors always go to stderr. Exit code 1 on any error.

## Database Schema

### Design Principles

1. **Normalized**: Proper foreign keys and relationships
2. **Indexed**: Strategic indexes on frequently queried columns
3. **Flexible**: JSON columns for extensibility (tags)
4. **Auditable**: Created/updated timestamps on all tables

### Key Relationships

```
users ──┐
        ├─── submissions ──── questions ──── challenges
teams ──┘

questions ──── hints ──── hint_unlocks ──── users
```

### Flag Masking

The `flag_mask` column auto-generates masked versions of flags:
- Input: `FLAG{secret_value}`
- Output: `FLAG{************}`

This is calculated in `database/queries.go:generateFlagMask()`.

## Authentication Flow

1. **Registration**:
   - User submits email/password/name
   - Password hashed with bcrypt (cost 12)
   - User created in database
   - JWT token generated and set as cookie

2. **Login**:
   - User submits email/password
   - Password verified against hash
   - JWT token generated with 7-day expiry
   - Token set as HttpOnly cookie

3. **Authorization**:
   - Middleware checks for auth_token cookie or Authorization header
   - JWT validated and claims extracted
   - User context added to request
   - Admin-only routes check `IsAdmin` claim

## SQL Playground

### Client-Side Execution

The SQL playground runs entirely in the browser for security:

1. **Server** exposes `/api/sql/snapshot` endpoint
2. **Snapshot** contains sanitized data (no flags, passwords)
3. **DuckDB WASM** loads snapshot into in-memory database
4. **User queries** execute locally, never hit server

### Data Sanitization

The snapshot excludes sensitive data:
- User passwords, emails (only names)
- Question flags (only masks)
- Incorrect submissions

This makes the SQL interface both safe and educational.

## Request Flow

### Challenge Submission

```
User submits flag
      ↓
HTMX POST /api/questions/{id}/submit
      ↓
RequireAuth middleware validates JWT
      ↓
Handler parses form, checks if already solved
      ↓
Database validates flag (case-sensitive check)
      ↓
Submission recorded
      ↓
HTML response: ✅ Correct or ❌ Incorrect
      ↓
HTMX swaps response into #result div
```

### Scoreboard Updates

```
Page loads with initial scoreboard HTML
      ↓
HTMX polls /api/scoreboard every 30s
      ↓
Handler queries database for top 100 users
      ↓
Results sorted by: points DESC, last_solve ASC
      ↓
HTML table rendered server-side
      ↓
HTMX swaps new table into DOM
```

## Performance Optimizations

### Database
- **Connection pooling**: SQLite with WAL mode
- **Strategic indexes**: On foreign keys and query columns
- **Compiled statements**: Reused for common queries

### Templates
- **Parsed once**: At startup, cached in memory
- **Efficient rendering**: Go's html/template is fast
- **Minimal logic**: Keep templates simple

### Client-Side
- **CDN assets**: Tailwind, HTMX from CDN (browser cached)
- **No build step**: Faster development, simpler deployment
- **Lazy loading**: SQL playground only loads when accessed

## Security Considerations

### SQL Injection
- **Server queries**: Use parameterized statements exclusively
- **Client queries**: Run in isolated WASM, can't access server

### XSS
- **Template escaping**: Go's html/template auto-escapes
- **User input**: Validated and sanitized
- **CSP headers**: Could be added for defense-in-depth

### Authentication
- **Password hashing**: bcrypt with cost 12
- **JWT secret**: Should be random in production
- **HttpOnly cookies**: Prevents XSS token theft
- **Session duration**: 7 days, configurable

### Authorization
- **Middleware**: Centralized auth checks
- **Admin routes**: Separate route group with RequireAdmin
- **Flag access**: Hidden from non-admin users

## Scalability

### Current Limits
- **SQLite**: Good for 1000s of users, 100s of concurrent requests
- **Single binary**: Easy to deploy, but single point of failure
- **No caching**: Database hit on every request

### Future Improvements
- **PostgreSQL**: For larger deployments
- **Redis**: For session storage and caching
- **Load balancing**: Multiple instances behind nginx
- **CDN**: For static assets

## Development Workflow

### Adding a Feature

1. **Model**: Define struct in `internal/models/`
2. **Migration**: Create SQL in `migrations/XXX_feature.up.sql`
3. **Queries**: Add database methods in `internal/database/queries.go`
4. **Handler**: Implement HTTP handler in `internal/handlers/`
5. **Route**: Register route in `main.go`
6. **Template**: Create HTML in `internal/views/templates/`
7. **Test**: Write tests in `*_test.go` files

### Testing Strategy

- **Unit tests**: Database queries, flag validation
- **Integration tests**: HTTP handlers with test database
- **Manual tests**: Browser-based UI testing

## Monitoring

### Logs
- Chi's logger middleware logs all requests
- Errors logged to stderr
- Can be redirected to syslog or file

### Metrics
Future: Prometheus metrics endpoint
- Request counts
- Response times
- Active users
- Solve rates

## Deployment Strategies

### Docker Compose (recommended)
- Docker image from `ghcr.io/ajesus37/hCTF2`
- Named volume for database persistence
- Environment variables for secrets (JWT_SECRET, SMTP)
- Reverse proxy (Caddy/Nginx) for TLS

### Cloud
- Any Docker-capable platform (AWS ECS, Google Cloud Run, Fly.io, etc.)
- Single container, single volume — no complex orchestration needed

## Code Organization

```
main.go              # Entry point — calls cmd.Execute(version)
cmd/                 # Cobra command tree
  root.go            # Root command, global flags (--server, --json, --quiet)
  serve.go           # Server subcommand — all server startup logic and routes
  auth.go            # login / logout / status
  challenge.go       # challenge list/get/create/delete/browse
  flag.go            # flag submit
  team.go            # team list/get/create/join
  competition.go     # competition list/create/start/end
  user.go            # user list/promote/demote/delete (admin)
  config.go          # config export/import (admin)
  client.go          # shared newClient() helper
  helpers.go         # shared CLI helpers (confirmIfTTY, boolToYesNo, abortedMsg)
internal/
  auth/              # JWT middleware and helpers
  client/            # HTTP client wrapping the REST API (CLI use)
    client.go        # base Client struct, Do(), decodeJSON()
    auth.go          # Login()
    challenges.go    # ListChallenges, GetChallenge, SubmitFlag, CreateChallenge, DeleteChallenge
    teams.go         # ListTeams, GetTeam, CreateTeam, JoinTeam
    competitions.go  # ListCompetitions, CreateCompetition, ForceStart, ForceEnd
    users.go         # ListUsers, PromoteUser, DeleteUser
  config/            # CLI config file (~/.config/hctf2/config.yaml)
    config.go        # Load() / Save() using gopkg.in/yaml.v3
  database/          # Database layer
    migrations/      # SQL migration files (001–017)
  handlers/          # HTTP handlers (auth, challenges, teams, competitions, etc.)
  models/            # Data structures
  tui/               # Charmbracelet terminal UI components
    theme.go         # Shared lipgloss styles
    table.go         # PrintTable() — width-aware lipgloss table renderer
    browse.go        # Bubbletea interactive challenge browser
  utils/             # Utility functions (markdown rendering)
  views/             # Templates & static files
    templates/       # HTML templates
    static/          # CSS, JS, images, DuckDB WASM files
Taskfile.yml         # Build automation (not Makefile)
handlers_test.go     # HTTP handler tests
```

## Design Decisions

### Why SQLite?
- **Simple**: Single file database
- **Fast**: Faster than PostgreSQL for reads
- **Portable**: No setup required
- **Reliable**: ACID compliant
- **Embedded**: No network overhead

### Why Server-Side Rendering?
- **Simple**: No build toolchain
- **Fast**: Faster than SPAs for initial load
- **SEO**: Search engine friendly
- **Progressive**: Works without JS

### Why No Framework?
- **Lightweight**: Minimal dependencies
- **Control**: Full control over behavior
- **Simplicity**: Less magic, easier debugging

## Future Work

- **Real-time Notifications** — WebSocket-based solve notifications
- **Challenge Docker Integration** — Per-challenge container spawning
- **S3 Storage Backend** — Alternative to local file storage
