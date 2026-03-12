package client

import (
	"net/http"
	"time"
)

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

func (c *Client) GetScoreboard() ([]ScoreboardEntry, error) {
	req, _ := http.NewRequest("GET", c.ServerURL+"/api/scoreboard", nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out []ScoreboardEntry
	return out, decodeJSON(resp, &out)
}
