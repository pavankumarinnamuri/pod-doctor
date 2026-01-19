package analyzer

import (
	"context"
	"fmt"
	"strings"

	"github.com/pavanInnamuri/pod-doctor/internal/domain"
	"github.com/pavanInnamuri/pod-doctor/internal/kubernetes"
	corev1 "k8s.io/api/core/v1"
)

// EventAnalyzer analyzes Kubernetes events for issues
type EventAnalyzer struct{}

// NewEventAnalyzer creates a new EventAnalyzer
func NewEventAnalyzer() *EventAnalyzer {
	return &EventAnalyzer{}
}

// Name returns the analyzer name
func (e *EventAnalyzer) Name() string {
	return "events"
}

// Analyze checks events for warning patterns
func (e *EventAnalyzer) Analyze(ctx context.Context, pod *corev1.Pod, client *kubernetes.Client) ([]domain.Issue, error) {
	var issues []domain.Issue

	events, err := client.GetPodEvents(ctx, pod.Namespace, pod.Name)
	if err != nil {
		return nil, err
	}

	for _, event := range events {
		if event.Type == "Warning" {
			issue := e.analyzeWarningEvent(event)
			if issue != nil {
				issues = append(issues, *issue)
			}
		}
	}

	return issues, nil
}

// analyzeWarningEvent converts a warning event to an issue
func (e *EventAnalyzer) analyzeWarningEvent(event domain.EventInfo) *domain.Issue {
	severity := domain.SeverityWarning
	category := "events"

	// Determine severity based on event reason
	switch event.Reason {
	case "Failed", "FailedScheduling", "FailedMount", "FailedAttachVolume":
		severity = domain.SeverityCritical
	case "Unhealthy", "ProbeWarning":
		severity = domain.SeverityWarning
	case "BackOff":
		severity = domain.SeverityCritical
	}

	// Categorize the event
	switch {
	case strings.Contains(event.Reason, "Scheduling"):
		category = "scheduling"
	case strings.Contains(event.Reason, "Volume") || strings.Contains(event.Reason, "Mount"):
		category = "storage"
	case strings.Contains(event.Reason, "Probe") || event.Reason == "Unhealthy":
		category = "health"
	case strings.Contains(event.Reason, "Pull"):
		category = "container"
	case strings.Contains(event.Reason, "OOM"):
		category = "resources"
	}

	// Skip certain non-actionable events
	if event.Reason == "Scheduled" || event.Reason == "Pulled" || event.Reason == "Created" || event.Reason == "Started" {
		return nil
	}

	return &domain.Issue{
		Severity:    severity,
		Category:    category,
		Title:       event.Reason,
		Description: event.Message,
		Details: map[string]string{
			"count":   formatCount(event.Count),
			"source":  event.Source,
			"last_seen": event.LastSeen.Format("2006-01-02 15:04:05"),
		},
	}
}

func formatCount(count int32) string {
	if count <= 1 {
		return "1"
	}
	return fmt.Sprintf("%d times", count)
}
