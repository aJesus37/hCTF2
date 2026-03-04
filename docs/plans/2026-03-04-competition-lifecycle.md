# Competition Lifecycle Management Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a competition system to hCTF2 that supports time-bounded events with registered teams, per-competition scoreboards, automatic lifecycle transitions, and scoreboard blackout — while preserving the existing global scoreboard and challenge pool.

**Architecture:** Three new DB tables (`competitions`, `competition_challenges`, `competition_teams`) are added via migration 015. Challenges and teams remain global. Competition scoreboards filter `score_events` by competition window, participating teams, and included challenges. A 60s background goroutine auto-transitions competition states.

**Tech Stack:** Go 1.24, modernc.org/sqlite, Chi router, HTMX + Alpine.js + Tailwind CSS, Go html/template

---

## Task 1: Database Migration

**Files:**
- Create: `internal/database/migrations/015_competitions.up.sql`
- Create: `internal/database/migrations/015_competitions.down.sql`

**Step 1: Create up migration**

```sql
-- internal/database/migrations/015_competitions.up.sql
CREATE TABLE IF NOT EXISTS competitions (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    name                TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    rules_html          TEXT NOT NULL DEFAULT '',
    start_at            DATETIME,
    end_at              DATETIME,
    registration_start  DATETIME,
    registration_end    DATETIME,
    scoreboard_frozen   INTEGER NOT NULL DEFAULT 0,
    scoreboard_blackout INTEGER NOT NULL DEFAULT 0,
    status              TEXT NOT NULL DEFAULT 'draft',
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS competition_challenges (
    competition_id INTEGER NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    challenge_id   TEXT    NOT NULL REFERENCES challenges(id)   ON DELETE CASCADE,
    PRIMARY KEY (competition_id, challenge_id)
);

CREATE TABLE IF NOT EXISTS competition_teams (
    competition_id INTEGER NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    team_id        TEXT    NOT NULL REFERENCES teams(id)        ON DELETE CASCADE,
    joined_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (competition_id, team_id)
);
```

**Step 2: Create down migration**

```sql
-- internal/database/migrations/015_competitions.down.sql
DROP TABLE IF EXISTS competition_teams;
DROP TABLE IF EXISTS competition_challenges;
DROP TABLE IF EXISTS competitions;
```

**Step 3: Rebuild and verify migration runs**

```bash
task rebuild
./hctf2 --port 8093 --dev --db /tmp/hctf2_comp_test.db --admin-email admin@test.com --admin-password testpass123 &
sleep 2
# Check tables were created
sqlite3 /tmp/hctf2_comp_test.db ".tables" | grep competition
# Expected output: competition_challenges  competition_teams  competitions
kill %1
```

**Step 4: Commit**

```bash
git add internal/database/migrations/015_competitions.up.sql internal/database/migrations/015_competitions.down.sql
git commit -m "feat(db): add competitions, competition_challenges, competition_teams tables"
```

---

## Task 2: Models

**Files:**
- Modify: `internal/models/models.go` (append at end of file)

**Step 1: Add Competition model and related types**

Append to `internal/models/models.go`:

```go
// Competition represents a time-bounded CTF event
type Competition struct {
	ID                 int64      `json:"id"`
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	RulesHTML          string     `json:"rules_html"`
	StartAt            *time.Time `json:"start_at,omitempty"`
	EndAt              *time.Time `json:"end_at,omitempty"`
	RegistrationStart  *time.Time `json:"registration_start,omitempty"`
	RegistrationEnd    *time.Time `json:"registration_end,omitempty"`
	ScoreboardFrozen   bool       `json:"scoreboard_frozen"`
	ScoreboardBlackout bool       `json:"scoreboard_blackout"`
	Status             string     `json:"status"` // draft|registration|running|ended
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// CompetitionStatus constants
const (
	CompStatusDraft        = "draft"
	CompStatusRegistration = "registration"
	CompStatusRunning      = "running"
	CompStatusEnded        = "ended"
)

// CompetitionScoreboardEntry is one row in a competition scoreboard
type CompetitionScoreboardEntry struct {
	Rank      int    `json:"rank"`
	TeamID    string `json:"team_id"`
	TeamName  string `json:"team_name"`
	Score     int    `json:"score"`
	SolveCount int   `json:"solve_count"`
	LastSolve *time.Time `json:"last_solve,omitempty"`
}
```

**Step 2: Verify it compiles**

```bash
go build ./...
# Expected: no output (success)
```

**Step 3: Commit**

```bash
git add internal/models/models.go
git commit -m "feat(models): add Competition, CompetitionScoreboardEntry types"
```

---

## Task 3: Database Query Functions

**Files:**
- Modify: `internal/database/queries.go` (append new section at end)

**Step 1: Add competition CRUD and scoreboard queries**

Append to `internal/database/queries.go`:

```go
// ============================================================
// Competition queries
// ============================================================

// ListCompetitions returns all competitions ordered by start_at desc.
func (db *DB) ListCompetitions() ([]models.Competition, error) {
	rows, err := db.Query(`
		SELECT id, name, description, rules_html,
		       start_at, end_at, registration_start, registration_end,
		       scoreboard_frozen, scoreboard_blackout, status, created_at, updated_at
		FROM competitions ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var comps []models.Competition
	for rows.Next() {
		c, err := scanCompetition(rows)
		if err != nil {
			return nil, err
		}
		comps = append(comps, c)
	}
	return comps, nil
}

// GetCompetitionByID returns a single competition.
func (db *DB) GetCompetitionByID(id int64) (*models.Competition, error) {
	row := db.QueryRow(`
		SELECT id, name, description, rules_html,
		       start_at, end_at, registration_start, registration_end,
		       scoreboard_frozen, scoreboard_blackout, status, created_at, updated_at
		FROM competitions WHERE id = ?`, id)
	c, err := scanCompetition(row)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// scanCompetition scans a competition row (works with *sql.Row and *sql.Rows).
type rowScanner interface {
	Scan(dest ...any) error
}

func scanCompetition(row rowScanner) (models.Competition, error) {
	var c models.Competition
	var startAt, endAt, regStart, regEnd sql.NullString
	err := row.Scan(
		&c.ID, &c.Name, &c.Description, &c.RulesHTML,
		&startAt, &endAt, &regStart, &regEnd,
		&c.ScoreboardFrozen, &c.ScoreboardBlackout, &c.Status,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return c, err
	}
	if startAt.Valid && startAt.String != "" {
		t, _ := time.Parse(time.RFC3339, startAt.String)
		c.StartAt = &t
	}
	if endAt.Valid && endAt.String != "" {
		t, _ := time.Parse(time.RFC3339, endAt.String)
		c.EndAt = &t
	}
	if regStart.Valid && regStart.String != "" {
		t, _ := time.Parse(time.RFC3339, regStart.String)
		c.RegistrationStart = &t
	}
	if regEnd.Valid && regEnd.String != "" {
		t, _ := time.Parse(time.RFC3339, regEnd.String)
		c.RegistrationEnd = &t
	}
	return c, nil
}

// CreateCompetition inserts a new competition and returns it.
func (db *DB) CreateCompetition(name, description, rulesHTML string,
	startAt, endAt, regStart, regEnd *time.Time) (*models.Competition, error) {
	toStr := func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return t.UTC().Format(time.RFC3339)
	}
	result, err := db.Exec(`
		INSERT INTO competitions (name, description, rules_html, start_at, end_at, registration_start, registration_end)
		VALUES (?, ?, ?, NULLIF(?,''::text), NULLIF(?,''::text), NULLIF(?,''::text), NULLIF(?,''::text))`,
		name, description, rulesHTML,
		toStr(startAt), toStr(endAt), toStr(regStart), toStr(regEnd))
	if err != nil {
		// SQLite doesn't support ::text cast; use simpler approach
		result, err = db.Exec(`
			INSERT INTO competitions (name, description, rules_html, start_at, end_at, registration_start, registration_end)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			name, description, rulesHTML,
			nullIfEmpty(toStr(startAt)), nullIfEmpty(toStr(endAt)),
			nullIfEmpty(toStr(regStart)), nullIfEmpty(toStr(regEnd)))
		if err != nil {
			return nil, err
		}
	}
	id, _ := result.LastInsertId()
	return db.GetCompetitionByID(id)
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// UpdateCompetition updates all mutable fields of a competition.
func (db *DB) UpdateCompetition(id int64, name, description, rulesHTML string,
	startAt, endAt, regStart, regEnd *time.Time, status string) error {
	toStr := func(t *time.Time) interface{} {
		if t == nil {
			return nil
		}
		return t.UTC().Format(time.RFC3339)
	}
	_, err := db.Exec(`
		UPDATE competitions
		SET name=?, description=?, rules_html=?,
		    start_at=?, end_at=?, registration_start=?, registration_end=?,
		    status=?, updated_at=datetime('now')
		WHERE id=?`,
		name, description, rulesHTML,
		toStr(startAt), toStr(endAt), toStr(regStart), toStr(regEnd),
		status, id)
	return err
}

// DeleteCompetition removes a competition and its join records.
func (db *DB) DeleteCompetition(id int64) error {
	_, err := db.Exec(`DELETE FROM competitions WHERE id=?`, id)
	return err
}

// SetCompetitionStatus updates just the status field.
func (db *DB) SetCompetitionStatus(id int64, status string) error {
	_, err := db.Exec(`UPDATE competitions SET status=?, updated_at=datetime('now') WHERE id=?`, status, id)
	return err
}

// SetCompetitionFreeze sets the scoreboard_frozen flag.
func (db *DB) SetCompetitionFreeze(id int64, frozen bool) error {
	v := 0
	if frozen {
		v = 1
	}
	_, err := db.Exec(`UPDATE competitions SET scoreboard_frozen=?, updated_at=datetime('now') WHERE id=?`, v, id)
	return err
}

// SetCompetitionBlackout sets the scoreboard_blackout flag.
func (db *DB) SetCompetitionBlackout(id int64, blackout bool) error {
	v := 0
	if blackout {
		v = 1
	}
	_, err := db.Exec(`UPDATE competitions SET scoreboard_blackout=?, updated_at=datetime('now') WHERE id=?`, v, id)
	return err
}

// AddChallengeToCompetition links a challenge to a competition.
func (db *DB) AddChallengeToCompetition(compID int64, challengeID string) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO competition_challenges (competition_id, challenge_id) VALUES (?,?)`,
		compID, challengeID)
	return err
}

// RemoveChallengeFromCompetition unlinks a challenge from a competition.
func (db *DB) RemoveChallengeFromCompetition(compID int64, challengeID string) error {
	_, err := db.Exec(`DELETE FROM competition_challenges WHERE competition_id=? AND challenge_id=?`, compID, challengeID)
	return err
}

// GetCompetitionChallenges returns all challenges linked to a competition.
func (db *DB) GetCompetitionChallenges(compID int64) ([]models.Challenge, error) {
	rows, err := db.Query(`
		SELECT c.id, c.name, c.description, c.category, c.difficulty,
		       c.tags, c.visible, c.sql_enabled, c.sql_dataset_url, c.sql_schema_hint,
		       c.dynamic_scoring, c.initial_points, c.minimum_points, c.decay_threshold,
		       c.file_url, c.created_at, c.updated_at
		FROM challenges c
		JOIN competition_challenges cc ON cc.challenge_id = c.id
		WHERE cc.competition_id = ?
		ORDER BY c.category, c.name`, compID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var challenges []models.Challenge
	for rows.Next() {
		var ch models.Challenge
		err := rows.Scan(
			&ch.ID, &ch.Name, &ch.Description, &ch.Category, &ch.Difficulty,
			&ch.Tags, &ch.Visible, &ch.SQLEnabled, &ch.SQLDatasetURL, &ch.SQLSchemaHint,
			&ch.DynamicScoring, &ch.InitialPoints, &ch.MinimumPoints, &ch.DecayThreshold,
			&ch.FileURL, &ch.CreatedAt, &ch.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		challenges = append(challenges, ch)
	}
	return challenges, nil
}

// RegisterTeamForCompetition adds a team to a competition.
// Returns error if registration window is closed.
func (db *DB) RegisterTeamForCompetition(compID int64, teamID string) error {
	comp, err := db.GetCompetitionByID(compID)
	if err != nil {
		return err
	}
	now := time.Now()
	if comp.RegistrationEnd != nil && now.After(*comp.RegistrationEnd) {
		return fmt.Errorf("registration is closed")
	}
	if comp.Status == models.CompStatusEnded {
		return fmt.Errorf("competition has ended")
	}
	_, err = db.Exec(`INSERT OR IGNORE INTO competition_teams (competition_id, team_id) VALUES (?,?)`,
		compID, teamID)
	return err
}

// GetCompetitionTeams returns teams registered for a competition.
func (db *DB) GetCompetitionTeams(compID int64) ([]models.Team, error) {
	rows, err := db.Query(`
		SELECT t.id, t.name, t.description, t.owner_id, t.invite_id, t.invite_permission, t.created_at, t.updated_at
		FROM teams t
		JOIN competition_teams ct ON ct.team_id = t.id
		WHERE ct.competition_id = ?
		ORDER BY t.name`, compID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var teams []models.Team
	for rows.Next() {
		var t models.Team
		err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.OwnerID, &t.InviteID, &t.InvitePermission, &t.CreatedAt, &t.UpdatedAt)
		if err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, nil
}

// IsTeamRegistered returns true if the team is registered for the competition.
func (db *DB) IsTeamRegistered(compID int64, teamID string) bool {
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM competition_teams WHERE competition_id=? AND team_id=?`, compID, teamID).Scan(&count)
	return count > 0
}

// GetCompetitionScoreboard returns ranked teams for a competition.
// Only counts first-solve submissions within the competition window, for registered teams, on competition challenges.
func (db *DB) GetCompetitionScoreboard(compID int64) ([]models.CompetitionScoreboardEntry, error) {
	comp, err := db.GetCompetitionByID(compID)
	if err != nil {
		return nil, err
	}

	// Build time window filter
	startFilter := "1=1"
	var args []interface{}
	args = append(args, compID, compID)
	if comp.StartAt != nil {
		startFilter = "s.created_at >= ?"
		args = append(args, comp.StartAt.UTC().Format(time.RFC3339))
	}
	endCutoff := "1=1"
	if comp.ScoreboardFrozen && comp.EndAt != nil {
		endCutoff = "s.created_at <= ?"
		args = append(args, comp.EndAt.UTC().Format(time.RFC3339))
	} else if comp.EndAt != nil {
		endCutoff = "s.created_at <= ?"
		args = append(args, comp.EndAt.UTC().Format(time.RFC3339))
	}

	query := fmt.Sprintf(`
		WITH first_solves AS (
			SELECT s.team_id, s.question_id, MIN(s.created_at) as solved_at,
			       q.points
			FROM submissions s
			JOIN questions q ON q.id = s.question_id
			JOIN challenges c ON c.id = q.challenge_id
			JOIN competition_challenges cc ON cc.challenge_id = c.id AND cc.competition_id = ?
			JOIN competition_teams ct ON ct.team_id = s.team_id AND ct.competition_id = ?
			WHERE s.correct = 1 AND %s AND %s
			GROUP BY s.team_id, s.question_id
		)
		SELECT t.id, t.name,
		       COALESCE(SUM(fs.points), 0) as score,
		       COUNT(DISTINCT fs.question_id) as solve_count,
		       MAX(fs.solved_at) as last_solve
		FROM competition_teams ct
		JOIN teams t ON t.id = ct.team_id
		LEFT JOIN first_solves fs ON fs.team_id = t.id
		WHERE ct.competition_id = ?
		GROUP BY t.id, t.name
		ORDER BY score DESC, last_solve ASC`, startFilter, endCutoff)

	// Append competition_id for WHERE clause
	args = append(args, compID)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.CompetitionScoreboardEntry
	rank := 1
	for rows.Next() {
		var e models.CompetitionScoreboardEntry
		var lastSolveStr sql.NullString
		if err := rows.Scan(&e.TeamID, &e.TeamName, &e.Score, &e.SolveCount, &lastSolveStr); err != nil {
			return nil, err
		}
		if lastSolveStr.Valid && lastSolveStr.String != "" {
			t, _ := time.Parse(time.RFC3339, lastSolveStr.String)
			e.LastSolve = &t
		}
		e.Rank = rank
		rank++
		entries = append(entries, e)
	}
	return entries, nil
}

// TickCompetitionLifecycle auto-transitions competitions based on current time.
// Call this from a background goroutine every 60 seconds.
func (db *DB) TickCompetitionLifecycle() {
	now := time.Now().UTC().Format(time.RFC3339)

	// registration → running: start_at has passed
	db.Exec(`
		UPDATE competitions
		SET status='running', updated_at=datetime('now')
		WHERE status='registration' AND start_at IS NOT NULL AND start_at <= ?`, now)

	// draft → running (if no registration period configured)
	db.Exec(`
		UPDATE competitions
		SET status='running', updated_at=datetime('now')
		WHERE status='draft' AND start_at IS NOT NULL AND start_at <= ?
		  AND registration_start IS NULL`, now)

	// running → ended: end_at has passed; also freeze scoreboard
	db.Exec(`
		UPDATE competitions
		SET status='ended', scoreboard_frozen=1, updated_at=datetime('now')
		WHERE status='running' AND end_at IS NOT NULL AND end_at <= ?`, now)
}
```

**Step 2: Verify it compiles**

```bash
go build ./...
# Expected: no output
```

**Step 3: Commit**

```bash
git add internal/database/queries.go
git commit -m "feat(db): add competition query functions and lifecycle ticker"
```

---

## Task 4: Competition Handler

**Files:**
- Create: `internal/handlers/competitions.go`

**Step 1: Create the handler file**

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/yourusername/hctf2/internal/auth"
	"github.com/yourusername/hctf2/internal/database"
	"github.com/yourusername/hctf2/internal/models"
)

type CompetitionHandler struct {
	db *database.DB
}

func NewCompetitionHandler(db *database.DB) *CompetitionHandler {
	return &CompetitionHandler{db: db}
}

// parseCompID extracts and parses the competition ID from URL.
func parseCompID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// ListCompetitions godoc
// @Summary List all competitions
// @Tags Competitions
// @Produce json
// @Success 200 {array} models.Competition
// @Router /api/competitions [get]
func (h *CompetitionHandler) ListCompetitions(w http.ResponseWriter, r *http.Request) {
	comps, err := h.db.ListCompetitions()
	if err != nil {
		http.Error(w, "Failed to list competitions", http.StatusInternalServerError)
		return
	}
	if comps == nil {
		comps = []models.Competition{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comps)
}

// GetCompetition godoc
// @Summary Get competition by ID
// @Tags Competitions
// @Param id path int true "Competition ID"
// @Success 200 {object} models.Competition
// @Failure 404 {object} object{error=string}
// @Router /api/competitions/{id} [get]
func (h *CompetitionHandler) GetCompetition(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	comp, err := h.db.GetCompetitionByID(id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comp)
}

// RegisterTeam godoc
// @Summary Register current user's team for a competition
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{error=string}
// @Router /api/competitions/{id}/register [post]
func (h *CompetitionHandler) RegisterTeam(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil || user.TeamID == nil {
		http.Error(w, "You must be in a team to register", http.StatusBadRequest)
		return
	}
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := h.db.RegisterTeamForCompetition(id, *user.TeamID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "registered"})
}

// GetScoreboard godoc
// @Summary Get competition scoreboard
// @Tags Competitions
// @Param id path int true "Competition ID"
// @Success 200 {array} models.CompetitionScoreboardEntry
// @Router /api/competitions/{id}/scoreboard [get]
func (h *CompetitionHandler) GetScoreboard(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	comp, err := h.db.GetCompetitionByID(id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Check blackout for non-admins
	claims := auth.GetUserFromContext(r.Context())
	isAdmin := claims != nil && claims.IsAdmin
	if comp.ScoreboardBlackout && !isAdmin {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Scores hidden until reveal"})
		return
	}

	entries, err := h.db.GetCompetitionScoreboard(id)
	if err != nil {
		http.Error(w, "Failed to fetch scoreboard", http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []models.CompetitionScoreboardEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// ---- Admin handlers ----

// CreateCompetition godoc
// @Summary Create a competition (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Success 200 {object} models.Competition
// @Router /api/admin/competitions [post]
func (h *CompetitionHandler) CreateCompetition(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}
	description := r.FormValue("description")
	rulesHTML := r.FormValue("rules_html")
	startAt := parseOptionalTime(r.FormValue("start_at"))
	endAt := parseOptionalTime(r.FormValue("end_at"))
	regStart := parseOptionalTime(r.FormValue("registration_start"))
	regEnd := parseOptionalTime(r.FormValue("registration_end"))

	comp, err := h.db.CreateCompetition(name, description, rulesHTML, startAt, endAt, regStart, regEnd)
	if err != nil {
		http.Error(w, "Failed to create competition", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comp)
}

// UpdateCompetition godoc
// @Summary Update a competition (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Router /api/admin/competitions/{id} [put]
func (h *CompetitionHandler) UpdateCompetition(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	description := r.FormValue("description")
	rulesHTML := r.FormValue("rules_html")
	status := r.FormValue("status")
	if status == "" {
		status = models.CompStatusDraft
	}
	startAt := parseOptionalTime(r.FormValue("start_at"))
	endAt := parseOptionalTime(r.FormValue("end_at"))
	regStart := parseOptionalTime(r.FormValue("registration_start"))
	regEnd := parseOptionalTime(r.FormValue("registration_end"))

	if err := h.db.UpdateCompetition(id, name, description, rulesHTML, startAt, endAt, regStart, regEnd, status); err != nil {
		http.Error(w, "Failed to update", http.StatusInternalServerError)
		return
	}
	comp, _ := h.db.GetCompetitionByID(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comp)
}

// DeleteCompetition godoc
// @Summary Delete a competition (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Router /api/admin/competitions/{id} [delete]
func (h *CompetitionHandler) DeleteCompetition(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := h.db.DeleteCompetition(id); err != nil {
		http.Error(w, "Failed to delete", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AddChallenge godoc
// @Summary Add challenge to competition (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Router /api/admin/competitions/{id}/challenges [post]
func (h *CompetitionHandler) AddChallenge(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	r.ParseForm()
	challengeID := r.FormValue("challenge_id")
	if challengeID == "" {
		http.Error(w, "challenge_id required", http.StatusBadRequest)
		return
	}
	if err := h.db.AddChallengeToCompetition(id, challengeID); err != nil {
		http.Error(w, "Failed to add challenge", http.StatusInternalServerError)
		return
	}
	// Return updated challenge list as JSON
	challenges, _ := h.db.GetCompetitionChallenges(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(challenges)
}

// RemoveChallenge godoc
// @Summary Remove challenge from competition (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Param cid path string true "Challenge ID"
// @Router /api/admin/competitions/{id}/challenges/{cid} [delete]
func (h *CompetitionHandler) RemoveChallenge(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	cid := chi.URLParam(r, "cid")
	if err := h.db.RemoveChallengeFromCompetition(id, cid); err != nil {
		http.Error(w, "Failed to remove", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListTeams godoc
// @Summary List teams registered for competition (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Router /api/admin/competitions/{id}/teams [get]
func (h *CompetitionHandler) ListTeams(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	teams, err := h.db.GetCompetitionTeams(id)
	if err != nil {
		http.Error(w, "Failed to fetch teams", http.StatusInternalServerError)
		return
	}
	if teams == nil {
		teams = []models.Team{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(teams)
}

// ForceStart godoc
// @Summary Force-start a competition (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Router /api/admin/competitions/{id}/force-start [post]
func (h *CompetitionHandler) ForceStart(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := h.db.SetCompetitionStatus(id, models.CompStatusRunning); err != nil {
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": models.CompStatusRunning})
}

// ForceEnd godoc
// @Summary Force-end a competition (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Router /api/admin/competitions/{id}/force-end [post]
func (h *CompetitionHandler) ForceEnd(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := h.db.SetCompetitionStatus(id, models.CompStatusEnded); err != nil {
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}
	h.db.SetCompetitionFreeze(id, true) // auto-freeze on end
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": models.CompStatusEnded})
}

// SetFreeze godoc
// @Summary Toggle competition scoreboard freeze (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Router /api/admin/competitions/{id}/freeze [post]
func (h *CompetitionHandler) SetFreeze(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	r.ParseForm()
	frozen := r.FormValue("frozen") == "1"
	if err := h.db.SetCompetitionFreeze(id, frozen); err != nil {
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"frozen": frozen})
}

// SetBlackout godoc
// @Summary Toggle competition scoreboard blackout (admin only)
// @Tags Competitions
// @Security CookieAuth
// @Param id path int true "Competition ID"
// @Router /api/admin/competitions/{id}/blackout [post]
func (h *CompetitionHandler) SetBlackout(w http.ResponseWriter, r *http.Request) {
	id, err := parseCompID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	r.ParseForm()
	blackout := r.FormValue("blackout") == "1"
	if err := h.db.SetCompetitionBlackout(id, blackout); err != nil {
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"blackout": blackout})
}

// parseOptionalTime parses "2006-01-02T15:04" (datetime-local input format).
func parseOptionalTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return &t
		}
	}
	return nil
}
```

Note: add `"time"` to the import block.

**Step 2: Verify it compiles**

```bash
go build ./...
# Expected: no output
```

**Step 3: Commit**

```bash
git add internal/handlers/competitions.go
git commit -m "feat(handlers): add CompetitionHandler with full CRUD and lifecycle controls"
```

---

## Task 5: Wire Handler + Background Goroutine into main.go

**Files:**
- Modify: `main.go`

**Step 1: Add competitionH field to Server struct**

Find the `Server` struct (around line 85). Add after the last handler field:

```go
competitionH     *handlers.CompetitionHandler
```

**Step 2: Initialize handler in newServer() (around line 300)**

After `scoreboardH` initialization:

```go
competitionH:     handlers.NewCompetitionHandler(db),
```

**Step 3: Start background lifecycle ticker in main() or newServer()**

In `main()`, after the server is created but before `ListenAndServe`, add:

```go
// Start competition lifecycle watcher
go func() {
    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()
    for range ticker.C {
        db.TickCompetitionLifecycle()
    }
}()
```

Make sure `db` is in scope (it is — it's created before `newServer()`).

**Step 4: Register routes**

In `setupRoutes()`, after the scoreboard routes, add:

```go
// Competitions (public)
r.Get("/api/competitions", s.competitionH.ListCompetitions)
r.Get("/api/competitions/{id}", s.competitionH.GetCompetition)
r.Get("/api/competitions/{id}/scoreboard", s.competitionH.GetScoreboard)

r.With(s.requireAuth).Post("/api/competitions/{id}/register", s.competitionH.RegisterTeam)

// Competitions (admin)
r.Group(func(r chi.Router) {
    r.Use(s.requireAdmin)
    r.Post("/api/admin/competitions", s.competitionH.CreateCompetition)
    r.Put("/api/admin/competitions/{id}", s.competitionH.UpdateCompetition)
    r.Delete("/api/admin/competitions/{id}", s.competitionH.DeleteCompetition)
    r.Post("/api/admin/competitions/{id}/challenges", s.competitionH.AddChallenge)
    r.Delete("/api/admin/competitions/{id}/challenges/{cid}", s.competitionH.RemoveChallenge)
    r.Get("/api/admin/competitions/{id}/teams", s.competitionH.ListTeams)
    r.Post("/api/admin/competitions/{id}/force-start", s.competitionH.ForceStart)
    r.Post("/api/admin/competitions/{id}/force-end", s.competitionH.ForceEnd)
    r.Post("/api/admin/competitions/{id}/freeze", s.competitionH.SetFreeze)
    r.Post("/api/admin/competitions/{id}/blackout", s.competitionH.SetBlackout)
})

// Competition pages
r.Get("/competitions", s.handleCompetitionList)
r.Get("/competitions/{id}", s.handleCompetitionDetail)
```

Also add `requireAuth` middleware reference — check how it's used in existing auth-guarded routes (it's `s.requireAuth` or a middleware group).

**Step 5: Add page handlers in main.go**

```go
func (s *Server) handleCompetitionList(w http.ResponseWriter, r *http.Request) {
	comps, err := s.db.ListCompetitions()
	if err != nil {
		comps = []models.Competition{}
	}
	data := map[string]interface{}{
		"Title":        "Competitions",
		"Page":         "competitions",
		"User":         auth.GetUserFromContext(r.Context()),
		"Competitions": comps,
	}
	s.render(w, "base.html", data)
}

func (s *Server) handleCompetitionDetail(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	comp, err := s.db.GetCompetitionByID(id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	claims := auth.GetUserFromContext(r.Context())
	var teamRegistered bool
	if claims != nil {
		user, err := s.db.GetUserByID(claims.UserID)
		if err == nil && user.TeamID != nil {
			teamRegistered = s.db.IsTeamRegistered(id, *user.TeamID)
		}
	}
	challenges, _ := s.db.GetCompetitionChallenges(id)
	entries, _ := s.db.GetCompetitionScoreboard(id)
	isAdmin := claims != nil && claims.IsAdmin
	if comp.ScoreboardBlackout && !isAdmin {
		entries = nil
	}
	data := map[string]interface{}{
		"Title":          comp.Name,
		"Page":           "competitions",
		"User":           claims,
		"Competition":    comp,
		"Challenges":     challenges,
		"Entries":        entries,
		"TeamRegistered": teamRegistered,
		"BlackedOut":     comp.ScoreboardBlackout && !isAdmin,
	}
	s.render(w, "base.html", data)
}
```

**Step 6: Verify it compiles**

```bash
go build ./...
```

**Step 7: Commit**

```bash
git add main.go
git commit -m "feat(main): wire competition handler, background lifecycle ticker, and routes"
```

---

## Task 6: Add competitions.html template

**Files:**
- Create: `internal/views/templates/competitions.html`

**Step 1: Create competitions list template**

```html
{{define "content"}}
<div class="max-w-5xl mx-auto px-4 py-8">
    <h1 class="text-3xl font-bold text-gray-900 dark:text-white mb-6">Competitions</h1>

    {{if not .Competitions}}
    <div class="text-center py-16 text-gray-500 dark:text-gray-400">
        <p class="text-lg">No competitions yet.</p>
    </div>
    {{else}}
    <div class="grid gap-4">
        {{range .Competitions}}
        <a href="/competitions/{{.ID}}"
           class="block bg-white dark:bg-dark-card border border-gray-200 dark:border-dark-border rounded-xl p-6 hover:border-purple-400 transition-colors">
            <div class="flex items-start justify-between">
                <div>
                    <h2 class="text-xl font-semibold text-gray-900 dark:text-white">{{.Name}}</h2>
                    {{if .Description}}
                    <p class="text-gray-500 dark:text-gray-400 mt-1 text-sm">{{.Description}}</p>
                    {{end}}
                    <div class="flex gap-4 mt-3 text-sm text-gray-500 dark:text-gray-400">
                        {{if .StartAt}}<span>Start: {{.StartAt.Format "Jan 2, 2006 15:04 UTC"}}</span>{{end}}
                        {{if .EndAt}}<span>End: {{.EndAt.Format "Jan 2, 2006 15:04 UTC"}}</span>{{end}}
                    </div>
                </div>
                <span class="text-xs font-semibold px-3 py-1 rounded-full
                    {{if eq .Status "running"}}bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300
                    {{else if eq .Status "ended"}}bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400
                    {{else if eq .Status "registration"}}bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300
                    {{else}}bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300{{end}}">
                    {{.Status}}
                </span>
            </div>
        </a>
        {{end}}
    </div>
    {{end}}
</div>
{{end}}
```

**Step 2: Create competition detail template**

Create: `internal/views/templates/competition.html`

```html
{{define "content"}}
{{$comp := .Competition}}
<div class="max-w-5xl mx-auto px-4 py-8 space-y-8">
    <!-- Header -->
    <div class="flex items-start justify-between">
        <div>
            <a href="/competitions" class="text-sm text-purple-500 hover:underline">&larr; All Competitions</a>
            <h1 class="text-3xl font-bold text-gray-900 dark:text-white mt-2">{{$comp.Name}}</h1>
            <div class="flex gap-4 mt-2 text-sm text-gray-500 dark:text-gray-400">
                {{if $comp.StartAt}}<span>Start: {{$comp.StartAt.Format "Jan 2, 2006 15:04 UTC"}}</span>{{end}}
                {{if $comp.EndAt}}<span>End: {{$comp.EndAt.Format "Jan 2, 2006 15:04 UTC"}}</span>{{end}}
            </div>
        </div>
        <div class="flex flex-col items-end gap-2">
            <span class="text-xs font-semibold px-3 py-1 rounded-full
                {{if eq $comp.Status "running"}}bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300
                {{else if eq $comp.Status "ended"}}bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400
                {{else if eq $comp.Status "registration"}}bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300
                {{else}}bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300{{end}}">
                {{$comp.Status}}
            </span>
            {{if and .User (not .TeamRegistered) (or (eq $comp.Status "registration") (eq $comp.Status "running"))}}
            <form hx-post="/api/competitions/{{$comp.ID}}/register" hx-swap="outerHTML">
                <button class="text-sm bg-purple-600 hover:bg-purple-700 text-white px-4 py-2 rounded-lg transition-colors">
                    Register My Team
                </button>
            </form>
            {{else if .TeamRegistered}}
            <span class="text-sm text-green-500 font-medium">✓ Registered</span>
            {{end}}
        </div>
    </div>

    <!-- Rules -->
    {{if $comp.RulesHTML}}
    <div class="bg-white dark:bg-dark-card border border-gray-200 dark:border-dark-border rounded-xl p-6">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-3">Rules</h2>
        <div class="prose prose-sm dark:prose-invert max-w-none">{{$comp.RulesHTML}}</div>
    </div>
    {{end}}

    <!-- Challenges -->
    {{if .Challenges}}
    <div class="bg-white dark:bg-dark-card border border-gray-200 dark:border-dark-border rounded-xl p-6">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-3">Challenges ({{len .Challenges}})</h2>
        <div class="grid grid-cols-2 md:grid-cols-3 gap-2">
            {{range .Challenges}}
            <a href="/challenges/{{.ID}}" class="text-sm text-purple-500 hover:underline truncate">{{.Name}}</a>
            {{end}}
        </div>
    </div>
    {{end}}

    <!-- Scoreboard -->
    <div class="bg-white dark:bg-dark-card border border-gray-200 dark:border-dark-border rounded-xl p-6">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">Scoreboard</h2>
        {{if .BlackedOut}}
        <p class="text-center py-8 text-gray-500 dark:text-gray-400 text-lg">Scores hidden until reveal</p>
        {{else if not .Entries}}
        <p class="text-center py-8 text-gray-500 dark:text-gray-400">No scores yet.</p>
        {{else}}
        <table class="w-full text-sm">
            <thead class="border-b border-gray-200 dark:border-dark-border">
                <tr class="text-left text-xs text-gray-500 dark:text-gray-400 uppercase">
                    <th class="py-2 pr-4">#</th>
                    <th class="py-2 pr-4">Team</th>
                    <th class="py-2 pr-4">Points</th>
                    <th class="py-2">Solves</th>
                </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-border">
                {{range .Entries}}
                <tr class="text-gray-700 dark:text-gray-300">
                    <td class="py-2 pr-4 font-bold text-gray-900 dark:text-white">{{.Rank}}</td>
                    <td class="py-2 pr-4">
                        <a href="/teams/{{.TeamID}}/profile" class="text-purple-500 hover:underline">{{.TeamName}}</a>
                    </td>
                    <td class="py-2 pr-4 font-semibold">{{.Score}}</td>
                    <td class="py-2">{{.SolveCount}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
        {{end}}
    </div>
</div>
{{end}}
```

**Step 3: Rebuild and verify templates load**

```bash
task rebuild
./hctf2 --port 8093 --dev --db /tmp/hctf2_comp_test.db --admin-email admin@test.com --admin-password testpass123 &
sleep 2
npx agent-browser --session hctf2comp open http://localhost:8093/competitions
npx agent-browser --session hctf2comp screenshot --full /tmp/competitions_list.png
# Read /tmp/competitions_list.png with the Read tool
kill %1
```

**Step 4: Commit**

```bash
git add internal/views/templates/competitions.html internal/views/templates/competition.html
git commit -m "feat(templates): add competition list and detail pages"
```

---

## Task 7: Admin UI — Competitions Tab

**Files:**
- Modify: `internal/views/templates/admin.html`
- Modify: `main.go` — `handleAdminDashboard` (add `Competitions` to data map)

**Step 1: Add Competitions to admin dashboard data**

In `handleAdminDashboard` in `main.go`, add before `s.render`:

```go
competitions, _ := s.db.ListCompetitions()
allChallenges, _ := s.db.GetChallenges(false) // for challenge picker
```

And in the data map:

```go
"Competitions": competitions,
"AllChallenges": allChallenges,
```

(`AllChallenges` already exists as `"Challenges"` — reuse that.)

**Step 2: Add "Competitions" tab button in admin.html**

Find the tab buttons section (around the line with `@click="tab = 'settings'"`). Add a new tab button:

```html
<button
    @click="tab = 'competitions'"
    :class="tab === 'competitions' ? 'border-b-2 border-purple-500 text-purple-500 dark:text-purple-400' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'"
    class="pb-3 px-1 text-sm font-medium transition-colors whitespace-nowrap"
>Competitions</button>
```

**Step 3: Add competition tab content panel**

In `admin.html`, after the settings panel (`x-show="tab === 'settings'"`), add:

```html
<!-- Competitions Tab -->
<div x-show="tab === 'competitions'" class="space-y-6">
    <div class="flex items-center justify-between">
        <h2 class="text-xl font-bold text-gray-900 dark:text-white">Competitions</h2>
        <button
            @click="$dispatch('open-create-comp')"
            class="bg-purple-600 hover:bg-purple-700 text-white text-sm px-4 py-2 rounded-lg transition-colors">
            + New Competition
        </button>
    </div>

    <!-- Competition list -->
    {{if not .Competitions}}
    <p class="text-gray-500 dark:text-gray-400 text-sm">No competitions yet.</p>
    {{else}}
    <div class="space-y-3" id="comp-list">
        {{range .Competitions}}
        <div class="bg-white dark:bg-dark-card border border-gray-200 dark:border-dark-border rounded-xl p-4"
             x-data="{ open: false }">
            <div class="flex items-center justify-between">
                <div>
                    <div class="flex items-center gap-3">
                        <h3 class="font-semibold text-gray-900 dark:text-white">{{.Name}}</h3>
                        <span class="text-xs px-2 py-0.5 rounded-full font-medium
                            {{if eq .Status "running"}}bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300
                            {{else if eq .Status "ended"}}bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400
                            {{else if eq .Status "registration"}}bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300
                            {{else}}bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300{{end}}">
                            {{.Status}}
                        </span>
                        {{if .ScoreboardFrozen}}<span class="text-xs px-2 py-0.5 rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">Frozen</span>{{end}}
                        {{if .ScoreboardBlackout}}<span class="text-xs px-2 py-0.5 rounded-full bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300">Blackout</span>{{end}}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400 mt-1 flex gap-3">
                        {{if .StartAt}}<span>Start: {{.StartAt.Format "Jan 2 15:04 UTC"}}</span>{{end}}
                        {{if .EndAt}}<span>End: {{.EndAt.Format "Jan 2 15:04 UTC"}}</span>{{end}}
                    </div>
                </div>
                <div class="flex items-center gap-2">
                    <!-- Force Start/End -->
                    {{if or (eq .Status "draft") (eq .Status "registration")}}
                    <button
                        hx-post="/api/admin/competitions/{{.ID}}/force-start"
                        hx-confirm="Force start this competition?"
                        hx-on::after-request="window.location.reload()"
                        class="text-xs bg-green-600 hover:bg-green-700 text-white px-3 py-1.5 rounded-lg">
                        Force Start
                    </button>
                    {{end}}
                    {{if eq .Status "running"}}
                    <button
                        hx-post="/api/admin/competitions/{{.ID}}/force-end"
                        hx-confirm="Force end this competition? This will freeze the scoreboard."
                        hx-on::after-request="window.location.reload()"
                        class="text-xs bg-red-600 hover:bg-red-700 text-white px-3 py-1.5 rounded-lg">
                        Force End
                    </button>
                    <!-- Freeze toggle -->
                    <form hx-post="/api/admin/competitions/{{.ID}}/freeze" hx-on::after-request="window.location.reload()">
                        <input type="hidden" name="frozen" value="{{if .ScoreboardFrozen}}0{{else}}1{{end}}">
                        <button type="submit" class="text-xs bg-blue-600 hover:bg-blue-700 text-white px-3 py-1.5 rounded-lg">
                            {{if .ScoreboardFrozen}}Unfreeze{{else}}Freeze{{end}}
                        </button>
                    </form>
                    <!-- Blackout toggle -->
                    <form hx-post="/api/admin/competitions/{{.ID}}/blackout" hx-on::after-request="window.location.reload()">
                        <input type="hidden" name="blackout" value="{{if .ScoreboardBlackout}}0{{else}}1{{end}}">
                        <button type="submit" class="text-xs bg-orange-600 hover:bg-orange-700 text-white px-3 py-1.5 rounded-lg">
                            {{if .ScoreboardBlackout}}End Blackout{{else}}Blackout{{end}}
                        </button>
                    </form>
                    {{end}}
                    <!-- Expand for challenges/teams -->
                    <button @click="open = !open" class="text-xs text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 px-2 py-1.5">
                        <span x-text="open ? '▲ Less' : '▼ More'"></span>
                    </button>
                    <!-- Delete -->
                    <button
                        hx-delete="/api/admin/competitions/{{.ID}}"
                        hx-confirm="Delete competition '{{.Name}}'? This cannot be undone."
                        hx-on::after-request="window.location.reload()"
                        class="text-xs text-red-500 hover:text-red-700 px-2 py-1.5">
                        Delete
                    </button>
                </div>
            </div>

            <!-- Expandable: challenge picker + registered teams -->
            <div x-show="open" x-cloak class="mt-4 border-t border-gray-100 dark:border-dark-border pt-4 grid grid-cols-2 gap-4">
                <!-- Challenge picker -->
                <div>
                    <h4 class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase mb-2">Add Challenge</h4>
                    <form hx-post="/api/admin/competitions/{{.ID}}/challenges"
                          hx-on::after-request="window.location.reload()"
                          class="flex gap-2">
                        <select name="challenge_id" class="flex-1 text-sm bg-gray-50 dark:bg-dark-bg border border-gray-200 dark:border-dark-border rounded-lg px-2 py-1">
                            <option value="">Select challenge...</option>
                            {{range $.Challenges}}
                            <option value="{{.ID}}">{{.Name}}</option>
                            {{end}}
                        </select>
                        <button type="submit" class="text-xs bg-purple-600 text-white px-3 py-1 rounded-lg">Add</button>
                    </form>
                    <p class="text-xs text-gray-400 mt-2">
                        <a href="/competitions/{{.ID}}" class="text-purple-400 hover:underline">View competition page</a> to see current challenges.
                    </p>
                </div>
                <!-- Registered teams (loaded on expand) -->
                <div>
                    <h4 class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase mb-2">Registered Teams</h4>
                    <div hx-get="/api/admin/competitions/{{.ID}}/teams"
                         hx-trigger="revealed"
                         hx-swap="innerHTML"
                         class="text-sm text-gray-500 dark:text-gray-400">Loading...</div>
                </div>
            </div>
        </div>
        {{end}}
    </div>
    {{end}}

    <!-- Create Competition Form (modal-style, Alpine toggle) -->
    <div x-data="{ show: false }"
         @open-create-comp.window="show = true"
         x-show="show" x-cloak
         class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
        <div class="bg-white dark:bg-dark-card rounded-xl p-6 w-full max-w-lg shadow-xl" @click.outside="show = false">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">Create Competition</h3>
            <form hx-post="/api/admin/competitions"
                  hx-on::after-request="show = false; window.location.reload()"
                  class="space-y-3">
                <div>
                    <label class="text-xs text-gray-500 dark:text-gray-400">Name *</label>
                    <input type="text" name="name" required
                           class="w-full mt-1 text-sm bg-gray-50 dark:bg-dark-bg border border-gray-200 dark:border-dark-border rounded-lg px-3 py-2 text-gray-900 dark:text-white">
                </div>
                <div>
                    <label class="text-xs text-gray-500 dark:text-gray-400">Description</label>
                    <input type="text" name="description"
                           class="w-full mt-1 text-sm bg-gray-50 dark:bg-dark-bg border border-gray-200 dark:border-dark-border rounded-lg px-3 py-2 text-gray-900 dark:text-white">
                </div>
                <div class="grid grid-cols-2 gap-3">
                    <div>
                        <label class="text-xs text-gray-500 dark:text-gray-400">Registration Opens</label>
                        <input type="datetime-local" name="registration_start"
                               class="w-full mt-1 text-sm bg-gray-50 dark:bg-dark-bg border border-gray-200 dark:border-dark-border rounded-lg px-3 py-2 text-gray-900 dark:text-white">
                    </div>
                    <div>
                        <label class="text-xs text-gray-500 dark:text-gray-400">Registration Closes</label>
                        <input type="datetime-local" name="registration_end"
                               class="w-full mt-1 text-sm bg-gray-50 dark:bg-dark-bg border border-gray-200 dark:border-dark-border rounded-lg px-3 py-2 text-gray-900 dark:text-white">
                    </div>
                    <div>
                        <label class="text-xs text-gray-500 dark:text-gray-400">Competition Start</label>
                        <input type="datetime-local" name="start_at"
                               class="w-full mt-1 text-sm bg-gray-50 dark:bg-dark-bg border border-gray-200 dark:border-dark-border rounded-lg px-3 py-2 text-gray-900 dark:text-white">
                    </div>
                    <div>
                        <label class="text-xs text-gray-500 dark:text-gray-400">Competition End</label>
                        <input type="datetime-local" name="end_at"
                               class="w-full mt-1 text-sm bg-gray-50 dark:bg-dark-bg border border-gray-200 dark:border-dark-border rounded-lg px-3 py-2 text-gray-900 dark:text-white">
                    </div>
                </div>
                <div class="flex gap-3 pt-2">
                    <button type="submit" class="flex-1 bg-purple-600 hover:bg-purple-700 text-white text-sm py-2 rounded-lg">Create</button>
                    <button type="button" @click="show = false" class="flex-1 bg-gray-100 dark:bg-dark-bg text-gray-700 dark:text-gray-300 text-sm py-2 rounded-lg">Cancel</button>
                </div>
            </form>
        </div>
    </div>
</div>
```

Note: the HTMX `hx-get` for teams returns JSON; we need it to return an HTML fragment. Update `ListTeams` handler to return HTML when `HX-Request` header is present:

In `competitions.go`, update `ListTeams`:

```go
func (h *CompetitionHandler) ListTeams(w http.ResponseWriter, r *http.Request) {
    id, err := parseCompID(r)
    if err != nil {
        http.Error(w, "Invalid ID", http.StatusBadRequest)
        return
    }
    teams, err := h.db.GetCompetitionTeams(id)
    if err != nil {
        http.Error(w, "Failed to fetch teams", http.StatusInternalServerError)
        return
    }
    if r.Header.Get("HX-Request") == "true" {
        w.Header().Set("Content-Type", "text/html")
        if len(teams) == 0 {
            fmt.Fprint(w, `<p class="text-sm text-gray-400">No teams registered.</p>`)
            return
        }
        fmt.Fprint(w, `<ul class="space-y-1">`)
        for _, t := range teams {
            fmt.Fprintf(w, `<li class="text-sm text-gray-700 dark:text-gray-300">%s</li>`, t.Name)
        }
        fmt.Fprint(w, `</ul>`)
        return
    }
    if teams == nil {
        teams = []models.Team{}
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(teams)
}
```

Add `"fmt"` to the import if not already there.

**Step 4: Rebuild and validate admin UI**

```bash
task rebuild
./hctf2 --port 8093 --dev --db /tmp/hctf2_comp_test.db --admin-email admin@test.com --admin-password testpass123 &
sleep 2
npx agent-browser --session hctf2comp open http://localhost:8093/login && \
npx agent-browser --session hctf2comp fill 'input[name="email"]' admin@test.com && \
npx agent-browser --session hctf2comp fill 'input[name="password"]' testpass123 && \
npx agent-browser --session hctf2comp find role button click --name Login && \
npx agent-browser --session hctf2comp open http://localhost:8093/admin && \
npx agent-browser --session hctf2comp screenshot --full /tmp/admin_competitions.png
# Read the screenshot — click Competitions tab if needed
npx agent-browser --session hctf2comp find text click --name Competitions && \
npx agent-browser --session hctf2comp screenshot --full /tmp/admin_comp_tab.png
kill %1
```

**Step 5: Commit**

```bash
git add internal/views/templates/admin.html main.go internal/handlers/competitions.go
git commit -m "feat(admin): add Competitions tab with CRUD, lifecycle controls, and challenge/team management"
```

---

## Task 8: Add navigation link + final UI validation

**Files:**
- Modify: `internal/views/templates/base.html`

**Step 1: Add "Competitions" nav link in base.html**

Find the navigation links in `base.html` (look for "Scoreboard" or "Challenges" links). Add:

```html
<a href="/competitions"
   class="{{if eq .Page "competitions"}}text-purple-500{{else}}text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white{{end}} transition-colors text-sm font-medium">
   Competitions
</a>
```

**Step 2: Rebuild and check nav in both themes**

```bash
task rebuild
./hctf2 --port 8093 --dev --db /tmp/hctf2_comp_test.db --admin-email admin@test.com --admin-password testpass123 &
sleep 2
npx agent-browser --session hctf2comp open http://localhost:8093 && \
npx agent-browser --session hctf2comp screenshot --full /tmp/nav_light.png
# Toggle dark theme and screenshot
npx agent-browser --session hctf2comp click '[data-theme-toggle]' && \
npx agent-browser --session hctf2comp screenshot --full /tmp/nav_dark.png
kill %1
```

Read both screenshots to confirm nav link appears and competitions page loads.

**Step 3: End-to-end flow test**

```bash
task rebuild
./hctf2 --port 8093 --dev --db /tmp/hctf2_comp_test2.db --admin-email admin@test.com --admin-password testpass123 &
sleep 2

# Login
npx agent-browser --session hctf2e2e open http://localhost:8093/login && \
npx agent-browser --session hctf2e2e fill 'input[name="email"]' admin@test.com && \
npx agent-browser --session hctf2e2e fill 'input[name="password"]' testpass123 && \
npx agent-browser --session hctf2e2e find role button click --name Login

# Go to admin > Competitions > Create
npx agent-browser --session hctf2e2e open http://localhost:8093/admin && \
npx agent-browser --session hctf2e2e find text click --name Competitions && \
npx agent-browser --session hctf2e2e click 'button:has-text("+ New Competition")' && \
npx agent-browser --session hctf2e2e fill 'input[name="name"]' "Test CTF 2026" && \
npx agent-browser --session hctf2e2e click 'button[type="submit"]:has-text("Create")' && \
npx agent-browser --session hctf2e2e screenshot --full /tmp/after_create.png

# View public page
npx agent-browser --session hctf2e2e open http://localhost:8093/competitions && \
npx agent-browser --session hctf2e2e screenshot --full /tmp/comp_list_public.png

kill %1
```

Read both screenshots. Confirm competition created and appears on public page.

**Step 4: Commit**

```bash
git add internal/views/templates/base.html
git commit -m "feat(nav): add Competitions link to navigation"
```

---

## Task 9: Run tests and generate OpenAPI spec

**Step 1: Run existing tests**

```bash
task test
# Expected: all pass (competitions don't affect existing tests)
```

**Step 2: Generate OpenAPI spec**

```bash
task generate-openapi
git add docs/openapi.yaml
git commit -m "docs(openapi): update spec with competition endpoints"
```

**Step 3: Final commit and tag check**

```bash
git log --oneline -10
# Review all competition-related commits are present
```

---

## Notes for implementer

- **Import path**: the module is `github.com/yourusername/hctf2` — verify in `go.mod` and use the same import path in new files.
- **`rowScanner` interface**: if a `rowScanner` interface already exists in `queries.go`, don't redeclare it — just use the existing one or inline the scan.
- **`nullIfEmpty` helper**: check if a similar helper already exists in `queries.go` before adding it.
- **`time` import in competitions.go**: make sure the import block includes `"fmt"` and `"time"` alongside the standard `net/http` etc.
- **Template function `not`**: Go templates don't have `not` by default. Replace `{{if not .Competitions}}` with `{{if not .Competitions}}` — actually use `{{if eq (len .Competitions) 0}}` or just `{{if not .Competitions}}` (works when nil/empty slice). Test this carefully.
- **`$.Challenges`**: in the admin template range loop, `$.Challenges` accesses the root data's `Challenges` field (all challenges) for the challenge picker. Verify this is `$.Challenges` (all challenges, not competition challenges).
- **`hx-on::after-request`**: HTMX 1.x uses `hx-on:htmx:after-request` — check which HTMX version is in use. Look at existing admin.html for the pattern already used in the codebase and match it.
