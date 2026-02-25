# hCTF2 Open Source Readiness Review

**Date**: 2026-02-21
**Version Reviewed**: v0.5.0 (commit 0748688)
**Reviewer**: Automated audit (Claude)

---

## Executive Summary

**Verdict: Not quite ready for launch, but close.**

hCTF2 has a genuinely compelling differentiator -- single-binary, zero-dependency CTF platform -- that no actively-maintained competitor offers. The code is solid, the UX is professional, and the documentation is above average for the CTF platform space. However, there are blocking issues that would hurt first impressions and adoption if launched today.

**Estimated effort to launch-ready: 2-3 focused days.**

### Scoring Summary

| Category | Score | Weight | Notes |
|----------|-------|--------|-------|
| Feature completeness | 6.5/10 | 20% | Missing dynamic scoring, file uploads |
| Code quality & security | 7/10 | 25% | JWT secret is critical; rest is solid |
| User experience | 8/10 | 15% | Professional frontend, good UX |
| Documentation | 7/10 | 15% | Great technical docs, missing governance |
| Open source readiness | 4/10 | 15% | No CI, releases, contributing guide |
| Competitive positioning | 8/10 | 10% | Strong niche, unique value prop |

**Weighted Score: 6.7/10** | **After fixing P0 blockers: ~8/10**

---

## 1. Feature Completeness

### What hCTF2 Does Well (Competitive Parity)

- Challenge & question management with full CRUD
- Team management with secure invite codes (128-bit cryptographic codes)
- Hints system with point cost deduction
- Live scoreboard (individual + team views)
- Admin dashboard with tabbed interface
- Markdown descriptions (Goldmark)
- Dark/light theme with persistence
- Password reset flow (secure token-based)
- OpenAPI documentation (auto-generated, served at `/api/openapi`)
- Docker deployment (multi-stage build, non-root user, health checks)
- OpenTelemetry instrumentation
- Client-side challenge search and filtering

### hCTF2's Unique Advantages

1. **Single binary deployment** -- no Redis, Postgres, Python, Node.js, or Docker required to run the platform
2. **SQL Playground** (DuckDB WASM) -- no other CTF platform has this; enables SQL injection training and data analysis challenges
3. **Zero external dependencies** -- ideal for classrooms, workshops, air-gapped environments
4. **Go performance** -- handles concurrency far better than Python-based CTFd with lower memory usage
5. **Embedded everything** -- assets, migrations, templates all compiled into the binary

### Critical Missing Features

| Feature | Impact | Why It Matters | Effort |
|---------|--------|----------------|--------|
| **Dynamic scoring** (decay as more teams solve) | HIGH | Expected by most CTF organizers for competitive events | Medium |
| **File attachments** for challenges | HIGH | Required for crypto, forensics, binary exploitation, and reversing challenges | Medium |
| **Score freezing** at competition end | MEDIUM | Standard for timed competitions to prevent last-second gaming | Low |
| **Challenge import/export** (JSON/YAML) | MEDIUM | Portability between platforms; reuse challenges across events | Medium |
| **CTFtime.org JSON export** | MEDIUM | Visibility in the competitive CTF community | Low |
| **Rate limiting on flag submissions** | MEDIUM | Anti-bruteforce protection | Low |
| **Email verification** | LOW | Optional for most use cases but expected for public-facing instances | Medium |

### Features That Can Wait (Post-Launch)

- Docker per-team containers (nice-to-have, not blocking)
- Plugin system (don't compete with CTFd here)
- Real-time WebSocket scoreboard (already planned, not blocking)
- Prerequisite/unlockable challenges
- Multi-language support (i18n)

### Assessment: 6.5/10

Usable for educational and workshop CTFs today. Missing dynamic scoring and file attachments blocks adoption for competitive CTFs. The SQL Playground is a genuine differentiator that should be highlighted in marketing.

---

## 2. Code Quality & Bugs

### Security Audit

| Area | Status | Details |
|------|--------|---------|
| SQL injection | EXCELLENT | 100% parameterized queries across all database operations |
| Password hashing | EXCELLENT | bcrypt cost 12, constant-time comparison |
| XSS prevention | GOOD | Go `html/template` auto-escaping throughout |
| Authentication | GOOD | JWT + middleware consistently applied to protected routes |
| Authorization | GOOD | `RequireAdmin` and `RequireAuth` middleware properly used |
| Password reset | EXCELLENT | 32-byte crypto/rand tokens with 30-minute expiration |
| Flag visibility | GOOD | Flags hidden from non-admin users via `json:"-"` tag |
| **JWT secret** | **CRITICAL** | Hardcoded with no way to configure -- see Bug #1 |
| CORS | NEEDS FIX | `Access-Control-Allow-Origin: *` is overly permissive |
| Cookie security | GOOD | HttpOnly + SameSite=Lax, but missing conditional `Secure` flag |

### Confirmed Bugs

#### Bug #1: JWT Secret Not Configurable (CRITICAL)

**File**: `internal/auth/middleware.go:26`
```go
var jwtSecret = []byte("change-this-secret-in-production")
```

The JWT signing secret is a hardcoded package-level variable with **no CLI flag, environment variable, or `SetJWTSecret()` function** to override it. Every deployment uses the identical secret unless someone edits the source code and recompiles.

**Impact**: Any attacker can forge valid JWT tokens for any user, including admin accounts.

**Fix**: Add `--jwt-secret` CLI flag and `JWT_SECRET` environment variable. Refuse to start if the default value is used when not in development mode.

#### Bug #2: Missing WAL Mode for SQLite (HIGH)

**File**: `internal/database/db.go`

No `PRAGMA journal_mode = WAL` is set. SQLite defaults to DELETE journal mode, where every write blocks all readers and vice versa. During a CTF competition with concurrent users, this causes:
- Slow page loads during flag submissions
- Database locked errors under load
- Degraded scoreboard performance

**Fix**: Add one line after the foreign keys pragma:
```go
db.Exec("PRAGMA journal_mode = WAL")
```

#### Bug #3: CORS Allows All Origins (MEDIUM)

**File**: `main.go` (CORS middleware)
```go
w.Header().Set("Access-Control-Allow-Origin", "*")
```

This allows any website to make authenticated requests to the API.

**Fix**: Make CORS origin configurable or restrict to same-origin.

#### Bug #4: Scoreboard Tie Handling (MEDIUM)

Scoreboard rank is an incrementing counter. Two users with identical scores receive different ranks (rank 1 and rank 2 instead of both rank 1).

**Impact**: Visible to users, affects fairness perception in competitive events.

**Fix**: Implement standard competition ranking (same score = same rank, next rank skips).

#### Bug #5: HintUnlock Model Missing TeamID (LOW)

**File**: `internal/models/models.go:76-81`

Migration 006 added `team_id` to the `hint_unlocks` table, but the `HintUnlock` Go struct was never updated. The struct is currently unused (queries scan into raw variables), so this doesn't cause runtime errors, but it's a code hygiene issue.

### Code Quality Issues (Non-Blocking)

| Issue | Severity | Location |
|-------|----------|----------|
| N+1 query in admin dashboard | MEDIUM | `main.go:788-793` -- loops per-question for hints |
| Ignored errors with `_` | LOW | Stats queries in `main.go:500-502` |
| Magic numbers | LOW | `168 * time.Hour` (7 days), `32` (token length) |
| No input validation layer | MEDIUM | No max length checks on team names, descriptions, etc. |
| No transactions for multi-step operations | MEDIUM | Hint unlock + score deduction not atomic |
| HTML strings embedded in Go code | LOW | `main.go:877-934` |
| No connection pool configuration | LOW | Using `database/sql` defaults |
| Duplicate admin middleware | LOW | Two implementations: `auth.RequireAdmin` and `s.requireAdmin` |

### Assessment: 7/10

The JWT secret is the only showstopper. SQL injection protection and password security are excellent. The rest is polish work.

---

## 3. User Experience & Frontend

### Strengths

- **Professional design**: Clean dark theme with full light mode support, custom color palette (`#0f172a` background, `#1e293b` surface, purple accents)
- **Responsive**: Works from mobile (single column) to ultrawide (5-column grid) with proper breakpoints
- **HTMX integration**: Snappy, app-like interactions -- form submissions, challenge filtering, hint unlocking all happen without page reloads
- **Alpine.js state**: Lightweight reactive state for filters, tabs, modals, theme toggle
- **Form UX**: Proper labels, autofocus on first field, minlength validation, inline error messages
- **Error pages**: Styled 403/404/405 pages matching the site theme
- **Challenge filtering**: Category + difficulty dropdowns + text search work independently and together
- **Theme persistence**: localStorage-based, prevents flash of unstyled content on load

### Frontend Issues

| Issue | Severity | Details |
|-------|----------|---------|
| Tailwind loaded from CDN | MEDIUM | Contradicts air-gapped/single-binary promise; breaks offline use |
| No skip-to-main-content link | LOW | Accessibility (WCAG 2.4.1) |
| Modal dialogs lack focus trap | MEDIUM | Tab key can escape modal overlay |
| No loading/disabled state on submit buttons | LOW | Double-submit possible during slow connections |
| Search results don't show count | LOW | "3 challenges found" feedback would help |
| Theme toggle lacks `aria-label` | LOW | Screen reader accessibility |
| No first-time user guidance | LOW | No in-app help for CTF beginners |

### Accessibility Summary

| Category | Score | Notes |
|----------|-------|-------|
| Semantic HTML | 9/10 | Proper use of `nav`, `main`, `section`, `header`, `footer` |
| Form labels | 10/10 | All inputs have associated labels |
| ARIA attributes | 5/10 | Minimal usage, mostly not needed due to good semantics |
| Keyboard navigation | 7/10 | Works but focus management in modals needs improvement |
| Color contrast | 8/10 | Dark theme excellent; light theme likely passing but needs WCAG audit |
| Error messages | 9/10 | Clear, color-coded, visible |
| Screen reader compatibility | 7/10 | Good structure, could use more aria-labels |

### Assessment: 8/10

The frontend is one of hCTF2's strengths. Professional quality that competes well with established platforms.

---

## 4. Documentation

### What Exists

| Document | Lines | Quality | Coverage |
|----------|-------|---------|----------|
| README.md | ~120 | Good | Quick start, features, security, license |
| ARCHITECTURE.md | ~220 | Excellent | Tech stack, patterns, schema, auth flow |
| CONFIGURATION.md | ~380 | Excellent | All CLI flags, env vars, SMTP, OTel, examples |
| OPERATIONS.md | ~570 | Excellent | Deployment (systemd/Docker/nginx), monitoring, troubleshooting |
| TESTING.md | ~330 | Excellent | Unit/smoke/E2E tests, CI/CD examples, browser debugging |
| SQL_PLAYGROUND.md | ~220 | Good | Setup, architecture, usage, troubleshooting |
| CLAUDE.md | ~450 | Excellent | Developer guide, patterns, security requirements |

**Total**: ~2,300 lines of documentation across 7 files.

### What's Missing

| Document | Priority | Why It's Needed |
|----------|----------|-----------------|
| **CHANGELOG.md** | HIGH | Users need to know what changed between versions; no git tags exist either |
| **CONTRIBUTING.md** | HIGH | Contributors won't know PR workflow, commit format, or dev setup |
| **SECURITY.md** | HIGH | No vulnerability reporting process; critical for security-focused tool |
| **GitHub Actions CI** | HIGH | No automated testing = no contributor confidence |
| **Issue templates** | MEDIUM | Structured bug reports and feature requests |
| **PR template** | MEDIUM | Consistent pull request descriptions |
| CODE_OF_CONDUCT.md | LOW | Standard for community-facing projects |

### Documentation Quality Issues

- No version/release tags in git history (v0.5.0 mentioned in config but never tagged)
- README Contributing section is 3 lines with no real guidance
- Some docs reference deleted files (KNOWN_ISSUES.md was removed)
- No documentation index linking all docs from README
- new-features.md exists at root but is a backlog, not user-facing documentation

### Assessment: 7/10

Technical documentation is above average for the CTF platform space (CTFd's docs are criticized as outdated, GZCTF's are primarily in Chinese). Open source governance documentation is entirely missing.

---

## 5. Database Layer

### Schema Design: Good

- 8 migrations, all with up + down files
- Proper foreign key constraints with appropriate CASCADE/SET NULL behavior
- UNIQUE constraints where needed (email, team invite_id, hint_id+user_id)
- 8 indexes covering common query patterns
- UUIDv7 for primary keys (migration 008)

### Database Issues

| Issue | Severity | Details |
|-------|----------|---------|
| No WAL mode | HIGH | Concurrent reads block on writes (see Bug #2) |
| No connection pool config | LOW | Using `database/sql` defaults |
| Missing indexes | LOW | `teams(owner_id)`, `hint_unlocks(user_id)` |
| Migration 006 down loses UNIQUE constraint | MEDIUM | Rollback doesn't preserve `UNIQUE(hint_id, user_id)` |
| Migration 008 down is a no-op | LOW | `SELECT 1;` -- not truly reversible |
| No transactions for multi-step operations | MEDIUM | Hint unlock + score update not atomic |
| Time parsing silently falls back to `time.Now()` | LOW | `queries.go:289-293` masks potential data issues |

### Query Quality

- **SQL injection**: Zero risk -- 100% parameterized queries verified
- **N+1 queries**: Admin dashboard fetches hints per-question in a loop (`main.go:788-793`)
- **Complex queries**: Scoreboard and team scoring queries are complex but correct
- **Performance**: Adequate for hundreds of concurrent users; would benefit from caching for 1000+ user events

---

## 6. Competitive Landscape

### Market Context

| Platform | Stars | Language | Deploy Complexity | Status |
|----------|-------|----------|-------------------|--------|
| **CTFd** | ~6,500 | Python | Medium-High (Redis + DB required) | Active, dominant |
| **GZCTF** | ~1,300 | C# | High (.NET + Postgres + Docker) | Active, growing |
| **RootTheBox** | ~800 | Python | Medium | Active, niche |
| **rCTF** | ~500 | Node.js | Medium (Postgres + Redis) | Active |
| **Mellivora** | ~500 | PHP | Medium | Stagnant |
| **FBCTF** | ~6,600 | PHP | High | Archived/dead |
| **xctf** | ~80 | Go | Low | Inactive |
| **hCTF2** | New | Go | **Very Low** | Active |

### Feature Comparison (Key Differentiators)

| Feature | CTFd | GZCTF | rCTF | hCTF2 |
|---------|------|-------|------|-------|
| Single binary deploy | No | No | No | **Yes** |
| Zero external deps | No | No | No | **Yes** |
| SQL Playground | No | No | No | **Yes** |
| Dynamic scoring | Yes | Yes | Yes | No |
| File attachments | Yes | Yes | Yes | No |
| Plugin system | Yes | No | No | No |
| Docker containers | Plugin | Native | Separate | No |
| Per-team instances | Plugin | Native | Separate | No |
| Real-time scoreboard | No | Yes | Partial | No |
| Import/export | Yes | Yes | Partial | No |
| CTFtime integration | Yes | No | Yes | No |
| Score freezing | Yes | Yes | Yes | No |
| OpenAPI docs | No | Yes | No | **Yes** |
| Air-gapped support | No | No | No | **Yes** |

### Strategic Positioning

hCTF2 should **not** try to be "CTFd but in Go." CTFd has years of community momentum and a plugin ecosystem.

**Target niche**: Simplicity-first CTF platform for:
- University professors running classroom CTFs (no IT department needed)
- Security trainers doing workshops (bring a USB stick with the binary)
- Small security teams doing internal training
- Air-gapped or restricted environments
- Anyone who has struggled with CTFd deployment

**Positioning**: "The CTF platform that just works. One binary. No dependencies. Start a competition in 60 seconds."

**Competitive moat**: The single-binary, zero-dependency approach is architecturally impossible to replicate in Python (CTFd), C# (GZCTF), or Node.js (rCTF) without a complete rewrite.

---

## 7. Open Source Repository Hygiene

### Current State

| Item | Status | Notes |
|------|--------|-------|
| LICENSE (MIT) | Present | Appropriate for the project |
| .gitignore | Comprehensive | 46 lines, covers all common patterns |
| .dockerignore | Present | Proper exclusions |
| .env.example | Present | Key settings documented |
| config.example.yaml | Present | Complete configuration reference |
| Taskfile.yml | Excellent | 25 tasks, well-documented |
| Dockerfile | Excellent | Multi-stage, non-root, health checks |
| docker-compose.yml | Good | Production-ready with volumes |
| `.github/` directory | **MISSING** | No workflows, templates, or automation |
| Git tags/releases | **MISSING** | No versioned releases available |
| CI/CD workflows | **MISSING** | No automated testing on push/PR |
| Issue templates | **MISSING** | No structured bug/feature templates |
| PR template | **MISSING** | No pull request guidelines |
| CHANGELOG.md | **MISSING** | No version history |
| CONTRIBUTING.md | **MISSING** | No contributor guidelines |
| SECURITY.md | **MISSING** | No vulnerability reporting process |
| CI status badge | **MISSING** | No build status in README |

### First Impression Analysis

When someone lands on the GitHub repo page:

| Signal | Status | Impact |
|--------|--------|--------|
| README with clear description | Good | Positive first impression |
| MIT License | Good | Welcoming to contributors |
| Recent commits | Good | Signals active maintenance |
| CI badge (green/passing) | Missing | Signals unmaintained or amateur |
| Releases with binaries | Missing | No easy way to download and try |
| Issue templates | Missing | Contributors don't know how to report |
| Star count | Zero (new) | No social proof yet |

---

## 8. Launch Blocklist

> **Status as of 2026-02-25:** P0 and P1 are fully complete. P2 is in progress (3/7 done).

### P0 -- Showstoppers ✅ ALL COMPLETE

1. ✅ **Make JWT secret configurable** -- `--jwt-secret` / `JWT_SECRET`, refuses to start with default unless `--dev`
2. ✅ **Enable WAL mode for SQLite** -- `PRAGMA journal_mode = WAL` in `db.go`
3. ✅ **Fix CORS configuration** -- `--cors-origins` / `CORS_ORIGINS`, no wildcard default
4. ✅ **Create CHANGELOG.md**
5. ✅ **Create CONTRIBUTING.md**
6. ✅ **Create SECURITY.md**
7. ✅ **Add GitHub Actions CI** -- `.github/workflows/ci.yml`, multi-version Go matrix, badges in README
8. ✅ **Create first GitHub Release (v0.5.0)** -- release workflow builds Linux/macOS/Windows amd64+arm64 binaries

### P1 -- Should Fix Before Launch ✅ ALL COMPLETE

9. ✅ **Add git tags** -- `v0.1.0`, `v0.2.1`, `v0.5.0`
10. ✅ **Add GitHub issue templates** -- bug report + feature request in `.github/ISSUE_TEMPLATE/`
11. ✅ **Add PR template** -- `.github/pull_request_template.md`
12. ✅ **Fix scoreboard tie handling** -- 1224 rule implemented in both `GetScoreboard` and `GetTeamScoreboard`
13. ✅ **Add CI status badge** -- CI and Release badges in README
14. ✅ **Self-host Tailwind CSS** -- served from `/static/css/tailwind.css`, no CDN dependency
15. ✅ **`--production` flag** -- treated as resolved: `--dev` is opt-in, production is the secure default

### P2 -- Soon After Launch (3/7 complete)

16. ✅ **Dynamic scoring (decay-based)** -- linear decay with `initial_points`, `minimum_points`, `decay_threshold` per challenge
17. ⏳ **File attachments for challenges** -- `FileURL` field exists on questions; upload handler not yet implemented
18. ✅ **Score freezing at competition end** -- scheduled datetime + manual toggle, admin Settings UI, scoreboard filters respected
19. ✅ **CTFtime.org JSON scoreboard export** -- public `GET /api/ctftime`, freeze-aware, URL in admin Settings
20. ✅ **Rate limiting on flag submissions** -- `--submission-rate-limit` (default 5/min per user), HTTP 429 HTMX response
21. ⏳ **Challenge import/export (JSON/YAML)** -- not yet implemented
22. ⏳ **Skip-to-main-content link for accessibility** -- not yet implemented
23. ⏳ **Focus trap in modal dialogs** -- not yet implemented

### P3 -- Future Roadmap

24. Docker per-team container integration
25. Real-time WebSocket scoreboard updates
26. Challenge prerequisite chains
27. Multi-language support (i18n)
28. Email verification (optional)

---

## 9. README Suggestions

The current README is functional but could be more compelling for an open source launch. Consider restructuring to lead with the value proposition:

### Recommended README Structure

1. **Hero section**: Project name + one-line tagline + badges (CI, license, release)
2. **Why hCTF2**: 3-4 bullet points on the single-binary advantage vs. CTFd/GZCTF complexity
3. **Quick Start**: `curl | tar | ./hctf2` in 3 commands
4. **Screenshots**: Dark theme challenges page, admin dashboard, scoreboard, SQL playground
5. **Features**: Organized table or checklist
6. **Documentation**: Links to all docs
7. **Contributing**: Link to CONTRIBUTING.md
8. **License**: MIT

### Missing from README

- Screenshots or demo GIF (visual proof of quality)
- Comparison table vs CTFd/GZCTF
- "Why not CTFd?" section addressing the target audience
- Pre-built binary download links (after releases exist)
- Discord/Matrix community link (if applicable)

---

## 10. Testing Infrastructure

### Current Coverage

| Test Type | Tool | Coverage | Quality |
|-----------|------|----------|---------|
| Unit tests | `handlers_test.go` | Page rendering, navigation, API endpoints | Good |
| ID generation tests | `internal/database/id_test.go` | UUIDv7 generation | Good |
| Email tests | `internal/email/email_test.go` | Email sending | Good |
| Smoke tests | `scripts/smoke-test.sh` | Health checks, public pages, API | Good |
| E2E tests | `scripts/e2e-test.sh` | Full browser automation | Excellent |
| Browser tests | `scripts/browser-automation-tests.sh` | Feature-by-feature validation | Excellent |
| Test data seeding | `scripts/seed-test-data.sh` | Sample CTF data | Good |

### Testing Gaps

| Missing | Priority | Impact |
|---------|----------|--------|
| Auth unit tests (JWT, password hashing) | HIGH | Core security code untested |
| Database query tests | HIGH | Score calculation, hint costs untested |
| Handler error cases (400, 401, 403, 404) | MEDIUM | Error paths untested |
| Race condition tests (concurrent submissions) | MEDIUM | Data integrity under load |
| Input validation tests (empty, oversized, special chars) | MEDIUM | Edge cases |
| Migration rollback tests | LOW | Verify down migrations work |

### Recommendation

Before launch, add at minimum:
- `internal/auth/auth_test.go` -- JWT generation, validation, expiration
- `internal/database/queries_test.go` -- scoreboard calculation, hint cost deduction
- Negative test cases in `handlers_test.go` -- unauthorized access, invalid input

---

## Appendix A: File Inventory

### Project Structure (Key Files)

```
hCTF2/
├── main.go                          # Entry point, router, middleware (44KB)
├── handlers_test.go                 # HTTP handler tests (13KB)
├── Taskfile.yml                     # Build automation (25 tasks)
├── Dockerfile                       # Multi-stage production build
├── docker-compose.yml               # Production compose
├── docker-compose.dev.yml           # Development compose
├── LICENSE                          # MIT
├── .gitignore                       # Comprehensive (46 lines)
├── .env.example                     # Environment template
├── config.example.yaml              # Configuration reference
├── internal/
│   ├── auth/
│   │   ├── middleware.go            # JWT auth + middleware
│   │   └── password.go             # bcrypt hashing
│   ├── database/
│   │   ├── db.go                   # Connection + migrations
│   │   ├── queries.go              # All SQL queries (~1100 lines)
│   │   ├── id.go                   # UUIDv7 generation
│   │   └── migrations/             # 8 migration pairs (up + down)
│   ├── handlers/
│   │   ├── auth.go                 # Login, register, password reset
│   │   ├── challenges.go           # Challenge + question CRUD
│   │   ├── hints.go                # Hint viewing + unlocking
│   │   ├── profile.go              # User profiles
│   │   ├── scoreboard.go           # Rankings
│   │   ├── settings.go             # Admin settings + user management
│   │   ├── sql.go                  # SQL Playground API
│   │   └── teams.go                # Team management
│   ├── models/
│   │   └── models.go               # All data structures
│   ├── telemetry/                   # OpenTelemetry instrumentation
│   ├── email/                       # SMTP email sending
│   ├── utils/
│   │   └── markdown.go             # Goldmark renderer + StripMarkdown
│   └── views/
│       ├── templates/              # 16 HTML templates
│       └── static/                 # CSS, JS, images, DuckDB WASM
├── scripts/                         # Test + automation scripts
└── docs/                            # Documentation + plans
```

### Template Inventory

| Template | Size | Purpose |
|----------|------|---------|
| base.html | 8.6KB | Master layout, navigation, theme |
| index.html | 2.3KB | Landing page |
| login.html | 2.6KB | Login form |
| register.html | 2.8KB | Registration form |
| forgot_password.html | 1.7KB | Password reset request |
| reset_password.html | 3.0KB | Password reset with token |
| challenges.html | 4.5KB | Challenge browser with filters |
| challenge.html | 5.7KB | Single challenge detail |
| scoreboard.html | 7.0KB | Individual + team rankings |
| teams.html | 16.7KB | Team management |
| team_profile.html | 14.3KB | Team profile view |
| profile.html | 5.3KB | User profile & stats |
| admin.html | 54.3KB | Admin dashboard (CRUD) |
| sql.html | 36.0KB | SQL Playground (DuckDB WASM) |
| docs.html | 10.3KB | API documentation (Swagger UI) |
| error.html | 0.8KB | Error pages (403/404/405) |

---

## Appendix B: Competitive Landscape Detail

### CTFd (Market Leader)

- **Stars**: ~6,500 | **Language**: Python (Flask) | **License**: Apache 2.0
- **Requires**: Python, pip, Redis, MySQL/Postgres
- **Strengths**: Plugin ecosystem, theme system, large community, CTFtime integration
- **Weaknesses**: Heavy deployment, pip dependency hell, outdated docs, plugins break across versions
- **Pain points users report**: Complex setup, Redis requirement for basic usage, plugin conflicts

### GZCTF (Rising Challenger)

- **Stars**: ~1,300 | **Language**: C# (ASP.NET Core) | **License**: AGPLv3
- **Requires**: .NET runtime, PostgreSQL, Docker/Kubernetes
- **Strengths**: Native Docker/K8s integration, per-team containers, dynamic flags, SignalR real-time
- **Weaknesses**: Heavy deps, documentation primarily in Chinese, AGPLv3 license

### rCTF

- **Stars**: ~500 | **Language**: Node.js | **License**: BSD-3-Clause
- **Requires**: Node.js, PostgreSQL, Redis
- **Strengths**: Clean modern UI, used by top CTF teams (redpwnCTF, UIUCTF)
- **Weaknesses**: Requires Postgres + Redis, limited docs, small community

### Key Insight

Every major CTF platform requires multiple services (database server, cache server, runtime). hCTF2 is the only actively-maintained platform where `./hctf2` is literally all you need. This is a genuine architectural advantage that competitors cannot replicate without a complete rewrite.

---

## Appendix C: Recommended Taglines

Based on the competitive analysis, these positions resonate with the target audience:

1. "CTF in a single binary"
2. "Zero-dependency CTF platform"
3. "The CTF platform that just works"
4. "Start a CTF competition in 60 seconds"
5. "No Redis. No Postgres. No Docker. Just run it."

---

*This review was generated by analyzing the full codebase, all documentation, all templates, the database schema, and the competitive CTF platform landscape.*
