package models

import "time"

type User struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	Name         string     `json:"name"`
	AvatarURL    *string    `json:"avatar_url,omitempty"`
	TeamID       *string    `json:"team_id,omitempty"`
	IsAdmin      bool       `json:"is_admin"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type Team struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Description        *string   `json:"description,omitempty"`
	OwnerID            string    `json:"owner_id"`
	InviteID           string    `json:"invite_id"`
	InvitePermission   string    `json:"invite_permission"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type Challenge struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Category       string    `json:"category"`
	Difficulty     string    `json:"difficulty"`
	Tags           *string   `json:"tags,omitempty"`
	Visible        bool      `json:"visible"`
	SQLEnabled     bool      `json:"sql_enabled"`
	SQLDatasetURL  *string   `json:"sql_dataset_url,omitempty"`
	SQLSchemaHint  *string   `json:"sql_schema_hint,omitempty"`
	DynamicScoring bool      `json:"dynamic_scoring"`
	InitialPoints  int       `json:"initial_points"`
	MinimumPoints  int       `json:"minimum_points"`
	DecayThreshold int       `json:"decay_threshold"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Question struct {
	ID            string    `json:"id"`
	ChallengeID   string    `json:"challenge_id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Flag          string    `json:"-"`
	FlagMask      *string   `json:"flag_mask,omitempty"`
	CaseSensitive bool      `json:"case_sensitive"`
	Points        int       `json:"points"`
	FileURL       *string   `json:"file_url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Hint struct {
	ID         string    `json:"id"`
	QuestionID string    `json:"question_id"`
	Content    string    `json:"content"`
	Cost       int       `json:"cost"`
	Order      int       `json:"order"`
	CreatedAt  time.Time `json:"created_at"`
}

type Submission struct {
	ID            string    `json:"id"`
	QuestionID    string    `json:"question_id"`
	UserID        string    `json:"user_id"`
	TeamID        *string   `json:"team_id,omitempty"`
	SubmittedFlag string    `json:"submitted_flag"`
	IsCorrect     bool      `json:"is_correct"`
	CreatedAt     time.Time `json:"created_at"`
}

type HintUnlock struct {
	ID        string    `json:"id"`
	HintID    string    `json:"hint_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type ScoreboardEntry struct {
	Rank       int       `json:"rank"`
	UserID     string    `json:"user_id"`
	UserName   string    `json:"user_name"`
	TeamID     *string   `json:"team_id,omitempty"`
	TeamName   *string   `json:"team_name,omitempty"`
	Points     int       `json:"points"`
	SolveCount int       `json:"solve_count"`
	LastSolve  time.Time `json:"last_solve"`
}

// CategoryOption represents a configurable challenge category
type CategoryOption struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

// DifficultyOption represents a configurable difficulty level
type DifficultyOption struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	TextColor string    `json:"text_color"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

// QuestionWithChallenge is used in admin forms to display challenge name with question
type QuestionWithChallenge struct {
	ID            string    `json:"id"`
	ChallengeID   string    `json:"challenge_id"`
	ChallengeName string    `json:"challenge_name"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Flag          string    `json:"-"`
	FlagMask      *string   `json:"flag_mask,omitempty"`
	CaseSensitive bool      `json:"case_sensitive"`
	Points        int       `json:"points"`
	FileURL       *string   `json:"file_url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SiteSetting represents a key-value configuration setting
type SiteSetting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CustomCode holds custom HTML/JS code to inject into pages
type CustomCode struct {
	HeadHTML    string `json:"head_html"`
	BodyEndHTML string `json:"body_end_html"`
}
