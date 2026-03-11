package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Team struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	InviteID   string `json:"invite_id,omitempty"`
	OwnerID    string `json:"owner_id,omitempty"`
}

func (c *Client) ListTeams() ([]Team, error) {
	req, _ := http.NewRequest("GET", c.ServerURL+"/api/teams", nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out []Team
	return out, decodeJSON(resp, &out)
}

func (c *Client) GetTeam(id string) (*Team, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/teams/%s", c.ServerURL, id), nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	// The endpoint wraps the response: {"team": {...}, "members": [...]}.
	var envelope struct {
		Team Team `json:"team"`
	}
	if err := decodeJSON(resp, &envelope); err != nil {
		return nil, err
	}
	return &envelope.Team, nil
}

func (c *Client) CreateTeam(name string) (*Team, error) {
	body, _ := json.Marshal(map[string]string{"name": name})
	req, _ := http.NewRequest("POST", c.ServerURL+"/api/teams", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out Team
	return &out, decodeJSON(resp, &out)
}

func (c *Client) JoinTeam(inviteCode string) error {
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/teams/join/%s", c.ServerURL, inviteCode), nil)
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
