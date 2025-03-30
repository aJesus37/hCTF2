package config

// Config holds application configuration
type Config struct {
	// Add configuration fields as needed
	ServerPort string
	Debug      bool
}

// Load loads configuration from environment or files
func Load() (*Config, error) {
	// Implementation to load config from env vars or config file
	return &Config{
		ServerPort: "8090", // Default value
		Debug:      true,
	}, nil
}
