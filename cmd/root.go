package cmd

import (
	"fmt"
	"os"

	"github.com/pavanInnamuri/pod-doctor/internal/tui"
	"github.com/spf13/cobra"
)

var (
	kubeconfigPath string
	namespace      string
	outputFormat   string
)

var rootCmd = &cobra.Command{
	Use:   "pod-doctor",
	Short: "Diagnose Kubernetes pod issues",
	Long: `pod-doctor is a CLI tool for diagnosing Kubernetes pod issues.

It analyzes pod status, container states, events, logs, and node health
to identify problems and provide actionable recommendations.

Run without arguments to launch the interactive TUI.

Examples:
  # Launch interactive TUI
  pod-doctor

  # Diagnose a specific pod
  pod-doctor diagnose my-pod -n default

  # Scan all pods in a namespace for issues
  pod-doctor scan -n production

  # Scan all namespaces
  pod-doctor scan --all-namespaces`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := tui.Run(kubeconfigPath); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&kubeconfigPath, "kubeconfig", "", "path to kubeconfig file (default: ~/.kube/config)")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "kubernetes namespace")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "console", "output format (console, json, yaml)")
}
