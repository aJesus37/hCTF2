package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	ServerURL  string
	Token      string
	httpClient *http.Client
}

func New(serverURL, token string) *Client {
	return &Client{
		ServerURL:  serverURL,
		Token:      token,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Do executes a request, injecting the auth cookie if a token is set.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c.Token != "" {
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: c.Token})
	}
	return c.httpClient.Do(req)
}

// decodeJSON decodes a JSON response body into v.
func decodeJSON(resp *http.Response, v any) error {
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("admin privileges required")
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("not authenticated — run 'hctf2 login'")
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		var e struct{ Error string `json:"error"` }
		if json.Unmarshal(b, &e) == nil && e.Error != "" {
			return fmt.Errorf("server error: %s", e.Error)
		}
		if msg := strings.TrimSpace(string(b)); msg != "" {
			return fmt.Errorf("server error: %s", msg)
		}
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func jsonBody(data []byte) *bytes.Reader {
	return bytes.NewReader(data)
}
