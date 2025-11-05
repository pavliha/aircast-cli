package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AuthError represents an authentication error (401)
type AuthError struct {
	StatusCode int
	Message    string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("authentication failed (status %d): %s", e.StatusCode, e.Message)
}

// IsAuthError checks if an error is an AuthError
func IsAuthError(err error) bool {
	_, ok := err.(*AuthError)
	return ok
}

// Client handles API communication
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

// Device represents a device from the API
type Device struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	LastSeenAt   string `json:"last_seen_at"`
	RegisteredAt string `json:"registered_at"`
	Role         string `json:"role"`
	IsOnline     bool   `json:"-"` // Populated from status endpoint
}

// DeviceStatus represents device online status
type DeviceStatus struct {
	DeviceID    string `json:"device_id"`
	IsOnline    bool   `json:"is_online"`
	ConnectedAt *int64 `json:"connected_at,omitempty"`
}

// DeviceStatusResponse represents the status API response
type DeviceStatusResponse struct {
	Devices []DeviceStatus `json:"devices"`
	Summary struct {
		Total  int `json:"total"`
		Online int `json:"online"`
	} `json:"summary"`
}

// NewClient creates a new API client
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		token: token,
	}
}

// GetDevices fetches the list of devices with their online status
func (c *Client) GetDevices(ctx context.Context) ([]Device, error) {
	// Fetch devices list
	devicesURL := fmt.Sprintf("%s/v1/user/devices", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", devicesURL, nil)
	if err != nil {
		return nil, err
	}

	// Add authentication - try both cookie and header
	req.AddCookie(&http.Cookie{
		Name:  "session",
		Value: c.token,
	})
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch devices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, &AuthError{
				StatusCode: resp.StatusCode,
				Message:    string(body),
			}
		}
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var devices []Device
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		return nil, fmt.Errorf("failed to parse devices response: %w", err)
	}

	// Fetch status for all devices
	statusURL := fmt.Sprintf("%s/v1/user/devices/status", c.baseURL)
	statusReq, err := http.NewRequestWithContext(ctx, "GET", statusURL, nil)
	if err != nil {
		fmt.Printf("Debug: Failed to create status request: %v\n", err)
		return devices, nil // Return devices without status if status fetch fails
	}

	statusReq.AddCookie(&http.Cookie{
		Name:  "session",
		Value: c.token,
	})
	statusReq.Header.Set("Authorization", "Bearer "+c.token)

	statusResp, err := c.httpClient.Do(statusReq)
	if err != nil {
		fmt.Printf("Debug: Failed to fetch status: %v\n", err)
		return devices, nil // Return devices without status
	}
	defer statusResp.Body.Close()

	fmt.Printf("Debug: Status response code: %d\n", statusResp.StatusCode)

	if statusResp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(statusResp.Body)
		fmt.Printf("Debug: Status response body: %s\n", string(body))

		var statusResponse DeviceStatusResponse
		if err := json.Unmarshal(body, &statusResponse); err == nil {
			fmt.Printf("Debug: Parsed %d statuses (total: %d, online: %d)\n",
				len(statusResponse.Devices), statusResponse.Summary.Total, statusResponse.Summary.Online)

			// Create a map for quick lookup
			statusMap := make(map[string]bool)
			for _, s := range statusResponse.Devices {
				fmt.Printf("Debug: Device %s is online: %v\n", s.DeviceID, s.IsOnline)
				statusMap[s.DeviceID] = s.IsOnline
			}

			// Update devices with status
			for i := range devices {
				if online, ok := statusMap[devices[i].ID]; ok {
					fmt.Printf("Debug: Setting device %s online status to: %v\n", devices[i].ID, online)
					devices[i].IsOnline = online
				}
			}
		} else {
			fmt.Printf("Debug: Failed to parse status: %v\n", err)
		}
	}

	return devices, nil
}
