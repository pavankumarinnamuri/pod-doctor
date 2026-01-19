package analyzer

import (
	"context"
	"sort"
	"strings"

	"github.com/pavanInnamuri/pod-doctor/internal/domain"
	"github.com/pavanInnamuri/pod-doctor/internal/kubernetes"
	corev1 "k8s.io/api/core/v1"
)

// Analyzer is the interface for pod analysis components
type Analyzer interface {
	// Name returns the analyzer name
	Name() string
	// Analyze performs analysis on the pod and returns issues
	Analyze(ctx context.Context, pod *corev1.Pod, client *kubernetes.Client) ([]domain.Issue, error)
}

// PodAnalyzer orchestrates all analyzers
type PodAnalyzer struct {
	client    *kubernetes.Client
	analyzers []Analyzer
}

// NewPodAnalyzer creates a new PodAnalyzer with default analyzers
func NewPodAnalyzer(client *kubernetes.Client) *PodAnalyzer {
	return &PodAnalyzer{
		client: client,
		analyzers: []Analyzer{
			NewStatusAnalyzer(),
			NewEventAnalyzer(),
			NewLogAnalyzer(),
			NewNodeAnalyzer(),
			NewResourceAnalyzer(),
			NewProbeAnalyzer(),
		},
	}
}

// Diagnose performs a complete diagnosis on a pod
func (p *PodAnalyzer) Diagnose(ctx context.Context, namespace, name string) (*domain.Diagnosis, error) {
	// Get the pod
	pod, err := p.client.GetPod(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	// Extract pod info
	podInfo := kubernetes.ExtractPodInfo(pod)
	diagnosis := domain.NewDiagnosis(podInfo)

	// Detect overall status
	diagnosis.Status = detectPodStatus(pod)

	// Run all analyzers
	for _, analyzer := range p.analyzers {
		issues, err := analyzer.Analyze(ctx, pod, p.client)
		if err != nil {
			// Log warning but continue with other analyzers
			continue
		}
		for _, issue := range issues {
			diagnosis.AddIssue(issue)
		}
	}

	// Get events
	events, err := p.client.GetPodEvents(ctx, namespace, name)
	if err == nil {
		diagnosis.Events = events
	}

	// Get node health if pod is scheduled
	if pod.Spec.NodeName != "" {
		nodeHealth, err := p.client.GetNodeHealth(ctx, pod.Spec.NodeName)
		if err == nil {
			diagnosis.Node = nodeHealth
		}
	}

	// Generate recommendations
	diagnosis.Recommendations = generateRecommendations(diagnosis)

	return diagnosis, nil
}

// detectPodStatus determines the high-level status of a pod
func detectPodStatus(pod *corev1.Pod) domain.PodStatus {
	// Check if pod is being deleted
	if pod.DeletionTimestamp != nil {
		return domain.StatusTerminating
	}

	// Check container statuses
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			switch cs.State.Waiting.Reason {
			case "CrashLoopBackOff":
				return domain.StatusCrashLoop
			case "ImagePullBackOff", "ErrImagePull":
				return domain.StatusImagePull
			case "CreateContainerError":
				return domain.StatusCreateError
			case "CreateContainerConfigError":
				return domain.StatusConfigError
			}
		}

		if cs.LastTerminationState.Terminated != nil {
			if cs.LastTerminationState.Terminated.Reason == "OOMKilled" {
				return domain.StatusOOMKilled
			}
		}
	}

	// Check pod phase
	switch pod.Status.Phase {
	case corev1.PodPending:
		return domain.StatusPending
	case corev1.PodFailed:
		if pod.Status.Reason == "Evicted" {
			return domain.StatusEvicted
		}
		return domain.StatusError
	case corev1.PodRunning:
		// Check if all containers are ready
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				return domain.StatusNotReady
			}
		}
		return domain.StatusHealthy
	case corev1.PodSucceeded:
		return domain.StatusHealthy
	}

	return domain.StatusUnknown
}

// generateRecommendations creates recommendations based on issues
func generateRecommendations(diagnosis *domain.Diagnosis) []domain.Recommendation {
	var recs []domain.Recommendation
	seenRecs := make(map[string]bool)

	for _, issue := range diagnosis.Issues {
		newRecs := getRecommendationsForIssue(issue, diagnosis.Pod)
		for _, rec := range newRecs {
			if !seenRecs[rec.Title] {
				recs = append(recs, rec)
				seenRecs[rec.Title] = true
			}
		}
	}

	// Sort by priority
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].Priority < recs[j].Priority
	})

	return recs
}

// getRecommendationsForIssue returns recommendations for a specific issue
func getRecommendationsForIssue(issue domain.Issue, pod domain.PodInfo) []domain.Recommendation {
	var recs []domain.Recommendation

	switch issue.Category {
	case "container":
		if issue.Title == "CrashLoopBackOff" || containsReason(issue, "CrashLoopBackOff") {
			recs = append(recs, domain.Recommendation{
				Priority:    1,
				Title:       "Check container logs",
				Description: "Review container logs to identify the crash cause",
				Command:     "kubectl logs " + pod.Name + " -n " + pod.Namespace + " --previous",
			})
		}
		if containsReason(issue, "ImagePullBackOff") || containsReason(issue, "ErrImagePull") {
			recs = append(recs, domain.Recommendation{
				Priority:    1,
				Title:       "Verify image exists",
				Description: "Check if the image exists and is accessible",
				Command:     "kubectl describe pod " + pod.Name + " -n " + pod.Namespace,
			})
			recs = append(recs, domain.Recommendation{
				Priority:    2,
				Title:       "Check image pull secrets",
				Description: "Ensure imagePullSecrets are configured if using a private registry",
			})
		}

	case "resources":
		if containsReason(issue, "OOMKilled") {
			recs = append(recs, domain.Recommendation{
				Priority:    1,
				Title:       "Increase memory limit",
				Description: "Container exceeded memory limit; consider increasing it",
				Command:     "kubectl set resources deployment/<deployment-name> -c <container> --limits=memory=<new-limit>",
			})
		}
		if strings.Contains(issue.Title, "No resource limits") {
			recs = append(recs, domain.Recommendation{
				Priority:    2,
				Title:       "Add resource limits",
				Description: "Set resource limits to prevent resource contention",
				Command:     "kubectl set resources deployment/<deployment-name> -c <container> --limits=cpu=500m,memory=256Mi",
			})
		}
		if strings.Contains(issue.Title, "BestEffort QoS") {
			recs = append(recs, domain.Recommendation{
				Priority:    2,
				Title:       "Configure resource requests and limits",
				Description: "BestEffort pods are first to be evicted; add resources for better QoS",
			})
		}

	case "probes":
		if strings.Contains(issue.Title, "probe failed") {
			recs = append(recs, domain.Recommendation{
				Priority:    1,
				Title:       "Check probe endpoint",
				Description: "Verify the probe endpoint is responding correctly",
				Command:     "kubectl exec " + pod.Name + " -n " + pod.Namespace + " -- curl -v localhost:<port>/<path>",
			})
		}
		if strings.Contains(issue.Title, "No health probes") {
			recs = append(recs, domain.Recommendation{
				Priority:    3,
				Title:       "Add health probes",
				Description: "Consider adding liveness and readiness probes for better health monitoring",
			})
		}
		if strings.Contains(issue.Title, "running but not ready") {
			recs = append(recs, domain.Recommendation{
				Priority:    1,
				Title:       "Debug readiness probe",
				Description: "Check why readiness probe is failing",
				Command:     "kubectl describe pod " + pod.Name + " -n " + pod.Namespace + " | grep -A10 'Readiness'",
			})
		}

	case "scheduling":
		recs = append(recs, domain.Recommendation{
			Priority:    1,
			Title:       "Check node resources",
			Description: "Verify cluster has nodes with sufficient resources",
			Command:     "kubectl describe nodes | grep -A5 'Allocated resources'",
		})
		recs = append(recs, domain.Recommendation{
			Priority:    2,
			Title:       "Review pod tolerations",
			Description: "Check if pod has required tolerations for tainted nodes",
		})

	case "node":
		recs = append(recs, domain.Recommendation{
			Priority:    1,
			Title:       "Check node status",
			Description: "Review node conditions and events",
			Command:     "kubectl describe node " + pod.Node,
		})

	case "logs":
		recs = append(recs, domain.Recommendation{
			Priority:    2,
			Title:       "Review full logs",
			Description: "Check complete container logs for more context",
			Command:     "kubectl logs " + pod.Name + " -n " + pod.Namespace + " --tail=100",
		})
	}

	return recs
}

// containsReason checks if the issue contains a specific reason
func containsReason(issue domain.Issue, reason string) bool {
	if issue.Details != nil {
		if r, ok := issue.Details["reason"]; ok && r == reason {
			return true
		}
	}
	return issue.Title == reason
}
