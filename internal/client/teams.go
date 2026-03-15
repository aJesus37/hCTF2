package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Team struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	InviteID string `json:"invite_id,omitempty"`
	OwnerID  string `json:"owner_id,omitempty"`
}

type Member struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
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

func (c *Client) GetTeam(id string) (*Team, []Member, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/teams/%s", c.ServerURL, id), nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, nil, err
	}
	// The endpoint wraps the response: {"team": {...}, "members": [...]}.
	var envelope struct {
		Team    Team     `json:"team"`
		Members []Member `json:"members"`
	}
	if err := decodeJSON(resp, &envelope); err != nil {
		return nil, nil, err
	}
	return &envelope.Team, envelope.Members, nil
}

func (c *Client) TransferOwnership(newOwnerID string) error {
	body, _ := json.Marshal(map[string]string{"new_owner_id": newOwnerID})
	return c.doStatus("POST", "/api/teams/transfer-ownership", bytes.NewReader(body))
}

func (c *Client) RegenerateInvite() (string, error) {
	req, _ := http.NewRequest("POST", c.ServerURL+"/api/teams/regenerate-invite", nil)
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	var out struct {
		InviteID string `json:"invite_id"`
	}
	if err := decodeJSON(resp, &out); err != nil {
		return "", err
	}
	return out.InviteID, nil
}

func (c *Client) CreateTeam(name string) (*Team, error) {
	body, _ := json.Marshal(map[string]string{"name": name})
	var out Team
	return &out, c.doJSON("POST", "/api/teams", bytes.NewReader(body), &out)
}

func (c *Client) JoinTeam(inviteCode string) error {
	return c.doNoBody("POST", "/api/teams/join/"+inviteCode)
}

func (c *Client) LeaveTeam() error {
	return c.doNoBody("POST", "/api/teams/leave")
}

func (c *Client) DisbandTeam() error {
	return c.doNoBody("POST", "/api/teams/disband")
}
