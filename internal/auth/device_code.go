package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

// DeviceCodeAuth implements OAuth2 Device Code Flow (RFC 8628)
type DeviceCodeAuth struct {
	apiURL string
	logger *log.Entry
}

// DeviceCodeResponse represents the initial device code response
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// TokenResponse represents the token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// TokenErrorResponse represents error during polling
type TokenErrorResponse struct {
	ErrorCode        string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// NewDeviceCodeAuth creates a new device code authenticator
func NewDeviceCodeAuth(apiURL string, logger *log.Entry) *DeviceCodeAuth {
	if logger == nil {
		logger = log.WithField("component", "device_code_auth")
	}

	return &DeviceCodeAuth{
		apiURL: apiURL,
		logger: logger,
	}
}

// Authenticate performs OAuth2 Device Code Flow
func (d *DeviceCodeAuth) Authenticate(ctx context.Context) (string, error) {
	// Step 1: Request device code
	deviceResp, err := d.requestDeviceCode(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to request device code: %w", err)
	}

	// Step 2: Display instructions to user
	d.displayInstructions(deviceResp)

	// Step 3: Poll for token
	token, err := d.pollForToken(ctx, deviceResp)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	fmt.Println("\n✓ Authentication successful!")
	fmt.Println()

	return token, nil
}

// requestDeviceCode requests a device code from the API
func (d *DeviceCodeAuth) requestDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	url := fmt.Sprintf("%s/v1/oauth2/cli/code", d.apiURL)

	// Request body with client_id
	reqBody := map[string]string{
		"client_id": "aircast-cli",
	}
	reqJSON, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqJSON))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var deviceResp DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceResp); err != nil {
		return nil, err
	}

	return &deviceResp, nil
}

// displayInstructions shows authentication instructions to the user
func (d *DeviceCodeAuth) displayInstructions(resp *DeviceCodeResponse) {
	fmt.Println("\n╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                   Aircast Authentication                      ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("To authenticate aircast-cli, visit this URL:")
	fmt.Println()
	fmt.Printf("  %s\n", resp.VerificationURIComplete)
	fmt.Println()
	fmt.Printf("Code expires in %d minutes.\n", resp.ExpiresIn/60)
	fmt.Println()
	fmt.Println("Waiting for authorization...")
	fmt.Println()
}

// pollForToken polls the API for token
func (d *DeviceCodeAuth) pollForToken(ctx context.Context, deviceResp *DeviceCodeResponse) (string, error) {
	url := fmt.Sprintf("%s/v1/oauth2/cli/token", d.apiURL)
	interval := time.Duration(deviceResp.Interval) * time.Second
	expires := time.Now().Add(time.Duration(deviceResp.ExpiresIn) * time.Second)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			if time.Now().After(expires) {
				return "", fmt.Errorf("device code expired")
			}

			token, err := d.attemptTokenRequest(ctx, url, deviceResp)
			if err != nil {
				// Check if it's a polling error or fatal error
				if tokenErr, ok := err.(*TokenErrorResponse); ok {
					switch tokenErr.ErrorCode {
					case "authorization_pending":
						// Continue polling
						d.logger.Debug("Still waiting for user authorization")
						continue
					case "slow_down":
						// Increase polling interval
						interval = interval + (5 * time.Second)
						ticker.Reset(interval)
						d.logger.Debug("Slowing down polling")
						continue
					case "expired_token":
						return "", fmt.Errorf("device code expired")
					case "access_denied":
						return "", fmt.Errorf("user denied authorization")
					default:
						return "", fmt.Errorf("authorization error: %s", tokenErr.ErrorDescription)
					}
				}
				// Other errors
				d.logger.WithError(err).Debug("Token request failed")
				continue
			}

			// Success!
			return token, nil
		}
	}
}

// attemptTokenRequest attempts to get the token
func (d *DeviceCodeAuth) attemptTokenRequest(ctx context.Context, url string, deviceResp *DeviceCodeResponse) (string, error) {
	reqBody := map[string]string{
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
		"device_code": deviceResp.DeviceCode,
		"client_id":   "aircast-cli",
	}
	reqJSON, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqJSON))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Parse response (success or error in same structure)
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if response contains an error
	if tokenResp.Error != "" {
		return "", &TokenErrorResponse{
			ErrorCode:        tokenResp.Error,
			ErrorDescription: tokenResp.ErrorDesc,
		}
	}

	// Success - return access token
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("no access token in response")
	}

	return tokenResp.AccessToken, nil
}

// Error implements error interface for TokenErrorResponse
func (e *TokenErrorResponse) Error() string {
	if e.ErrorDescription != "" {
		return fmt.Sprintf("%s: %s", e.ErrorCode, e.ErrorDescription)
	}
	return e.ErrorCode
}
