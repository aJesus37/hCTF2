# Competition Lifecycle Management — Design

**Date**: 2026-03-04
**Issue**: #4 — Add proper competition lifecycle management
**Status**: Approved

## Overview

Add a formal competition system to hCTF2 that allows the platform to operate in two modes simultaneously:

1. **Open mode** — challenges published globally, accessible anytime, global scoreboard tracks all-time stats.
2. **Competition mode** — time-bounded events with selected challenges, registered teams, per-competition scoring, and lifecycle automation.

## Guiding Principles

- Challenges and teams remain **global** — no existing tables are structurally broken.
- The global scoreboard is **unchanged** — all historical data remains intact.
- Competitions are **overlays**: they reference global challenges and teams without owning them.
- YAGNI: no participant-level registration (team = registration unit), no challenge duplication.

## Data Model

### New table: `competitions`

```sql
CREATE TABLE competitions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    rules_html TEXT DEFAULT '',
    start_at DATETIME,
    end_at DATETIME,
    registration_start DATETIME,
    registration_end DATETIME,
    scoreboard_frozen INTEGER NOT NULL DEFAULT 0,
    scoreboard_blackout INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
-- status values: draft | registration | running | ended
```

### New table: `competition_challenges`

```sql
CREATE TABLE competition_challenges (
    competition_id INTEGER NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    challenge_id   TEXT    NOT NULL REFERENCES challenges(id)   ON DELETE CASCADE,
    PRIMARY KEY (competition_id, challenge_id)
);
```

### New table: `competition_teams`

```sql
CREATE TABLE competition_teams (
    competition_id INTEGER NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    team_id        TEXT    NOT NULL REFERENCES teams(id)        ON DELETE CASCADE,
    joined_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (competition_id, team_id)
);
```

### No changes to existing tables

`challenges`, `teams`, `score_events`, `site_settings` — all unchanged.

## Lifecycle & Auto-Transitions

A background goroutine (60s ticker, started in `main.go`) inspects all competitions:

| Condition | Action |
|-----------|--------|
| `now >= start_at AND status = 'registration'` | Set `status = 'running'` |
| `now >= end_at AND status = 'running'` | Set `status = 'ended'`, `scoreboard_frozen = 1` |
| Registration gating | Reject `competition_teams` inserts when `now > registration_end` |

Manual overrides (force-start, force-end, freeze, blackout) are always available in the admin panel.

## Competition Scoreboard

The competition scoreboard counts submissions where:
- The submitting team is in `competition_teams` for that competition
- The challenge is in `competition_challenges` for that competition
- The submission timestamp falls within `[start_at, end_at]` (or `[start_at, freeze_at]` if frozen)

### Scoreboard Blackout

During `scoreboard_blackout = 1`, non-admin requests to:
- `/scoreboard?competition=:id`
- `/api/competitions/:id/scoreboard`
- `/api/competitions/:id/scoreboard/evolution`

…return a "Scores hidden until reveal" message. Admins see the real data. Score recording continues unaffected.

Global site_settings retains `scoreboard_blackout_enabled` for non-competition use (existing issue request).

## Routes

### Public pages

| Route | Description |
|-------|-------------|
| `GET /competitions` | List of competitions (status, dates) |
| `GET /competitions/:id` | Competition detail page (rules, dates, registered team count) |
| `GET /scoreboard?competition=:id` | Competition scoreboard page (or global if no param) |

### Public API

| Route | Description |
|-------|-------------|
| `GET /api/competitions` | List competitions |
| `GET /api/competitions/:id` | Competition details |
| `POST /api/competitions/:id/register` | Register current user's team |
| `GET /api/competitions/:id/scoreboard` | Competition scoreboard JSON |
| `GET /api/competitions/:id/scoreboard/evolution` | Score evolution chart data (TODO: not yet implemented) |

### Admin API

| Route | Description |
|-------|-------------|
| `POST /api/admin/competitions` | Create competition |
| `PUT /api/admin/competitions/:id` | Update competition |
| `DELETE /api/admin/competitions/:id` | Delete competition |
| `POST /api/admin/competitions/:id/challenges` | Add challenge to competition |
| `DELETE /api/admin/competitions/:id/challenges/:cid` | Remove challenge |
| `GET /api/admin/competitions/:id/teams` | List registered teams |
| `POST /api/admin/competitions/:id/force-start` | Force transition to running |
| `POST /api/admin/competitions/:id/force-end` | Force transition to ended |
| `POST /api/admin/competitions/:id/freeze` | Toggle scoreboard freeze |
| `POST /api/admin/competitions/:id/blackout` | Toggle scoreboard blackout |

## Admin UI

New **"Competitions"** tab in the admin panel (`admin.html`) with:

- Competition list with status badges (draft / registration / running / ended)
- Create/Edit form: name, description, rules (markdown), date pickers for start/end/registration window
- Challenge picker: searchable list of all challenges, toggle inclusion
- Registered teams list (read-only)
- Lifecycle action buttons: Force Start, Force End, Freeze Scoreboard, Blackout Scoreboard

## Handler Organization

New file: `internal/handlers/competitions.go`

`CompetitionHandler` struct with `db *database.DB` dependency. Methods:

- `ListCompetitions`, `GetCompetition`
- `RegisterTeam`
- `GetScoreboard`, `GetScoreboardEvolution`
- `CreateCompetition`, `UpdateCompetition`, `DeleteCompetition` (admin)
- `AddChallenge`, `RemoveChallenge` (admin)
- `ListTeams`, `ForceStart`, `ForceEnd`, `SetFreeze`, `SetBlackout` (admin)

Background ticker: `StartLifecycleWatcher(db *database.DB)` function in `internal/database/` or `main.go`.

## Models

New `Competition` struct in `internal/models/models.go`:

```go
type Competition struct {
    ID                  int64
    Name                string
    Description         string
    RulesHTML           string
    StartAt             *time.Time
    EndAt               *time.Time
    RegistrationStart   *time.Time
    RegistrationEnd     *time.Time
    ScoreboardFrozen    bool
    ScoreboardBlackout  bool
    Status              string   // draft|registration|running|ended
    CreatedAt           time.Time
    UpdatedAt           time.Time
}
```

## Migration

Single migration file pair:
- `internal/database/migrations/012_competitions.up.sql`
- `internal/database/migrations/012_competitions.down.sql`

(Check current highest migration number before numbering.)

## Out of Scope

- Per-user registration (teams register, not individuals)
- Challenge visibility gating per competition (challenges remain globally visible)
- Email notifications on lifecycle transitions
- CTFtime export per competition (global export unchanged)
