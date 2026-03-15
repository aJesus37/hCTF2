package client

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

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

func (c *Client) CreateCategory(name string, order int) (*Category, error) {
	form := url.Values{"name": {name}, "sort_order": {strconv.Itoa(order)}}
	req, _ := http.NewRequest("POST", c.ServerURL+"/api/admin/categories", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return &Category{Name: name}, nil
}

func (c *Client) DeleteCategory(id string) error {
	return c.doNoBody("DELETE", "/api/admin/categories/"+id)
}

func (c *Client) CreateDifficulty(name string, order int) (*Difficulty, error) {
	form := url.Values{"name": {name}, "sort_order": {strconv.Itoa(order)}}
	req, _ := http.NewRequest("POST", c.ServerURL+"/api/admin/difficulties", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return &Difficulty{Name: name}, nil
}

func (c *Client) DeleteDifficulty(id string) error {
	return c.doNoBody("DELETE", "/api/admin/difficulties/"+id)
}
