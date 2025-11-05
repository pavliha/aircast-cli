package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pavliha/aircast/aircast-cli/internal/api"
)

type devicePickerModel struct {
	devices  []api.Device
	cursor   int
	selected int
	done     bool
}

func (m devicePickerModel) Init() tea.Cmd {
	return nil
}

func (m devicePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.devices)-1 {
				m.cursor++
			}
		case "enter", " ":
			m.selected = m.cursor
			m.done = true
			return m, tea.Quit
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			// Allow number selection too
			num := int(msg.String()[0] - '0')
			if num > 0 && num <= len(m.devices) {
				m.selected = num - 1
				m.done = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m devicePickerModel) View() string {
	if m.done {
		return ""
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		Padding(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")).
		Bold(true).
		PaddingLeft(2)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7")).
		PaddingLeft(2)

	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(titleStyle.Render("Select a Device"))
	s.WriteString("\n\n")

	for i, device := range m.devices {
		cursor := " "
		if m.cursor == i {
			cursor = "❯"
		}

		style := normalStyle
		if m.cursor == i {
			style = selectedStyle
		}

		deviceLine := fmt.Sprintf("%s [%d] %s", cursor, i+1, formatDevice(device))
		s.WriteString(style.Render(deviceLine))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  ↑/↓: Navigate • Enter: Select • 1-9: Quick select • q: Quit"))
	s.WriteString("\n\n")

	return s.String()
}

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

	// Run interactive picker
	m := devicePickerModel{
		devices:  devices,
		cursor:   0,
		selected: -1,
		done:     false,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		// Fallback to old style if bubbletea fails
		return fallbackPicker(devices)
	}

	result := finalModel.(devicePickerModel)
	if !result.done || result.selected < 0 {
		return nil, fmt.Errorf("no device selected")
	}

	selectedDevice := &devices[result.selected]
	fmt.Printf("\n✓ Selected: %s\n\n", selectedDevice.Name)

	return selectedDevice, nil
}

// fallbackPicker is the old number-based picker as fallback
func fallbackPicker(devices []api.Device) (*api.Device, error) {
	fmt.Println("\n╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    Select a Device                            ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	for i, device := range devices {
		fmt.Printf("[%d] %s\n", i+1, formatDevice(device))
	}

	fmt.Println()

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
