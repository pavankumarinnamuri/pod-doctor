package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("39")  // Blue
	successColor   = lipgloss.Color("82")  // Green
	warningColor   = lipgloss.Color("214") // Orange
	criticalColor  = lipgloss.Color("196") // Red
	mutedColor     = lipgloss.Color("245") // Gray
	highlightColor = lipgloss.Color("212") // Pink
	selectedBg     = lipgloss.Color("236") // Dark gray

	// Base styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginBottom(1)

	// List styles
	listItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(1).
				Foreground(highlightColor).
				Bold(true)

	cursorStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			Bold(true)

	// Status styles
	healthyStyle = lipgloss.NewStyle().
			Foreground(successColor)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	criticalStyle = lipgloss.NewStyle().
			Foreground(criticalColor)

	mutedStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Panel styles
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2)

	// Help styles
	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)

	// Filter styles
	filterPromptStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	filterInputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255"))

	// Spinner style
	spinnerStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	// Badge styles
	namespaceBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("255")).
			Padding(0, 1)

	statusBadge = func(status string) lipgloss.Style {
		var bg lipgloss.Color
		switch status {
		case "Running", "Healthy":
			bg = lipgloss.Color("22") // Dark green
		case "Pending", "Initializing":
			bg = lipgloss.Color("58") // Dark yellow
		default:
			bg = lipgloss.Color("52") // Dark red
		}
		return lipgloss.NewStyle().
			Background(bg).
			Foreground(lipgloss.Color("255")).
			Padding(0, 1)
	}
)

// StatusIcon returns an icon for the given status
func StatusIcon(healthy bool) string {
	if healthy {
		return healthyStyle.Render("●")
	}
	return criticalStyle.Render("●")
}

// SeverityIcon returns an icon for the given severity
func SeverityIcon(severity string) string {
	switch severity {
	case "critical":
		return criticalStyle.Render("✗")
	case "warning":
		return warningStyle.Render("!")
	default:
		return lipgloss.NewStyle().Foreground(primaryColor).Render("•")
	}
}
