# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.8.0] - 2026-03-15

### Added
- `config export` / `config import` CLI commands — full platform config backup and restore (challenges, competitions, settings) with JSON and YAML support
- YAML auto-detection for config import/export (by file extension or `--format yaml` flag)
- Docker as the primary and recommended deployment method

### Changed
- Documentation overhaul: Docker is now the primary deployment method across README, OPERATIONS, and ARCHITECTURE
- Removed all systemd/systemctl references from documentation — use Docker Compose instead
- Fixed broken `docker run` commands in README (missing `serve` subcommand, nonexistent `DATABASE_PATH` env var)
- Fixed `--jwt-secret` and bare `./hctf2` commands missing `serve` subcommand
- Dockerfile uses scratch base image with non-root user (UID 1000)

### Fixed
- README `docker run` example used nonexistent `DATABASE_PATH` environment variable (now uses `--db` flag)
- Multiple documentation commands missing required `serve` subcommand

## [0.7.0] - 2026-03-14

### Added
- Full CLI parity with web UI — all CRUD operations available from the command line
- `question update` — update name, flag, points, and case-sensitivity via CLI
- `hint update` — update content, cost, and order via CLI
- `competition update` — update name and description via CLI
- `competition teams` — list teams registered for a competition
- `competition blackout` / `unblackout` — scoreboard blackout toggle via CLI
- `competition scoreboard` — per-competition scoreboard from CLI
- `scoreboard freeze` / `unfreeze` — global scoreboard freeze toggle
- `submissions` command — live submission feed with `--competition` filter and `--watch` auto-refresh mode
- `user profile` — view own or another user's profile (name, rank, points, solves)
- `challenge export` / `import` — JSON bundle for backup and sharing
- `challenge update` — `--visible`, `--min-points`, `--decay` flags for dynamic scoring
- `competition create` — now interactive on TTY (prompts for name and description)
- `team create` — now interactive on TTY (prompts for name when not given as arg)
- `challenge delete` — confirmation prompt on TTY (consistent with competition/team delete)
- CLI integration tests (`cli_integration_test.go`) — 137 tests covering all commands, JSON output, error cases; TestMain pattern builds real binary and starts server subprocess
- `cmd/helpers.go` — shared CLI helpers: `confirmIfTTY`, `boolToYesNo`, `abortedMsg`

### Changed
- `hint create` — one field per huh form page (matches challenge/question UX, enables back navigation)
- All huh forms use one `huh.NewGroup` per field so back navigation works between pages
- `tui.Truncate()` standardized across all table ID columns (removed manual `[:8] + "..."` patterns)
- `parseCompetitionID()` helper extracted; `confirmIfTTY()` replaces inline confirmation blocks in 4 commands
- CI test command uses `-count=1 -timeout 120s` to prevent caching

### Fixed
- Category/difficulty list showed truncated IDs (widened column to 34 for 32-char hex IDs)
- Smoke test CI used stale `./hctf2 --port` invocation (fixed to `./hctf2 serve --port`)
- `UpdateQuestion`/`UpdateHint`/`SetCompetitionBlackout` now return 404 on nonexistent IDs
- `GetSubmissionFeed` validates competition exists before querying (returns 404 for unknown IDs)
- `ImportChallenges` client now uses correct `multipart/form-data` with `file` field
- vet warning in `cmd/submissions.go` — non-constant format string in `fmt.Fprintf`
- errcheck lint warnings in integration tests

## [0.6.0] - 2026-03-08

### Added
- Competition Lifecycle Management — time-bounded competitions with draft→registration→running→ended auto-transitions, registered teams, per-competition scoreboards, and scoreboard blackout
- Live Submission Feed — per-competition and global `/submissions` page polling every 10s; public view shows correct solves, admin view shows all attempts with submitted flag text
- Competition score evolution chart — Chart.js line chart on competition page derived directly from submissions
- Challenge cards on competition page — difficulty badge, category, progress bar, solved/total question count
- Question anchor links — each question card has `#question-{id}` anchor; hover-to-copy `#` button on title; live feed and profile activity link directly to the question
- Score evolution chart on scoreboard — visual timeline showing top 20 competitors' scores over time using Chart.js
- Background score recorder — captures score snapshots every 15 minutes for historical tracking
- Admin visibility toggle — control whether admins appear in scoreboard and chart (default: hidden)
- Configurable JWT secret via --jwt-secret flag, JWT_SECRET env var, or config file
- SQLite WAL mode for improved concurrent performance
- Configurable CORS origins (default: same-origin only)
- SECURITY.md with vulnerability reporting process
- CONTRIBUTING.md with development workflow

### Fixed
- Multiple wrong submissions now allowed per user per question (removed `UNIQUE(question_id, user_id)` constraint via migration 017)
- Wrong-answer feedback no longer flickers between sequential submissions
- Live submission feed now covers all challenges, not just competition-scoped ones
- Competition challenge cards showed 0/0 questions for hidden challenges (removed `visible=1` filter)
- Blank Chart.js canvas when no score data (canvas hidden instead of empty render)

### Security
- JWT secret is now required in production mode
- CORS defaults to same-origin only (removed wildcard)

## [0.5.0] - 2026-02-21

### Added
- UUIDv7 for all ID generation (migration from random hex)
- OpenTelemetry metrics and tracing support
- Prometheus /metrics endpoint
- SMTP email configuration for password reset
- Health check endpoints (/healthz, /readyz)
- Password reset flow with secure token-based authentication
- E2E browser automation test suite
- SQL Playground documentation (SQL_PLAYGROUND.md)

### Changed
- Improved password reset error feedback using hx-on::response-error
- Password reset tokens now stored in UTC

### Fixed
- Challenges page error for unauthenticated users
- Password reset token validation timezone issues

## [0.4.0] - 2026-02-15

### Added
- Team management with secure invite codes (128-bit cryptographic)
- Team profile pages with member listing
- Team-based scoreboard view
- Team invite/join flow

### Changed
- Enhanced admin dashboard with team management

## [0.3.0] - 2026-02-10

### Added
- Hint system with point cost deduction
- Hint unlock tracking per user/team
- Admin hint management in dashboard
- Hints displayed on challenge pages

## [0.2.0] - 2026-02-05

### Added
- Challenge categories and difficulty levels
- Challenge filtering and search
- Markdown rendering for challenge descriptions
- Scoreboard with individual rankings
- User profiles with statistics
- OpenAPI documentation at /api/openapi

### Changed
- HTMX integration for dynamic interactions
- Alpine.js for reactive UI components

## [0.1.0] - 2026-02-01

### Added
- Initial release
- User registration and authentication
- Basic challenge CRUD (admin)
- Question/flag management
- SQLite database with migrations
- Dark/light theme support
- Docker deployment support
- Task-based build system (Taskfile.yml)

[Unreleased]: https://github.com/ajesus37/hCTF2/compare/v0.8.0...HEAD
[0.8.0]: https://github.com/ajesus37/hCTF2/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/ajesus37/hCTF2/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/ajesus37/hCTF2/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/ajesus37/hCTF2/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/ajesus37/hCTF2/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/ajesus37/hCTF2/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/ajesus37/hCTF2/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/ajesus37/hCTF2/releases/tag/v0.1.0
