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
├── cmd/server/           # Application entry point
│   └── main.go          # Router setup, middleware, handlers
├── internal/            # Private application code
│   ├── auth/           # Authentication & middleware
│   ├── database/       # Database layer with embedded migrations
│   ├── handlers/       # HTTP handlers
│   ├── models/         # Data structures
│   └── views/          # Templates & static files (embedded)
├── migrations/         # SQL migrations (legacy, prefer internal/database/migrations)
├── Taskfile.yml        # Build automation (NOT Makefile)
├── go.mod              # Go dependencies
└── *.md                # Documentation
```

## Development Workflow

### Making Changes

1. **Read Before Editing**: Always read existing code before modifying
2. **Test Locally**: Changes should be testable with `task run`
3. **Update Docs**: If changing APIs or behavior, update relevant .md files
4. **Commit Properly**: Use conventional commits (see below)

### Adding New Features

Follow this order:

1. **Model** - Add struct to `internal/models/models.go`
2. **Migration** - Create SQL migration in `internal/database/migrations/`
3. **Database** - Add queries to `internal/database/queries.go`
4. **Handler** - Create handler in `internal/handlers/`
5. **Route** - Register route in `cmd/server/main.go`
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
func (h *Handler) GetChallenge(w http.ResponseWriter, r *http.Request) {
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
w.Write([]byte(`<div class="text-green-400">✅ Correct!</div>`))
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
task build        # Build binary
task run          # Run with admin setup
task run-dev      # Run without admin setup
task clean        # Clean build artifacts
task test         # Run tests
task fmt          # Format code
task build-prod   # Production build
task deps         # Install dependencies
```

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

Current version: **v0.1.0** (initial release)

### When to Bump

- **PATCH** (0.1.0 → 0.1.1): Bug fixes, small improvements
- **MINOR** (0.1.0 → 0.2.0): New features, non-breaking changes
- **MAJOR** (0.1.0 → 1.0.0): Breaking API changes, major refactoring

## Testing

### Current State
- ⏳ Unit tests not yet implemented
- ⏳ Integration tests not yet implemented
- ✅ Manual testing via browser

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
2. Register route in `cmd/server/main.go`
3. Add authentication middleware if needed
4. Document in `API.md`
5. Update relevant templates if UI changes

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
3. Add route handler in `cmd/server/main.go`
4. Render with `s.render(w, "base.html", data)` + `s.templates.ExecuteTemplate(w, "pagename.html", data)`

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

## Phase 2 Features (Planned)

When implementing these, follow the patterns above:

1. **Admin Web UI** - Currently API-only, needs CRUD forms
2. **Team Management** - Schema exists, needs UI
3. **Hints System** - Schema exists, needs unlock UI
4. **File Uploads** - Add local storage or S3 integration
5. **Markdown Support** - Add markdown renderer for descriptions

## Questions to Ask

Before implementing something, consider:

1. **Does this break single binary deployment?** (CGO, external files)
2. **Does this follow the established patterns?** (handlers, queries, templates)
3. **Is this properly secured?** (SQL injection, XSS, auth)
4. **Is this documented?** (API.md, README.md, etc.)
5. **Does this use Task, not Make?** (build commands)

## Useful References

- **Project Docs**: README.md, INSTALL.md, QUICKSTART.md
- **Architecture**: ARCHITECTURE.md
- **API**: API.md
- **Status**: IMPLEMENTATION_STATUS.md
- **Go Style**: https://go.dev/doc/effective_go
- **Chi Router**: https://go-chi.io/
- **HTMX**: https://htmx.org/
- **Taskfile**: https://taskfile.dev/

## Emergency Fixes

If something breaks:

1. **Database corruption**: `task db-reset` (WARNING: deletes data)
2. **Build errors**: `task clean && task deps && task build`
3. **Port conflicts**: Change port: `./hctf2 --port 3000`
4. **Template errors**: Check embed paths, rebuild binary

## Summary

- Use **Task**, not Make
- Keep it **simple** and **secure**
- Follow **conventional commits**
- Use **SemVer** for releases
- **No CGO**, **no heavy frameworks**
- **Server-side rendering** with HTMX
- Read code before changing
- Document changes
- Test locally

Happy coding! 🚀
