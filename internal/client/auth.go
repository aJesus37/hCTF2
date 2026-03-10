package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type LoginResponse struct {
	Token   string `json:"token"`
	UserID  string `json:"user_id"`
	IsAdmin bool   `json:"is_admin"`
}

func (c *Client) Login(email, password string) (*LoginResponse, error) {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req, _ := http.NewRequest("POST", c.ServerURL+"/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	var lr LoginResponse
	if err := decodeJSON(resp, &lr); err != nil {
		return nil, err
	}
	return &lr, nil
}
