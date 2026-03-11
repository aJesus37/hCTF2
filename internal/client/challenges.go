package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Challenge struct {
	ID             string `json:"id"`
	Title          string `json:"name"`      // server field is "name"
	Category       string `json:"category"`
	Difficulty     string `json:"difficulty"`
	InitialPoints  int    `json:"initial_points"` // server field is "initial_points"
	Description    string `json:"description"`
	Visible        bool   `json:"visible"`
}

type Question struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	FlagMask string `json:"flag_mask"`
	Points   int    `json:"points"`
	Solved   bool   `json:"solved"`
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
	// The endpoint wraps the response: {"challenge": {...}, "questions": [...]}.
	var envelope struct {
		Challenge Challenge `json:"challenge"`
	}
	if err := decodeJSON(resp, &envelope); err != nil {
		return nil, err
	}
	return &envelope.Challenge, nil
}

// GetChallengeWithQuestions returns the challenge and its questions.
func (c *Client) GetChallengeWithQuestions(id string) (*Challenge, []Question, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/challenges/%s", c.ServerURL, id), nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, nil, err
	}
	var envelope struct {
		Challenge Challenge  `json:"challenge"`
		Questions []Question `json:"questions"`
	}
	if err := decodeJSON(resp, &envelope); err != nil {
		return nil, nil, err
	}
	return &envelope.Challenge, envelope.Questions, nil
}

func (c *Client) SubmitFlag(questionID, flag string) (*SubmitResult, error) {
	form := url.Values{"flag": {flag}}
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/questions/%s/submit", c.ServerURL, questionID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("unauthorized")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	body := string(bodyBytes)
	correct := strings.Contains(body, "Correct")
	return &SubmitResult{Correct: correct}, nil
}

func (c *Client) CreateChallenge(title, category, difficulty, description string, points int) (*Challenge, error) {
	body, _ := json.Marshal(map[string]any{
		"name": title, "category": category, "difficulty": difficulty,
		"description": description, "initial_points": points,
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
