package client

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Competition struct {
	ID     int64  `json:"id"`
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
	body := strings.NewReader(url.Values{"name": {name}}.Encode())
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
