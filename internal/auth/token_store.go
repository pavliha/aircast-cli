package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TokenStore handles persistent storage of authentication tokens
type TokenStore struct {
	configDir string
}

// StoredToken represents a persisted authentication token
type StoredToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scope        string    `json:"scope,omitempty"`
	APIURL       string    `json:"api_url"`
}

// NewTokenStore creates a new token store
func NewTokenStore() (*TokenStore, error) {
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

	return &TokenStore{
		configDir: configDir,
	}, nil
}

// GetTokenPath returns the path to the token file
func (ts *TokenStore) GetTokenPath() string {
	return filepath.Join(ts.configDir, "token.json")
}

// SaveToken saves a token to disk
func (ts *TokenStore) SaveToken(token *StoredToken) error {
	tokenPath := ts.GetTokenPath()

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	// Write with restrictive permissions (only user can read/write)
	if err := os.WriteFile(tokenPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// LoadToken loads a token from disk
func (ts *TokenStore) LoadToken() (*StoredToken, error) {
	tokenPath := ts.GetTokenPath()

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No token found, not an error
		}
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var token StoredToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token file: %w", err)
	}

	return &token, nil
}

// DeleteToken deletes the stored token
func (ts *TokenStore) DeleteToken() error {
	tokenPath := ts.GetTokenPath()

	if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete token file: %w", err)
	}

	return nil
}

// IsTokenValid checks if a token is still valid
func (ts *TokenStore) IsTokenValid(token *StoredToken) bool {
	if token == nil {
		return false
	}

	// Check if token has expired (with 5 minute buffer)
	return time.Now().Before(token.ExpiresAt.Add(-5 * time.Minute))
}
