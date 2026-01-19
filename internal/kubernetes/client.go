package kubernetes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pavanInnamuri/pod-doctor/internal/domain"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps the Kubernetes clientset
type Client struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
}

// NewClient creates a new Kubernetes client
func NewClient(kubeconfigPath string) (*Client, error) {
	config, err := buildConfig(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		clientset: clientset,
		config:    config,
	}, nil
}

// buildConfig builds a Kubernetes config from kubeconfig file or in-cluster config
func buildConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		// Try in-cluster config first
		if config, err := rest.InClusterConfig(); err == nil {
			return config, nil
		}
		// Fall back to default kubeconfig location
		kubeconfigPath = defaultKubeconfigPath()
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
}

// defaultKubeconfigPath returns the default kubeconfig path
func defaultKubeconfigPath() string {
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}

// GetPod retrieves a pod by name and namespace
func (c *Client) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	return c.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ListPods lists pods in a namespace with optional label selector
func (c *Client) ListPods(ctx context.Context, namespace string, labelSelector string) (*corev1.PodList, error) {
	opts := metav1.ListOptions{}
	if labelSelector != "" {
		opts.LabelSelector = labelSelector
	}
	return c.clientset.CoreV1().Pods(namespace).List(ctx, opts)
}

// ListAllPods lists pods across all namespaces
func (c *Client) ListAllPods(ctx context.Context) (*corev1.PodList, error) {
	return c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
}

// GetPodLogs retrieves logs from a pod's container
func (c *Client) GetPodLogs(ctx context.Context, namespace, name, container string, tailLines int64, previous bool) (string, error) {
	opts := &corev1.PodLogOptions{
		Container: container,
		TailLines: &tailLines,
		Previous:  previous,
	}

	req := c.clientset.CoreV1().Pods(namespace).GetLogs(name, opts)
	result, err := req.Do(ctx).Raw()
	if err != nil {
		return "", err
	}

	return string(result), nil
}

// GetPodEvents retrieves events related to a pod
func (c *Client) GetPodEvents(ctx context.Context, namespace, name string) ([]domain.EventInfo, error) {
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.namespace=%s,involvedObject.kind=Pod", name, namespace)

	events, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, err
	}

	result := make([]domain.EventInfo, 0, len(events.Items))
	for _, e := range events.Items {
		result = append(result, domain.EventInfo{
			Type:      e.Type,
			Reason:    e.Reason,
			Message:   e.Message,
			Count:     e.Count,
			FirstSeen: e.FirstTimestamp.Time,
			LastSeen:  e.LastTimestamp.Time,
			Source:    e.Source.Component,
		})
	}

	return result, nil
}

// GetNode retrieves a node by name
func (c *Client) GetNode(ctx context.Context, name string) (*corev1.Node, error) {
	return c.clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
}

// GetNodeHealth returns health information for a node
func (c *Client) GetNodeHealth(ctx context.Context, nodeName string) (*domain.NodeHealth, error) {
	node, err := c.GetNode(ctx, nodeName)
	if err != nil {
		return nil, err
	}

	health := &domain.NodeHealth{
		Name: nodeName,
	}

	for _, condition := range node.Status.Conditions {
		switch condition.Type {
		case corev1.NodeReady:
			health.Ready = condition.Status == corev1.ConditionTrue
		case corev1.NodeMemoryPressure:
			health.MemoryPressure = condition.Status == corev1.ConditionTrue
		case corev1.NodeDiskPressure:
			health.DiskPressure = condition.Status == corev1.ConditionTrue
		case corev1.NodePIDPressure:
			health.PIDPressure = condition.Status == corev1.ConditionTrue
		case corev1.NodeNetworkUnavailable:
			health.NetworkUnavail = condition.Status == corev1.ConditionTrue
		}
	}

	return health, nil
}

// GetNamespaces returns a list of all namespaces
func (c *Client) GetNamespaces(ctx context.Context) ([]string, error) {
	namespaces, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(namespaces.Items))
	for _, ns := range namespaces.Items {
		result = append(result, ns.Name)
	}

	return result, nil
}

// ExtractPodInfo extracts domain.PodInfo from a Kubernetes Pod
func ExtractPodInfo(pod *corev1.Pod) domain.PodInfo {
	info := domain.PodInfo{
		Name:       pod.Name,
		Namespace:  pod.Namespace,
		Node:       pod.Spec.NodeName,
		Phase:      string(pod.Status.Phase),
		IP:         pod.Status.PodIP,
		Labels:     pod.Labels,
		Containers: make([]domain.ContainerInfo, 0),
	}

	// Calculate age
	if !pod.CreationTimestamp.IsZero() {
		info.Age = time.Since(pod.CreationTimestamp.Time)
	}

	// Extract container info
	containerStatuses := make(map[string]corev1.ContainerStatus)
	for _, cs := range pod.Status.ContainerStatuses {
		containerStatuses[cs.Name] = cs
		info.Restarts += cs.RestartCount
	}

	for _, container := range pod.Spec.Containers {
		ci := domain.ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
		}

		if status, ok := containerStatuses[container.Name]; ok {
			ci.Ready = status.Ready
			ci.RestartCount = status.RestartCount

			if status.State.Running != nil {
				ci.State = "running"
				ci.StartedAt = status.State.Running.StartedAt.Time
			} else if status.State.Waiting != nil {
				ci.State = "waiting"
				ci.Reason = status.State.Waiting.Reason
				ci.Message = status.State.Waiting.Message
			} else if status.State.Terminated != nil {
				ci.State = "terminated"
				ci.Reason = status.State.Terminated.Reason
				ci.Message = status.State.Terminated.Message
				ci.ExitCode = status.State.Terminated.ExitCode
				ci.FinishedAt = status.State.Terminated.FinishedAt.Time
			}
		}

		info.Containers = append(info.Containers, ci)
	}

	return info
}

// Clientset returns the underlying Kubernetes clientset
func (c *Client) Clientset() *kubernetes.Clientset {
	return c.clientset
}
