package domain

// Recommendation represents a suggested fix for an issue
type Recommendation struct {
	Priority    int    `json:"priority"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"` // Suggested kubectl command
}

// NewRecommendation creates a new recommendation
func NewRecommendation(priority int, title, description string) Recommendation {
	return Recommendation{
		Priority:    priority,
		Title:       title,
		Description: description,
	}
}

// WithCommand adds a suggested command to the recommendation
func (r Recommendation) WithCommand(cmd string) Recommendation {
	r.Command = cmd
	return r
}
