package output

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/pavanInnamuri/pod-doctor/internal/domain"
)

var (
	// Colors
	criticalColor = lipgloss.Color("196") // Red
	warningColor  = lipgloss.Color("214") // Orange
	infoColor     = lipgloss.Color("39")  // Blue
	successColor  = lipgloss.Color("82")  // Green
	mutedColor    = lipgloss.Color("245") // Gray

	// Styles
	criticalStyle = lipgloss.NewStyle().Foreground(criticalColor).Bold(true)
	warningStyle  = lipgloss.NewStyle().Foreground(warningColor)
	infoStyle     = lipgloss.NewStyle().Foreground(infoColor)
	successStyle  = lipgloss.NewStyle().Foreground(successColor)
	mutedStyle    = lipgloss.NewStyle().Foreground(mutedColor)
	headerStyle   = lipgloss.NewStyle().Bold(true).Underline(true)
	boldStyle     = lipgloss.NewStyle().Bold(true)

	// Box styles
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)
)

// PrintDiagnosis prints a diagnosis result to the console
func PrintDiagnosis(d *domain.Diagnosis) {
	// Header
	fmt.Println()
	printHeader(d)
	fmt.Println()

	// Pod Info
	printPodInfo(d)
	fmt.Println()

	// Issues
	printIssues(d.Issues)
	fmt.Println()

	// Events (if any warnings)
	printEvents(d.Events)

	// Node Health
	if d.Node != nil {
		printNodeHealth(d.Node)
	}

	// Recommendations
	printRecommendations(d.Recommendations)

	fmt.Println()
}

// printHeader prints the diagnosis header
func printHeader(d *domain.Diagnosis) {
	title := fmt.Sprintf("Diagnosis: %s/%s", d.Pod.Namespace, d.Pod.Name)
	fmt.Println(headerStyle.Render(title))
	fmt.Println(mutedStyle.Render(fmt.Sprintf("Diagnosed at: %s", d.DiagnosedAt.Format("2006-01-02 15:04:05"))))
}

// printPodInfo prints pod information
func printPodInfo(d *domain.Diagnosis) {
	// Status with color
	statusStyle := successStyle
	statusIcon := "✓"
	switch d.Status {
	case domain.StatusHealthy:
		statusStyle = successStyle
		statusIcon = "✓"
	case domain.StatusCrashLoop, domain.StatusOOMKilled, domain.StatusError, domain.StatusImagePull:
		statusStyle = criticalStyle
		statusIcon = "✗"
	case domain.StatusPending, domain.StatusNotReady, domain.StatusTerminating:
		statusStyle = warningStyle
		statusIcon = "!"
	default:
		statusStyle = warningStyle
		statusIcon = "?"
	}

	fmt.Printf("Status: %s %s\n", statusIcon, statusStyle.Render(string(d.Status)))
	fmt.Printf("Node: %s | Phase: %s | Age: %s | Restarts: %d\n",
		valueOrNA(d.Pod.Node),
		d.Pod.Phase,
		formatDuration(d.Pod.Age),
		d.Pod.Restarts,
	)

	if d.Pod.IP != "" {
		fmt.Printf("Pod IP: %s\n", d.Pod.IP)
	}

	// Container summary
	if len(d.Pod.Containers) > 0 {
		fmt.Println()
		fmt.Println(boldStyle.Render("Containers:"))
		for _, c := range d.Pod.Containers {
			stateStyle := successStyle
			if c.State != "running" || !c.Ready {
				stateStyle = warningStyle
			}
			readyStr := "not ready"
			if c.Ready {
				readyStr = "ready"
			}
			fmt.Printf("  • %s: %s (%s, restarts: %d)\n",
				c.Name,
				stateStyle.Render(c.State),
				readyStr,
				c.RestartCount,
			)
			if c.Reason != "" {
				fmt.Printf("    Reason: %s\n", mutedStyle.Render(c.Reason))
			}
		}
	}
}

// printIssues prints detected issues
func printIssues(issues []domain.Issue) {
	if len(issues) == 0 {
		fmt.Println(successStyle.Render("✓ No issues detected"))
		return
	}

	// Count by severity
	var critical, warning, info int
	for _, issue := range issues {
		switch issue.Severity {
		case domain.SeverityCritical:
			critical++
		case domain.SeverityWarning:
			warning++
		case domain.SeverityInfo:
			info++
		}
	}

	summary := fmt.Sprintf("Issues Found: %d critical, %d warnings, %d info",
		critical, warning, info)
	fmt.Println(headerStyle.Render(summary))
	fmt.Println()

	for _, issue := range issues {
		printIssue(issue)
	}
}

// printIssue prints a single issue
func printIssue(issue domain.Issue) {
	var icon string
	var style lipgloss.Style

	switch issue.Severity {
	case domain.SeverityCritical:
		icon = "✗"
		style = criticalStyle
	case domain.SeverityWarning:
		icon = "!"
		style = warningStyle
	default:
		icon = "•"
		style = infoStyle
	}

	fmt.Printf("  %s %s\n", style.Render(icon), style.Render(issue.Title))
	fmt.Printf("    %s\n", issue.Description)

	// Print relevant details
	if len(issue.Details) > 0 {
		for key, value := range issue.Details {
			if key != "container" && key != "reason" && value != "" {
				// Truncate long values
				if len(value) > 100 {
					value = value[:97] + "..."
				}
				fmt.Printf("    %s: %s\n", mutedStyle.Render(key), value)
			}
		}
	}
	fmt.Println()
}

// printEvents prints warning events
func printEvents(events []domain.EventInfo) {
	var warnings []domain.EventInfo
	for _, e := range events {
		if e.Type == "Warning" {
			warnings = append(warnings, e)
		}
	}

	if len(warnings) == 0 {
		return
	}

	fmt.Println(headerStyle.Render("Recent Warning Events:"))
	for _, event := range warnings {
		fmt.Printf("  • [%s] %s: %s\n",
			warningStyle.Render(event.Reason),
			mutedStyle.Render(event.LastSeen.Format("15:04:05")),
			truncate(event.Message, 80),
		)
	}
	fmt.Println()
}

// printNodeHealth prints node health information
func printNodeHealth(node *domain.NodeHealth) {
	if node.Ready && !node.MemoryPressure && !node.DiskPressure && !node.PIDPressure && !node.NetworkUnavail {
		return // Node is healthy, skip
	}

	fmt.Println(headerStyle.Render("Node Health:"))
	fmt.Printf("  Node: %s\n", node.Name)

	if !node.Ready {
		fmt.Printf("  %s Node is not ready\n", criticalStyle.Render("✗"))
	}
	if node.MemoryPressure {
		fmt.Printf("  %s Memory pressure\n", warningStyle.Render("!"))
	}
	if node.DiskPressure {
		fmt.Printf("  %s Disk pressure\n", warningStyle.Render("!"))
	}
	if node.PIDPressure {
		fmt.Printf("  %s PID pressure\n", warningStyle.Render("!"))
	}
	if node.NetworkUnavail {
		fmt.Printf("  %s Network unavailable\n", criticalStyle.Render("✗"))
	}
	fmt.Println()
}

// printRecommendations prints fix recommendations
func printRecommendations(recs []domain.Recommendation) {
	if len(recs) == 0 {
		return
	}

	fmt.Println(headerStyle.Render("Recommendations:"))
	for i, rec := range recs {
		fmt.Printf("  %d. %s\n", i+1, boldStyle.Render(rec.Title))
		fmt.Printf("     %s\n", rec.Description)
		if rec.Command != "" {
			fmt.Printf("     %s %s\n", mutedStyle.Render("$"), infoStyle.Render(rec.Command))
		}
	}
}

// Helper functions

func valueOrNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd%dh", days, hours)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// PrintScanSummary prints a summary of scanned pods
func PrintScanSummary(diagnoses []*domain.Diagnosis) {
	fmt.Println()
	fmt.Println(headerStyle.Render("Scan Summary"))
	fmt.Println()

	var healthy, unhealthy int
	for _, d := range diagnoses {
		if d.IsHealthy() {
			healthy++
		} else {
			unhealthy++
		}
	}

	fmt.Printf("Total pods scanned: %d\n", len(diagnoses))
	fmt.Printf("  %s Healthy: %d\n", successStyle.Render("✓"), healthy)
	fmt.Printf("  %s Unhealthy: %d\n", criticalStyle.Render("✗"), unhealthy)
	fmt.Println()

	// List unhealthy pods
	if unhealthy > 0 {
		fmt.Println(headerStyle.Render("Unhealthy Pods:"))
		for _, d := range diagnoses {
			if !d.IsHealthy() {
				critical, warning, _ := d.IssueCount()
				statusStyle := warningStyle
				if critical > 0 {
					statusStyle = criticalStyle
				}
				fmt.Printf("  • %s/%s: %s (%d critical, %d warnings)\n",
					d.Pod.Namespace,
					d.Pod.Name,
					statusStyle.Render(string(d.Status)),
					critical,
					warning,
				)
			}
		}
	}
}

// PrintError prints an error message
func PrintError(msg string) {
	fmt.Println(criticalStyle.Render("Error: " + msg))
}

// PrintSuccess prints a success message
func PrintSuccess(msg string) {
	fmt.Println(successStyle.Render("✓ " + msg))
}

// PrintInfo prints an info message
func PrintInfo(msg string) {
	fmt.Println(infoStyle.Render(msg))
}

// Spinner characters for loading animation
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// GetSpinnerFrame returns a spinner frame for animation
func GetSpinnerFrame(frame int) string {
	return infoStyle.Render(spinnerFrames[frame%len(spinnerFrames)])
}

// FormatJSON formats diagnosis as indented JSON (for -o json flag)
func FormatJSON(d *domain.Diagnosis) (string, error) {
	// This is a placeholder - we'll use encoding/json in the actual implementation
	var sb strings.Builder
	sb.WriteString("{\n")
	sb.WriteString(fmt.Sprintf("  \"pod\": \"%s/%s\",\n", d.Pod.Namespace, d.Pod.Name))
	sb.WriteString(fmt.Sprintf("  \"status\": \"%s\",\n", d.Status))
	sb.WriteString(fmt.Sprintf("  \"issueCount\": %d\n", len(d.Issues)))
	sb.WriteString("}")
	return sb.String(), nil
}
