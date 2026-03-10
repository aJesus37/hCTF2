package client

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type User struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"is_admin"`
}

func (c *Client) ListUsers() ([]User, error) {
	req, _ := http.NewRequest("GET", c.ServerURL+"/api/admin/users", nil)
	req.Header.Set("Accept", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out []User
	return out, decodeJSON(resp, &out)
}

func (c *Client) PromoteUser(id string, admin bool) error {
	body, _ := json.Marshal(map[string]bool{"is_admin": admin})
	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/admin/users/%s/admin", c.ServerURL, id), jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
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

func (c *Client) DeleteUser(id string) error {
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/admin/users/%s", c.ServerURL, id), nil)
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
