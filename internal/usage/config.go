package usage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type fileConfig struct {
	Source          string            `json:"source"`
	Sources         []string          `json:"sources"`
	View            string            `json:"view"`
	Format          string            `json:"format"`
	Output          string            `json:"output"`
	Breakdown       *bool             `json:"breakdown"`
	Instances       *bool             `json:"instances"`
	Active          *bool             `json:"active"`
	Recent          *bool             `json:"recent"`
	Compact         *bool             `json:"compact"`
	Debug           *bool             `json:"debug"`
	Progress        *bool             `json:"progress"`
	Since           string            `json:"since"`
	Until           string            `json:"until"`
	Timezone        string            `json:"timezone"`
	Order           string            `json:"order"`
	StartOfWeek     string            `json:"start_of_week"`
	Mode            string            `json:"mode"`
	Project         string            `json:"project"`
	ID              string            `json:"id"`
	Top             *int              `json:"top"`
	Path            string            `json:"path"`
	ClaudePath      string            `json:"claude_path"`
	CodexPath       string            `json:"codex_path"`
	OpenCodePath    string            `json:"opencode_path"`
	AmpPath         string            `json:"amp_path"`
	PIPath          string            `json:"pi_path"`
	TokenLimit      configString      `json:"token_limit"`
	SessionLength   configString      `json:"session_length"`
	RefreshInterval configString      `json:"refresh_interval"`
	Speed           string            `json:"speed"`
	Fields          string            `json:"fields"`
	Paths           map[string]string `json:"paths"`
}

type configString string

func (s *configString) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*s = configString(str)
		return nil
	}
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		*s = configString(strconv.FormatFloat(n, 'f', -1, 64))
		return nil
	}
	return fmt.Errorf("expected string or number")
}

func loadConfigFile(path string) (fileConfig, bool, error) {
	var cfg fileConfig
	if path == "" {
		return cfg, false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, false, nil
		}
		return cfg, false, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, true, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, true, nil
}

func defaultConfigPath() string {
	if path := os.Getenv("LLMUT_CONFIG"); path != "" {
		return path
	}
	if root := os.Getenv("XDG_CONFIG_HOME"); root != "" {
		return filepath.Join(root, "llmut", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "llmut", "config.json")
}
