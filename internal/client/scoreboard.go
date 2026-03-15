package client

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
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

func (c *Client) SetScoreboardFreeze(frozen bool) error {
	val := "0"
	if frozen {
		val = "1"
	}
	body := strings.NewReader(url.Values{"freeze_enabled": {val}}.Encode())
	req, _ := http.NewRequest("POST", c.ServerURL+"/api/admin/settings/freeze", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
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
