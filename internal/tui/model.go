package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pavanInnamuri/pod-doctor/internal/analyzer"
	"github.com/pavanInnamuri/pod-doctor/internal/domain"
	"github.com/pavanInnamuri/pod-doctor/internal/kubernetes"
)

// View represents the current view state
type View int

const (
	ViewNamespaceList View = iota
	ViewPodList
	ViewDiagnosis
	ViewLoading
)

// PodItem represents a pod in the list
type PodItem struct {
	Name      string
	Namespace string
	Status    string
	Ready     string
	Restarts  int32
	Age       string
	Node      string
}

// Model is the main TUI model
type Model struct {
	// State
	view           View
	prevView       View
	namespaces     []string
	pods           []PodItem
	filteredPods   []PodItem
	selectedNS     string
	selectedPod    string
	diagnosis      *domain.Diagnosis
	err            error
	loading        bool
	loadingMessage string

	// UI Components
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	spinner     spinner.Model
	keys        KeyMap

	// Dimensions
	width  int
	height int

	// Services
	client   *kubernetes.Client
	analyzer *analyzer.PodAnalyzer
}

// Messages
type namespacesLoadedMsg struct {
	namespaces []string
	err        error
}

type podsLoadedMsg struct {
	pods []PodItem
	err  error
}

type diagnosisCompleteMsg struct {
	diagnosis *domain.Diagnosis
	err       error
}

// NewModel creates a new TUI model
func NewModel(client *kubernetes.Client) Model {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.CharLimit = 50

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return Model{
		view:        ViewLoading,
		keys:        DefaultKeyMap(),
		filterInput: ti,
		spinner:     s,
		client:      client,
		analyzer:    analyzer.NewPodAnalyzer(client),
		width:       80,
		height:      24,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadNamespaces(),
	)
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle filter input when filtering
		if m.filtering {
			return m.handleFilterInput(msg)
		}
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case namespacesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.namespaces = msg.namespaces
		m.view = ViewNamespaceList
		m.cursor = 0

	case podsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.pods = msg.pods
		m.filteredPods = msg.pods
		m.view = ViewPodList
		m.cursor = 0

	case diagnosisCompleteMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.diagnosis = msg.diagnosis
		m.view = ViewDiagnosis
	}

	return m, tea.Batch(cmds...)
}

// handleKeyPress handles key presses based on current view
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Filter):
		if m.view == ViewPodList {
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink
		}

	case key.Matches(msg, m.keys.Back):
		return m.handleBack()

	case key.Matches(msg, m.keys.Up):
		m.moveCursor(-1)
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.moveCursor(1)
		return m, nil

	case key.Matches(msg, m.keys.PageUp):
		m.moveCursor(-10)
		return m, nil

	case key.Matches(msg, m.keys.PageDown):
		m.moveCursor(10)
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		return m.handleEnter()

	case key.Matches(msg, m.keys.Refresh):
		return m.handleRefresh()
	}

	return m, nil
}

// handleFilterInput handles input when filtering
func (m Model) handleFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filter = ""
		m.filterInput.SetValue("")
		m.filteredPods = m.pods
		m.cursor = 0
		return m, nil

	case "enter":
		m.filtering = false
		m.filter = m.filterInput.Value()
		m.applyFilter()
		return m, nil

	default:
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.filter = m.filterInput.Value()
		m.applyFilter()
		return m, cmd
	}
}

// handleBack handles the back action
func (m Model) handleBack() (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewPodList:
		m.view = ViewNamespaceList
		m.cursor = 0
		m.filter = ""
		m.filterInput.SetValue("")
	case ViewDiagnosis:
		m.view = ViewPodList
		m.cursor = 0
	}
	return m, nil
}

// handleEnter handles the enter key
func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewNamespaceList:
		if m.cursor < len(m.namespaces) {
			m.selectedNS = m.namespaces[m.cursor]
			m.loading = true
			m.loadingMessage = "Loading pods..."
			m.view = ViewLoading
			return m, tea.Batch(m.spinner.Tick, m.loadPods(m.selectedNS))
		}

	case ViewPodList:
		if m.cursor < len(m.filteredPods) {
			pod := m.filteredPods[m.cursor]
			m.selectedPod = pod.Name
			m.loading = true
			m.loadingMessage = fmt.Sprintf("Diagnosing %s...", pod.Name)
			m.view = ViewLoading
			return m, tea.Batch(m.spinner.Tick, m.runDiagnosis(pod.Namespace, pod.Name))
		}
	}
	return m, nil
}

// handleRefresh refreshes the current view
func (m Model) handleRefresh() (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewNamespaceList:
		m.loading = true
		m.loadingMessage = "Loading namespaces..."
		m.view = ViewLoading
		return m, tea.Batch(m.spinner.Tick, m.loadNamespaces())

	case ViewPodList:
		m.loading = true
		m.loadingMessage = "Loading pods..."
		m.view = ViewLoading
		return m, tea.Batch(m.spinner.Tick, m.loadPods(m.selectedNS))

	case ViewDiagnosis:
		m.loading = true
		m.loadingMessage = fmt.Sprintf("Diagnosing %s...", m.selectedPod)
		m.view = ViewLoading
		return m, tea.Batch(m.spinner.Tick, m.runDiagnosis(m.selectedNS, m.selectedPod))
	}
	return m, nil
}

// moveCursor moves the cursor by delta
func (m *Model) moveCursor(delta int) {
	var maxItems int
	switch m.view {
	case ViewNamespaceList:
		maxItems = len(m.namespaces)
	case ViewPodList:
		maxItems = len(m.filteredPods)
	default:
		return
	}

	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= maxItems {
		m.cursor = maxItems - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// applyFilter filters the pod list
func (m *Model) applyFilter() {
	if m.filter == "" {
		m.filteredPods = m.pods
		return
	}

	filter := strings.ToLower(m.filter)
	m.filteredPods = nil
	for _, pod := range m.pods {
		if strings.Contains(strings.ToLower(pod.Name), filter) ||
			strings.Contains(strings.ToLower(pod.Status), filter) ||
			strings.Contains(strings.ToLower(pod.Node), filter) {
			m.filteredPods = append(m.filteredPods, pod)
		}
	}
	m.cursor = 0
}

// Commands

func (m Model) loadNamespaces() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		namespaces, err := m.client.GetNamespaces(ctx)
		return namespacesLoadedMsg{namespaces: namespaces, err: err}
	}
}

func (m Model) loadPods(namespace string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		podList, err := m.client.ListPods(ctx, namespace, "")
		if err != nil {
			return podsLoadedMsg{err: err}
		}

		var pods []PodItem
		for _, p := range podList.Items {
			var restarts int32
			ready := 0
			total := len(p.Spec.Containers)
			for _, cs := range p.Status.ContainerStatuses {
				restarts += cs.RestartCount
				if cs.Ready {
					ready++
				}
			}

			pods = append(pods, PodItem{
				Name:      p.Name,
				Namespace: p.Namespace,
				Status:    string(p.Status.Phase),
				Ready:     fmt.Sprintf("%d/%d", ready, total),
				Restarts:  restarts,
				Age:       formatAge(time.Since(p.CreationTimestamp.Time)),
				Node:      p.Spec.NodeName,
			})
		}

		return podsLoadedMsg{pods: pods}
	}
}

func (m Model) runDiagnosis(namespace, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		diagnosis, err := m.analyzer.Diagnose(ctx, namespace, name)
		return diagnosisCompleteMsg{diagnosis: diagnosis, err: err}
	}
}

// View renders the UI
func (m Model) View() string {
	if m.err != nil {
		return m.renderError()
	}

	switch m.view {
	case ViewLoading:
		return m.renderLoading()
	case ViewNamespaceList:
		return m.renderNamespaceList()
	case ViewPodList:
		return m.renderPodList()
	case ViewDiagnosis:
		return m.renderDiagnosis()
	default:
		return "Unknown view"
	}
}

// Render functions

func (m Model) renderLoading() string {
	msg := m.loadingMessage
	if msg == "" {
		msg = "Loading..."
	}
	return fmt.Sprintf("\n\n   %s %s\n\n", m.spinner.View(), msg)
}

func (m Model) renderError() string {
	return lipgloss.NewStyle().
		Foreground(criticalColor).
		Padding(2).
		Render(fmt.Sprintf("Error: %v\n\nPress 'q' to quit or 'r' to retry", m.err))
}

func (m Model) renderNamespaceList() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🔍 pod-doctor"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("Select a namespace"))
	b.WriteString("\n\n")

	// Calculate visible range
	visibleHeight := m.height - 10
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	start := 0
	if m.cursor >= visibleHeight {
		start = m.cursor - visibleHeight + 1
	}
	end := start + visibleHeight
	if end > len(m.namespaces) {
		end = len(m.namespaces)
	}

	for i := start; i < end; i++ {
		ns := m.namespaces[i]
		if i == m.cursor {
			b.WriteString(cursorStyle.Render("▸ "))
			b.WriteString(selectedItemStyle.Render(ns))
		} else {
			b.WriteString("  ")
			b.WriteString(listItemStyle.Render(ns))
		}
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(m.namespaces) > visibleHeight {
		b.WriteString(fmt.Sprintf("\n%s", mutedStyle.Render(fmt.Sprintf("  %d/%d namespaces", m.cursor+1, len(m.namespaces)))))
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓: navigate • enter: select • q: quit"))

	return b.String()
}

func (m Model) renderPodList() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🔍 pod-doctor"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render(fmt.Sprintf("Namespace: %s", namespaceBadge.Render(m.selectedNS))))
	b.WriteString("\n")

	// Filter bar
	if m.filtering {
		b.WriteString(filterPromptStyle.Render("Filter: "))
		b.WriteString(m.filterInput.View())
		b.WriteString("\n\n")
	} else if m.filter != "" {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("Filter: %s", m.filter)))
		b.WriteString("\n\n")
	} else {
		b.WriteString("\n")
	}

	if len(m.filteredPods) == 0 {
		b.WriteString(mutedStyle.Render("  No pods found"))
		b.WriteString("\n")
	} else {
		// Header
		header := fmt.Sprintf("  %-40s %-12s %-8s %-10s %-8s", "NAME", "STATUS", "READY", "RESTARTS", "AGE")
		b.WriteString(mutedStyle.Render(header))
		b.WriteString("\n")

		// Calculate visible range
		visibleHeight := m.height - 14
		if visibleHeight < 5 {
			visibleHeight = 5
		}

		start := 0
		if m.cursor >= visibleHeight {
			start = m.cursor - visibleHeight + 1
		}
		end := start + visibleHeight
		if end > len(m.filteredPods) {
			end = len(m.filteredPods)
		}

		for i := start; i < end; i++ {
			pod := m.filteredPods[i]
			line := m.renderPodLine(pod, i == m.cursor)
			b.WriteString(line)
			b.WriteString("\n")
		}

		// Scroll indicator
		if len(m.filteredPods) > visibleHeight {
			b.WriteString(fmt.Sprintf("\n%s", mutedStyle.Render(fmt.Sprintf("  %d/%d pods", m.cursor+1, len(m.filteredPods)))))
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓: navigate • enter: diagnose • /: filter • esc: back • r: refresh • q: quit"))

	return b.String()
}

func (m Model) renderPodLine(pod PodItem, selected bool) string {
	// Status icon
	icon := StatusIcon(pod.Status == "Running" && pod.Restarts < 5)

	// Truncate name if needed
	name := pod.Name
	if len(name) > 38 {
		name = name[:35] + "..."
	}

	line := fmt.Sprintf("%s %-38s %-12s %-8s %-10d %-8s",
		icon, name, pod.Status, pod.Ready, pod.Restarts, pod.Age)

	if selected {
		return cursorStyle.Render("▸") + " " + selectedItemStyle.Render(line)
	}
	return "  " + listItemStyle.Render(line)
}

func (m Model) renderDiagnosis() string {
	if m.diagnosis == nil {
		return "No diagnosis available"
	}

	var b strings.Builder
	d := m.diagnosis

	// Header
	b.WriteString(titleStyle.Render("🔍 pod-doctor - Diagnosis"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render(fmt.Sprintf("%s/%s", d.Pod.Namespace, d.Pod.Name)))
	b.WriteString("\n\n")

	// Status
	statusStr := string(d.Status)
	var statusStyled string
	switch d.Status {
	case domain.StatusHealthy:
		statusStyled = healthyStyle.Render("● " + statusStr)
	case domain.StatusCrashLoop, domain.StatusOOMKilled, domain.StatusError, domain.StatusImagePull:
		statusStyled = criticalStyle.Render("● " + statusStr)
	default:
		statusStyled = warningStyle.Render("● " + statusStr)
	}
	b.WriteString(fmt.Sprintf("Status: %s\n", statusStyled))
	b.WriteString(fmt.Sprintf("Node: %s | Age: %s | Restarts: %d\n",
		valueOrNA(d.Pod.Node),
		formatDuration(d.Pod.Age),
		d.Pod.Restarts))
	b.WriteString("\n")

	// Issues
	if len(d.Issues) == 0 {
		b.WriteString(healthyStyle.Render("✓ No issues detected"))
		b.WriteString("\n")
	} else {
		critical, warning, _ := d.IssueCount()
		b.WriteString(fmt.Sprintf("Issues: %s critical, %s warnings\n\n",
			criticalStyle.Render(fmt.Sprintf("%d", critical)),
			warningStyle.Render(fmt.Sprintf("%d", warning))))

		// Show max 10 issues to fit screen
		maxIssues := 10
		if len(d.Issues) < maxIssues {
			maxIssues = len(d.Issues)
		}
		for i := 0; i < maxIssues; i++ {
			issue := d.Issues[i]
			icon := SeverityIcon(string(issue.Severity))
			b.WriteString(fmt.Sprintf("  %s %s\n", icon, issue.Title))
			if issue.Description != "" {
				desc := issue.Description
				if len(desc) > 60 {
					desc = desc[:57] + "..."
				}
				b.WriteString(fmt.Sprintf("    %s\n", mutedStyle.Render(desc)))
			}
		}
		if len(d.Issues) > maxIssues {
			b.WriteString(fmt.Sprintf("\n  %s\n", mutedStyle.Render(fmt.Sprintf("... and %d more issues", len(d.Issues)-maxIssues))))
		}
	}

	// Recommendations
	if len(d.Recommendations) > 0 {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Bold(true).Render("Recommendations:"))
		b.WriteString("\n")
		maxRecs := 5
		if len(d.Recommendations) < maxRecs {
			maxRecs = len(d.Recommendations)
		}
		for i := 0; i < maxRecs; i++ {
			rec := d.Recommendations[i]
			b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, rec.Title))
			if rec.Command != "" {
				cmd := rec.Command
				if len(cmd) > 60 {
					cmd = cmd[:57] + "..."
				}
				b.WriteString(fmt.Sprintf("     %s\n", lipgloss.NewStyle().Foreground(primaryColor).Render("$ "+cmd)))
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("esc: back • r: refresh • q: quit"))

	return b.String()
}

// Helper functions

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours()) / 24
	return fmt.Sprintf("%dd", days)
}

func formatDuration(d time.Duration) string {
	return formatAge(d)
}

func valueOrNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}
