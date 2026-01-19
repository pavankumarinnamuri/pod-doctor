package analyzer

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/pavanInnamuri/pod-doctor/internal/domain"
	"github.com/pavanInnamuri/pod-doctor/internal/kubernetes"
	corev1 "k8s.io/api/core/v1"
)

// LogAnalyzer analyzes container logs for error patterns
type LogAnalyzer struct {
	patterns []errorPattern
}

type errorPattern struct {
	Pattern     *regexp.Regexp
	Title       string
	Description string
	Severity    domain.Severity
}

// NewLogAnalyzer creates a new LogAnalyzer with default patterns
func NewLogAnalyzer() *LogAnalyzer {
	return &LogAnalyzer{
		patterns: []errorPattern{
			{regexp.MustCompile(`(?i)panic:`), "Panic detected", "Application panicked", domain.SeverityCritical},
			{regexp.MustCompile(`(?i)fatal\s*(error)?:`), "Fatal error", "Fatal error occurred", domain.SeverityCritical},
			{regexp.MustCompile(`(?i)out\s*of\s*memory`), "Out of memory", "Application ran out of memory", domain.SeverityCritical},
			{regexp.MustCompile(`(?i)killed`), "Process killed", "Process was killed", domain.SeverityWarning},
			{regexp.MustCompile(`(?i)connection\s*refused`), "Connection refused", "Cannot connect to a service", domain.SeverityWarning},
			{regexp.MustCompile(`(?i)ECONNREFUSED`), "Connection refused", "TCP connection refused", domain.SeverityWarning},
			{regexp.MustCompile(`(?i)permission\s*denied`), "Permission denied", "Insufficient permissions", domain.SeverityWarning},
			{regexp.MustCompile(`(?i)access\s*denied`), "Access denied", "Access was denied", domain.SeverityWarning},
			{regexp.MustCompile(`(?i)no\s*such\s*file`), "File not found", "Required file not found", domain.SeverityWarning},
			{regexp.MustCompile(`(?i)timeout|timed?\s*out`), "Timeout", "Operation timed out", domain.SeverityWarning},
			{regexp.MustCompile(`(?i)deadline\s*exceeded`), "Deadline exceeded", "Operation deadline was exceeded", domain.SeverityWarning},
			{regexp.MustCompile(`(?i)certificate\s*(verify|validation)\s*failed`), "Certificate error", "TLS certificate validation failed", domain.SeverityWarning},
			{regexp.MustCompile(`(?i)authentication\s*failed`), "Auth failed", "Authentication failed", domain.SeverityWarning},
			{regexp.MustCompile(`(?i)unauthorized`), "Unauthorized", "Unauthorized access attempt", domain.SeverityWarning},
			{regexp.MustCompile(`(?i)segmentation\s*fault`), "Segfault", "Segmentation fault occurred", domain.SeverityCritical},
			{regexp.MustCompile(`(?i)stack\s*overflow`), "Stack overflow", "Stack overflow error", domain.SeverityCritical},
			{regexp.MustCompile(`(?i)null\s*pointer`), "Null pointer", "Null pointer exception", domain.SeverityCritical},
		},
	}
}

// Name returns the analyzer name
func (l *LogAnalyzer) Name() string {
	return "logs"
}

// Analyze checks container logs for error patterns
func (l *LogAnalyzer) Analyze(ctx context.Context, pod *corev1.Pod, client *kubernetes.Client) ([]domain.Issue, error) {
	var issues []domain.Issue

	for _, container := range pod.Spec.Containers {
		containerIssues, err := l.analyzeContainerLogs(ctx, client, pod.Namespace, pod.Name, container.Name, false)
		if err != nil {
			// Try previous logs if current logs fail
			containerIssues, _ = l.analyzeContainerLogs(ctx, client, pod.Namespace, pod.Name, container.Name, true)
		}
		issues = append(issues, containerIssues...)
	}

	return issues, nil
}

// analyzeContainerLogs analyzes logs from a specific container
func (l *LogAnalyzer) analyzeContainerLogs(ctx context.Context, client *kubernetes.Client, namespace, podName, containerName string, previous bool) ([]domain.Issue, error) {
	var issues []domain.Issue

	logs, err := client.GetPodLogs(ctx, namespace, podName, containerName, 100, previous)
	if err != nil {
		return nil, err
	}

	if logs == "" {
		return issues, nil
	}

	lines := strings.Split(logs, "\n")
	matchedPatterns := make(map[string][]string) // pattern title -> matching lines

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		for _, pattern := range l.patterns {
			if pattern.Pattern.MatchString(line) {
				matchedPatterns[pattern.Title] = append(matchedPatterns[pattern.Title], truncateLine(line, 200))
			}
		}
	}

	// Create issues for matched patterns
	for _, pattern := range l.patterns {
		if matches, ok := matchedPatterns[pattern.Title]; ok {
			issue := domain.Issue{
				Severity:    pattern.Severity,
				Category:    "logs",
				Title:       fmt.Sprintf("[%s] %s", containerName, pattern.Title),
				Description: pattern.Description,
				Details: map[string]string{
					"container":    containerName,
					"match_count":  fmt.Sprintf("%d", len(matches)),
					"sample_match": matches[0],
				},
			}
			if len(matches) > 1 {
				issue.Details["additional_matches"] = fmt.Sprintf("%d more occurrences", len(matches)-1)
			}
			issues = append(issues, issue)
		}
	}

	return issues, nil
}

// truncateLine truncates a line to maxLen characters
func truncateLine(line string, maxLen int) string {
	if len(line) <= maxLen {
		return line
	}
	return line[:maxLen-3] + "..."
}
