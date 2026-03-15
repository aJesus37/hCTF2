package client

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// ConfigImportResult mirrors models.ConfigImportResult.
type ConfigImportResult struct {
	ChallengesImported  int      `json:"challenges_imported"`
	CompetitionsCreated int      `json:"competitions_created"`
	Renamed             []string `json:"renamed,omitempty"`
	Errors              []string `json:"errors,omitempty"`
}

// ExportConfig fetches the full platform config bundle as raw JSON bytes.
func (c *Client) ExportConfig() ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.ServerURL+"/api/admin/config/export", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", string(b))
	}
	return io.ReadAll(resp.Body)
}

// ImportConfig uploads a config bundle (raw JSON bytes) via multipart form.
func (c *Client) ImportConfig(data []byte) (*ConfigImportResult, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "config.json")
	if err != nil {
		return nil, err
	}
	if _, err := fw.Write(data); err != nil {
		return nil, err
	}
	mw.Close()
	req, err := http.NewRequest(http.MethodPost, c.ServerURL+"/api/admin/config/import", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var result ConfigImportResult
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
