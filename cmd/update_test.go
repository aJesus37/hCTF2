package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func makeTarGz(t *testing.T, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: "hctf", Size: int64(len(content)), Mode: 0755})
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("tw.Write: %v", err)
	}
	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func TestDownloadAndExtract(t *testing.T) {
	want := []byte("fake-binary-content")
	tarball := makeTarGz(t, want)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(tarball)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "hctf-new")
	if err := downloadAndExtract(srv.URL, dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading dest: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("content mismatch: got %q want %q", got, want)
	}

	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat dest: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected mode 0755, got %v", info.Mode().Perm())
	}
}

func TestAtomicReplace(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "hctf")
	newBin := filepath.Join(dir, "hctf.new")

	if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
		t.Fatalf("WriteFile target: %v", err)
	}
	if err := os.WriteFile(newBin, []byte("new"), 0755); err != nil {
		t.Fatalf("WriteFile newBin: %v", err)
	}

	if err := atomicReplace(newBin, target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new" {
		t.Errorf("expected new content, got %q", got)
	}
	// newBin should be gone after rename
	if _, err := os.Stat(newBin); !os.IsNotExist(err) {
		t.Error("temp file should be gone after replace")
	}
}

func TestCanWriteExec(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hctf")
	os.WriteFile(path, []byte("x"), 0755)
	if !canWriteExec(path) {
		t.Error("expected writable file to return true")
	}
}

func TestSudoReexecArgs(t *testing.T) {
	args := buildSudoArgs("/usr/local/bin/hctf", []string{"update", "--yes"})
	if len(args) < 3 {
		t.Fatalf("expected at least 3 args, got %d", len(args))
	}
	if args[0] != "sudo" {
		t.Errorf("expected sudo at [0], got %s", args[0])
	}
	if args[1] != "/usr/local/bin/hctf" {
		t.Errorf("expected binary path at [1], got %s", args[1])
	}
	if args[2] != "update" {
		t.Errorf("expected 'update' at [2], got %s", args[2])
	}
}
