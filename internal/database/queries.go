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
	query := `INSERT INTO users (email, password_hash, name, is_admin)
	          VALUES (?, ?, ?, ?) RETURNING id, email, name, is_admin, created_at, updated_at`

	var user models.User
	err := db.QueryRow(query, email, passwordHash, name, isAdmin).Scan(
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

// Challenge queries
func (db *DB) CreateChallenge(name, description, category, difficulty string, tags *string, visible bool, sqlEnabled bool, sqlDatasetURL, sqlSchemaHint *string) (*models.Challenge, error) {
	query := `INSERT INTO challenges (name, description, category, difficulty, tags, visible, sql_enabled, sql_dataset_url, sql_schema_hint)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	          RETURNING id, name, description, category, difficulty, tags, visible, sql_enabled, sql_dataset_url, sql_schema_hint, created_at, updated_at`

	var c models.Challenge
	err := db.QueryRow(query, name, description, category, difficulty, tags, visible, sqlEnabled, sqlDatasetURL, sqlSchemaHint).Scan(
		&c.ID, &c.Name, &c.Description, &c.Category, &c.Difficulty, &c.Tags, &c.Visible, &c.SQLEnabled, &c.SQLDatasetURL, &c.SQLSchemaHint, &c.CreatedAt, &c.UpdatedAt,
	)
	return &c, err
}

func (db *DB) GetChallenges(visibleOnly bool) ([]models.Challenge, error) {
	query := `SELECT id, name, description, category, difficulty, tags, visible, sql_enabled, sql_dataset_url, sql_schema_hint, created_at, updated_at
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
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Category, &c.Difficulty, &c.Tags, &c.Visible, &c.SQLEnabled, &c.SQLDatasetURL, &c.SQLSchemaHint, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		challenges = append(challenges, c)
	}
	return challenges, nil
}

func (db *DB) GetChallengeByID(id string) (*models.Challenge, error) {
	query := `SELECT id, name, description, category, difficulty, tags, visible, sql_enabled, sql_dataset_url, sql_schema_hint, created_at, updated_at
	          FROM challenges WHERE id = ?`

	var c models.Challenge
	err := db.QueryRow(query, id).Scan(
		&c.ID, &c.Name, &c.Description, &c.Category, &c.Difficulty, &c.Tags, &c.Visible, &c.SQLEnabled, &c.SQLDatasetURL, &c.SQLSchemaHint, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (db *DB) UpdateChallenge(id, name, description, category, difficulty string, tags *string, visible bool, sqlEnabled bool, sqlDatasetURL, sqlSchemaHint *string) error {
	query := `UPDATE challenges
	          SET name = ?, description = ?, category = ?, difficulty = ?, tags = ?, visible = ?, sql_enabled = ?, sql_dataset_url = ?, sql_schema_hint = ?, updated_at = CURRENT_TIMESTAMP
	          WHERE id = ?`
	_, err := db.Exec(query, name, description, category, difficulty, tags, visible, sqlEnabled, sqlDatasetURL, sqlSchemaHint, id)
	return err
}

func (db *DB) DeleteChallenge(id string) error {
	_, err := db.Exec("DELETE FROM challenges WHERE id = ?", id)
	return err
}

// Question queries
func (db *DB) CreateQuestion(challengeID, name, description, flag string, flagMask *string, caseSensitive bool, points int, fileURL *string) (*models.Question, error) {
	// Auto-generate flag mask if not provided
	if flagMask == nil || *flagMask == "" {
		mask := generateFlagMask(flag)
		flagMask = &mask
	}

	query := `INSERT INTO questions (challenge_id, name, description, flag, flag_mask, case_sensitive, points, file_url)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	          RETURNING id, challenge_id, name, description, flag, flag_mask, case_sensitive, points, file_url, created_at, updated_at`

	var q models.Question
	err := db.QueryRow(query, challengeID, name, description, flag, flagMask, caseSensitive, points, fileURL).Scan(
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
	query := `INSERT INTO submissions (question_id, user_id, team_id, submitted_flag, is_correct)
	          VALUES (?, ?, ?, ?, ?)`
	_, err := db.Exec(query, questionID, userID, teamID, submittedFlag, isCorrect)
	return err
}

func (db *DB) HasUserSolved(questionID, userID string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM submissions WHERE question_id = ? AND user_id = ? AND is_correct = 1", questionID, userID).Scan(&count)
	return count > 0, err
}

func (db *DB) GetScoreboard(limit int) ([]models.ScoreboardEntry, error) {
	// SQLite doesn't support ROW_NUMBER() in the same way, so we calculate rank in Go
	query := `
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
		LEFT JOIN submissions s ON u.id = s.user_id AND s.is_correct = 1
		LEFT JOIN questions q ON s.question_id = q.id
		LEFT JOIN (
			SELECT hu.user_id, SUM(h.cost) as total_cost
			FROM hint_unlocks hu
			JOIN hints h ON hu.hint_id = h.id
			GROUP BY hu.user_id
		) hint_costs ON u.id = hint_costs.user_id
		GROUP BY u.id, u.name, u.team_id, t.name, hint_costs.total_cost
		ORDER BY points DESC, last_solve ASC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.ScoreboardEntry
	rank := 1
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
		e.Rank = rank
		rank++
		entries = append(entries, e)
	}
	return entries, nil
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
	query := `INSERT INTO teams (name, description, owner_id)
	          VALUES (?, ?, ?)
	          RETURNING id, name, description, owner_id, invite_id, invite_permission, created_at, updated_at`

	var t models.Team
	err := db.QueryRow(query, name, description, ownerID).Scan(
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
func (db *DB) GetTeamScoreboard(limit int) ([]models.ScoreboardEntry, error) {
	query := `
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
				SUM(q.points) as points,
				COUNT(*) as solve_count,
				MAX(s.created_at) as last_solve
			FROM (
				SELECT 
					s.team_id,
					s.question_id, 
					MIN(s.created_at) as created_at
				FROM submissions s
				WHERE s.is_correct = 1
					AND s.team_id IS NOT NULL
				GROUP BY s.team_id, s.question_id
			) s
			JOIN questions q ON q.id = s.question_id
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
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.ScoreboardEntry
	rank := 1
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
		e.Rank = rank
		rank++
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
	query := `INSERT INTO hints (question_id, content, cost, "order")
	          VALUES (?, ?, ?, ?)
	          RETURNING id, question_id, content, cost, "order", created_at`

	var h models.Hint
	err := db.QueryRow(query, questionID, content, cost, order).Scan(
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
	query := `INSERT INTO hint_unlocks (hint_id, user_id, team_id) VALUES (?, ?, ?)
	          ON CONFLICT(hint_id, user_id) DO NOTHING`
	_, err := db.Exec(query, hintID, userID, teamID)
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
	_, err := db.Exec(query, token, expires, userID)
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
	query := `INSERT INTO categories (name, sort_order) VALUES (?, ?)
	          RETURNING id, name, sort_order, created_at`
	var c models.CategoryOption
	err := db.QueryRow(query, name, sortOrder).Scan(&c.ID, &c.Name, &c.SortOrder, &c.CreatedAt)
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
	query := `INSERT INTO difficulties (name, color, text_color, sort_order) VALUES (?, ?, ?, ?)
	          RETURNING id, name, color, text_color, sort_order, created_at`
	var d models.DifficultyOption
	err := db.QueryRow(query, name, color, textColor, sortOrder).Scan(&d.ID, &d.Name, &d.Color, &d.TextColor, &d.SortOrder, &d.CreatedAt)
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

