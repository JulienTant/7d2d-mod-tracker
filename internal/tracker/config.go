package tracker

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Folder        string               `json:"folder"`
	Sources       map[string]SourceIDs `json:"sources"`
	NexusAPIKey   string               `json:"nexus_api_key,omitempty"`
	LegacySources map[string][]string  `json:"-"`
}

func DefaultConfig() Config {
	return Config{
		Sources:       make(map[string]SourceIDs),
		LegacySources: make(map[string][]string),
	}
}

func ConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir, _ = os.UserHomeDir()
	}
	return filepath.Join(dir, "7d2d-mod-tracker", "config.json")
}

func LoadConfig() Config {
	config := DefaultConfig()
	data, err := os.ReadFile(ConfigPath())
	if err == nil {
		var raw struct {
			Folder      string          `json:"folder"`
			Sources     json.RawMessage `json:"sources"`
			NexusAPIKey string          `json:"nexus_api_key"`
		}
		if json.Unmarshal(data, &raw) == nil {
			config.Folder = raw.Folder
			config.NexusAPIKey = raw.NexusAPIKey
			if json.Unmarshal(raw.Sources, &config.Sources) != nil {
				_ = json.Unmarshal(raw.Sources, &config.LegacySources)
			}
		}
	}
	if config.Sources == nil {
		config.Sources = make(map[string]SourceIDs)
	}
	if config.LegacySources == nil {
		config.LegacySources = make(map[string][]string)
	}
	return config
}

func SaveConfig(config Config) error {
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}
