package cmd

import (
	"fmt"

	"github.com/ajesus37/hCTF2/internal/client"
	"github.com/ajesus37/hCTF2/internal/config"
)

func newClient() (*client.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if serverOverride != "" {
		cfg.Server = serverOverride
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("not logged in — run 'hctf2 login'")
	}
	return client.New(cfg.Server, cfg.Token), nil
}
