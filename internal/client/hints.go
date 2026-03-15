package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Hint represents a hint for a question as returned by the JSON API.
type Hint struct {
	ID       string `json:"id"`
	Order    int    `json:"order"`
	Cost     int    `json:"cost"`
	Unlocked bool   `json:"unlocked"`
	Content  string `json:"content"` // empty when locked
}

// GetHints returns hints for a question. Content is only populated for unlocked hints.
func (c *Client) GetHints(questionID string) ([]Hint, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/questions/%s/hints", c.ServerURL, questionID), nil)
	req.Header.Set("Accept", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out []Hint
	return out, decodeJSON(resp, &out)
}

// GetHint returns a hint by ID (admin only).
func (c *Client) GetHint(id string) (*Hint, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/admin/hints/%s", c.ServerURL, id), nil)
	req.Header.Set("Accept", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out Hint
	return &out, decodeJSON(resp, &out)
}

// UpdateHint updates an existing hint (admin only).
func (c *Client) UpdateHint(id, content string, cost, order int) error {
	body, _ := json.Marshal(map[string]any{
		"content": content,
		"cost":    cost,
		"order":   order,
	})
	return c.doStatus("PUT", "/api/admin/hints/"+id, bytes.NewReader(body))
}

// CreateHint creates a new hint for a question (admin only).
func (c *Client) CreateHint(questionID, content string, cost int) (*Hint, error) {
	body, _ := json.Marshal(map[string]any{
		"question_id": questionID,
		"content":     content,
		"cost":        cost,
	})
	var out Hint
	return &out, c.doJSON("POST", "/api/admin/hints", bytes.NewReader(body), &out)
}

// DeleteHint deletes a hint by ID (admin only).
func (c *Client) DeleteHint(id string) error {
	return c.doNoBody("DELETE", "/api/admin/hints/"+id)
}

// UnlockHint unlocks a hint, spending the user's points.
// The server returns an empty body on success.
func (c *Client) UnlockHint(hintID string) error {
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/hints/%s/unlock", c.ServerURL, hintID), nil)
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return fmt.Errorf("not authenticated — run 'hctf2 login'")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}
