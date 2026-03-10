package config

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server       string    `yaml:"server"`
	Token        string    `yaml:"token,omitempty"`
	TokenExpires time.Time `yaml:"token_expires,omitempty"`
}

func path() string {
	if p := os.Getenv("HCTF2_CONFIG"); p != "" {
		return p
	}
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "hctf2", "config.yaml")
}

func Load() (*Config, error) {
	cfg := &Config{Server: "http://localhost:8090"}
	data, err := os.ReadFile(path())
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func Save(cfg *Config) error {
	p := path()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}
