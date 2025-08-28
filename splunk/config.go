package splunk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config stores all configuration options.
type Config struct {
	Host        string        `json:"host"`
	Token       string        `json:"token"`
	User        string        `json:"user"`
	Password    string        `json:"password"`
	App         string        `json:"app"`
	Owner       string        `json:"owner"`
	Insecure    bool          `json:"insecure"`
	HTTPTimeout time.Duration `json:"httpTimeout"`
	Limit       int           `json:"limit"`
	Debug       bool          `json:"-"` // Exclude from JSON marshalling
}

// LoadConfigFromFile loads configuration from the user's config directory.
// It now accepts an optional customConfigPath. If provided, it uses that path.
func LoadConfigFromFile(customConfigPath string) (Config, string, error) {
	var cfg Config
	configFile := customConfigPath // Use custom path if provided

	if configFile == "" { // If no custom path, use default
		home, err := os.UserHomeDir()
		if err != nil {
			return cfg, "", fmt.Errorf("could not get user home directory: %w", err)
		}
		configFile = filepath.Join(home, ".config", "splunk-cli", "config.json")
	}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return cfg, configFile, nil
	}

	file, err := os.Open(configFile)
	if err != nil {
		return cfg, configFile, fmt.Errorf("could not open config file: %w", err)
	}
	defer file.Close()

	type configHelper struct {
		Host        string `json:"host"`
		Token       string `json:"token"`
		User        string `json:"user"`
		Password    string `json:"password"`
		App         string `json:"app"`
		Owner       string `json:"owner"`
		Insecure    bool   `json:"insecure"`
		HTTPTimeout string `json:"httpTimeout"`
		Limit       int    `json:"limit"`
	}
	var helper configHelper
	if err := json.NewDecoder(file).Decode(&helper); err != nil {
		return cfg, configFile, fmt.Errorf("could not parse config file: %w", err)
	}

	cfg.Host = strings.TrimSpace(helper.Host)
	cfg.Token = strings.TrimSpace(helper.Token)
	cfg.User = strings.TrimSpace(helper.User)
	cfg.Password = strings.TrimSpace(helper.Password)
	cfg.App = strings.TrimSpace(helper.App)
	cfg.Owner = strings.TrimSpace(helper.Owner)
	cfg.Insecure = helper.Insecure
	cfg.Limit = helper.Limit
	if helper.HTTPTimeout != "" {
		parsedDuration, err := time.ParseDuration(helper.HTTPTimeout)
		if err != nil {
			return cfg, configFile, fmt.Errorf("invalid httpTimeout value in config: %w", err)
		}
		cfg.HTTPTimeout = parsedDuration
	}

	return cfg, configFile, nil
}

// ProcessEnvVars overwrites config with values from environment variables.
func ProcessEnvVars(cfg *Config) {
	if host := os.Getenv("SPLUNK_HOST"); host != "" {
		cfg.Host = host
	}
	if token := os.Getenv("SPLUNK_TOKEN"); token != "" {
		cfg.Token = token
	}
	if user := os.Getenv("SPLUNK_USER"); user != "" {
		cfg.User = user
	}
	if password := os.Getenv("SPLUNK_PASSWORD"); password != "" {
		cfg.Password = password
	}
	if app := os.Getenv("SPLUNK_APP"); app != "" {
		cfg.App = app
	}
}
