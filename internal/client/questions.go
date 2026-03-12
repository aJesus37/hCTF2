package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type QuestionDetail struct {
	ID          string  `json:"id"`
	ChallengeID string  `json:"challenge_id"`
	Name        string  `json:"name"`
	Points      int     `json:"points"`
	FlagMask    *string `json:"flag_mask"`
}

func (c *Client) ListQuestions(challengeID string) ([]Question, error) {
	_, qs, err := c.GetChallengeWithQuestions(challengeID)
	return qs, err
}

func (c *Client) CreateQuestion(challengeID, name, flag string, points int) (*QuestionDetail, error) {
	body, _ := json.Marshal(map[string]any{
		"challenge_id": challengeID,
		"name":         name,
		"flag":         flag,
		"points":       points,
	})
	req, _ := http.NewRequest("POST", c.ServerURL+"/api/admin/questions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out QuestionDetail
	return &out, decodeJSON(resp, &out)
}

func (c *Client) DeleteQuestion(id string) error {
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/admin/questions/%s", c.ServerURL, id), nil)
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
