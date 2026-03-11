package client

import "net/http"

type Category struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type Difficulty struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

func (c *Client) ListCategories() ([]Category, error) {
	req, _ := http.NewRequest("GET", c.ServerURL+"/api/categories", nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out []Category
	return out, decodeJSON(resp, &out)
}

func (c *Client) ListDifficulties() ([]Difficulty, error) {
	req, _ := http.NewRequest("GET", c.ServerURL+"/api/difficulties", nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var out []Difficulty
	return out, decodeJSON(resp, &out)
}
