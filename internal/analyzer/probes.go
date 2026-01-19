package analyzer

import (
	"context"
	"fmt"
	"strings"

	"github.com/pavanInnamuri/pod-doctor/internal/domain"
	"github.com/pavanInnamuri/pod-doctor/internal/kubernetes"
	corev1 "k8s.io/api/core/v1"
)

// ProbeAnalyzer analyzes pod probe configurations and failures
type ProbeAnalyzer struct{}

// NewProbeAnalyzer creates a new ProbeAnalyzer
func NewProbeAnalyzer() *ProbeAnalyzer {
	return &ProbeAnalyzer{}
}

// Name returns the analyzer name
func (p *ProbeAnalyzer) Name() string {
	return "probes"
}

// Analyze checks probe configurations and detects failures
func (p *ProbeAnalyzer) Analyze(ctx context.Context, pod *corev1.Pod, client *kubernetes.Client) ([]domain.Issue, error) {
	var issues []domain.Issue

	// Analyze container probe configurations
	for _, container := range pod.Spec.Containers {
		issues = append(issues, p.analyzeContainerProbes(container)...)
	}

	// Check events for probe failures
	events, err := client.GetPodEvents(ctx, pod.Namespace, pod.Name)
	if err == nil {
		issues = append(issues, p.analyzeProbeEvents(events)...)
	}

	// Check container statuses for probe-related issues
	for _, cs := range pod.Status.ContainerStatuses {
		issues = append(issues, p.analyzeContainerStatus(cs)...)
	}

	return issues, nil
}

// analyzeContainerProbes checks probe configurations
func (p *ProbeAnalyzer) analyzeContainerProbes(container corev1.Container) []domain.Issue {
	var issues []domain.Issue

	// Check if no probes are configured
	hasLiveness := container.LivenessProbe != nil
	hasReadiness := container.ReadinessProbe != nil
	hasStartup := container.StartupProbe != nil

	if !hasLiveness && !hasReadiness {
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityInfo,
			Category:    "probes",
			Title:       fmt.Sprintf("No health probes for %s", container.Name),
			Description: "Container has no liveness or readiness probes configured",
			Details: map[string]string{
				"container":      container.Name,
				"recommendation": "Consider adding probes for better health monitoring",
			},
		})
	}

	// Analyze liveness probe if present
	if hasLiveness {
		issues = append(issues, p.analyzeLivenessProbe(container.Name, container.LivenessProbe)...)
	}

	// Analyze readiness probe if present
	if hasReadiness {
		issues = append(issues, p.analyzeReadinessProbe(container.Name, container.ReadinessProbe)...)
	}

	// Analyze startup probe if present
	if hasStartup {
		issues = append(issues, p.analyzeStartupProbe(container.Name, container.StartupProbe)...)
	}

	// Check for common misconfigurations
	if hasLiveness && !hasStartup {
		// Check if liveness probe might kill slow-starting containers
		liveness := container.LivenessProbe
		initialDelay := liveness.InitialDelaySeconds
		if initialDelay < 10 {
			issues = append(issues, domain.Issue{
				Severity:    domain.SeverityWarning,
				Category:    "probes",
				Title:       fmt.Sprintf("Low liveness initialDelaySeconds for %s", container.Name),
				Description: "Liveness probe starts very early, may kill slow-starting containers",
				Details: map[string]string{
					"container":            container.Name,
					"initial_delay":        fmt.Sprintf("%ds", initialDelay),
					"recommendation":       "Consider using a startupProbe or increasing initialDelaySeconds",
				},
			})
		}
	}

	return issues
}

// analyzeLivenessProbe checks liveness probe configuration
func (p *ProbeAnalyzer) analyzeLivenessProbe(containerName string, probe *corev1.Probe) []domain.Issue {
	var issues []domain.Issue

	// Check for aggressive settings that might cause unnecessary restarts
	if probe.PeriodSeconds > 0 && probe.PeriodSeconds < 5 {
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityWarning,
			Category:    "probes",
			Title:       fmt.Sprintf("Aggressive liveness probe for %s", containerName),
			Description: "Liveness probe runs very frequently, may cause unnecessary restarts",
			Details: map[string]string{
				"container":     containerName,
				"period":        fmt.Sprintf("%ds", probe.PeriodSeconds),
				"recommendation": "Consider increasing periodSeconds to at least 10s",
			},
		})
	}

	// Check for low failure threshold
	if probe.FailureThreshold > 0 && probe.FailureThreshold < 3 {
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityWarning,
			Category:    "probes",
			Title:       fmt.Sprintf("Low liveness failureThreshold for %s", containerName),
			Description: "Container will restart after very few probe failures",
			Details: map[string]string{
				"container":        containerName,
				"failure_threshold": fmt.Sprintf("%d", probe.FailureThreshold),
				"recommendation":   "Consider increasing failureThreshold to at least 3",
			},
		})
	}

	// Check for very short timeout
	if probe.TimeoutSeconds > 0 && probe.TimeoutSeconds < 2 {
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityInfo,
			Category:    "probes",
			Title:       fmt.Sprintf("Short liveness timeout for %s", containerName),
			Description: "Liveness probe timeout is very short",
			Details: map[string]string{
				"container":      containerName,
				"timeout":        fmt.Sprintf("%ds", probe.TimeoutSeconds),
				"recommendation": "Consider increasing timeoutSeconds if probe target may be slow",
			},
		})
	}

	return issues
}

// analyzeReadinessProbe checks readiness probe configuration
func (p *ProbeAnalyzer) analyzeReadinessProbe(containerName string, probe *corev1.Probe) []domain.Issue {
	var issues []domain.Issue

	// Check for very long initial delay
	if probe.InitialDelaySeconds > 60 {
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityInfo,
			Category:    "probes",
			Title:       fmt.Sprintf("Long readiness initialDelaySeconds for %s", containerName),
			Description: "Readiness probe starts very late, pod won't receive traffic for a while",
			Details: map[string]string{
				"container":     containerName,
				"initial_delay": fmt.Sprintf("%ds", probe.InitialDelaySeconds),
			},
		})
	}

	return issues
}

// analyzeStartupProbe checks startup probe configuration
func (p *ProbeAnalyzer) analyzeStartupProbe(containerName string, probe *corev1.Probe) []domain.Issue {
	var issues []domain.Issue

	// Calculate max startup time
	maxStartupTime := int(probe.FailureThreshold) * int(probe.PeriodSeconds)
	if maxStartupTime > 0 && maxStartupTime < 30 {
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityWarning,
			Category:    "probes",
			Title:       fmt.Sprintf("Short startup window for %s", containerName),
			Description: "Startup probe allows very little time for container to start",
			Details: map[string]string{
				"container":        containerName,
				"max_startup_time": fmt.Sprintf("%ds", maxStartupTime),
				"recommendation":   "Increase failureThreshold or periodSeconds",
			},
		})
	}

	return issues
}

// analyzeProbeEvents checks events for probe failures
func (p *ProbeAnalyzer) analyzeProbeEvents(events []domain.EventInfo) []domain.Issue {
	var issues []domain.Issue

	for _, event := range events {
		if event.Type != "Warning" {
			continue
		}

		// Check for probe failure events
		if event.Reason == "Unhealthy" {
			severity := domain.SeverityWarning
			probeType := "Unknown"

			if strings.Contains(event.Message, "Liveness") {
				probeType = "Liveness"
				severity = domain.SeverityCritical // Liveness failures cause restarts
			} else if strings.Contains(event.Message, "Readiness") {
				probeType = "Readiness"
			} else if strings.Contains(event.Message, "Startup") {
				probeType = "Startup"
				severity = domain.SeverityCritical
			}

			issues = append(issues, domain.Issue{
				Severity:    severity,
				Category:    "probes",
				Title:       fmt.Sprintf("%s probe failed", probeType),
				Description: event.Message,
				Details: map[string]string{
					"probe_type": probeType,
					"count":      fmt.Sprintf("%d", event.Count),
					"last_seen":  event.LastSeen.Format("15:04:05"),
				},
			})
		}
	}

	return issues
}

// analyzeContainerStatus checks container status for probe-related issues
func (p *ProbeAnalyzer) analyzeContainerStatus(cs corev1.ContainerStatus) []domain.Issue {
	var issues []domain.Issue

	// Check if container is not ready due to probe failure
	if !cs.Ready && cs.State.Running != nil {
		// Container is running but not ready - likely readiness probe failing
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityWarning,
			Category:    "probes",
			Title:       fmt.Sprintf("Container %s running but not ready", cs.Name),
			Description: "Container is running but readiness probe is failing",
			Details: map[string]string{
				"container": cs.Name,
				"state":     "running",
				"ready":     "false",
			},
		})
	}

	// Check for restarts that might be caused by liveness probe
	if cs.RestartCount > 0 && cs.LastTerminationState.Terminated != nil {
		terminated := cs.LastTerminationState.Terminated
		// Exit code 137 often indicates SIGKILL (possibly from liveness probe)
		if terminated.ExitCode == 137 {
			issues = append(issues, domain.Issue{
				Severity:    domain.SeverityWarning,
				Category:    "probes",
				Title:       fmt.Sprintf("Container %s killed (exit 137)", cs.Name),
				Description: "Container was killed with SIGKILL, possibly by liveness probe or OOM",
				Details: map[string]string{
					"container":     cs.Name,
					"exit_code":     "137",
					"restart_count": fmt.Sprintf("%d", cs.RestartCount),
					"reason":        terminated.Reason,
				},
			})
		}
	}

	return issues
}
