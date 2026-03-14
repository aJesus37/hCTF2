package client

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Competition struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Status             string `json:"status"`
	Description        string `json:"description"`
	ScoreboardFrozen   bool   `json:"scoreboard_frozen"`
	ScoreboardBlackout bool   `json:"scoreboard_blackout"`
	StartAt            string `json:"start_at,omitempty"`
	EndAt              string `json:"end_at,omitempty"`
}

func (c *Client) ListCompetitions() ([]Competition, error) {
	req, _ := http.NewRequest("GET", c.ServerURL+"/api/competitions", nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out []Competition
	return out, decodeJSON(resp, &out)
}

func (c *Client) CreateCompetition(name, description string) (*Competition, error) {
	body := strings.NewReader(url.Values{"name": {name}, "description": {description}}.Encode())
	req, _ := http.NewRequest("POST", c.ServerURL+"/api/admin/competitions", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out Competition
	return &out, decodeJSON(resp, &out)
}

func (c *Client) ForceStartCompetition(id int64) error {
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/admin/competitions/%d/force-start", c.ServerURL, id), nil)
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

func (c *Client) ForceEndCompetition(id int64) error {
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/admin/competitions/%d/force-end", c.ServerURL, id), nil)
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

func (c *Client) RegisterForCompetition(id int64) error {
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/competitions/%d/register", c.ServerURL, id), nil)
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

func (c *Client) GetCompetition(id int64) (*Competition, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/competitions/%d", c.ServerURL, id), nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out Competition
	return &out, decodeJSON(resp, &out)
}

func (c *Client) AddChallengeToCompetition(compID int64, challengeID string) error {
	body := strings.NewReader(url.Values{"challenge_id": {challengeID}}.Encode())
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/admin/competitions/%d/challenges", c.ServerURL, compID), body)
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

func (c *Client) RemoveChallengeFromCompetition(compID int64, challengeID string) error {
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/admin/competitions/%d/challenges/%s", c.ServerURL, compID, challengeID), nil)
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

func (c *Client) SetCompetitionFreeze(id int64, frozen bool) error {
	val := "0"
	if frozen {
		val = "1"
	}
	body := strings.NewReader(url.Values{"frozen": {val}}.Encode())
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/admin/competitions/%d/freeze", c.ServerURL, id), body)
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

func (c *Client) SetCompetitionBlackout(id int64, blackout bool) error {
	val := "0"
	if blackout {
		val = "1"
	}
	body := strings.NewReader(url.Values{"blackout": {val}}.Encode())
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/admin/competitions/%d/blackout", c.ServerURL, id), body)
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

func (c *Client) DeleteCompetition(id int64) error {
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/admin/competitions/%d", c.ServerURL, id), nil)
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

func (c *Client) UpdateCompetition(id int64, name, description string) (*Competition, error) {
	body := strings.NewReader(url.Values{"name": {name}, "description": {description}}.Encode())
	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/admin/competitions/%d", c.ServerURL, id), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out Competition
	return &out, decodeJSON(resp, &out)
}

type CompetitionScoreboardEntry struct {
	Rank       int    `json:"rank"`
	TeamID     string `json:"team_id"`
	TeamName   string `json:"team_name"`
	Score      int    `json:"score"`
	SolveCount int    `json:"solve_count"`
}

func (c *Client) GetCompetitionScoreboard(id int64) ([]CompetitionScoreboardEntry, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/competitions/%d/scoreboard", c.ServerURL, id), nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out []CompetitionScoreboardEntry
	return out, decodeJSON(resp, &out)
}

type Submission struct {
	ChallengeID   string `json:"challenge_id"`
	QuestionID    string `json:"question_id"`
	TeamName      string `json:"team_name"`
	UserName      string `json:"user_name"`
	ChallengeName string `json:"challenge_name"`
	QuestionName  string `json:"question_name"`
	IsCorrect     bool   `json:"is_correct"`
	SubmittedFlag string `json:"submitted_flag,omitempty"`
	SubmittedAt   string `json:"submitted_at"`
}

// GetSubmissions returns the submission feed. competitionID=0 fetches the global feed.
func (c *Client) GetSubmissions(competitionID int64) ([]Submission, error) {
	var url string
	if competitionID == 0 {
		url = c.ServerURL + "/api/competitions/submissions"
	} else {
		url = fmt.Sprintf("%s/api/competitions/%d/submissions", c.ServerURL, competitionID)
	}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out []Submission
	return out, decodeJSON(resp, &out)
}

type UserProfile struct {
	UserID           string  `json:"user_id"`
	Name             string  `json:"name"`
	Email            string  `json:"email,omitempty"`
	TeamID           *string `json:"team_id,omitempty"`
	TeamName         *string `json:"team_name,omitempty"`
	Rank             int     `json:"rank"`
	TotalPoints      int     `json:"total_points"`
	SolvedCount      int     `json:"solved_count"`
	TotalSubmissions int     `json:"total_submissions"`
}

// GetUserProfile returns stats for a user. Pass "" to get own profile.
func (c *Client) GetUserProfile(userID string) (*UserProfile, error) {
	id := userID
	if id == "" {
		id = "me"
	}
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/users/%s/profile", c.ServerURL, id), nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out UserProfile
	return &out, decodeJSON(resp, &out)
}

type CompetitionTeam struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *Client) ListCompetitionTeams(id int64) ([]CompetitionTeam, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/admin/competitions/%d/teams", c.ServerURL, id), nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out []CompetitionTeam
	return out, decodeJSON(resp, &out)
}
