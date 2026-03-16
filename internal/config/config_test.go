package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ajesus37/hCTF/internal/config"
)

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	t.Setenv("HCTF_CONFIG", path)

	cfg := &config.Config{
		Server:       "http://localhost:8090",
		Token:        "tok123",
		TokenExpires: time.Now().Add(time.Hour).UTC().Truncate(time.Second),
	}
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Server != cfg.Server {
		t.Errorf("server: got %q want %q", loaded.Server, cfg.Server)
	}
	if loaded.Token != cfg.Token {
		t.Errorf("token: got %q want %q", loaded.Token, cfg.Token)
	}
}

func TestLoadMissing(t *testing.T) {
	t.Setenv("HCTF_CONFIG", "/tmp/hctf-nonexistent-test.yaml")
	os.Remove("/tmp/hctf-nonexistent-test.yaml")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal("Load() should not error on missing file:", err)
	}
	if cfg.Server != "http://localhost:8090" {
		t.Errorf("expected default server, got %q", cfg.Server)
	}
}
