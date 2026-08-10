package setup

// ChangeType describes how an artifact would change on disk.
type ChangeType string

const (
	ChangeCreate   ChangeType = "create"
	ChangeModify   ChangeType = "modify"
	ChangeAppend   ChangeType = "append"
	ChangeUnchanged ChangeType = "unchanged"
)

// ApplyMode controls whether the UI can write the artifact directly.
type ApplyMode string

const (
	ApplyWrite     ApplyMode = "write"
	ApplyCopyOnly  ApplyMode = "copy_only"
)

// DisplayMode controls how the integration UI presents an artifact.
type DisplayMode string

const (
	DisplayDiff     DisplayMode = "diff"
	DisplayReplace  DisplayMode = "replace"
)

// Artifact is one file or config target shown in the integration UI.
type Artifact struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Path        string      `json:"path"`
	Description string      `json:"description"`
	Exists      bool        `json:"exists"`
	Current     string      `json:"current"`
	Proposed    string      `json:"proposed"`
	ChangeType  ChangeType  `json:"change_type"`
	ApplyMode   ApplyMode   `json:"apply_mode"`
	DisplayMode DisplayMode `json:"display_mode"`
	Warnings    []string    `json:"warnings"`
	Language    string      `json:"language"`
}

// AgentState is the preview payload for one coding agent.
type AgentState struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Detected     bool     `json:"detected"`
	HookStatus   string   `json:"hook_status"`
	RecallStatus string   `json:"recall_status"`
	Artifacts    []Artifact `json:"artifacts"`
}

// AgentSummary is returned by the status endpoint.
type AgentSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Detected     bool   `json:"detected"`
	HookStatus   string `json:"hook_status"`
	RecallStatus string `json:"recall_status"`
}

// StatusResponse lists all supported agents.
type StatusResponse struct {
	Agents []AgentSummary `json:"agents"`
}

// PreviewResponse wraps a single agent preview.
type PreviewResponse struct {
	Agent AgentState `json:"agent"`
}

// ApplyRequest selects artifacts to write.
type ApplyRequest struct {
	Artifacts []string `json:"artifacts"`
}

// ApplyResponse returns post-apply preview.
type ApplyResponse struct {
	Applied []string   `json:"applied"`
	Agent   AgentState `json:"agent"`
}
