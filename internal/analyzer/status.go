package analyzer

import (
	"context"
	"fmt"

	"github.com/pavanInnamuri/pod-doctor/internal/domain"
	"github.com/pavanInnamuri/pod-doctor/internal/kubernetes"
	corev1 "k8s.io/api/core/v1"
)

// StatusAnalyzer analyzes pod and container statuses
type StatusAnalyzer struct{}

// NewStatusAnalyzer creates a new StatusAnalyzer
func NewStatusAnalyzer() *StatusAnalyzer {
	return &StatusAnalyzer{}
}

// Name returns the analyzer name
func (s *StatusAnalyzer) Name() string {
	return "status"
}

// Analyze checks pod status for issues
func (s *StatusAnalyzer) Analyze(ctx context.Context, pod *corev1.Pod, client *kubernetes.Client) ([]domain.Issue, error) {
	var issues []domain.Issue

	// Check container statuses
	for _, cs := range pod.Status.ContainerStatuses {
		issues = append(issues, s.analyzeContainerStatus(cs)...)
	}

	// Check init container statuses
	for _, cs := range pod.Status.InitContainerStatuses {
		issues = append(issues, s.analyzeInitContainerStatus(cs)...)
	}

	// Check pod conditions
	issues = append(issues, s.analyzePodConditions(pod)...)

	// Check for high restart count
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.RestartCount > 5 {
			issues = append(issues, domain.Issue{
				Severity:    domain.SeverityWarning,
				Category:    "container",
				Title:       fmt.Sprintf("High restart count for %s", cs.Name),
				Description: fmt.Sprintf("Container has restarted %d times", cs.RestartCount),
				Details: map[string]string{
					"container":     cs.Name,
					"restart_count": fmt.Sprintf("%d", cs.RestartCount),
				},
			})
		}
	}

	return issues, nil
}

// analyzeContainerStatus checks a container's status for issues
func (s *StatusAnalyzer) analyzeContainerStatus(cs corev1.ContainerStatus) []domain.Issue {
	var issues []domain.Issue

	// Check waiting state
	if cs.State.Waiting != nil {
		waiting := cs.State.Waiting

		switch waiting.Reason {
		case "CrashLoopBackOff":
			issues = append(issues, domain.Issue{
				Severity:    domain.SeverityCritical,
				Category:    "container",
				Title:       fmt.Sprintf("Container %s in CrashLoopBackOff", cs.Name),
				Description: "Container is repeatedly crashing after starting",
				Details: map[string]string{
					"container":     cs.Name,
					"reason":        waiting.Reason,
					"message":       waiting.Message,
					"restart_count": fmt.Sprintf("%d", cs.RestartCount),
				},
			})

		case "ImagePullBackOff", "ErrImagePull":
			issues = append(issues, domain.Issue{
				Severity:    domain.SeverityCritical,
				Category:    "container",
				Title:       fmt.Sprintf("Cannot pull image for %s", cs.Name),
				Description: waiting.Message,
				Details: map[string]string{
					"container": cs.Name,
					"reason":    waiting.Reason,
					"image":     cs.Image,
				},
			})

		case "CreateContainerConfigError":
			issues = append(issues, domain.Issue{
				Severity:    domain.SeverityCritical,
				Category:    "container",
				Title:       fmt.Sprintf("Config error for %s", cs.Name),
				Description: waiting.Message,
				Details: map[string]string{
					"container": cs.Name,
					"reason":    waiting.Reason,
				},
			})

		case "CreateContainerError":
			issues = append(issues, domain.Issue{
				Severity:    domain.SeverityCritical,
				Category:    "container",
				Title:       fmt.Sprintf("Cannot create container %s", cs.Name),
				Description: waiting.Message,
				Details: map[string]string{
					"container": cs.Name,
					"reason":    waiting.Reason,
				},
			})

		default:
			if waiting.Reason != "" && waiting.Reason != "ContainerCreating" && waiting.Reason != "PodInitializing" {
				issues = append(issues, domain.Issue{
					Severity:    domain.SeverityWarning,
					Category:    "container",
					Title:       fmt.Sprintf("Container %s waiting: %s", cs.Name, waiting.Reason),
					Description: waiting.Message,
					Details: map[string]string{
						"container": cs.Name,
						"reason":    waiting.Reason,
					},
				})
			}
		}
	}

	// Check last termination state
	if cs.LastTerminationState.Terminated != nil {
		terminated := cs.LastTerminationState.Terminated

		if terminated.Reason == "OOMKilled" {
			issues = append(issues, domain.Issue{
				Severity:    domain.SeverityCritical,
				Category:    "resources",
				Title:       fmt.Sprintf("Container %s was OOMKilled", cs.Name),
				Description: "Container exceeded memory limit and was killed",
				Details: map[string]string{
					"container": cs.Name,
					"reason":    "OOMKilled",
					"exit_code": fmt.Sprintf("%d", terminated.ExitCode),
				},
			})
		} else if terminated.ExitCode != 0 {
			issues = append(issues, domain.Issue{
				Severity:    domain.SeverityWarning,
				Category:    "container",
				Title:       fmt.Sprintf("Container %s exited with code %d", cs.Name, terminated.ExitCode),
				Description: terminated.Message,
				Details: map[string]string{
					"container": cs.Name,
					"reason":    terminated.Reason,
					"exit_code": fmt.Sprintf("%d", terminated.ExitCode),
				},
			})
		}
	}

	// Check if container terminated with error
	if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
		terminated := cs.State.Terminated
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityCritical,
			Category:    "container",
			Title:       fmt.Sprintf("Container %s terminated with exit code %d", cs.Name, terminated.ExitCode),
			Description: terminated.Message,
			Details: map[string]string{
				"container": cs.Name,
				"reason":    terminated.Reason,
				"exit_code": fmt.Sprintf("%d", terminated.ExitCode),
			},
		})
	}

	return issues
}

// analyzeInitContainerStatus checks init container status
func (s *StatusAnalyzer) analyzeInitContainerStatus(cs corev1.ContainerStatus) []domain.Issue {
	var issues []domain.Issue

	// Check if init container is stuck
	if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityWarning,
			Category:    "container",
			Title:       fmt.Sprintf("Init container %s waiting: %s", cs.Name, cs.State.Waiting.Reason),
			Description: cs.State.Waiting.Message,
			Details: map[string]string{
				"container": cs.Name,
				"type":      "init",
				"reason":    cs.State.Waiting.Reason,
			},
		})
	}

	// Check if init container failed
	if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityCritical,
			Category:    "container",
			Title:       fmt.Sprintf("Init container %s failed", cs.Name),
			Description: fmt.Sprintf("Exit code: %d - %s", cs.State.Terminated.ExitCode, cs.State.Terminated.Message),
			Details: map[string]string{
				"container": cs.Name,
				"type":      "init",
				"exit_code": fmt.Sprintf("%d", cs.State.Terminated.ExitCode),
			},
		})
	}

	return issues
}

// analyzePodConditions checks pod conditions for issues
func (s *StatusAnalyzer) analyzePodConditions(pod *corev1.Pod) []domain.Issue {
	var issues []domain.Issue

	for _, cond := range pod.Status.Conditions {
		switch cond.Type {
		case corev1.PodScheduled:
			if cond.Status == corev1.ConditionFalse {
				issues = append(issues, domain.Issue{
					Severity:    domain.SeverityCritical,
					Category:    "scheduling",
					Title:       "Pod cannot be scheduled",
					Description: cond.Message,
					Details: map[string]string{
						"reason": cond.Reason,
					},
				})
			}

		case corev1.PodReady:
			if cond.Status == corev1.ConditionFalse && pod.Status.Phase == corev1.PodRunning {
				issues = append(issues, domain.Issue{
					Severity:    domain.SeverityWarning,
					Category:    "container",
					Title:       "Pod is not ready",
					Description: cond.Message,
					Details: map[string]string{
						"reason": cond.Reason,
					},
				})
			}

		case corev1.ContainersReady:
			if cond.Status == corev1.ConditionFalse && pod.Status.Phase == corev1.PodRunning {
				issues = append(issues, domain.Issue{
					Severity:    domain.SeverityWarning,
					Category:    "container",
					Title:       "Containers not ready",
					Description: cond.Message,
					Details: map[string]string{
						"reason": cond.Reason,
					},
				})
			}
		}
	}

	// Check if pod was evicted
	if pod.Status.Phase == corev1.PodFailed && pod.Status.Reason == "Evicted" {
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityCritical,
			Category:    "resources",
			Title:       "Pod was evicted",
			Description: pod.Status.Message,
			Details: map[string]string{
				"reason": "Evicted",
			},
		})
	}

	return issues
}
