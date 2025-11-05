package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// OAuth2Config holds OAuth2 configuration
type OAuth2Config struct {
	APIURL string // e.g., http://localhost:3333 or https://api.dev.aircast.one
	Logger *log.Entry
}

// TokenStatus represents the status of an auth token
type TokenStatus struct {
	Status       string `json:"status"`        // "pending" or "completed"
	SessionToken string `json:"session_token"` // Present when completed
	User         *User  `json:"user"`          // Present when completed
}

// User represents basic user info
type User struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// OAuth2Authenticator handles OAuth2 device flow authentication
type OAuth2Authenticator struct {
	config *OAuth2Config
	logger *log.Entry
}

// NewOAuth2Authenticator creates a new OAuth2 authenticator
func NewOAuth2Authenticator(config *OAuth2Config) *OAuth2Authenticator {
	if config.Logger == nil {
		config.Logger = log.WithField("component", "oauth2")
	}

	return &OAuth2Authenticator{
		config: config,
		logger: config.Logger,
	}
}

// Authenticate performs the OAuth2 device flow
func (a *OAuth2Authenticator) Authenticate(ctx context.Context) (string, error) {
	// Generate auth token
	authToken := uuid.New().String()

	// Construct the auth URL
	authURL := fmt.Sprintf("%s/v1/oauth2/user/google?token=%s", a.config.APIURL, authToken)

	// Display instructions to user
	fmt.Println("\n╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                   Aircast Authentication                      ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("To authenticate, please visit this URL in your browser:")
	fmt.Println()
	fmt.Printf("  %s\n", authURL)
	fmt.Println()
	fmt.Println("Waiting for authentication...")
	fmt.Println()

	// Poll for completion
	sessionToken, err := a.pollForToken(ctx, authToken)
	if err != nil {
		return "", err
	}

	fmt.Println("✓ Authentication successful!")
	fmt.Println()

	return sessionToken, nil
}

// pollForToken polls the API for token status
func (a *OAuth2Authenticator) pollForToken(ctx context.Context, authToken string) (string, error) {
	statusURL := fmt.Sprintf("%s/v1/oauth2/user/token/%s/status", a.config.APIURL, authToken)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(15 * time.Minute) // 15 minute timeout

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("authentication timeout after 15 minutes")
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, "GET", statusURL, nil)
			if err != nil {
				a.logger.WithError(err).Debug("Failed to create request")
				continue
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				a.logger.WithError(err).Debug("Failed to check token status")
				continue
			}

			body, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()

			if err != nil {
				a.logger.WithError(err).Debug("Failed to read response")
				continue
			}

			// Handle different status codes
			if resp.StatusCode == http.StatusNotFound {
				// Token not found or expired
				return "", fmt.Errorf("authentication token expired or invalid")
			}

			if resp.StatusCode != http.StatusOK {
				a.logger.WithField("status", resp.StatusCode).Debug("Unexpected status code")
				continue
			}

			var status TokenStatus
			if err := json.Unmarshal(body, &status); err != nil {
				a.logger.WithError(err).Debug("Failed to parse response")
				continue
			}

			if status.Status == "completed" {
				if status.SessionToken == "" {
					return "", fmt.Errorf("authentication completed but no session token received")
				}
				return status.SessionToken, nil
			}

			// Status is still "pending", continue polling
		}
	}
}

// ValidateToken checks if a session token is valid
func (a *OAuth2Authenticator) ValidateToken(ctx context.Context, sessionToken string) (bool, error) {
	validateURL := fmt.Sprintf("%s/v1/oauth2/user/sessions/me", a.config.APIURL)

	req, err := http.NewRequestWithContext(ctx, "GET", validateURL, nil)
	if err != nil {
		return false, err
	}

	// Add session cookie
	req.AddCookie(&http.Cookie{
		Name:  "session",
		Value: sessionToken,
	})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}
