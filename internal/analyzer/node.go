package analyzer

import (
	"context"
	"fmt"

	"github.com/pavanInnamuri/pod-doctor/internal/domain"
	"github.com/pavanInnamuri/pod-doctor/internal/kubernetes"
	corev1 "k8s.io/api/core/v1"
)

// NodeAnalyzer analyzes the node where the pod is running
type NodeAnalyzer struct{}

// NewNodeAnalyzer creates a new NodeAnalyzer
func NewNodeAnalyzer() *NodeAnalyzer {
	return &NodeAnalyzer{}
}

// Name returns the analyzer name
func (n *NodeAnalyzer) Name() string {
	return "node"
}

// Analyze checks the node health
func (n *NodeAnalyzer) Analyze(ctx context.Context, pod *corev1.Pod, client *kubernetes.Client) ([]domain.Issue, error) {
	var issues []domain.Issue

	// Skip if pod isn't scheduled to a node
	if pod.Spec.NodeName == "" {
		return issues, nil
	}

	nodeHealth, err := client.GetNodeHealth(ctx, pod.Spec.NodeName)
	if err != nil {
		return nil, err
	}

	// Check if node is not ready
	if !nodeHealth.Ready {
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityCritical,
			Category:    "node",
			Title:       fmt.Sprintf("Node %s is not ready", nodeHealth.Name),
			Description: "The node where this pod is running is not in Ready state",
			Details: map[string]string{
				"node": nodeHealth.Name,
			},
		})
	}

	// Check for memory pressure
	if nodeHealth.MemoryPressure {
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityWarning,
			Category:    "node",
			Title:       fmt.Sprintf("Node %s has memory pressure", nodeHealth.Name),
			Description: "The node is experiencing memory pressure, which may cause pod evictions",
			Details: map[string]string{
				"node":      nodeHealth.Name,
				"condition": "MemoryPressure",
			},
		})
	}

	// Check for disk pressure
	if nodeHealth.DiskPressure {
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityWarning,
			Category:    "node",
			Title:       fmt.Sprintf("Node %s has disk pressure", nodeHealth.Name),
			Description: "The node is running low on disk space",
			Details: map[string]string{
				"node":      nodeHealth.Name,
				"condition": "DiskPressure",
			},
		})
	}

	// Check for PID pressure
	if nodeHealth.PIDPressure {
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityWarning,
			Category:    "node",
			Title:       fmt.Sprintf("Node %s has PID pressure", nodeHealth.Name),
			Description: "The node is running low on process IDs",
			Details: map[string]string{
				"node":      nodeHealth.Name,
				"condition": "PIDPressure",
			},
		})
	}

	// Check for network unavailable
	if nodeHealth.NetworkUnavail {
		issues = append(issues, domain.Issue{
			Severity:    domain.SeverityCritical,
			Category:    "node",
			Title:       fmt.Sprintf("Node %s network unavailable", nodeHealth.Name),
			Description: "The node's network is not properly configured",
			Details: map[string]string{
				"node":      nodeHealth.Name,
				"condition": "NetworkUnavailable",
			},
		})
	}

	return issues, nil
}
