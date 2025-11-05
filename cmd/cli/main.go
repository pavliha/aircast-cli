package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/pavliha/aircast/aircast-cli/internal/api"
	"github.com/pavliha/aircast/aircast-cli/internal/auth"
	"github.com/pavliha/aircast/aircast-cli/internal/cli"
	"github.com/pavliha/aircast/aircast-cli/internal/ui"
	log "github.com/sirupsen/logrus"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Load .env file if it exists (silent fail if not present)
	_ = godotenv.Load()

	// Command line flags - simplified!
	var (
		deviceID    = flag.String("device", "", "Device ID to connect to (optional - will prompt to select)")
		apiURL      = flag.String("api", getEnv("AIRCAST_API_URL", "https://api.aircast.one"), "API base URL")
		tcpListen   = flag.String("tcp", getEnv("AIRCAST_TCP_LISTEN", "127.0.0.1:14550"), "TCP listen address for MAVLink clients")
		udpListen   = flag.String("udp", getEnv("AIRCAST_UDP_LISTEN", ""), "UDP listen address for MAVLink clients (optional)")
		doLogin     = flag.Bool("login", false, "Force re-authentication (clear stored token)")
		doLogout    = flag.Bool("logout", false, "Clear stored authentication token")
		logLevel    = flag.String("log-level", getEnv("LOG_LEVEL", "info"), "Log level (trace, debug, info, warn, error)")
		showVersion = flag.Bool("version", false, "Show version information")
	)

	flag.Parse()

	// Show version
	if *showVersion {
		fmt.Printf("aircast-cli version %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	// Configure logging
	level, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.WithError(err).Fatal("Invalid log level")
	}
	log.SetLevel(level)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	logger := log.WithField("app", "aircast-cli")

	// Initialize token store
	tokenStore, err := auth.NewTokenStore()
	if err != nil {
		logger.WithError(err).Fatal("Failed to initialize token store")
	}

	// Handle logout
	if *doLogout {
		if err := tokenStore.DeleteToken(); err != nil {
			logger.WithError(err).Fatal("Failed to delete token")
		}
		fmt.Println("✓ Logged out successfully")
		fmt.Printf("Token removed from: %s\n", tokenStore.GetTokenPath())
		os.Exit(0)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Get or authenticate token
	var accessToken string

	// Force login if requested
	if *doLogin {
		logger.Info("Forcing re-authentication")
		_ = tokenStore.DeleteToken()
	}

	// Try to load existing token
	storedToken, err := tokenStore.LoadToken()
	if err != nil {
		logger.WithError(err).Warn("Failed to load stored token")
	}

	// Check if we have a valid token
	if storedToken != nil && tokenStore.IsTokenValid(storedToken) && storedToken.APIURL == *apiURL {
		logger.Debug("Using stored authentication token")
		accessToken = storedToken.AccessToken
	} else {
		// Need to authenticate
		if storedToken != nil {
			logger.Debug("Stored token is invalid or expired, re-authenticating")
		}

		fmt.Println("Authentication required...")
		fmt.Println()

		authenticator := auth.NewDeviceCodeAuth(*apiURL, logger)
		accessToken, err = authenticator.Authenticate(ctx)
		if err != nil {
			logger.WithError(err).Fatal("Authentication failed")
		}

		// Save token for future use
		newToken := &auth.StoredToken{
			AccessToken: accessToken,
			TokenType:   "Bearer",
			ExpiresAt:   time.Now().Add(24 * time.Hour), // Tokens expire in 24 hours
			APIURL:      *apiURL,
		}

		if err := tokenStore.SaveToken(newToken); err != nil {
			logger.WithError(err).Warn("Failed to save token (will need to re-authenticate next time)")
		} else {
			fmt.Printf("✓ Token saved to: %s\n", tokenStore.GetTokenPath())
			fmt.Println()
		}
	}

	// Get device ID (from flag or interactive selection)
	selectedDeviceID := *deviceID

	if selectedDeviceID == "" {
		// Fetch devices from API
		apiClient := api.NewClient(*apiURL, accessToken)
		devices, err := apiClient.GetDevices(ctx)
		if err != nil {
			// If authentication failed, delete token and re-authenticate
			if api.IsAuthError(err) {
				logger.Warn("Token is invalid or expired, re-authenticating...")
				_ = tokenStore.DeleteToken()

				fmt.Println()
				fmt.Println("Your session has expired. Re-authenticating...")
				fmt.Println()

				authenticator := auth.NewDeviceCodeAuth(*apiURL, logger)
				accessToken, err = authenticator.Authenticate(ctx)
				if err != nil {
					logger.WithError(err).Fatal("Authentication failed")
				}

				// Save new token
				newToken := &auth.StoredToken{
					AccessToken: accessToken,
					TokenType:   "Bearer",
					ExpiresAt:   time.Now().Add(24 * time.Hour),
					APIURL:      *apiURL,
				}

				if err := tokenStore.SaveToken(newToken); err != nil {
					logger.WithError(err).Warn("Failed to save token")
				} else {
					fmt.Printf("✓ Token saved to: %s\n", tokenStore.GetTokenPath())
					fmt.Println()
				}

				// Retry fetching devices with new token
				apiClient = api.NewClient(*apiURL, accessToken)
				devices, err = apiClient.GetDevices(ctx)
				if err != nil {
					logger.WithError(err).Fatal("Failed to fetch devices")
				}
			} else {
				logger.WithError(err).Fatal("Failed to fetch devices")
			}
		}

		// Let user pick a device
		selectedDevice, err := ui.PickDevice(devices)
		if err != nil {
			logger.WithError(err).Fatal("Failed to select device")
		}

		selectedDeviceID = selectedDevice.ID
	}

	// Build WebSocket URL
	wsURL := buildWebSocketURL(*apiURL, selectedDeviceID)

	// Create bridge configuration
	config := &cli.Config{
		WebSocketURL: wsURL,
		AuthToken:    accessToken,
		TCPAddress:   *tcpListen,
		UDPAddress:   *udpListen,
		Logger:       logger,
	}

	// Create and start bridge
	b, err := cli.New(config)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create bridge")
	}

	if err := b.Start(); err != nil {
		logger.WithError(err).Fatal("Failed to start bridge")
	}

	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║          MAVLink Bridge Running                               ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  Device ID:  %s\n", selectedDeviceID)
	fmt.Printf("  TCP Port:   %s\n", *tcpListen)
	if *udpListen != "" {
		fmt.Printf("  UDP Port:   %s\n", *udpListen)
	}
	fmt.Println()
	fmt.Println("Connect your ground control station to:")
	fmt.Printf("  tcp://%s\n", *tcpListen)
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop...")
	fmt.Println()

	logger.WithFields(log.Fields{
		"websocket": wsURL,
		"tcp":       *tcpListen,
		"udp":       *udpListen,
	}).Info("Bridge started")

	// Wait for interrupt signal
	<-ctx.Done()

	fmt.Println()
	logger.Info("Shutting down...")
	if err := b.Stop(); err != nil {
		logger.WithError(err).Error("Error during shutdown")
	}
	fmt.Println("✓ Bridge stopped")
}

// buildWebSocketURL constructs the WebSocket URL from API URL and device ID
func buildWebSocketURL(apiURL, deviceID string) string {
	wsURL := fmt.Sprintf("%s/v1/mavlink/web/%s/ws", apiURL, deviceID)

	// Replace http with ws, https with wss
	if len(wsURL) >= 7 && wsURL[:7] == "http://" {
		return "ws://" + wsURL[7:]
	} else if len(wsURL) >= 8 && wsURL[:8] == "https://" {
		return "wss://" + wsURL[8:]
	}

	return wsURL
}

// getEnv gets an environment variable with a fallback default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
