package analyzer

import (
	"context"
	"fmt"

	"github.com/pavanInnamuri/pod-doctor/internal/domain"
	"github.com/pavanInnamuri/pod-doctor/internal/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ResourceAnalyzer analyzes pod resource configurations and usage
type ResourceAnalyzer struct{}

// NewResourceAnalyzer creates a new ResourceAnalyzer
func NewResourceAnalyzer() *ResourceAnalyzer {
	return &ResourceAnalyzer{}
}

// Name returns the analyzer name
func (r *ResourceAnalyzer) Name() string {
	return "resources"
}

// Analyze checks resource configurations for issues
func (r *ResourceAnalyzer) Analyze(ctx context.Context, pod *corev1.Pod, client *kubernetes.Client) ([]domain.Issue, error) {
	var issues []domain.Issue

	for _, container := range pod.Spec.Containers {
		issues = append(issues, r.analyzeContainer(container)...)
	}

	for _, container := range pod.Spec.InitContainers {
		issues = append(issues, r.analyzeContainer(container)...)
	}

	return issues, nil
}

// analyzeContainer checks a container's resource configuration
func (r *ResourceAnalyzer) analyzeContainer(container corev1.Container) []domain.Issue {
	var issues []domain.Issue
	resources := container.Resources

	// Check if no resource limits are set
	if len(resources.Limits) == 0 {
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityWarning,
			Category:    "resources",
			Title:       fmt.Sprintf("No resource limits for %s", container.Name),
			Description: "Container has no resource limits set, which may lead to resource contention",
			Details: map[string]string{
				"container":      container.Name,
				"recommendation": "Set CPU and memory limits to prevent resource starvation",
			},
		})
	}

	// Check if no resource requests are set
	if len(resources.Requests) == 0 {
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityInfo,
			Category:    "resources",
			Title:       fmt.Sprintf("No resource requests for %s", container.Name),
			Description: "Container has no resource requests set, which may affect scheduling",
			Details: map[string]string{
				"container":      container.Name,
				"recommendation": "Set resource requests for better scheduling decisions",
			},
		})
	}

	// Check memory limits
	memLimit := resources.Limits.Memory()
	memRequest := resources.Requests.Memory()

	if memLimit != nil && !memLimit.IsZero() {
		// Check for very low memory limit (< 64Mi)
		minMemory := resource.MustParse("64Mi")
		if memLimit.Cmp(minMemory) < 0 {
			issues = append(issues, domain.Issue{
				Severity:    domain.SeverityWarning,
				Category:    "resources",
				Title:       fmt.Sprintf("Low memory limit for %s", container.Name),
				Description: "Memory limit is very low and may cause OOMKill",
				Details: map[string]string{
					"container":    container.Name,
					"memory_limit": memLimit.String(),
					"minimum_recommended": "64Mi",
				},
			})
		}

		// Check if request > limit (invalid but K8s allows it by setting request = limit)
		if memRequest != nil && !memRequest.IsZero() && memRequest.Cmp(*memLimit) > 0 {
			issues = append(issues, domain.Issue{
				Severity:    domain.SeverityWarning,
				Category:    "resources",
				Title:       fmt.Sprintf("Memory request > limit for %s", container.Name),
				Description: "Memory request exceeds limit, request will be set to limit",
				Details: map[string]string{
					"container":      container.Name,
					"memory_request": memRequest.String(),
					"memory_limit":   memLimit.String(),
				},
			})
		}
	}

	// Check CPU limits
	cpuLimit := resources.Limits.Cpu()
	cpuRequest := resources.Requests.Cpu()

	if cpuLimit != nil && !cpuLimit.IsZero() {
		// Check for very low CPU limit (< 100m)
		minCPU := resource.MustParse("50m")
		if cpuLimit.Cmp(minCPU) < 0 {
			issues = append(issues, domain.Issue{
				Severity:    domain.SeverityWarning,
				Category:    "resources",
				Title:       fmt.Sprintf("Very low CPU limit for %s", container.Name),
				Description: "CPU limit is very low and may cause severe throttling",
				Details: map[string]string{
					"container":         container.Name,
					"cpu_limit":         cpuLimit.String(),
					"minimum_recommended": "50m",
				},
			})
		}

		// Check if CPU request > limit
		if cpuRequest != nil && !cpuRequest.IsZero() && cpuRequest.Cmp(*cpuLimit) > 0 {
			issues = append(issues, domain.Issue{
				Severity:    domain.SeverityWarning,
				Category:    "resources",
				Title:       fmt.Sprintf("CPU request > limit for %s", container.Name),
				Description: "CPU request exceeds limit, request will be set to limit",
				Details: map[string]string{
					"container":   container.Name,
					"cpu_request": cpuRequest.String(),
					"cpu_limit":   cpuLimit.String(),
				},
			})
		}
	}

	// Check for Guaranteed QoS class indicators (requests == limits)
	// This is informational, not an issue
	if r.isGuaranteedQoS(resources) {
		// No issue, this is good
	} else if r.isBurstableQoS(resources) {
		// Burstable is okay but worth noting for resource-sensitive apps
	} else {
		// BestEffort - no requests or limits
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityWarning,
			Category:    "resources",
			Title:       fmt.Sprintf("BestEffort QoS for %s", container.Name),
			Description: "Container has BestEffort QoS class and will be first to be evicted under memory pressure",
			Details: map[string]string{
				"container": container.Name,
				"qos_class": "BestEffort",
			},
		})
	}

	// Check for ephemeral storage limits
	ephemeralLimit := resources.Limits.StorageEphemeral()
	if ephemeralLimit == nil || ephemeralLimit.IsZero() {
		// Only warn if container might write logs/data
		// This is a soft warning
	}

	return issues
}

// isGuaranteedQoS checks if resources qualify for Guaranteed QoS
func (r *ResourceAnalyzer) isGuaranteedQoS(resources corev1.ResourceRequirements) bool {
	// Guaranteed: requests == limits for both CPU and memory
	cpuLimit := resources.Limits.Cpu()
	cpuRequest := resources.Requests.Cpu()
	memLimit := resources.Limits.Memory()
	memRequest := resources.Requests.Memory()

	if cpuLimit == nil || cpuLimit.IsZero() || memLimit == nil || memLimit.IsZero() {
		return false
	}

	cpuMatch := cpuRequest != nil && cpuLimit.Cmp(*cpuRequest) == 0
	memMatch := memRequest != nil && memLimit.Cmp(*memRequest) == 0

	return cpuMatch && memMatch
}

// isBurstableQoS checks if resources qualify for Burstable QoS
func (r *ResourceAnalyzer) isBurstableQoS(resources corev1.ResourceRequirements) bool {
	// Burstable: at least one request or limit set, but not Guaranteed
	hasRequest := len(resources.Requests) > 0
	hasLimit := len(resources.Limits) > 0

	return (hasRequest || hasLimit) && !r.isGuaranteedQoS(resources)
}

// GetResourceSummary returns a summary of container resources
func GetResourceSummary(container corev1.Container) domain.ResourceUsage {
	resources := container.Resources
	summary := domain.ResourceUsage{}

	if req := resources.Requests.Cpu(); req != nil && !req.IsZero() {
		summary.CPURequests = req.String()
	}
	if lim := resources.Limits.Cpu(); lim != nil && !lim.IsZero() {
		summary.CPULimits = lim.String()
	}
	if req := resources.Requests.Memory(); req != nil && !req.IsZero() {
		summary.MemoryRequests = req.String()
	}
	if lim := resources.Limits.Memory(); lim != nil && !lim.IsZero() {
		summary.MemoryLimits = lim.String()
	}

	return summary
}
