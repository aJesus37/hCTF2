package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchLatestRelease_Stable(t *testing.T) {
	releases := []ghRelease{
		{TagName: "v1.0.0", Prerelease: false, Assets: []ghAsset{{Name: "hctf_linux_amd64.tar.gz", DownloadURL: "http://example.com/hctf_linux_amd64.tar.gz"}}},
		{TagName: "v1.1.0-beta.1", Prerelease: true, Assets: []ghAsset{{Name: "hctf_linux_amd64.tar.gz", DownloadURL: "http://example.com/beta.tar.gz"}}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(releases)
	}))
	defer srv.Close()

	rel, err := fetchLatestRelease(srv.URL, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.TagName != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", rel.TagName)
	}
}

func TestFetchLatestRelease_Beta(t *testing.T) {
	releases := []ghRelease{
		{TagName: "v1.1.0-beta.1", Prerelease: true}, // newest, comes first in API response
		{TagName: "v1.0.0", Prerelease: false},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(releases)
	}))
	defer srv.Close()

	rel, err := fetchLatestRelease(srv.URL, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.TagName != "v1.1.0-beta.1" {
		t.Errorf("expected v1.1.0-beta.1, got %s", rel.TagName)
	}
}

func TestFindAsset(t *testing.T) {
	rel := &ghRelease{
		Assets: []ghAsset{
			{Name: "hctf_linux_amd64.tar.gz", DownloadURL: "http://example.com/linux"},
			{Name: "hctf_darwin_arm64.tar.gz", DownloadURL: "http://example.com/darwin"},
		},
	}
	asset, err := findAsset(rel, "linux", "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if asset.DownloadURL != "http://example.com/linux" {
		t.Errorf("wrong asset selected: %s", asset.DownloadURL)
	}
}

func TestFindAsset_NotFound(t *testing.T) {
	rel := &ghRelease{Assets: []ghAsset{}}
	_, err := findAsset(rel, "plan9", "mips")
	if err == nil {
		t.Error("expected error for missing asset")
	}
}

func TestResolveChannel(t *testing.T) {
	if resolveChannel(true, "") != true {
		t.Error("--beta flag should force beta")
	}
	if resolveChannel(false, "beta") != true {
		t.Error("config beta should use beta")
	}
	if resolveChannel(false, "stable") != false {
		t.Error("config stable should use stable")
	}
	if resolveChannel(false, "") != false {
		t.Error("empty config should default to stable")
	}
}
