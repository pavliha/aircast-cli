package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/pavliha/aircast/aircast-cli/internal/api"
)

// PickDevice presents an interactive menu to select a device
func PickDevice(devices []api.Device) (*api.Device, error) {
	if len(devices) == 0 {
		return nil, fmt.Errorf("no devices found in your account")
	}

	// If only one device, auto-select it
	if len(devices) == 1 {
		fmt.Printf("Found 1 device: %s\n", devices[0].Name)
		return &devices[0], nil
	}

	// Display header
	fmt.Println("\n╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    Select a Device                            ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Display devices
	for i, device := range devices {
		fmt.Printf("[%d] %s\n", i+1, formatDevice(device))
	}

	fmt.Println()

	// Get user selection
	var selection int
	for {
		fmt.Printf("Select device (1-%d): ", len(devices))
		_, err := fmt.Scanln(&selection)
		if err != nil || selection < 1 || selection > len(devices) {
			fmt.Println("Invalid selection. Please try again.")
			continue
		}
		break
	}

	selectedDevice := &devices[selection-1]
	fmt.Printf("\n✓ Selected: %s\n\n", selectedDevice.Name)

	return selectedDevice, nil
}

// formatDevice formats a device for display
func formatDevice(device api.Device) string {
	var parts []string

	// Name (truncate if too long)
	name := device.Name
	if len(name) > 40 {
		name = name[:37] + "..."
	}
	parts = append(parts, fmt.Sprintf("%-40s", name))

	// Status indicator
	if device.IsOnline {
		parts = append(parts, "🟢 Online")
	} else {
		parts = append(parts, "⚫ Offline")
	}

	// Last seen
	if device.LastSeenAt != "" {
		lastSeenTime, err := time.Parse(time.RFC3339, device.LastSeenAt)
		if err == nil {
			lastSeen := formatTimeSince(lastSeenTime)
			parts = append(parts, fmt.Sprintf("(Last seen: %s)", lastSeen))
		}
	}

	return strings.Join(parts, " ")
}

// formatTimeSince formats a duration in a human-readable way
func formatTimeSince(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
