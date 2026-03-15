package client

import (
	"encoding/json"
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
	return c.doStatus("PUT", "/api/admin/users/"+id+"/admin", jsonBody(body))
}

func (c *Client) DeleteUser(id string) error {
	return c.doNoBody("DELETE", "/api/admin/users/"+id)
}
