package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information - set at build time
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("pod-doctor %s\n", Version)
		fmt.Printf("  Commit: %s\n", Commit)
		fmt.Printf("  Built: %s\n", BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
