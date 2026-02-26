# Claude Instructions for hCTF2

This file provides guidance for Claude (or other AI assistants) when working on the hCTF2 project.

## Project Overview

**hCTF2** is a modern CTF (Capture The Flag) platform built with Go, featuring:
- User authentication and authorization (JWT-based)
- Challenge and question management
- Flag submission with auto-masking
- Live scoreboard
- **Unique feature**: SQL Playground (DuckDB WASM, client-side)
- Dark theme UI (HTMX + Tailwind CSS + Alpine.js)
- Single binary deployment (all assets embedded)

## Tech Stack

- **Backend**: Go 1.24+, Chi router, SQLite (modernc.org/sqlite - pure Go, no CGO)
- **Frontend**: Server-side rendered HTML with HTMX for interactivity
- **Database**: SQLite with embedded migrations
- **Auth**: JWT tokens with bcrypt password hashing
- **Build**: Taskfile (not Make)

## Core Principles

1. **Simplicity**: Keep it simple, avoid over-engineering
2. **No CGO**: Use pure Go libraries only (modernc.org/sqlite, not mattn/go-sqlite3)
3. **Single Binary**: All assets must be embedded using Go's `embed` directive
4. **Server-Side Rendering**: No React/Vue/Angular, use Go templates + HTMX
5. **Security First**: Always use parameterized queries, bcrypt for passwords, validate input

## Project Structure

```
hctf2/
├── main.go              # Application entry point (router, middleware, handlers)
├── internal/            # Private application code
│   ├── auth/           # Authentication & middleware
│   ├── database/       # Database layer with embedded migrations
│   │   └── migrations/ # SQL migrations (001-006)
│   ├── handlers/       # HTTP handlers (auth, challenges, teams, hints, etc.)
│   ├── models/         # Data structures
│   ├── utils/          # Utility functions (markdown rendering)
│   └── views/          # Templates & static files (embedded)
│       ├── templates/  # HTML templates
│       └── static/     # CSS, JS, images, DuckDB WASM files
├── migrations/         # SQL migrations (legacy, kept for reference)
├── Taskfile.yml        # Build automation (NOT Makefile)
├── go.mod              # Go dependencies
├── handlers_test.go    # HTTP handler tests
└── *.md                # Documentation
```

## Development Workflow

### Making Changes

1. **Read Before Editing**: Always read existing code before modifying
1a. **Validate UI with agent-browser**: Use `npx agent-browser` to screenshot and verify EVERY UI change before marking it done — no exceptions, even for trivial changes. Run `task rebuild`, start server on a free port (`./hctf2 --port 8092 --dev`), then take a screenshot. NEVER use Python Playwright scripts — agent-browser is faster.
2. **Force Rebuild**: Before running server, use `task rebuild` to ensure binary is fresh (task build uses caching)
3. **Test Locally**: Changes should be testable with `task run`
4. **Validate with agent-browser**: For UI changes, validate using agent-browser (see Validation section)
5. **Update Docs**: If changing APIs or behavior, update relevant .md files
6. **Commit Properly**: Use conventional commits (see below)

### Validation with agent-browser

For browser-based projects like hCTF2, always validate UI changes using `npx agent-browser`. Commands chain with `&&` in a single shell call — the browser persists via daemon so there's no per-command startup cost.

```bash
# 1. Force rebuild
task rebuild

# 2. Start server (background)
./hctf2 --port 8092 --dev --db /tmp/hctf2_test.db \
  --admin-email admin@test.com --admin-password testpass123 &

# 3. Login and navigate in one chained call
npx agent-browser --session hctf2 open http://localhost:8092/login && \
npx agent-browser --session hctf2 fill 'input[name="email"]' admin@test.com && \
npx agent-browser --session hctf2 fill 'input[name="password"]' testpass123 && \
npx agent-browser --session hctf2 find role button click --name Login && \
npx agent-browser --session hctf2 open http://localhost:8092/admin

# 4. Interact and screenshot
npx agent-browser --session hctf2 click 'button:has-text("+ Create Challenge")' && \
npx agent-browser --session hctf2 screenshot --full /tmp/result.png

# 5. Read the screenshot with the Read tool to inspect it
```

Key flags:
- `--session <name>` — isolates browser state; daemon keeps browser alive between calls
- `--full` — full-page screenshot
- `--annotate` — numbered element labels for precise clicking
- `snapshot -i` — accessibility tree of interactive elements (faster than screenshot for finding selectors)

**What to validate:**
- Both light and dark themes (toggle with ☀️/🌙 button)
- Responsive layouts at different screen sizes
- Interactive elements (forms, buttons, modals)
- HTMX dynamic content updates
- Browser console for JavaScript errors

**Common issues to catch:**
- Missing `dark:` prefixes for dark mode support
- Cached binary not reflecting code changes (always use `task rebuild`)
- HTMX responses with hardcoded dark theme classes
- Poor contrast in light mode

### Adding New Features

Follow this order:

1. **Model** - Add struct to `internal/models/models.go`
2. **Migration** - Create SQL migration in `internal/database/migrations/`
3. **Database** - Add queries to `internal/database/queries.go`
4. **Handler** - Create handler in `internal/handlers/`
5. **Route** - Register route in `main.go`
6. **Template** - Add HTML template in `internal/views/templates/`
7. **Documentation** - Update relevant .md files

### Database Changes

- **Always create migrations**: Don't modify schema directly
- **Use parameterized queries**: Never concatenate SQL strings
- **Test foreign keys**: Ensure cascade delete works as expected
- **Add indexes**: For frequently queried columns

### Template Changes

- Templates are **embedded** at build time
- Changes require **rebuild**: `task clean && task build`
- Use Go's `html/template` syntax (auto-escapes HTML)
- Keep logic minimal in templates

## Code Style

### Go Code

- **Format**: Run `task fmt` before committing
- **Naming**: Use Go conventions (camelCase for private, PascalCase for public)
- **Errors**: Always check errors, don't use `_` unless justified
- **Context**: Pass `context.Context` as first parameter
- **Logging**: Use `log.Printf` for now (TODO: structured logging)

### Example Patterns

**Database Query**:
```go
func (db *DB) GetUserByEmail(email string) (*models.User, error) {
    query := `SELECT id, email, name FROM users WHERE email = ?`
    var user models.User
    err := db.QueryRow(query, email).Scan(&user.ID, &user.Email, &user.Name)
    if err != nil {
        return nil, err
    }
    return &user, nil
}
```

**HTTP Handler**:
```go
func (h *ChallengeHandler) GetChallenge(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")

    challenge, err := h.db.GetChallengeByID(id)
    if err != nil {
        http.Error(w, "Not found", http.StatusNotFound)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(challenge)
}
```

**HTMX Response**:
```go
// Return HTML fragment for HTMX to swap in
w.Write([]byte(`<div class="text-green-400">Correct!</div>`))
```

**Markdown in Templates**:
```html
<!-- Use markdown function in templates -->
<div class="prose prose-invert">{{markdown .Description}}</div>
```

## Security Requirements

### Always

- ✅ Use parameterized queries for SQL
- ✅ Hash passwords with bcrypt (cost 12)
- ✅ Validate JWT tokens in middleware
- ✅ Use HttpOnly cookies for tokens
- ✅ Escape HTML in templates (automatic with html/template)
- ✅ Check user permissions (admin vs regular user)

### Never

- ❌ Concatenate SQL strings
- ❌ Store plaintext passwords
- ❌ Return detailed error messages to users
- ❌ Expose internal paths or stack traces
- ❌ Trust user input without validation

## Build System (Taskfile)

**Important**: This project uses **Taskfile**, not Make.

Common tasks:
```bash
task build        # Build binary (incremental, cached)
task rebuild      # Force rebuild (deletes binary first) - USE THIS when testing changes
task run          # Run with admin setup
task run-dev      # Run without admin setup
task clean        # Clean build artifacts (preserves database)
task clean-all    # Clean build artifacts AND database (destructive)
task test         # Run tests
task fmt          # Format code
task build-prod   # Production build
task deps         # Install dependencies
```

**Critical**: `task build` uses caching based on source file timestamps. If your changes don't appear:
1. Use `task rebuild` to force a fresh build
2. Or run `rm hctf2 && task build`

When documenting or writing scripts, **always use `task`**, never `make`.

## Commit Messages

Use **Conventional Commits** format:

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- **feat**: New feature
- **fix**: Bug fix
- **docs**: Documentation only
- **style**: Code style (formatting, no logic change)
- **refactor**: Code refactoring
- **perf**: Performance improvement
- **test**: Add/update tests
- **build**: Build system or dependencies
- **ci**: CI/CD changes
- **chore**: Other changes (release, etc.)

### Examples

```
feat(auth): add password reset functionality

- Add reset token generation
- Send reset email
- Update password with token validation

Closes #42
```

```
fix(database): prevent SQL injection in search

Use parameterized queries instead of string concatenation

BREAKING CHANGE: Search API now requires exact match
```

```
docs(readme): update installation instructions

Replace Make commands with Task commands
```

## Versioning (SemVer)

Follow Semantic Versioning: `MAJOR.MINOR.PATCH`

- **MAJOR**: Breaking changes (incompatible API)
- **MINOR**: New features (backwards compatible)
- **PATCH**: Bug fixes (backwards compatible)

Current version: **v0.5.0** (User Management, Challenge Completion Indicators, SQL Playground per challenge, Profile Links, and Theme Toggle)

### When to Bump

- **PATCH** (0.1.0 → 0.1.1): Bug fixes, small improvements
- **MINOR** (0.1.0 → 0.2.0): New features, non-breaking changes
- **MAJOR** (0.1.0 → 1.0.0): Breaking API changes, major refactoring

## Testing

### Current State
- ✅ Unit tests implemented in `handlers_test.go`
- ⏳ Integration tests not yet implemented
- ✅ Manual testing via browser

### Running Tests

```bash
task test         # Run all tests
go test ./... -v  # Run with verbose output
```

### When Adding Tests

1. Create `*_test.go` files next to code
2. Use table-driven tests
3. Test edge cases (empty input, nil, errors)
4. Mock database with interfaces

Example:
```go
func TestHashPassword(t *testing.T) {
    tests := []struct {
        name     string
        password string
        wantErr  bool
    }{
        {"valid password", "test123", false},
        {"empty password", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            hash, err := HashPassword(tt.password)
            if (err != nil) != tt.wantErr {
                t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
            }
            if !tt.wantErr && hash == "" {
                t.Error("HashPassword() returned empty hash")
            }
        })
    }
}
```

## Common Tasks

### Adding a New Challenge Category

1. No code changes needed - categories are strings in database
2. Add to challenge creation via API
3. Update UI filtering if needed in `templates/challenges.html`

### Adding a New API Endpoint

1. Create handler function in `internal/handlers/`
2. Register route in `main.go`
3. Add authentication middleware if needed
4. **CRITICAL**: Add swag annotations to the handler, then run `task generate-openapi` to update `docs/openapi.yaml`
5. Update relevant templates if UI changes

### OpenAPI Specification

**Location**: `docs/openapi.yaml` (auto-generated — do NOT edit by hand)

**Generation**: Run `task generate-openapi` after any API change.

**How it works**: swaggo/swag reads `// @Summary`, `// @Router`, and related annotations from handler function comments and generates `docs/openapi.yaml`.

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

### Changing Database Schema

1. Create migration files in `internal/database/migrations/`
   - `XXX_description.up.sql` - apply changes
   - `XXX_description.down.sql` - rollback changes
2. Update models in `internal/models/models.go`
3. Update queries in `internal/database/queries.go`
4. Rebuild: `task clean && task build`
5. Test migration by running server

### Adding a New Page

1. Create template in `internal/views/templates/pagename.html`
2. Define "content" block (see existing templates)
3. Add route handler in `main.go`
4. Render with `s.render(w, "base.html", data)`

## What NOT to Do

### Code

- ❌ Don't use CGO dependencies (breaks single binary)
- ❌ Don't add heavy frameworks (Gin, Echo - use Chi)
- ❌ Don't use ORMs (GORM, etc. - use raw SQL)
- ❌ Don't store secrets in code (use env vars/flags)
- ❌ Don't ignore errors with `_`
- ❌ Don't use global variables (except embedded FS)

### Dependencies

- ❌ Don't use `mattn/go-sqlite3` (requires CGO)
- ✅ Use `modernc.org/sqlite` (pure Go)
- ❌ Don't use `gorilla/mux` (too heavy)
- ✅ Use `go-chi/chi` (lightweight)
- ❌ Don't add frontend frameworks (React, Vue)
- ✅ Use HTMX + Alpine.js + Tailwind
- ✅ Use `github.com/yuin/goldmark` for Markdown (pure Go)
- ✅ Use `golang.org/x/crypto` for bcrypt
- ✅ Use `github.com/golang-jwt/jwt` for authentication
- ✅ Use `github.com/golang-migrate/migrate` for database migrations

### Documentation

- ❌ Don't reference `make` (use `task`)
- ❌ Don't add emojis unless user requests
- ❌ Don't create new .md files without reason
- ✅ Update existing docs when changing features

### Git

- ❌ Don't commit binaries or databases
- ❌ Don't commit without conventional commit format
- ❌ Don't skip semver tags for releases
- ✅ Keep commits atomic and focused

## Implemented Features (Formerly Phase 2)

The following features have been implemented:

1. ✅ **Admin Web UI** - Full CRUD forms for challenges, questions, hints, categories, difficulties
2. ✅ **Team Management** - Complete UI with team creation, join via invite code, ownership transfer, disband
3. ✅ **Hints System** - Full unlock UI with point cost display
4. ✅ **Markdown Support** - Goldmark-based renderer in `internal/utils/markdown.go`
5. ✅ **Search Functionality** - Client-side challenge search with Alpine.js
6. ✅ **User Profiles** - `/profile` (own) and `/users/{id}` (public) with stats and activity
7. ✅ **Password Reset** - Secure token-based reset flow
8. ✅ **Site Settings** - Custom categories, difficulties, and HTML/JS code injection
9. ✅ **User Management Admin Panel** - Promote/demote users, delete users
10. ✅ **Challenge Completion Indicators** - Progress bars, completion styling
11. ✅ **SQL Playground for Challenges** - Enable SQL mode per challenge
12. ✅ **Enhanced Profile Links** - Clickable user names throughout
13. ✅ **Dark/Light Theme Toggle** - Theme switching with persistence

### Planned Features

1. **File Uploads** - Add local storage or S3 integration for challenge files
2. **Real-time Notifications** - WebSocket-based solve notifications
3. **Challenge Docker Integration** - Per-challenge container spawning

## Questions to Ask

Before implementing something, consider:

1. **Does this break single binary deployment?** (CGO, external files)
2. **Does this follow the established patterns?** (handlers, queries, templates)
3. **Is this properly secured?** (SQL injection, XSS, auth)
4. **Is this documented?** (CLAUDE.md, README.md, etc.)
5. **Does this use Task, not Make?** (build commands)

## Handler Organization

Handlers are organized by domain in `internal/handlers/`:

| Handler | File | Purpose |
|---------|------|---------|
| AuthHandler | `auth.go` | Login, register, logout, password reset |
| ChallengeHandler | `challenges.go` | Challenges, questions, submissions, hints CRUD |
| TeamHandler | `teams.go` | Team creation, joining, management |
| HintHandler | `hints.go` | Hint viewing and unlocking |
| ScoreboardHandler | `scoreboard.go` | Scoreboard data and rankings |
| ProfileHandler | `profile.go` | User profile pages and stats |
| SettingsHandler | `settings.go` | Admin site settings, categories, difficulties, user management |
| SQLHandler | `sql.go` | SQL Playground snapshot API |
| UserHandler | `settings.go` | User management (admin panel for users) |

Page handlers (in `main.go`) route to templates; API handlers return JSON or HTMX fragments.

## Useful References

- **Project Docs**: README.md
- **Architecture**: ARCHITECTURE.md
- **SQL Playground**: SQL_PLAYGROUND.md
- **Testing**: TESTING.md
- **Go Style**: https://go.dev/doc/effective_go
- **Chi Router**: https://go-chi.io/
- **HTMX**: https://htmx.org/
- **Alpine.js**: https://alpinejs.dev/
- **Taskfile**: https://taskfile.dev/

## Emergency Fixes

If something breaks:

1. **Database corruption**: `task db-reset` (WARNING: deletes all data)
2. **Build errors**: `task clean && task deps && task build`
3. **Port conflicts**: Change port: `./hctf2 --port 3000` or `task run-dev -- --port 3000`
4. **Template errors**: Check embed paths, rebuild binary: `task clean && task build`
5. **DuckDB WASM not loading**: Run `task setup-sql` to download WASM files
6. **Migration failures**: Check `internal/database/migrations/` for syntax errors

## Summary

- Use **Task**, not Make
- Keep it **simple** and **secure**
- Follow **conventional commits**
- Use **SemVer** for releases
- **No CGO**, **no heavy frameworks**
- **Server-side rendering** with HTMX + Alpine.js
- Read code before changing
- Document changes
- Test locally (`task test`)

Happy coding!
