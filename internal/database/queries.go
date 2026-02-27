package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/yourusername/hctf2/internal/models"
)

// Ping checks database connectivity
func (db *DB) Ping() error {
	return db.DB.Ping()
}

// generateRandomCode creates a cryptographically secure random hex string
func generateRandomCode() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// User queries
func (db *DB) CreateUser(email, passwordHash, name string, isAdmin bool) (*models.User, error) {
	id := GenerateID()
	query := `INSERT INTO users (id, email, password_hash, name, is_admin)
	          VALUES (?, ?, ?, ?, ?) RETURNING id, email, name, is_admin, created_at, updated_at`

	var user models.User
	err := db.QueryRow(query, id, email, passwordHash, name, isAdmin).Scan(
		&user.ID, &user.Email, &user.Name, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt,
	)
	user.PasswordHash = passwordHash
	return &user, err
}

func (db *DB) GetUserByEmail(email string) (*models.User, error) {
	query := `SELECT id, email, password_hash, name, avatar_url, team_id, is_admin, created_at, updated_at
	          FROM users WHERE email = ?`

	var user models.User
	err := db.QueryRow(query, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name,
		&user.AvatarURL, &user.TeamID, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (db *DB) GetUserByID(id string) (*models.User, error) {
	query := `SELECT id, email, password_hash, name, avatar_url, team_id, is_admin, created_at, updated_at
	          FROM users WHERE id = ?`

	var user models.User
	err := db.QueryRow(query, id).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name,
		&user.AvatarURL, &user.TeamID, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// CalculateDynamicScore computes the current point value for a question based on solve count
// Uses linear decay from initial_points down to minimum_points based on decay_threshold
func CalculateDynamicScore(challenge models.Challenge, solveCount int) int {
	if !challenge.DynamicScoring {
		return 0 // Return 0 to indicate static scoring should be used
	}
	if solveCount <= 0 {
		return challenge.InitialPoints
	}
	if solveCount >= challenge.DecayThreshold {
		return challenge.MinimumPoints
	}
	// Linear decay
	decay := float64(challenge.InitialPoints-challenge.MinimumPoints) *
		float64(solveCount) / float64(challenge.DecayThreshold)
	score := challenge.InitialPoints - int(decay)
	if score < challenge.MinimumPoints {
		return challenge.MinimumPoints
	}
	return score
}

// GetQuestionSolveCount returns the number of correct solves for a question
func (db *DB) GetQuestionSolveCount(questionID string) (int, error) {
	var count int
	query := `SELECT COUNT(DISTINCT user_id) FROM submissions WHERE question_id = ? AND is_correct = 1`
	err := db.QueryRow(query, questionID).Scan(&count)
	return count, err
}

// Challenge queries
func (db *DB) CreateChallenge(name, description, category, difficulty string, tags *string, visible bool, sqlEnabled bool, sqlDatasetURL, sqlSchemaHint *string, dynamicScoring bool, initialPoints, minimumPoints, decayThreshold int, fileURL *string) (*models.Challenge, error) {
	id := GenerateID()
	query := `INSERT INTO challenges (id, name, description, category, difficulty, tags, visible, sql_enabled, sql_dataset_url, sql_schema_hint, dynamic_scoring, initial_points, minimum_points, decay_threshold, file_url)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	          RETURNING id, name, description, category, difficulty, tags, visible, sql_enabled, sql_dataset_url, sql_schema_hint, dynamic_scoring, initial_points, minimum_points, decay_threshold, file_url, created_at, updated_at`

	var c models.Challenge
	err := db.QueryRow(query, id, name, description, category, difficulty, tags, visible, sqlEnabled, sqlDatasetURL, sqlSchemaHint, dynamicScoring, initialPoints, minimumPoints, decayThreshold, fileURL).Scan(
		&c.ID, &c.Name, &c.Description, &c.Category, &c.Difficulty, &c.Tags, &c.Visible, &c.SQLEnabled, &c.SQLDatasetURL, &c.SQLSchemaHint, &c.DynamicScoring, &c.InitialPoints, &c.MinimumPoints, &c.DecayThreshold, &c.FileURL, &c.CreatedAt, &c.UpdatedAt,
	)
	return &c, err
}

func (db *DB) GetChallenges(visibleOnly bool) ([]models.Challenge, error) {
	query := `SELECT id, name, description, category, difficulty, tags, visible, sql_enabled, sql_dataset_url, sql_schema_hint, COALESCE(dynamic_scoring, 0), COALESCE(initial_points, 500), COALESCE(minimum_points, 100), COALESCE(decay_threshold, 50), file_url, created_at, updated_at
	          FROM challenges`
	if visibleOnly {
		query += " WHERE visible = 1"
	}
	query += " ORDER BY created_at DESC"

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var challenges []models.Challenge
	for rows.Next() {
		var c models.Challenge
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Category, &c.Difficulty, &c.Tags, &c.Visible, &c.SQLEnabled, &c.SQLDatasetURL, &c.SQLSchemaHint, &c.DynamicScoring, &c.InitialPoints, &c.MinimumPoints, &c.DecayThreshold, &c.FileURL, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		challenges = append(challenges, c)
	}
	return challenges, nil
}

func (db *DB) GetChallengeByID(id string) (*models.Challenge, error) {
	query := `SELECT id, name, description, category, difficulty, tags, visible, sql_enabled, sql_dataset_url, sql_schema_hint, COALESCE(dynamic_scoring, 0), COALESCE(initial_points, 500), COALESCE(minimum_points, 100), COALESCE(decay_threshold, 50), file_url, created_at, updated_at
	          FROM challenges WHERE id = ?`

	var c models.Challenge
	err := db.QueryRow(query, id).Scan(
		&c.ID, &c.Name, &c.Description, &c.Category, &c.Difficulty, &c.Tags, &c.Visible, &c.SQLEnabled, &c.SQLDatasetURL, &c.SQLSchemaHint, &c.DynamicScoring, &c.InitialPoints, &c.MinimumPoints, &c.DecayThreshold, &c.FileURL, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (db *DB) UpdateChallenge(id, name, description, category, difficulty string, tags *string, visible bool, sqlEnabled bool, sqlDatasetURL, sqlSchemaHint *string, dynamicScoring bool, initialPoints, minimumPoints, decayThreshold int) error {
	query := `UPDATE challenges
	          SET name = ?, description = ?, category = ?, difficulty = ?, tags = ?, visible = ?, sql_enabled = ?, sql_dataset_url = ?, sql_schema_hint = ?, dynamic_scoring = ?, initial_points = ?, minimum_points = ?, decay_threshold = ?, updated_at = CURRENT_TIMESTAMP
	          WHERE id = ?`
	_, err := db.Exec(query, name, description, category, difficulty, tags, visible, sqlEnabled, sqlDatasetURL, sqlSchemaHint, dynamicScoring, initialPoints, minimumPoints, decayThreshold, id)
	return err
}

func (db *DB) DeleteChallenge(id string) error {
	_, err := db.Exec("DELETE FROM challenges WHERE id = ?", id)
	return err
}

// SetChallengeFileURL updates the file_url for a challenge.
func (db *DB) SetChallengeFileURL(challengeID, url string) error {
	var fileURL interface{}
	if url != "" {
		fileURL = url
	}
	_, err := db.Exec(`UPDATE challenges SET file_url = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, fileURL, challengeID)
	return err
}

// Question queries
func (db *DB) CreateQuestion(challengeID, name, description, flag string, flagMask *string, caseSensitive bool, points int, fileURL *string) (*models.Question, error) {
	// Auto-generate flag mask if not provided
	if flagMask == nil || *flagMask == "" {
		mask := generateFlagMask(flag)
		flagMask = &mask
	}

	id := GenerateID()
	query := `INSERT INTO questions (id, challenge_id, name, description, flag, flag_mask, case_sensitive, points, file_url)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	          RETURNING id, challenge_id, name, description, flag, flag_mask, case_sensitive, points, file_url, created_at, updated_at`

	var q models.Question
	err := db.QueryRow(query, id, challengeID, name, description, flag, flagMask, caseSensitive, points, fileURL).Scan(
		&q.ID, &q.ChallengeID, &q.Name, &q.Description, &q.Flag, &q.FlagMask, &q.CaseSensitive, &q.Points, &q.FileURL, &q.CreatedAt, &q.UpdatedAt,
	)
	return &q, err
}

func (db *DB) GetQuestionsByChallengeID(challengeID string) ([]models.Question, error) {
	query := `SELECT id, challenge_id, name, description, flag, flag_mask, case_sensitive, points, file_url, created_at, updated_at
	          FROM questions WHERE challenge_id = ? ORDER BY created_at`

	rows, err := db.Query(query, challengeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []models.Question
	for rows.Next() {
		var q models.Question
		if err := rows.Scan(&q.ID, &q.ChallengeID, &q.Name, &q.Description, &q.Flag, &q.FlagMask, &q.CaseSensitive, &q.Points, &q.FileURL, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, nil
}

func (db *DB) GetQuestionByID(id string) (*models.Question, error) {
	query := `SELECT id, challenge_id, name, description, flag, flag_mask, case_sensitive, points, file_url, created_at, updated_at
	          FROM questions WHERE id = ?`

	var q models.Question
	err := db.QueryRow(query, id).Scan(
		&q.ID, &q.ChallengeID, &q.Name, &q.Description, &q.Flag, &q.FlagMask, &q.CaseSensitive, &q.Points, &q.FileURL, &q.CreatedAt, &q.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (db *DB) GetAllQuestions() ([]models.Question, error) {
	query := `SELECT id, challenge_id, name, description, flag, flag_mask, case_sensitive, points, file_url, created_at, updated_at
	          FROM questions ORDER BY created_at DESC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []models.Question
	for rows.Next() {
		var q models.Question
		if err := rows.Scan(&q.ID, &q.ChallengeID, &q.Name, &q.Description, &q.Flag, &q.FlagMask, &q.CaseSensitive, &q.Points, &q.FileURL, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, nil
}

func (db *DB) UpdateQuestion(id, name, description, flag string, flagMask *string, caseSensitive bool, points int, fileURL *string) error {
	if flagMask == nil || *flagMask == "" {
		mask := generateFlagMask(flag)
		flagMask = &mask
	}

	query := `UPDATE questions
	          SET name = ?, description = ?, flag = ?, flag_mask = ?, case_sensitive = ?, points = ?, file_url = ?, updated_at = CURRENT_TIMESTAMP
	          WHERE id = ?`
	_, err := db.Exec(query, name, description, flag, flagMask, caseSensitive, points, fileURL, id)
	return err
}

func (db *DB) DeleteQuestion(id string) error {
	_, err := db.Exec("DELETE FROM questions WHERE id = ?", id)
	return err
}

// Submission queries
func (db *DB) CreateSubmission(questionID, userID string, teamID *string, submittedFlag string, isCorrect bool) error {
	id := GenerateID()
	query := `INSERT INTO submissions (id, question_id, user_id, team_id, submitted_flag, is_correct)
	          VALUES (?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query, id, questionID, userID, teamID, submittedFlag, isCorrect)
	return err
}

func (db *DB) HasUserSolved(questionID, userID string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM submissions WHERE question_id = ? AND user_id = ? AND is_correct = 1", questionID, userID).Scan(&count)
	return count > 0, err
}

func (db *DB) GetScoreboard(limit int) ([]models.ScoreboardEntry, error) {
	// SQLite doesn't support ROW_NUMBER() in the same way, so we calculate rank in Go
	freezeCond := ""
	var args []interface{}

	if ft := db.FreezeTimestamp(); ft != nil {
		freezeCond = " AND s.created_at <= ?"
		args = append(args, ft.UTC().Format("2006-01-02 15:04:05"))
	}

	// Check if admins should be visible
	adminVisible := db.GetAdminVisibleInScoreboard()
	adminFilter := ""
	if !adminVisible {
		adminFilter = " WHERE u.is_admin = 0"
	}

	query := fmt.Sprintf(`
		SELECT
			u.id as user_id,
			u.name as user_name,
			u.team_id,
			t.name as team_name,
			COALESCE(SUM(q.points), 0) - COALESCE(hint_costs.total_cost, 0) as points,
			COUNT(DISTINCT s.question_id) as solve_count,
			COALESCE(MAX(s.created_at), u.created_at) as last_solve
		FROM users u
		LEFT JOIN teams t ON u.team_id = t.id
		LEFT JOIN submissions s ON u.id = s.user_id AND s.is_correct = 1%s
		LEFT JOIN questions q ON s.question_id = q.id
		LEFT JOIN (
			SELECT hu.user_id, SUM(h.cost) as total_cost
			FROM hint_unlocks hu
			JOIN hints h ON hu.hint_id = h.id
			GROUP BY hu.user_id
		) hint_costs ON u.id = hint_costs.user_id
		%s
		GROUP BY u.id, u.name, u.team_id, t.name, hint_costs.total_cost
		ORDER BY points DESC, last_solve ASC
		LIMIT ?
	`, freezeCond, adminFilter)

	args = append(args, limit)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.ScoreboardEntry
	rank := 1
	var prevPoints int
	for rows.Next() {
		var e models.ScoreboardEntry
		var lastSolveStr string
		if err := rows.Scan(&e.UserID, &e.UserName, &e.TeamID, &e.TeamName, &e.Points, &e.SolveCount, &lastSolveStr); err != nil {
			return nil, err
		}

		// Parse the date string from SQLite
		parsedTime, err := time.Parse("2006-01-02 15:04:05", lastSolveStr)
		if err != nil {
			// If parsing fails, use current time
			parsedTime = time.Now()
		}
		e.LastSolve = parsedTime
		
		// Apply standard competition ranking (1224 rule)
		// Same score = same rank, next rank skips
		if len(entries) > 0 && e.Points < prevPoints {
			rank = len(entries) + 1
		}
		e.Rank = rank
		prevPoints = e.Points
		
		entries = append(entries, e)
	}
	return entries, nil
}

// InsertScoreHistory records a user's score snapshot
func (db *DB) InsertScoreHistory(userID, teamID string, score, solveCount int) error {
	query := `INSERT INTO score_history (id, user_id, team_id, score, solve_count) VALUES (?, ?, ?, ?, ?)`
	_, err := db.Exec(query, GenerateID(), userID, teamID, score, solveCount)
	return err
}

// ScoreEvolutionPoint represents a single data point for the chart
type ScoreEvolutionPoint struct {
	RecordedAt time.Time `json:"recorded_at"`
	Score      int       `json:"score"`
}

// ScoreEvolutionSeries represents one user's score over time
type ScoreEvolutionSeries struct {
	UserID string                `json:"id"`
	Name   string                `json:"name"`
	Scores []ScoreEvolutionPoint `json:"scores"`
}

// GetScoreEvolution returns score history for top N users
func (db *DB) GetScoreEvolution(limit int, since time.Time) ([]ScoreEvolutionSeries, error) {
	// Check if admins should be visible
	adminVisible := db.GetAdminVisibleInScoreboard()
	adminFilter := ""
	if !adminVisible {
		adminFilter = "WHERE u.is_admin = 0"
	}

	// Get top N users by current score
	topUsersQuery := fmt.Sprintf(`
		SELECT u.id, u.name, COALESCE(SUM(q.points), 0) - COALESCE(hint_costs.total_cost, 0) as points
		FROM users u
		LEFT JOIN submissions s ON u.id = s.user_id AND s.is_correct = 1
		LEFT JOIN questions q ON s.question_id = q.id
		LEFT JOIN (
			SELECT hu.user_id, SUM(h.cost) as total_cost
			FROM hint_unlocks hu
			JOIN hints h ON hu.hint_id = h.id
			GROUP BY hu.user_id
		) hint_costs ON u.id = hint_costs.user_id
		%s
		GROUP BY u.id, u.name, hint_costs.total_cost
		ORDER BY points DESC
		LIMIT ?
	`, adminFilter)

	rows, err := db.Query(topUsersQuery, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []string
	var userNames []string
	for rows.Next() {
		var id, name string
		var points int
		if err := rows.Scan(&id, &name, &points); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, id)
		userNames = append(userNames, name)
	}

	var result []ScoreEvolutionSeries
	for i, userID := range userIDs {
		historyQuery := `
			SELECT recorded_at, score 
			FROM score_history 
			WHERE user_id = ? AND recorded_at >= ?
			ORDER BY recorded_at ASC
		`
		histRows, err := db.Query(historyQuery, userID, since)
		if err != nil {
			return nil, err
		}

		var points []ScoreEvolutionPoint
		for histRows.Next() {
			var p ScoreEvolutionPoint
			if err := histRows.Scan(&p.RecordedAt, &p.Score); err != nil {
				histRows.Close()
				return nil, err
			}
			points = append(points, p)
		}
		histRows.Close()

		result = append(result, ScoreEvolutionSeries{
			UserID: userID,
			Name:   userNames[i],
			Scores: points,
		})
	}

	return result, nil
}

// CleanupScoreHistory removes old records beyond retention period
func (db *DB) CleanupScoreHistory(retentionDays int) error {
	query := `DELETE FROM score_history WHERE recorded_at < datetime('now', '-? days')`
	_, err := db.Exec(query, retentionDays)
	return err
}

// GetAdminVisibleInScoreboard returns whether admins should appear in scoreboard
func (db *DB) GetAdminVisibleInScoreboard() bool {
	query := `SELECT value FROM site_settings WHERE key = 'admin_visible_in_scoreboard'`
	var value string
	err := db.QueryRow(query).Scan(&value)
	if err != nil {
		return false // Default to hidden
	}
	return value == "1" || value == "true"
}

// SetAdminVisibleInScoreboard sets whether admins appear in scoreboard
func (db *DB) SetAdminVisibleInScoreboard(visible bool) error {
	value := "0"
	if visible {
		value = "1"
	}
	query := `INSERT INTO site_settings (key, value, updated_at) VALUES ('admin_visible_in_scoreboard', ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`
	_, err := db.Exec(query, value)
	return err
}

// Helper function to generate flag mask
func generateFlagMask(flag string) string {
	// Find the flag format (e.g., FLAG{...})
	if start := strings.Index(flag, "{"); start != -1 {
		if end := strings.Index(flag[start:], "}"); end != -1 {
			prefix := flag[:start+1]
			suffix := flag[start+end:]
			masked := strings.Repeat("*", end-1)
			return prefix + masked + suffix
		}
	}
	// Default: mask entire flag
	return strings.Repeat("*", len(flag))
}

// GetSQLSnapshot returns data for client-side SQL queries
func (db *DB) GetSQLSnapshot() (map[string]interface{}, error) {
	snapshot := make(map[string]interface{})

	// Public challenges
	challenges, err := db.GetChallenges(true)
	if err != nil {
		return nil, err
	}
	snapshot["challenges"] = challenges

	// Public questions (without flags)
	var questions []map[string]interface{}
	rows, err := db.Query(`
		SELECT id, challenge_id, name, description, flag_mask, case_sensitive, points, created_at
		FROM questions WHERE challenge_id IN (SELECT id FROM challenges WHERE visible = 1)
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, challengeID, name, desc string
		var flagMask sql.NullString
		var caseSensitive bool
		var points int
		var createdAt string

		if err := rows.Scan(&id, &challengeID, &name, &desc, &flagMask, &caseSensitive, &points, &createdAt); err != nil {
			return nil, err
		}

		questions = append(questions, map[string]interface{}{
			"id":             id,
			"challenge_id":   challengeID,
			"name":           name,
			"description":    desc,
			"flag_mask":      flagMask.String,
			"case_sensitive": caseSensitive,
			"points":         points,
			"created_at":     createdAt,
		})
	}
	snapshot["questions"] = questions

	// Public submissions (correct only, no flags)
	var submissions []map[string]interface{}
	rows2, err := db.Query(`
		SELECT s.id, s.question_id, s.user_id, s.team_id, s.is_correct, s.created_at, u.name as user_name
		FROM submissions s
		JOIN users u ON s.user_id = u.id
		WHERE s.is_correct = 1
	`)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()

	for rows2.Next() {
		var id, questionID, userID string
		var teamID sql.NullString
		var isCorrect bool
		var createdAt, userName string

		if err := rows2.Scan(&id, &questionID, &userID, &teamID, &isCorrect, &createdAt, &userName); err != nil {
			return nil, err
		}

		submissions = append(submissions, map[string]interface{}{
			"id":          id,
			"question_id": questionID,
			"user_id":     userID,
			"team_id":     teamID.String,
			"user_name":   userName,
			"is_correct":  isCorrect,
			"created_at":  createdAt,
		})
	}
	snapshot["submissions"] = submissions

	// Public users (name only)
	var users []map[string]interface{}
	rows3, err := db.Query(`SELECT id, name, team_id, created_at FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows3.Close()

	for rows3.Next() {
		var id, name string
		var teamID sql.NullString
		var createdAt string

		if err := rows3.Scan(&id, &name, &teamID, &createdAt); err != nil {
			return nil, err
		}

		users = append(users, map[string]interface{}{
			"id":         id,
			"name":       name,
			"team_id":    teamID.String,
			"created_at": createdAt,
		})
	}
	snapshot["users"] = users

	// Public teams (no invite info)
	var teams []map[string]interface{}
	rows4, err := db.Query(`SELECT id, name, COALESCE(description, '') as description, owner_id, created_at FROM teams`)
	if err != nil {
		return nil, err
	}
	defer rows4.Close()

	for rows4.Next() {
		var id, name, description, ownerID, createdAt string
		if err := rows4.Scan(&id, &name, &description, &ownerID, &createdAt); err != nil {
			return nil, err
		}
		teams = append(teams, map[string]interface{}{
			"id":          id,
			"name":        name,
			"description": description,
			"owner_id":    ownerID,
			"created_at":  createdAt,
		})
	}
	snapshot["teams"] = teams

	// Hints (schema only - content is sensitive and requires unlock)
	var hints []map[string]interface{}
	rows5, err := db.Query(`
		SELECT h.id, h.question_id, h.cost, h.created_at
		FROM hints h
		JOIN questions q ON h.question_id = q.id
		WHERE q.challenge_id IN (SELECT id FROM challenges WHERE visible = 1)
	`)
	if err != nil {
		return nil, err
	}
	defer rows5.Close()

	for rows5.Next() {
		var id, questionID string
		var cost int
		var createdAt string
		if err := rows5.Scan(&id, &questionID, &cost, &createdAt); err != nil {
			return nil, err
		}
		hints = append(hints, map[string]interface{}{
			"id":          id,
			"question_id": questionID,
			"cost":        cost,
			"created_at":  createdAt,
		})
	}
	snapshot["hints"] = hints

	// Hint unlocks (for SQL playground analysis of penalties)
	var hintUnlocks []map[string]interface{}
	rows6, err := db.Query(`
		SELECT hu.id, hu.hint_id, hu.user_id, hu.team_id, hu.created_at
		FROM hint_unlocks hu
		JOIN hints h ON hu.hint_id = h.id
		JOIN questions q ON h.question_id = q.id
		WHERE q.challenge_id IN (SELECT id FROM challenges WHERE visible = 1)
	`)
	if err != nil {
		return nil, err
	}
	defer rows6.Close()

	for rows6.Next() {
		var id, hintID, userID string
		var teamID sql.NullString
		var createdAt string
		if err := rows6.Scan(&id, &hintID, &userID, &teamID, &createdAt); err != nil {
			return nil, err
		}
		hintUnlocks = append(hintUnlocks, map[string]interface{}{
			"id":         id,
			"hint_id":    hintID,
			"user_id":    userID,
			"team_id":    teamID.String,
			"created_at": createdAt,
		})
	}
	snapshot["hint_unlocks"] = hintUnlocks

	return snapshot, nil
}

// GetChallengeCount returns the total number of visible challenges
func (db *DB) GetChallengeCount() (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM challenges WHERE visible = 1").Scan(&count)
	return count, err
}

// GetUserCount returns the total number of users
func (db *DB) GetUserCount() (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

// GetAllUsers returns all users with basic info (no password hash)
func (db *DB) GetAllUsers() ([]models.User, error) {
	query := `SELECT id, email, name, avatar_url, team_id, is_admin, created_at, updated_at FROM users ORDER BY created_at DESC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.TeamID, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// SetUserAdminStatus updates a user's admin status
func (db *DB) SetUserAdminStatus(userID string, isAdmin bool) error {
	query := `UPDATE users SET is_admin = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := db.Exec(query, isAdmin, userID)
	return err
}

// DeleteUser deletes a user (CASCADE will handle related data)
func (db *DB) DeleteUser(userID string) error {
	_, err := db.Exec("DELETE FROM users WHERE id = ?", userID)
	return err
}

// GetCorrectSubmissionCount returns the total number of correct submissions
func (db *DB) GetCorrectSubmissionCount() (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM submissions WHERE is_correct = 1").Scan(&count)
	return count, err
}

// Team queries

// CreateTeam creates a new team
func (db *DB) CreateTeam(name, description string, ownerID string) (*models.Team, error) {
	id := GenerateID()
	inviteID := generateRandomCode()
	query := `INSERT INTO teams (id, name, description, owner_id, invite_id)
	          VALUES (?, ?, ?, ?, ?)
	          RETURNING id, name, description, owner_id, invite_id, invite_permission, created_at, updated_at`

	var t models.Team
	err := db.QueryRow(query, id, name, description, ownerID, inviteID).Scan(
		&t.ID, &t.Name, &t.Description, &t.OwnerID, &t.InviteID, &t.InvitePermission, &t.CreatedAt, &t.UpdatedAt,
	)
	return &t, err
}

// GetTeamByID fetches a team by ID
func (db *DB) GetTeamByID(id string) (*models.Team, error) {
	query := `SELECT id, name, description, owner_id, invite_id, invite_permission, created_at, updated_at
	          FROM teams WHERE id = ?`

	var t models.Team
	err := db.QueryRow(query, id).Scan(
		&t.ID, &t.Name, &t.Description, &t.OwnerID, &t.InviteID, &t.InvitePermission, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetAllTeams fetches all teams ordered by name
func (db *DB) GetAllTeams() ([]models.Team, error) {
	query := `SELECT id, name, description, owner_id, invite_id, invite_permission, created_at, updated_at
	          FROM teams ORDER BY name ASC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []models.Team
	for rows.Next() {
		var t models.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.OwnerID, &t.InviteID, &t.InvitePermission, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, nil
}

// JoinTeam updates user's team_id
func (db *DB) JoinTeam(userID, teamID string) error {
	query := `UPDATE users SET team_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := db.Exec(query, teamID, userID)
	return err
}

// LeaveTeam sets user's team_id to NULL
func (db *DB) LeaveTeam(userID string) error {
	query := `UPDATE users SET team_id = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := db.Exec(query, userID)
	return err
}

// GetTeamMembers returns all users in a team
func (db *DB) GetTeamMembers(teamID string) ([]models.User, error) {
	query := `SELECT id, email, name, avatar_url, team_id, is_admin, created_at, updated_at
	          FROM users WHERE team_id = ? ORDER BY name ASC`

	rows, err := db.Query(query, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.TeamID, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// GetTeamScoreboard returns team rankings with aggregated points.
// Only submissions made while the user was in the team (s.team_id = t.id) count.
// Each unique question is counted once per team regardless of how many members solved it.
// Submissions/hints with NULL team_id were made before joining a team and don't count.
// Dynamic scoring: points decay based on number of solves before this team's solve.
func (db *DB) GetTeamScoreboard(limit int) ([]models.ScoreboardEntry, error) {
	freezeCond := ""
	var args []interface{}

	if ft := db.FreezeTimestamp(); ft != nil {
		freezeCond = " AND s.created_at <= ?"
		args = append(args, ft.UTC().Format("2006-01-02 15:04:05"))
	}

	query := fmt.Sprintf(`
		SELECT
			t.id as team_id,
			t.name as team_name,
			COALESCE(team_pts.points, 0) - COALESCE(hint_costs.total_cost, 0) as points,
			COALESCE(team_pts.solve_count, 0) as solve_count,
			COALESCE(team_pts.last_solve, t.created_at) as last_solve
		FROM teams t
		LEFT JOIN (
			SELECT
				s.team_id,
				SUM(
					CASE
						WHEN c.dynamic_scoring = 1 THEN
							CASE
								WHEN c.minimum_points > c.initial_points - CAST(
									(c.initial_points - c.minimum_points) *
									(SELECT COUNT(*) FROM submissions s2
									 WHERE s2.question_id = s.question_id
									 AND s2.is_correct = 1
									 AND s2.created_at < s.created_at) /
									CAST(c.decay_threshold AS REAL) AS INTEGER
								) THEN c.minimum_points
								ELSE c.initial_points - CAST(
									(c.initial_points - c.minimum_points) *
									(SELECT COUNT(*) FROM submissions s2
									 WHERE s2.question_id = s.question_id
									 AND s2.is_correct = 1
									 AND s2.created_at < s.created_at) /
									CAST(c.decay_threshold AS REAL) AS INTEGER
								)
							END
						ELSE q.points
					END
				) as points,
				COUNT(*) as solve_count,
				MAX(s.created_at) as last_solve
			FROM (
				SELECT
					s.team_id,
					s.question_id,
					MIN(s.created_at) as created_at
				FROM submissions s
				WHERE s.is_correct = 1
					AND s.team_id IS NOT NULL%s
				GROUP BY s.team_id, s.question_id
			) s
			JOIN questions q ON q.id = s.question_id
			JOIN challenges c ON q.challenge_id = c.id
			GROUP BY s.team_id
		) team_pts ON team_pts.team_id = t.id
		LEFT JOIN (
			SELECT
				hu.team_id,
				SUM(h.cost) as total_cost
			FROM hint_unlocks hu
			JOIN hints h ON hu.hint_id = h.id
			WHERE hu.team_id IS NOT NULL
			GROUP BY hu.team_id
		) hint_costs ON hint_costs.team_id = t.id
		ORDER BY points DESC, last_solve ASC
		LIMIT ?
	`, freezeCond)

	args = append(args, limit)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.ScoreboardEntry
	rank := 1
	var prevPoints int
	for rows.Next() {
		var e models.ScoreboardEntry
		var lastSolveStr string
		if err := rows.Scan(&e.TeamID, &e.TeamName, &e.Points, &e.SolveCount, &lastSolveStr); err != nil {
			return nil, err
		}

		parsedTime, err := time.Parse("2006-01-02 15:04:05", lastSolveStr)
		if err != nil {
			parsedTime = time.Now()
		}
		e.LastSolve = parsedTime
		
		// Apply standard competition ranking (1224 rule)
		// Same score = same rank, next rank skips
		if len(entries) > 0 && e.Points < prevPoints {
			rank = len(entries) + 1
		}
		e.Rank = rank
		prevPoints = e.Points
		
		entries = append(entries, e)
	}
	return entries, nil
}

// UpdateTeam updates team name and description
func (db *DB) UpdateTeam(id, name, description string) error {
	query := `UPDATE teams SET name = ?, description = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := db.Exec(query, name, description, id)
	return err
}

// TransferTeamOwnership updates the team owner
func (db *DB) TransferTeamOwnership(teamID, newOwnerID string) error {
	query := `UPDATE teams SET owner_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := db.Exec(query, newOwnerID, teamID)
	return err
}

// DeleteTeam deletes a team
func (db *DB) DeleteTeam(id string) error {
	_, err := db.Exec("DELETE FROM teams WHERE id = ?", id)
	return err
}

// GetTeamByInviteID fetches a team using the secret invite code
func (db *DB) GetTeamByInviteID(inviteID string) (*models.Team, error) {
	query := `SELECT id, name, description, owner_id, invite_id, invite_permission, created_at, updated_at
	          FROM teams WHERE invite_id = ?`

	var t models.Team
	err := db.QueryRow(query, inviteID).Scan(
		&t.ID, &t.Name, &t.Description, &t.OwnerID, &t.InviteID, &t.InvitePermission, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// RegenerateInviteID creates a new random invite code for a team
func (db *DB) RegenerateInviteID(teamID string) (string, error) {
	// Generate new random invite code
	newInviteID := generateRandomCode()

	query := `UPDATE teams SET invite_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := db.Exec(query, newInviteID, teamID)
	if err != nil {
		return "", err
	}
	return newInviteID, nil
}

// UpdateInvitePermission updates who can share team invites
func (db *DB) UpdateInvitePermission(teamID, permission string) error {
	// Validate permission value
	if permission != "owner_only" && permission != "all_members" {
		return fmt.Errorf("invalid permission value: %s", permission)
	}

	query := `UPDATE teams SET invite_permission = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := db.Exec(query, permission, teamID)
	return err
}

// Hint queries

// CreateHint creates a new hint for a question
func (db *DB) CreateHint(questionID, content string, cost, order int) (*models.Hint, error) {
	id := GenerateID()
	query := `INSERT INTO hints (id, question_id, content, cost, "order")
	          VALUES (?, ?, ?, ?, ?)
	          RETURNING id, question_id, content, cost, "order", created_at`

	var h models.Hint
	err := db.QueryRow(query, id, questionID, content, cost, order).Scan(
		&h.ID, &h.QuestionID, &h.Content, &h.Cost, &h.Order, &h.CreatedAt,
	)
	return &h, err
}

// GetHintsByQuestionID returns all hints for a question ordered by order field
func (db *DB) GetHintsByQuestionID(questionID string) ([]models.Hint, error) {
	query := `SELECT id, question_id, content, cost, "order", created_at
	          FROM hints WHERE question_id = ? ORDER BY "order" ASC`

	rows, err := db.Query(query, questionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hints []models.Hint
	for rows.Next() {
		var h models.Hint
		if err := rows.Scan(&h.ID, &h.QuestionID, &h.Content, &h.Cost, &h.Order, &h.CreatedAt); err != nil {
			return nil, err
		}
		hints = append(hints, h)
	}
	return hints, nil
}

// UnlockHint creates a hint unlock record (idempotent), recording the team the user was in at the time
func (db *DB) UnlockHint(hintID, userID string, teamID *string) error {
	id := GenerateID()
	query := `INSERT INTO hint_unlocks (id, hint_id, user_id, team_id) VALUES (?, ?, ?, ?)
	          ON CONFLICT(hint_id, user_id) DO NOTHING`
	_, err := db.Exec(query, id, hintID, userID, teamID)
	return err
}

// IsHintUnlocked checks if user has unlocked a hint
func (db *DB) IsHintUnlocked(hintID, userID string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM hint_unlocks WHERE hint_id = ? AND user_id = ?", hintID, userID).Scan(&count)
	return count > 0, err
}

// GetUserUnlockedHints returns all hint IDs unlocked by a user for a specific question
func (db *DB) GetUserUnlockedHints(userID, questionID string) ([]string, error) {
	query := `SELECT hu.hint_id FROM hint_unlocks hu
	          JOIN hints h ON hu.hint_id = h.id
	          WHERE hu.user_id = ? AND h.question_id = ?`

	rows, err := db.Query(query, userID, questionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hintIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		hintIDs = append(hintIDs, id)
	}
	return hintIDs, nil
}

// GetUserTotalHintCost calculates total points spent on hints by user
func (db *DB) GetUserTotalHintCost(userID string) (int, error) {
	query := `SELECT COALESCE(SUM(h.cost), 0) FROM hint_unlocks hu
	          JOIN hints h ON hu.hint_id = h.id
	          WHERE hu.user_id = ?`

	var total int
	err := db.QueryRow(query, userID).Scan(&total)
	return total, err
}

// GetUserHintCostForQuestion calculates total points spent on hints by user for a specific question
func (db *DB) GetUserHintCostForQuestion(userID, questionID string) (int, error) {
	query := `SELECT COALESCE(SUM(h.cost), 0) FROM hint_unlocks hu
	          JOIN hints h ON hu.hint_id = h.id
	          WHERE hu.user_id = ? AND h.question_id = ?`

	var total int
	err := db.QueryRow(query, userID, questionID).Scan(&total)
	return total, err
}

// UpdateHint updates hint content, cost, and order
func (db *DB) UpdateHint(id, content string, cost, order int) error {
	query := `UPDATE hints SET content = ?, cost = ?, "order" = ? WHERE id = ?`
	_, err := db.Exec(query, content, cost, order, id)
	return err
}

// DeleteHint deletes a hint
func (db *DB) DeleteHint(id string) error {
	_, err := db.Exec("DELETE FROM hints WHERE id = ?", id)
	return err
}

// GetHintByID fetches a single hint by ID
func (db *DB) GetHintByID(id string) (*models.Hint, error) {
	query := `SELECT id, question_id, content, cost, "order", created_at
	          FROM hints WHERE id = ?`

	var h models.Hint
	err := db.QueryRow(query, id).Scan(
		&h.ID, &h.QuestionID, &h.Content, &h.Cost, &h.Order, &h.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// GetAllQuestionsWithChallenge returns all questions with challenge names for admin dropdown
func (db *DB) GetAllQuestionsWithChallenge() ([]models.QuestionWithChallenge, error) {
	query := `
		SELECT
			q.id,
			q.challenge_id,
			c.name as challenge_name,
			q.name,
			q.description,
			q.flag,
			q.flag_mask,
			q.case_sensitive,
			q.points,
			q.file_url,
			q.created_at,
			q.updated_at
		FROM questions q
		JOIN challenges c ON q.challenge_id = c.id
		ORDER BY c.name, q.name
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []models.QuestionWithChallenge
	for rows.Next() {
		var q models.QuestionWithChallenge
		if err := rows.Scan(&q.ID, &q.ChallengeID, &q.ChallengeName, &q.Name, &q.Description,
			&q.Flag, &q.FlagMask, &q.CaseSensitive, &q.Points, &q.FileURL, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, nil
}

// GetNextHintOrder returns the next order number for a question's hints
func (db *DB) GetNextHintOrder(questionID string) (int, error) {
	var maxOrder int
	query := `SELECT COALESCE(MAX("order"), 0) + 1 FROM hints WHERE question_id = ?`
	err := db.QueryRow(query, questionID).Scan(&maxOrder)
	return maxOrder, err
}

// Profile queries
type UserStats struct {
	UserID          string
	Name            string
	Email           string
	AvatarURL       *string
	TeamID          *string
	TeamName        *string
	CreatedAt       time.Time
	TotalPoints     int
	SolvedCount     int
	TotalSubmissions int
	HintsCost       int
	HintsUnlocked   int
}

func (db *DB) GetUserStats(userID string) (*UserStats, error) {
	query := `
		SELECT
			u.id,
			u.name,
			u.email,
			u.avatar_url,
			u.team_id,
			t.name as team_name,
			u.created_at,
			COALESCE(SUM(q.points), 0) - COALESCE(hint_costs.total_cost, 0) as total_points,
			COUNT(DISTINCT CASE WHEN s.is_correct = 1 THEN s.question_id END) as solved_count,
			COUNT(DISTINCT s.id) as total_submissions,
			COALESCE(hint_costs.total_cost, 0) as hints_cost,
			COALESCE(hint_costs.hint_count, 0) as hints_unlocked
		FROM users u
		LEFT JOIN teams t ON u.team_id = t.id
		LEFT JOIN submissions s ON u.id = s.user_id
		LEFT JOIN questions q ON s.question_id = q.id AND s.is_correct = 1
		LEFT JOIN (
			SELECT hu.user_id, SUM(h.cost) as total_cost, COUNT(*) as hint_count
			FROM hint_unlocks hu
			JOIN hints h ON hu.hint_id = h.id
			GROUP BY hu.user_id
		) hint_costs ON u.id = hint_costs.user_id
		WHERE u.id = ?
		GROUP BY u.id, u.name, u.email, u.avatar_url, u.team_id, t.name, u.created_at
	`

	var stats UserStats
	var teamName sql.NullString
	var teamID sql.NullString
	var avatarURL sql.NullString

	err := db.QueryRow(query, userID).Scan(
		&stats.UserID, &stats.Name, &stats.Email, &avatarURL, &teamID, &teamName,
		&stats.CreatedAt, &stats.TotalPoints, &stats.SolvedCount, &stats.TotalSubmissions,
		&stats.HintsCost, &stats.HintsUnlocked,
	)
	if err != nil {
		return nil, err
	}

	if avatarURL.Valid {
		stats.AvatarURL = &avatarURL.String
	}
	if teamID.Valid {
		stats.TeamID = &teamID.String
	}
	if teamName.Valid {
		stats.TeamName = &teamName.String
	}

	return &stats, nil
}

type SubmissionHistory struct {
	ID            string
	CreatedAt     time.Time
	IsCorrect     bool
	QuestionName  string
	Points        int
	ChallengeName string
	ChallengeID   string
	Category      string
}

func (db *DB) GetUserRecentSubmissions(userID string, limit int) ([]SubmissionHistory, error) {
	query := `
		SELECT
			s.id,
			s.created_at,
			s.is_correct,
			q.name as question_name,
			q.points - COALESCE(hint_costs.cost, 0) as points,
			c.name as challenge_name,
			c.id as challenge_id,
			c.category
		FROM submissions s
		JOIN questions q ON s.question_id = q.id
		JOIN challenges c ON q.challenge_id = c.id
		LEFT JOIN (
			SELECT hu.user_id, h.question_id, SUM(h.cost) as cost
			FROM hint_unlocks hu
			JOIN hints h ON hu.hint_id = h.id
			GROUP BY hu.user_id, h.question_id
		) hint_costs ON s.user_id = hint_costs.user_id AND q.id = hint_costs.question_id
		WHERE s.user_id = ? AND s.is_correct = 1
		ORDER BY s.created_at DESC
		LIMIT ?
	`

	rows, err := db.Query(query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var submissions []SubmissionHistory
	for rows.Next() {
		var sub SubmissionHistory
		if err := rows.Scan(&sub.ID, &sub.CreatedAt, &sub.IsCorrect, &sub.QuestionName,
			&sub.Points, &sub.ChallengeName, &sub.ChallengeID, &sub.Category); err != nil {
			return nil, err
		}
		submissions = append(submissions, sub)
	}
	return submissions, nil
}

type ChallengeSummary struct {
	ID               string
	Name             string
	Category         string
	Difficulty       string
	SolvedQuestions  int
	TotalQuestions   int
}

// ChallengeCompletion tracks completion status
type ChallengeCompletion struct {
	ChallengeID     string
	TotalQuestions  int
	SolvedQuestions int
	IsComplete      bool
}

// TeamSubmission represents a submission by a team member
type TeamSubmission struct {
	ID            string
	QuestionID    string
	QuestionName  string
	Points        int
	ChallengeID   string
	ChallengeName string
	IsCorrect     bool
	CreatedAt     time.Time
	UserID        string
	UserName      string
	HintPenalty   int
}

// TeamChallengeSummary represents a challenge solved by team with points earned
type TeamChallengeSummary struct {
	ID               string
	Name             string
	Category         string
	Difficulty       string
	SolvedQuestions  int
	TotalQuestions   int
	PointsEarned     int
}

// GetChallengeCompletionForUser returns completion status for all challenges for a user
func (db *DB) GetChallengeCompletionForUser(userID string) (map[string]*ChallengeCompletion, error) {
	query := `
		SELECT 
			c.id as challenge_id,
			COUNT(DISTINCT q.id) as total_questions,
			COUNT(DISTINCT CASE WHEN s.is_correct = 1 THEN s.question_id END) as solved_questions
		FROM challenges c
		LEFT JOIN questions q ON c.id = q.challenge_id
		LEFT JOIN submissions s ON q.id = s.question_id AND s.user_id = ? AND s.is_correct = 1
		WHERE c.visible = 1
		GROUP BY c.id
	`

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	completions := make(map[string]*ChallengeCompletion)
	for rows.Next() {
		var c ChallengeCompletion
		if err := rows.Scan(&c.ChallengeID, &c.TotalQuestions, &c.SolvedQuestions); err != nil {
			return nil, err
		}
		c.IsComplete = c.SolvedQuestions > 0 && c.SolvedQuestions == c.TotalQuestions
		completions[c.ChallengeID] = &c
	}
	return completions, nil
}

func (db *DB) GetUserSolvedChallenges(userID string) ([]ChallengeSummary, error) {
	query := `
		SELECT DISTINCT
			c.id,
			c.name,
			c.category,
			c.difficulty,
			COUNT(DISTINCT s.question_id) as solved_questions,
			(SELECT COUNT(*) FROM questions WHERE challenge_id = c.id) as total_questions
		FROM challenges c
		JOIN questions q ON c.id = q.challenge_id
		JOIN submissions s ON q.id = s.question_id
		WHERE s.user_id = ? AND s.is_correct = 1
		GROUP BY c.id, c.name, c.category, c.difficulty
		ORDER BY c.name
	`

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var challenges []ChallengeSummary
	for rows.Next() {
		var ch ChallengeSummary
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Category, &ch.Difficulty,
			&ch.SolvedQuestions, &ch.TotalQuestions); err != nil {
			return nil, err
		}
		challenges = append(challenges, ch)
	}
	return challenges, nil
}

// Password reset queries
func (db *DB) CreatePasswordResetToken(userID, token string, expires time.Time) error {
	query := `UPDATE users
	          SET password_reset_token = ?, password_reset_expires = ?, updated_at = CURRENT_TIMESTAMP
	          WHERE id = ?`
	// Format time as SQLite datetime string in UTC to match CURRENT_TIMESTAMP
	expiresStr := expires.UTC().Format("2006-01-02 15:04:05")
	_, err := db.Exec(query, token, expiresStr, userID)
	return err
}

func (db *DB) GetUserByResetToken(token string) (*models.User, error) {
	query := `SELECT id, email, password_hash, name, avatar_url, team_id, is_admin, created_at, updated_at
	          FROM users
	          WHERE password_reset_token = ? AND password_reset_expires > CURRENT_TIMESTAMP`

	var user models.User
	err := db.QueryRow(query, token).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name,
		&user.AvatarURL, &user.TeamID, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (db *DB) ClearPasswordResetToken(userID string) error {
	query := `UPDATE users
	          SET password_reset_token = NULL, password_reset_expires = NULL, updated_at = CURRENT_TIMESTAMP
	          WHERE id = ?`
	_, err := db.Exec(query, userID)
	return err
}

func (db *DB) UpdatePassword(userID, passwordHash string) error {
	query := `UPDATE users
	          SET password_hash = ?, updated_at = CURRENT_TIMESTAMP
	          WHERE id = ?`
	_, err := db.Exec(query, passwordHash, userID)
	return err
}

// Category queries

func (db *DB) GetAllCategories() ([]models.CategoryOption, error) {
	query := `SELECT id, name, sort_order, created_at FROM categories ORDER BY sort_order, name`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []models.CategoryOption
	for rows.Next() {
		var c models.CategoryOption
		if err := rows.Scan(&c.ID, &c.Name, &c.SortOrder, &c.CreatedAt); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

func (db *DB) CreateCategory(name string, sortOrder int) (*models.CategoryOption, error) {
	id := GenerateID()
	query := `INSERT INTO categories (id, name, sort_order) VALUES (?, ?, ?)
	          RETURNING id, name, sort_order, created_at`
	var c models.CategoryOption
	err := db.QueryRow(query, id, name, sortOrder).Scan(&c.ID, &c.Name, &c.SortOrder, &c.CreatedAt)
	return &c, err
}

func (db *DB) UpdateCategory(id, name string, sortOrder int) (*models.CategoryOption, error) {
	query := `UPDATE categories SET name = ?, sort_order = ? WHERE id = ?
	          RETURNING id, name, sort_order, created_at`
	var c models.CategoryOption
	err := db.QueryRow(query, name, sortOrder, id).Scan(&c.ID, &c.Name, &c.SortOrder, &c.CreatedAt)
	return &c, err
}

func (db *DB) DeleteCategory(id string) error {
	_, err := db.Exec("DELETE FROM categories WHERE id = ?", id)
	return err
}

// Difficulty queries

func (db *DB) GetAllDifficulties() ([]models.DifficultyOption, error) {
	query := `SELECT id, name, color, text_color, sort_order, created_at FROM difficulties ORDER BY sort_order, name`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var difficulties []models.DifficultyOption
	for rows.Next() {
		var d models.DifficultyOption
		if err := rows.Scan(&d.ID, &d.Name, &d.Color, &d.TextColor, &d.SortOrder, &d.CreatedAt); err != nil {
			return nil, err
		}
		difficulties = append(difficulties, d)
	}
	return difficulties, nil
}

func (db *DB) CreateDifficulty(name, color, textColor string, sortOrder int) (*models.DifficultyOption, error) {
	id := GenerateID()
	query := `INSERT INTO difficulties (id, name, color, text_color, sort_order) VALUES (?, ?, ?, ?, ?)
	          RETURNING id, name, color, text_color, sort_order, created_at`
	var d models.DifficultyOption
	err := db.QueryRow(query, id, name, color, textColor, sortOrder).Scan(&d.ID, &d.Name, &d.Color, &d.TextColor, &d.SortOrder, &d.CreatedAt)
	return &d, err
}

func (db *DB) UpdateDifficulty(id, name, color, textColor string, sortOrder int) (*models.DifficultyOption, error) {
	query := `UPDATE difficulties SET name = ?, color = ?, text_color = ?, sort_order = ? WHERE id = ?
	          RETURNING id, name, color, text_color, sort_order, created_at`
	var d models.DifficultyOption
	err := db.QueryRow(query, name, color, textColor, sortOrder, id).Scan(&d.ID, &d.Name, &d.Color, &d.TextColor, &d.SortOrder, &d.CreatedAt)
	return &d, err
}

func (db *DB) DeleteDifficulty(id string) error {
	_, err := db.Exec("DELETE FROM difficulties WHERE id = ?", id)
	return err
}

func (db *DB) GetDifficultyByName(name string) (*models.DifficultyOption, error) {
	query := `SELECT id, name, color, text_color, sort_order, created_at FROM difficulties WHERE name = ?`
	var d models.DifficultyOption
	err := db.QueryRow(query, name).Scan(&d.ID, &d.Name, &d.Color, &d.TextColor, &d.SortOrder, &d.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// Site settings queries

func (db *DB) GetSetting(key string) (string, error) {
	var value string
	query := `SELECT value FROM site_settings WHERE key = ?`
	err := db.QueryRow(query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (db *DB) SetSetting(key, value string) error {
	query := `INSERT INTO site_settings (key, value, updated_at)
	          VALUES (?, ?, CURRENT_TIMESTAMP)
	          ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = CURRENT_TIMESTAMP`
	_, err := db.Exec(query, key, value, value)
	return err
}

func (db *DB) GetCustomCode(page string) (*models.CustomCode, error) {
	headHTML, _ := db.GetSetting("custom_head_html")
	bodyEndHTML, _ := db.GetSetting("custom_body_end_html")
	pagesJSON, _ := db.GetSetting("custom_code_pages")

	var pages []string
	if pagesJSON != "" {
		// Simple JSON array parsing: ["all", "login", ...]
		pagesJSON = strings.Trim(pagesJSON, "[]")
		if pagesJSON != "" {
			for _, p := range strings.Split(pagesJSON, ",") {
				p = strings.Trim(strings.Trim(p, `"`), " ")
				pages = append(pages, p)
			}
		}
	}

	// Check if code should be injected on this page
	inject := false
	for _, p := range pages {
		if p == "all" || p == page {
			inject = true
			break
		}
	}

	if !inject {
		return &models.CustomCode{}, nil
	}

	return &models.CustomCode{
		HeadHTML:    headHTML,
		BodyEndHTML: bodyEndHTML,
	}, nil
}

// GetTeamSolvedChallenges returns challenges that have been solved by team members (all activity)
func (db *DB) GetTeamSolvedChallenges(teamID string) ([]ChallengeSummary, error) {
	query := `
		SELECT 
			c.id,
			c.name,
			c.category,
			c.difficulty,
			COUNT(DISTINCT q.id) as total_questions,
			COUNT(DISTINCT s.question_id) as solved_questions
		FROM challenges c
		JOIN questions q ON c.id = q.challenge_id
		JOIN submissions s ON q.id = s.question_id AND s.is_correct = 1
		JOIN users u ON s.user_id = u.id
		WHERE u.team_id = ? AND c.visible = 1
		GROUP BY c.id, c.name, c.category, c.difficulty
		HAVING solved_questions > 0
		ORDER BY c.name ASC
	`

	rows, err := db.Query(query, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var challenges []ChallengeSummary
	for rows.Next() {
		var c ChallengeSummary
		if err := rows.Scan(&c.ID, &c.Name, &c.Category, &c.Difficulty, &c.TotalQuestions, &c.SolvedQuestions); err != nil {
			return nil, err
		}
		challenges = append(challenges, c)
	}
	return challenges, nil
}

// GetTeamScoringChallenges returns challenges with points earned by the team (only counting first solves)
// Only counts submissions made while the user was in the team (s.team_id is set)
func (db *DB) GetTeamScoringChallenges(teamID string) ([]TeamChallengeSummary, error) {
	query := `
		SELECT 
			c.id,
			c.name,
			c.category,
			c.difficulty,
			COUNT(DISTINCT q.id) as total_questions,
			COUNT(DISTINCT team_solves.question_id) as solved_questions,
			COALESCE(SUM(team_solves.points_earned), 0) as points_earned
		FROM challenges c
		JOIN questions q ON c.id = q.challenge_id
		LEFT JOIN (
			-- Only get the first solve for each question by this team
			-- Only counts submissions made while user was in the team
			SELECT 
				s.question_id,
				q.challenge_id,
				q.points - COALESCE(hint_costs.total_cost, 0) as points_earned
			FROM submissions s
			JOIN questions q ON s.question_id = q.id
			JOIN users u ON s.user_id = u.id
			LEFT JOIN (
				SELECT hu.user_id, h.question_id, SUM(h.cost) as total_cost
				FROM hint_unlocks hu
				JOIN hints h ON hu.hint_id = h.id
				GROUP BY hu.user_id, h.question_id
			) hint_costs ON s.user_id = hint_costs.user_id AND s.question_id = hint_costs.question_id
			WHERE s.is_correct = 1
			AND s.team_id = ?
			AND s.id = (
				-- First submission for this question by this team
				SELECT MIN(s2.id)
				FROM submissions s2
				WHERE s2.question_id = s.question_id 
				AND s2.is_correct = 1
				AND s2.team_id = ?
			)
		) team_solves ON team_solves.challenge_id = c.id AND team_solves.question_id = q.id
		WHERE c.visible = 1
		GROUP BY c.id, c.name, c.category, c.difficulty
		HAVING solved_questions > 0
		ORDER BY c.name ASC
	`

	rows, err := db.Query(query, teamID, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var challenges []TeamChallengeSummary
	for rows.Next() {
		var c TeamChallengeSummary
		if err := rows.Scan(&c.ID, &c.Name, &c.Category, &c.Difficulty, &c.TotalQuestions, &c.SolvedQuestions, &c.PointsEarned); err != nil {
			return nil, err
		}
		challenges = append(challenges, c)
	}
	return challenges, nil
}

// GetTeamRecentSubmissions returns recent submissions by team members (all activity)
func (db *DB) GetTeamRecentSubmissions(teamID string, limit int) ([]TeamSubmission, error) {
	query := `
		SELECT 
			s.id,
			s.question_id,
			q.name as question_name,
			q.points - COALESCE(hint_costs.total_cost, 0) as points,
			c.id as challenge_id,
			c.name as challenge_name,
			s.is_correct,
			s.created_at,
			u.id as user_id,
			u.name as user_name,
			COALESCE(hint_costs.total_cost, 0) as hint_penalty
		FROM submissions s
		JOIN questions q ON s.question_id = q.id
		JOIN challenges c ON q.challenge_id = c.id
		JOIN users u ON s.user_id = u.id
		LEFT JOIN (
			SELECT hu.user_id, h.question_id, SUM(h.cost) as total_cost
			FROM hint_unlocks hu
			JOIN hints h ON hu.hint_id = h.id
			GROUP BY hu.user_id, h.question_id
		) hint_costs ON s.user_id = hint_costs.user_id AND s.question_id = hint_costs.question_id
		WHERE u.team_id = ?
		ORDER BY s.created_at DESC
		LIMIT ?
	`

	rows, err := db.Query(query, teamID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var submissions []TeamSubmission
	for rows.Next() {
		var s TeamSubmission
		if err := rows.Scan(&s.ID, &s.QuestionID, &s.QuestionName, &s.Points, &s.ChallengeID, &s.ChallengeName, &s.IsCorrect, &s.CreatedAt, &s.UserID, &s.UserName, &s.HintPenalty); err != nil {
			return nil, err
		}
		submissions = append(submissions, s)
	}
	return submissions, nil
}

// GetTeamScoringSubmissions returns submissions that count toward team score (first solve per question only)
// Only counts submissions made while the user was in the team (s.team_id is set)
func (db *DB) GetTeamScoringSubmissions(teamID string, limit int) ([]TeamSubmission, error) {
	query := `
		SELECT 
			s.id,
			s.question_id,
			q.name as question_name,
			q.points - COALESCE(hint_costs.total_cost, 0) as points,
			c.id as challenge_id,
			c.name as challenge_name,
			s.is_correct,
			s.created_at,
			u.id as user_id,
			u.name as user_name,
			COALESCE(hint_costs.total_cost, 0) as hint_penalty
		FROM submissions s
		JOIN questions q ON s.question_id = q.id
		JOIN challenges c ON q.challenge_id = c.id
		JOIN users u ON s.user_id = u.id
		LEFT JOIN (
			SELECT hu.user_id, h.question_id, SUM(h.cost) as total_cost
			FROM hint_unlocks hu
			JOIN hints h ON hu.hint_id = h.id
			GROUP BY hu.user_id, h.question_id
		) hint_costs ON s.user_id = hint_costs.user_id AND s.question_id = hint_costs.question_id
		WHERE s.is_correct = 1
		AND s.team_id = ?
		AND s.id = (
			-- First submission for this question by this team
			SELECT MIN(s2.id)
			FROM submissions s2
			WHERE s2.question_id = s.question_id 
			AND s2.is_correct = 1
			AND s2.team_id = ?
		)
		ORDER BY s.created_at DESC
		LIMIT ?
	`

	rows, err := db.Query(query, teamID, teamID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var submissions []TeamSubmission
	for rows.Next() {
		var s TeamSubmission
		if err := rows.Scan(&s.ID, &s.QuestionID, &s.QuestionName, &s.Points, &s.ChallengeID, &s.ChallengeName, &s.IsCorrect, &s.CreatedAt, &s.UserID, &s.UserName, &s.HintPenalty); err != nil {
			return nil, err
		}
		submissions = append(submissions, s)
	}
	return submissions, nil
}


// GetScoreFreeze returns whether the scoreboard is frozen and when.
func (db *DB) GetScoreFreeze() (enabled bool, freezeAt *time.Time, err error) {
	enabledStr, _ := db.GetSetting("freeze_enabled")
	enabled = enabledStr == "1"

	freezeAtStr, _ := db.GetSetting("freeze_at")
	if freezeAtStr != "" {
		t, parseErr := time.Parse(time.RFC3339, freezeAtStr)
		if parseErr == nil {
			freezeAt = &t
		}
	}
	return enabled, freezeAt, nil
}

// SetScoreFreeze saves the freeze state.
func (db *DB) SetScoreFreeze(enabled bool, freezeAt *time.Time) error {
	enabledVal := "0"
	if enabled {
		enabledVal = "1"
	}
	if err := db.SetSetting("freeze_enabled", enabledVal); err != nil {
		return err
	}
	freezeAtVal := ""
	if freezeAt != nil {
		freezeAtVal = freezeAt.UTC().Format(time.RFC3339)
	}
	return db.SetSetting("freeze_at", freezeAtVal)
}

// IsFrozen returns true if the scoreboard is currently frozen.
func (db *DB) IsFrozen() bool {
	enabled, freezeAt, err := db.GetScoreFreeze()
	if err != nil || !enabled {
		return false
	}
	if freezeAt == nil {
		return true // enabled with no time = frozen immediately
	}
	return time.Now().After(*freezeAt)
}

// FreezeTimestamp returns the effective freeze cutoff time, or nil if not frozen.
func (db *DB) FreezeTimestamp() *time.Time {
	if !db.IsFrozen() {
		return nil
	}
	_, freezeAt, _ := db.GetScoreFreeze()
	if freezeAt == nil {
		now := time.Now()
		return &now
	}
	return freezeAt
}

// ExportBundle builds the full export payload.
func (db *DB) ExportBundle() (*models.ExportBundle, error) {
	bundle := &models.ExportBundle{
		Version:    1,
		ExportedAt: time.Now(),
	}

	// Categories
	cats, _ := db.GetAllCategories()
	for _, c := range cats {
		bundle.Categories = append(bundle.Categories, c.Name)
	}

	// Difficulties
	diffs, _ := db.GetAllDifficulties()
	for _, d := range diffs {
		bundle.Difficulties = append(bundle.Difficulties, d.Name)
	}

	// Challenges
	challenges, err := db.GetChallenges(false) // include hidden
	if err != nil {
		return nil, err
	}

	for _, c := range challenges {
		ec := models.ExportChallenge{
			Name:           c.Name,
			Description:    c.Description,
			Category:       c.Category,
			Difficulty:     c.Difficulty,
			Visible:        c.Visible,
			DynamicScoring: c.DynamicScoring,
			InitialPoints:  c.InitialPoints,
			MinimumPoints:  c.MinimumPoints,
			DecayThreshold: c.DecayThreshold,
		}
		if c.FileURL != nil {
			ec.FileURL = *c.FileURL
		}

		questions, err := db.GetQuestionsByChallengeID(c.ID)
		if err != nil {
			continue
		}
		for _, q := range questions {
			eq := models.ExportQuestion{
				Name:          q.Name,
				Description:   q.Description,
				Flag:          q.Flag,
				CaseSensitive: q.CaseSensitive,
				Points:        q.Points,
			}
			if q.FlagMask != nil {
				eq.FlagMask = *q.FlagMask
			}
			if q.FileURL != nil {
				eq.FileURL = *q.FileURL
			}

			hints, _ := db.GetHintsByQuestionID(q.ID)
			for _, h := range hints {
				eq.Hints = append(eq.Hints, models.ExportHint{
					Content: h.Content,
					Cost:    h.Cost,
					Order:   h.Order,
				})
			}
			ec.Questions = append(ec.Questions, eq)
		}
		bundle.Challenges = append(bundle.Challenges, ec)
	}

	return bundle, nil
}

// ImportBundle imports challenges from an export bundle.
func (db *DB) ImportBundle(categories, difficulties []string, challenges []models.ExportChallenge) (*models.ImportResult, error) {
	result := &models.ImportResult{}

	// Ensure categories exist
	for _, cat := range categories {
		db.Exec(`INSERT OR IGNORE INTO categories (id, name, sort_order, created_at) VALUES (?, ?, 0, CURRENT_TIMESTAMP)`, GenerateID(), cat)
	}
	for _, diff := range difficulties {
		db.Exec(`INSERT OR IGNORE INTO difficulties (id, name, color, text_color, sort_order, created_at) VALUES (?, ?, 'bg-gray-600', 'text-white', 0, CURRENT_TIMESTAMP)`, GenerateID(), diff)
	}

	for _, ec := range challenges {
		// Handle duplicate names
		name := ec.Name
		for i := 2; ; i++ {
			var count int
			db.QueryRow(`SELECT COUNT(*) FROM challenges WHERE name = ?`, name).Scan(&count)
			if count == 0 {
				break
			}
			if i == 2 && name == ec.Name {
				result.Renamed = append(result.Renamed, fmt.Sprintf("%s → %s (%d)", ec.Name, ec.Name, i))
			}
			name = fmt.Sprintf("%s (%d)", ec.Name, i)
		}

		cID := GenerateID()
		var fileURL interface{}
		if ec.FileURL != "" {
			fileURL = ec.FileURL
		}
		_, err := db.Exec(`
			INSERT INTO challenges (id, name, description, category, difficulty, visible,
				dynamic_scoring, initial_points, minimum_points, decay_threshold, file_url, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			cID, name, ec.Description, ec.Category, ec.Difficulty, ec.Visible,
			ec.DynamicScoring, ec.InitialPoints, ec.MinimumPoints, ec.DecayThreshold, fileURL)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to import %q: %v", ec.Name, err))
			continue
		}

		for _, eq := range ec.Questions {
			qID := GenerateID()
			var qFileURL interface{}
			if eq.FileURL != "" {
				qFileURL = eq.FileURL
			}
			db.Exec(`
				INSERT INTO questions (id, challenge_id, name, description, flag, flag_mask, case_sensitive, points, file_url, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
				qID, cID, eq.Name, eq.Description, eq.Flag, eq.FlagMask, eq.CaseSensitive, eq.Points, qFileURL)

			for _, eh := range eq.Hints {
				db.Exec(`
					INSERT INTO hints (id, question_id, content, cost, "order", created_at)
					VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
					GenerateID(), qID, eh.Content, eh.Cost, eh.Order)
			}
		}
		result.Imported++
	}

	return result, nil
}

// Challenge Files queries

// CreateChallengeFile creates a new file record for a challenge
func (db *DB) CreateChallengeFile(challengeID, filename, storageType, storagePath string, sizeBytes *int64) (*models.ChallengeFile, error) {
	id := GenerateID()
	query := `INSERT INTO challenge_files (id, challenge_id, filename, storage_type, storage_path, size_bytes)
	          VALUES (?, ?, ?, ?, ?, ?)
	          RETURNING id, challenge_id, filename, storage_type, storage_path, size_bytes, created_at`

	var f models.ChallengeFile
	err := db.QueryRow(query, id, challengeID, filename, storageType, storagePath, sizeBytes).Scan(
		&f.ID, &f.ChallengeID, &f.Filename, &f.StorageType, &f.StoragePath, &f.SizeBytes, &f.CreatedAt,
	)
	return &f, err
}

// GetChallengeFiles returns all files for a challenge
func (db *DB) GetChallengeFiles(challengeID string) ([]models.ChallengeFile, error) {
	query := `SELECT id, challenge_id, filename, storage_type, storage_path, size_bytes, created_at
	          FROM challenge_files WHERE challenge_id = ? ORDER BY created_at`

	rows, err := db.Query(query, challengeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []models.ChallengeFile
	for rows.Next() {
		var f models.ChallengeFile
		if err := rows.Scan(&f.ID, &f.ChallengeID, &f.Filename, &f.StorageType, &f.StoragePath, &f.SizeBytes, &f.CreatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

// GetChallengeFileByID returns a single file by ID
func (db *DB) GetChallengeFileByID(fileID string) (*models.ChallengeFile, error) {
	query := `SELECT id, challenge_id, filename, storage_type, storage_path, size_bytes, created_at
	          FROM challenge_files WHERE id = ?`

	var f models.ChallengeFile
	err := db.QueryRow(query, fileID).Scan(
		&f.ID, &f.ChallengeID, &f.Filename, &f.StorageType, &f.StoragePath, &f.SizeBytes, &f.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// DeleteChallengeFile deletes a file record
func (db *DB) DeleteChallengeFile(fileID string) error {
	_, err := db.Exec("DELETE FROM challenge_files WHERE id = ?", fileID)
	return err
}

// DeleteAllChallengeFiles deletes all files for a challenge
func (db *DB) DeleteAllChallengeFiles(challengeID string) error {
	_, err := db.Exec("DELETE FROM challenge_files WHERE challenge_id = ?", challengeID)
	return err
}
