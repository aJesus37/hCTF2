# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/yourusername/hctf2/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/yourusername/hctf2/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/yourusername/hctf2/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/yourusername/hctf2/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/yourusername/hctf2/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/yourusername/hctf2/releases/tag/v0.1.0
