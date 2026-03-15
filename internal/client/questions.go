package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type QuestionDetail struct {
	ID            string  `json:"id"`
	ChallengeID   string  `json:"challenge_id"`
	Name          string  `json:"name"`
	Points        int     `json:"points"`
	FlagMask      *string `json:"flag_mask"`
	CaseSensitive bool    `json:"case_sensitive"`
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
	var out QuestionDetail
	return &out, c.doJSON("POST", "/api/admin/questions", bytes.NewReader(body), &out)
}

func (c *Client) GetQuestion(id string) (*QuestionDetail, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/admin/questions/%s", c.ServerURL, id), nil)
	req.Header.Set("Accept", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out QuestionDetail
	return &out, decodeJSON(resp, &out)
}

func (c *Client) UpdateQuestion(id, name, flag string, points int, caseSensitive bool) error {
	body, _ := json.Marshal(map[string]any{
		"name":           name,
		"flag":           flag,
		"points":         points,
		"case_sensitive": caseSensitive,
	})
	return c.doStatus("PUT", "/api/admin/questions/"+id, bytes.NewReader(body))
}

func (c *Client) DeleteQuestion(id string) error {
	return c.doNoBody("DELETE", "/api/admin/questions/"+id)
}
