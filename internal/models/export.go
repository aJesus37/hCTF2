package models

import "time"

type ExportHint struct {
	Content string `json:"content"`
	Cost    int    `json:"cost"`
	Order   int    `json:"order"`
}

type ExportQuestion struct {
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	Flag          string       `json:"flag"`
	FlagMask      string       `json:"flag_mask,omitempty"`
	CaseSensitive bool         `json:"case_sensitive"`
	Points        int          `json:"points"`
	FileURL       string       `json:"file_url,omitempty"`
	Hints         []ExportHint `json:"hints,omitempty"`
}

type ExportChallenge struct {
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	Category       string           `json:"category"`
	Difficulty     string           `json:"difficulty"`
	Visible        bool             `json:"visible"`
	DynamicScoring bool             `json:"dynamic_scoring"`
	InitialPoints  int              `json:"initial_points"`
	MinimumPoints  int              `json:"minimum_points"`
	DecayThreshold int              `json:"decay_threshold"`
	FileURL        string           `json:"file_url,omitempty"`
	Questions      []ExportQuestion `json:"questions"`
}

type ExportBundle struct {
	Version      int               `json:"version"`
	ExportedAt   time.Time         `json:"exported_at"`
	Categories   []string          `json:"categories"`
	Difficulties []string          `json:"difficulties"`
	Challenges   []ExportChallenge `json:"challenges"`
}

type ImportResult struct {
	Imported int      `json:"imported"`
	Renamed  []string `json:"renamed"`
	Errors   []string `json:"errors"`
}

// ExportCategory preserves name and sort order.
type ExportCategory struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

// ExportDifficulty preserves name and sort order.
type ExportDifficulty struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

// ExportCompetition captures competition structure (no scores/registrations).
type ExportCompetition struct {
	Name              string     `json:"name"`
	Description       string     `json:"description"`
	RulesHTML         string     `json:"rules_html,omitempty"`
	StartAt           *time.Time `json:"start_at,omitempty"`
	EndAt             *time.Time `json:"end_at,omitempty"`
	RegistrationStart *time.Time `json:"registration_start,omitempty"`
	RegistrationEnd   *time.Time `json:"registration_end,omitempty"`
	FreezeAt          *time.Time `json:"freeze_at,omitempty"`
	// ChallengeNames lists the names of challenges linked to this competition.
	// On import, challenges are resolved by name after challenge import completes.
	ChallengeNames []string `json:"challenge_names,omitempty"`
}

// ConfigBundle is the full platform config backup (version 2).
// It is a superset of ExportBundle, adding competitions and site settings.
type ConfigBundle struct {
	Version      int                 `json:"version"`     // always 2
	ExportedAt   time.Time           `json:"exported_at"`
	Categories   []ExportCategory    `json:"categories"`
	Difficulties []ExportDifficulty  `json:"difficulties"`
	Challenges   []ExportChallenge   `json:"challenges"`
	Competitions []ExportCompetition `json:"competitions"`
	// SiteSettings holds customization keys only (custom_head_html,
	// custom_body_end_html, custom_code_pages, motd).
	SiteSettings map[string]string `json:"site_settings"`
}

// ConfigImportResult summarises what was created/skipped during config import.
type ConfigImportResult struct {
	ChallengesImported  int      `json:"challenges_imported"`
	CompetitionsCreated int      `json:"competitions_created"`
	Renamed             []string `json:"renamed,omitempty"`
	Errors              []string `json:"errors,omitempty"`
}
