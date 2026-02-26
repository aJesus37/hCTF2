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
