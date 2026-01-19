package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/pavanInnamuri/pod-doctor/internal/analyzer"
	"github.com/pavanInnamuri/pod-doctor/internal/kubernetes"
	"github.com/pavanInnamuri/pod-doctor/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose <pod-name>",
	Short: "Diagnose a specific pod",
	Long: `Diagnose a specific pod to identify issues and get recommendations.

This command analyzes:
  - Pod and container status
  - Container logs for error patterns
  - Kubernetes events
  - Node health (if pod is scheduled)
  - Resource usage

Examples:
  # Diagnose a pod in the default namespace
  pod-doctor diagnose my-pod

  # Diagnose a pod in a specific namespace
  pod-doctor diagnose my-pod -n production

  # Output as JSON
  pod-doctor diagnose my-pod -o json`,
	Args: cobra.ExactArgs(1),
	Run:  runDiagnose,
}

func init() {
	rootCmd.AddCommand(diagnoseCmd)
}

func runDiagnose(cmd *cobra.Command, args []string) {
	podName := args[0]
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create Kubernetes client
	client, err := kubernetes.NewClient(kubeconfigPath)
	if err != nil {
		output.PrintError(fmt.Sprintf("Failed to create Kubernetes client: %v", err))
		os.Exit(1)
	}

	// Create analyzer
	podAnalyzer := analyzer.NewPodAnalyzer(client)

	// Show loading message for console output
	if outputFormat == "console" {
		fmt.Printf("Diagnosing pod %s/%s...\n", namespace, podName)
	}

	// Run diagnosis
	diagnosis, err := podAnalyzer.Diagnose(ctx, namespace, podName)
	if err != nil {
		output.PrintError(fmt.Sprintf("Failed to diagnose pod: %v", err))
		os.Exit(1)
	}

	// Output results
	switch outputFormat {
	case "json":
		data, err := json.MarshalIndent(diagnosis, "", "  ")
		if err != nil {
			output.PrintError(fmt.Sprintf("Failed to marshal JSON: %v", err))
			os.Exit(1)
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(diagnosis)
		if err != nil {
			output.PrintError(fmt.Sprintf("Failed to marshal YAML: %v", err))
			os.Exit(1)
		}
		fmt.Println(string(data))
	default:
		output.PrintDiagnosis(diagnosis)
	}
}
