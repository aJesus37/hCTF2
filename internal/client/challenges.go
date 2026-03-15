package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

type Challenge struct {
	ID             string `json:"id"`
	Title          string `json:"name"`           // server field is "name"
	Category       string `json:"category"`
	Difficulty     string `json:"difficulty"`
	InitialPoints  int    `json:"initial_points"` // server field is "initial_points"
	Description    string `json:"description"`
	Visible        bool   `json:"visible"`
	MinimumPoints  int    `json:"minimum_points"`
	DecayThreshold int    `json:"decay_threshold"`
}

type Question struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	FlagMask  string `json:"flag_mask"`
	Points    int    `json:"points"`
	Solved    bool   `json:"solved"`
	HintCount int    `json:"hint_count"`
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
	req.Header.Set("Accept", "application/json")
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
	var result SubmitResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// GetQuestionSolution returns the flag for a question the user has already solved.
// Returns an error if the question is not solved or the user is not authenticated.
func (c *Client) GetQuestionSolution(questionID string) (string, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/questions/%s/solution", c.ServerURL, questionID), nil)
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	var out struct {
		Flag string `json:"flag"`
	}
	if err := decodeJSON(resp, &out); err != nil {
		return "", err
	}
	return out.Flag, nil
}

func (c *Client) CreateChallenge(title, category, difficulty, description string, points int, visible bool, minPoints, decay int) (*Challenge, error) {
	body, _ := json.Marshal(map[string]any{
		"name": title, "category": category, "difficulty": difficulty,
		"description": description, "initial_points": points,
		"visible": visible, "minimum_points": minPoints, "decay_threshold": decay,
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

func (c *Client) UpdateChallenge(id, title, category, difficulty, description string, points int, visible bool, minPoints, decay int) (*Challenge, error) {
	body, _ := json.Marshal(map[string]any{
		"name": title, "category": category, "difficulty": difficulty,
		"description": description, "initial_points": points,
		"visible": visible, "minimum_points": minPoints, "decay_threshold": decay,
	})
	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/admin/challenges/%s", c.ServerURL, id), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}
	// Server returns plain text "Challenge updated" on success; synthesise a
	// minimal Challenge with the supplied values so callers can display them.
	return &Challenge{
		ID:             id,
		Title:          title,
		Category:       category,
		Difficulty:     difficulty,
		Description:    description,
		InitialPoints:  points,
		Visible:        visible,
		MinimumPoints:  minPoints,
		DecayThreshold: decay,
	}, nil
}

func (c *Client) ExportChallenges() ([]byte, error) {
	req, _ := http.NewRequest("GET", c.ServerURL+"/api/admin/export", nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) ImportChallenges(data []byte) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "import.json")
	if err != nil {
		return err
	}
	if _, err := fw.Write(data); err != nil {
		return err
	}
	mw.Close()

	req, _ := http.NewRequest("POST", c.ServerURL+"/api/admin/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
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
