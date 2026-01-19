package domain

// Severity represents the severity level of an issue
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

// Issue represents a detected problem with a pod
type Issue struct {
	Severity    Severity          `json:"severity"`
	Category    string            `json:"category"` // container, node, network, resources, scheduling, logs
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Details     map[string]string `json:"details,omitempty"`
}

// NewIssue creates a new issue with the given parameters
func NewIssue(severity Severity, category, title, description string) Issue {
	return Issue{
		Severity:    severity,
		Category:    category,
		Title:       title,
		Description: description,
		Details:     make(map[string]string),
	}
}

// WithDetail adds a detail to the issue and returns the issue for chaining
func (i Issue) WithDetail(key, value string) Issue {
	if i.Details == nil {
		i.Details = make(map[string]string)
	}
	i.Details[key] = value
	return i
}

// IsCritical returns true if the issue is critical
func (i Issue) IsCritical() bool {
	return i.Severity == SeverityCritical
}
