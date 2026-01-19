package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pavanInnamuri/pod-doctor/internal/kubernetes"
)

// Run starts the TUI with the given kubeconfig path
func Run(kubeconfigPath string) error {
	client, err := kubernetes.NewClient(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	model := NewModel(client)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err = p.Run()
	return err
}
