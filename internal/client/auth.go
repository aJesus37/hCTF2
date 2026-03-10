package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type LoginResponse struct {
	Token   string `json:"token"`
	UserID  string `json:"user_id"`
	IsAdmin bool   `json:"is_admin"`
}

func (c *Client) Login(email, password string) (*LoginResponse, error) {
	form := url.Values{}
	form.Set("email", email)
	form.Set("password", password)

	// Use a transport that does NOT follow redirects so we can read the cookie.
	noRedirect := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: c.httpClient.Timeout,
	}

	req, _ := http.NewRequest("POST", c.ServerURL+"/api/auth/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := noRedirect.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid credentials")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	// On HTMX form submission the server sets a cookie and returns an empty body
	// with HX-Redirect. Try to extract the token from the Set-Cookie header first.
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "auth_token" && cookie.Value != "" {
			return &LoginResponse{Token: cookie.Value}, nil
		}
	}

	// Fallback: try JSON body (future API path)
	var raw struct {
		Token string `json:"token"`
		User  struct {
			ID      string `json:"id"`
			IsAdmin bool   `json:"is_admin"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err == nil && raw.Token != "" {
		return &LoginResponse{
			Token:   raw.Token,
			UserID:  raw.User.ID,
			IsAdmin: raw.User.IsAdmin,
		}, nil
	}

	return nil, fmt.Errorf("no token received from server")
}
