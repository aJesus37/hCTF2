package database

import (
	"database/sql"
	"strings"
	"time"

	"github.com/yourusername/hctf2/internal/models"
)

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
func (db *DB) CreateChallenge(name, description, category, difficulty string, tags *string, visible bool) (*models.Challenge, error) {
	query := `INSERT INTO challenges (name, description, category, difficulty, tags, visible)
	          VALUES (?, ?, ?, ?, ?, ?)
	          RETURNING id, name, description, category, difficulty, tags, visible, created_at, updated_at`

	var c models.Challenge
	err := db.QueryRow(query, name, description, category, difficulty, tags, visible).Scan(
		&c.ID, &c.Name, &c.Description, &c.Category, &c.Difficulty, &c.Tags, &c.Visible, &c.CreatedAt, &c.UpdatedAt,
	)
	return &c, err
}

func (db *DB) GetChallenges(visibleOnly bool) ([]models.Challenge, error) {
	query := `SELECT id, name, description, category, difficulty, tags, visible, created_at, updated_at
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
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Category, &c.Difficulty, &c.Tags, &c.Visible, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		challenges = append(challenges, c)
	}
	return challenges, nil
}

func (db *DB) GetChallengeByID(id string) (*models.Challenge, error) {
	query := `SELECT id, name, description, category, difficulty, tags, visible, created_at, updated_at
	          FROM challenges WHERE id = ?`

	var c models.Challenge
	err := db.QueryRow(query, id).Scan(
		&c.ID, &c.Name, &c.Description, &c.Category, &c.Difficulty, &c.Tags, &c.Visible, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (db *DB) UpdateChallenge(id, name, description, category, difficulty string, tags *string, visible bool) error {
	query := `UPDATE challenges
	          SET name = ?, description = ?, category = ?, difficulty = ?, tags = ?, visible = ?, updated_at = CURRENT_TIMESTAMP
	          WHERE id = ?`
	_, err := db.Exec(query, name, description, category, difficulty, tags, visible, id)
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
			COALESCE(SUM(q.points), 0) as points,
			COUNT(DISTINCT s.question_id) as solve_count,
			COALESCE(MAX(s.created_at), u.created_at) as last_solve
		FROM users u
		LEFT JOIN teams t ON u.team_id = t.id
		LEFT JOIN submissions s ON u.id = s.user_id AND s.is_correct = 1
		LEFT JOIN questions q ON s.question_id = q.id
		GROUP BY u.id, u.name, u.team_id, t.name
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

	return snapshot, nil
}
