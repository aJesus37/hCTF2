# New Features Design — 2026-02-19

## Scope

Implementation plan for items from `new-features.md`. Items 1 and 2 are already done.

---

## Already Done (No Action Required)

**Item 1 — Migrations support**: Already implemented. `golang-migrate` runs all embedded SQL files at startup via `runMigrations()` in `internal/database/db.go`. Existing users who replace the binary automatically receive schema upgrades without data loss.

**Item 2 — Static builds**: Already implemented. All Taskfile build commands use `CGO_ENABLED=0`, producing fully static binaries. All assets (templates, migrations, static files, OpenAPI spec) are embedded via Go's `embed` directive.

---

## Items to Implement

### A. Small standalone changes

#### Item 5 — Team name as link in profile
- **File**: `internal/views/templates/profile.html`
- **Change**: Replace `<span class="text-blue-400">{{.Stats.TeamName}}</span>` with `<a href="/teams/{{.Stats.TeamID}}" class="text-blue-400 hover:underline">{{.Stats.TeamName}}</a>`
- **Also needed**: Add `TeamID` to `UserStats` struct in `internal/models/models.go` and include it in the profile query in `internal/database/queries.go`

#### Item 6 — Comprehensive config.example.yaml
- **File**: `config.example.yaml`
- **Change**: Rewrite to document every possible config key with inline comments explaining what each does and what the default is if the key is omitted. Cover: server (port, host, read/write timeouts), database (path), auth (jwt_secret, session_duration, bcrypt_cost), admin (email, password), telemetry (enabled, path).

#### Item 8 — Metrics/monitoring documentation
- **File**: `OPERATIONS.md`
- **Change**: Add a "Metrics & Monitoring" section documenting: what the telemetry package exposes, the HTTP endpoint(s), how to wire up Prometheus scraping, relevant metrics names, and a sample Grafana dashboard query.

---

### B. README rewrite + logo (Items 3, 4)

#### Item 4 — Logo (SVG icon)
- **Approach**: Minimal geometric SVG combining a flag motif with terminal bracket aesthetics
- **Colors**: Purple `#A55EEA` (matches existing admin UI) on dark background
- **Output files**:
  - `internal/views/static/logo.svg` — full logo with text
  - `internal/views/static/favicon.svg` — icon-only variant for browser tab
- **Integration**: Update `internal/views/templates/base.html` to use `favicon.svg` as the favicon. Reference logo in README.

#### Item 3 — README rewrite
- **Audience**: Self-hosted / home-lab users
- **Tone**: Casual but professional
- **Structure**:
  1. Logo + name + one-line pitch + badges (license, release, Go version)
  2. Short "What it is" + feature list (concise bullets)
  3. Quick start: Docker Compose in 3 commands (front-and-center)
  4. Installation: Docker, native Go binary, config
  5. Self-hosting notes: volume mounts, reverse proxy example (Caddy/Nginx), backup (`cp hctf2.db backup/`)
  6. Upgrade notes: "replace the binary, migrations run automatically"
  7. Security: JWT auth, bcrypt passwords, no telemetry by default
  8. Links to: ARCHITECTURE.md, CONFIGURATION.md, OPERATIONS.md, SQL_PLAYGROUND.md
  9. Contributing + License
  10. "How this was built" — honest 1-paragraph AI assistance disclosure

- **Remove from README**: Anything that's now covered by linked docs
- **Delete these files** (absorbed into README or no longer needed):
  - `INSTALL.md` → content merged into README Installation section
  - `QUICKSTART.md` → content merged into README Quick Start section
  - `API.md` → replaced by auto-generated OpenAPI spec + Swagger UI at `/api/openapi`
  - `FEATURES_IMPLEMENTATION.md` — internal history doc, no user value
  - `IDEA.md` — internal planning doc
  - `IMPLEMENTATION_SUMMARY.md` — internal history doc
  - `IMPROVEMENTS_AND_ROADMAP.md` — superseded, no user value
  - `DOCKER.md` → content merged into README Self-hosting section

- **Keep**: `ARCHITECTURE.md`, `CONFIGURATION.md`, `KNOWN_ISSUES.md`, `OPERATIONS.md`, `SQL_PLAYGROUND.md`, `TESTING.md`

---

### C. OpenAPI automation with swaggo/swag (Item 7)

- **Decision**: Switch from manually maintained OpenAPI 3.0 to auto-generated Swagger 2.0 via `swaggo/swag`. The API is simple enough that 2.0 covers all features. Auto-generation keeps the spec permanently in sync.

- **Changes**:
  1. Install swag CLI: `go install github.com/swaggo/swag/cmd/swag@latest`
  2. Add `github.com/swaggo/swag` to `go.mod` (for annotation parsing only)
  3. Add general info block to `main.go`:
     ```go
     // @title hCTF2 API
     // @version 1.0
     // @description CTF platform API
     // @host localhost:8090
     // @BasePath /api
     // @securityDefinitions.apikey BearerAuth
     // @in header
     // @name Authorization
     ```
  4. Add `// @Summary`, `// @Tags`, `// @Param`, `// @Success`, `// @Failure`, `// @Router` annotations to every handler in `internal/handlers/`
  5. Add `task generate-openapi` to `Taskfile.yml`:
     ```yaml
     generate-openapi:
       desc: Generate OpenAPI spec from code annotations
       cmds:
         - swag init -g main.go -o docs/ --outputTypes yaml
     ```
  6. Run `task generate-openapi` to produce `docs/swagger.yaml` (replaces `docs/openapi.yaml`)
  7. Update `main.go` embed path to serve the generated file
  8. Update CLAUDE.md: "After changing any API handler, run `task generate-openapi`"

- **Handlers to annotate** (~30 endpoints):
  - `internal/handlers/auth.go`: Login, Register, Logout, ForgotPassword, ResetPassword
  - `internal/handlers/challenges.go`: List, Get, Submit, CreateChallenge, UpdateChallenge, DeleteChallenge, CreateQuestion, UpdateQuestion, DeleteQuestion, GetHints, GetAdminHints
  - `internal/handlers/teams.go`: List, Create, Get, Join, Leave, Disband, RegenerateInvite, UpdateInvitePermission, TransferOwnership
  - `internal/handlers/hints.go`: UnlockHint
  - `internal/handlers/scoreboard.go`: GetScoreboard, GetTeamScoreboard
  - `internal/handlers/profile.go`: GetProfile, GetUserProfile
  - `internal/handlers/settings.go`: GetSettings, UpdateSettings, admin user management endpoints
  - `internal/handlers/sql.go`: SQL snapshot endpoints

---

## Summary of Changes

| Item | Files Changed | Complexity |
|------|--------------|-----------|
| 5 - Team link | profile.html, models.go, queries.go | Low |
| 6 - Config example | config.example.yaml | Low |
| 8 - Monitoring docs | OPERATIONS.md | Low |
| 4 - Logo | logo.svg, favicon.svg, base.html | Medium |
| 3 - README rewrite | README.md, delete 8 files | Medium |
| 7 - OpenAPI swaggo | main.go, all handlers, Taskfile.yml, CLAUDE.md | High |

## Order of Implementation

1. Items 5, 6, 8 (small, independent, no risk)
2. Item 4 (logo — visual asset, no code risk)
3. Item 3 (README rewrite + file deletions)
4. Item 7 (swaggo annotations — largest change, do last)
