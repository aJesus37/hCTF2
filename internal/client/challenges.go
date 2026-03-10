package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Challenge struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Difficulty  string `json:"difficulty"`
	Points      int    `json:"points"`
	Description string `json:"description"`
	Solved      bool   `json:"solved"`
}

type SubmitResult struct {
	Correct bool   `json:"correct"`
	Message string `json:"message"`
	Points  int    `json:"points"`
}

func (c *Client) ListChallenges() ([]Challenge, error) {
	req, _ := http.NewRequest("GET", c.ServerURL+"/api/challenges", nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out []Challenge
	return out, decodeJSON(resp, &out)
}

func (c *Client) GetChallenge(id string) (*Challenge, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/challenges/%s", c.ServerURL, id), nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out Challenge
	return &out, decodeJSON(resp, &out)
}

func (c *Client) SubmitFlag(questionID, flag string) (*SubmitResult, error) {
	body, _ := json.Marshal(map[string]string{"flag": flag})
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/questions/%s/submit", c.ServerURL, questionID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out SubmitResult
	return &out, decodeJSON(resp, &out)
}

func (c *Client) CreateChallenge(title, category, difficulty, description string, points int) (*Challenge, error) {
	body, _ := json.Marshal(map[string]any{
		"title": title, "category": category, "difficulty": difficulty,
		"description": description, "points": points,
	})
	req, _ := http.NewRequest("POST", c.ServerURL+"/api/admin/challenges", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out Challenge
	return &out, decodeJSON(resp, &out)
}

func (c *Client) DeleteChallenge(id string) error {
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/admin/challenges/%s", c.ServerURL, id), nil)
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
