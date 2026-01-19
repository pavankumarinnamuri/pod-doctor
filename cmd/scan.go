package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/pavanInnamuri/pod-doctor/internal/analyzer"
	"github.com/pavanInnamuri/pod-doctor/internal/domain"
	"github.com/pavanInnamuri/pod-doctor/internal/kubernetes"
	"github.com/pavanInnamuri/pod-doctor/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	allNamespaces bool
	onlyUnhealthy bool
	labelSelector string
	concurrency   int
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan pods for issues",
	Long: `Scan multiple pods for issues.

By default, scans all pods in the specified namespace and shows
a summary of healthy/unhealthy pods.

Examples:
  # Scan all pods in default namespace
  pod-doctor scan

  # Scan all pods in production namespace
  pod-doctor scan -n production

  # Scan all pods across all namespaces
  pod-doctor scan --all-namespaces

  # Only show unhealthy pods
  pod-doctor scan --unhealthy

  # Filter by label selector
  pod-doctor scan -l app=nginx`,
	Run: runScan,
}

func init() {
	scanCmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "scan all namespaces")
	scanCmd.Flags().BoolVar(&onlyUnhealthy, "unhealthy", false, "only show unhealthy pods")
	scanCmd.Flags().StringVarP(&labelSelector, "selector", "l", "", "label selector to filter pods")
	scanCmd.Flags().IntVar(&concurrency, "concurrency", 5, "number of concurrent diagnoses")
	rootCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create Kubernetes client
	client, err := kubernetes.NewClient(kubeconfigPath)
	if err != nil {
		output.PrintError(fmt.Sprintf("Failed to create Kubernetes client: %v", err))
		os.Exit(1)
	}

	// Get pods
	var pods []podRef
	if allNamespaces {
		podList, err := client.ListAllPods(ctx)
		if err != nil {
			output.PrintError(fmt.Sprintf("Failed to list pods: %v", err))
			os.Exit(1)
		}
		for _, pod := range podList.Items {
			pods = append(pods, podRef{namespace: pod.Namespace, name: pod.Name})
		}
	} else {
		podList, err := client.ListPods(ctx, namespace, labelSelector)
		if err != nil {
			output.PrintError(fmt.Sprintf("Failed to list pods: %v", err))
			os.Exit(1)
		}
		for _, pod := range podList.Items {
			pods = append(pods, podRef{namespace: pod.Namespace, name: pod.Name})
		}
	}

	if len(pods) == 0 {
		output.PrintInfo("No pods found")
		return
	}

	if outputFormat == "console" {
		fmt.Printf("Scanning %d pods...\n", len(pods))
	}

	// Create analyzer
	podAnalyzer := analyzer.NewPodAnalyzer(client)

	// Scan pods concurrently
	diagnoses := scanPods(ctx, podAnalyzer, pods)

	// Filter if only unhealthy
	if onlyUnhealthy {
		var filtered []*domain.Diagnosis
		for _, d := range diagnoses {
			if !d.IsHealthy() {
				filtered = append(filtered, d)
			}
		}
		diagnoses = filtered
	}

	// Output results
	switch outputFormat {
	case "json":
		data, err := json.MarshalIndent(diagnoses, "", "  ")
		if err != nil {
			output.PrintError(fmt.Sprintf("Failed to marshal JSON: %v", err))
			os.Exit(1)
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(diagnoses)
		if err != nil {
			output.PrintError(fmt.Sprintf("Failed to marshal YAML: %v", err))
			os.Exit(1)
		}
		fmt.Println(string(data))
	default:
		output.PrintScanSummary(diagnoses)
	}
}

type podRef struct {
	namespace string
	name      string
}

func scanPods(ctx context.Context, podAnalyzer *analyzer.PodAnalyzer, pods []podRef) []*domain.Diagnosis {
	var (
		diagnoses []*domain.Diagnosis
		mu        sync.Mutex
		wg        sync.WaitGroup
		sem       = make(chan struct{}, concurrency)
	)

	for _, pod := range pods {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(p podRef) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			diagnosis, err := podAnalyzer.Diagnose(ctx, p.namespace, p.name)
			if err != nil {
				// Skip pods that fail to diagnose
				return
			}

			mu.Lock()
			diagnoses = append(diagnoses, diagnosis)
			mu.Unlock()
		}(pod)
	}

	wg.Wait()
	return diagnoses
}
