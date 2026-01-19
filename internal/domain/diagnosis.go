package domain

import "time"

// PodStatus represents the high-level status of a pod
type PodStatus string

const (
	StatusHealthy        PodStatus = "Healthy"
	StatusCrashLoop      PodStatus = "CrashLoopBackOff"
	StatusImagePull      PodStatus = "ImagePullBackOff"
	StatusPending        PodStatus = "Pending"
	StatusOOMKilled      PodStatus = "OOMKilled"
	StatusEvicted        PodStatus = "Evicted"
	StatusError          PodStatus = "Error"
	StatusTerminating    PodStatus = "Terminating"
	StatusUnknown        PodStatus = "Unknown"
	StatusNotReady       PodStatus = "NotReady"
	StatusInitializing   PodStatus = "Initializing"
	StatusCreateError    PodStatus = "CreateContainerError"
	StatusConfigError    PodStatus = "CreateContainerConfigError"
)

// ContainerInfo holds information about a container
type ContainerInfo struct {
	Name         string        `json:"name"`
	Image        string        `json:"image"`
	Ready        bool          `json:"ready"`
	RestartCount int32         `json:"restartCount"`
	State        string        `json:"state"` // running, waiting, terminated
	Reason       string        `json:"reason,omitempty"`
	Message      string        `json:"message,omitempty"`
	ExitCode     int32         `json:"exitCode,omitempty"`
	StartedAt    time.Time     `json:"startedAt,omitempty"`
	FinishedAt   time.Time     `json:"finishedAt,omitempty"`
}

// PodInfo holds basic information about the pod
type PodInfo struct {
	Name       string          `json:"name"`
	Namespace  string          `json:"namespace"`
	Node       string          `json:"node"`
	Age        time.Duration   `json:"age"`
	Phase      string          `json:"phase"`
	IP         string          `json:"ip,omitempty"`
	Restarts   int32           `json:"restarts"`
	Containers []ContainerInfo `json:"containers"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// EventInfo holds information about a Kubernetes event
type EventInfo struct {
	Type      string    `json:"type"` // Normal, Warning
	Reason    string    `json:"reason"`
	Message   string    `json:"message"`
	Count     int32     `json:"count"`
	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
	Source    string    `json:"source"`
}

// ResourceUsage holds resource usage information
type ResourceUsage struct {
	CPURequests    string `json:"cpuRequests,omitempty"`
	CPULimits      string `json:"cpuLimits,omitempty"`
	CPUUsage       string `json:"cpuUsage,omitempty"`
	MemoryRequests string `json:"memoryRequests,omitempty"`
	MemoryLimits   string `json:"memoryLimits,omitempty"`
	MemoryUsage    string `json:"memoryUsage,omitempty"`
}

// NodeHealth holds node health information
type NodeHealth struct {
	Name            string `json:"name"`
	Ready           bool   `json:"ready"`
	MemoryPressure  bool   `json:"memoryPressure"`
	DiskPressure    bool   `json:"diskPressure"`
	PIDPressure     bool   `json:"pidPressure"`
	NetworkUnavail  bool   `json:"networkUnavailable"`
}

// LogAnalysis holds analyzed log information
type LogAnalysis struct {
	HasErrors   bool     `json:"hasErrors"`
	ErrorLines  []string `json:"errorLines,omitempty"`
	LastLines   []string `json:"lastLines,omitempty"`
	TotalLines  int      `json:"totalLines"`
}

// Diagnosis represents the complete diagnosis result for a pod
type Diagnosis struct {
	Pod             PodInfo          `json:"pod"`
	Status          PodStatus        `json:"status"`
	Issues          []Issue          `json:"issues"`
	Events          []EventInfo      `json:"events,omitempty"`
	Logs            *LogAnalysis     `json:"logs,omitempty"`
	Resources       *ResourceUsage   `json:"resources,omitempty"`
	Node            *NodeHealth      `json:"node,omitempty"`
	Recommendations []Recommendation `json:"recommendations"`
	DiagnosedAt     time.Time        `json:"diagnosedAt"`
}

// NewDiagnosis creates a new diagnosis for a pod
func NewDiagnosis(pod PodInfo) *Diagnosis {
	return &Diagnosis{
		Pod:             pod,
		Status:          StatusUnknown,
		Issues:          make([]Issue, 0),
		Events:          make([]EventInfo, 0),
		Recommendations: make([]Recommendation, 0),
		DiagnosedAt:     time.Now(),
	}
}

// AddIssue adds an issue to the diagnosis
func (d *Diagnosis) AddIssue(issue Issue) {
	d.Issues = append(d.Issues, issue)
}

// AddRecommendation adds a recommendation to the diagnosis
func (d *Diagnosis) AddRecommendation(rec Recommendation) {
	d.Recommendations = append(d.Recommendations, rec)
}

// HasCriticalIssues returns true if there are any critical issues
func (d *Diagnosis) HasCriticalIssues() bool {
	for _, issue := range d.Issues {
		if issue.IsCritical() {
			return true
		}
	}
	return false
}

// IsHealthy returns true if no issues were found
func (d *Diagnosis) IsHealthy() bool {
	return len(d.Issues) == 0 && d.Status == StatusHealthy
}

// IssueCount returns the count of issues by severity
func (d *Diagnosis) IssueCount() (critical, warning, info int) {
	for _, issue := range d.Issues {
		switch issue.Severity {
		case SeverityCritical:
			critical++
		case SeverityWarning:
			warning++
		case SeverityInfo:
			info++
		}
	}
	return
}
