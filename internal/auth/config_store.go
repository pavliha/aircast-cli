package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ConfigStore handles persistent storage of user preferences
type ConfigStore struct {
	configDir string
}

// Config represents user configuration/preferences
type Config struct {
	LastDeviceID string `json:"last_device_id,omitempty"`
}

// NewConfigStore creates a new config store
func NewConfigStore() (*ConfigStore, error) {
	// Use ~/.aircast for config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".aircast")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	return &ConfigStore{
		configDir: configDir,
	}, nil
}

// GetConfigPath returns the path to the config file
func (cs *ConfigStore) GetConfigPath() string {
	return filepath.Join(cs.configDir, "config.json")
}

// SaveConfig saves configuration to disk
func (cs *ConfigStore) SaveConfig(config *Config) error {
	configPath := cs.GetConfigPath()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write with restrictive permissions (only user can read/write)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LoadConfig loads configuration from disk
func (cs *ConfigStore) LoadConfig() (*Config, error) {
	configPath := cs.GetConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil // No config found, return empty config
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// SaveLastDevice saves the last used device ID
func (cs *ConfigStore) SaveLastDevice(deviceID string) error {
	config, err := cs.LoadConfig()
	if err != nil {
		return err
	}

	config.LastDeviceID = deviceID

	return cs.SaveConfig(config)
}

// GetLastDevice returns the last used device ID
func (cs *ConfigStore) GetLastDevice() (string, error) {
	config, err := cs.LoadConfig()
	if err != nil {
		return "", err
	}

	return config.LastDeviceID, nil
}
