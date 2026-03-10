package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Competition struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
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

func (c *Client) CreateCompetition(name string) (*Competition, error) {
	body, _ := json.Marshal(map[string]string{"name": name})
	req, _ := http.NewRequest("POST", c.ServerURL+"/api/admin/competitions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out Competition
	return &out, decodeJSON(resp, &out)
}

func (c *Client) ForceStartCompetition(id string) error {
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/admin/competitions/%s/force-start", c.ServerURL, id), nil)
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

func (c *Client) ForceEndCompetition(id string) error {
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/admin/competitions/%s/force-end", c.ServerURL, id), nil)
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
