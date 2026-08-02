package browse

// Overview is the dashboard home summary.
type Overview struct {
	Counts struct {
		Sessions       int64 `json:"sessions"`
		Turns          int64 `json:"turns"`
		Atoms          int64 `json:"atoms"`
		Scenes         int64 `json:"scenes"`
		Memories       int64 `json:"memories"`
		PipelineStates int64 `json:"pipeline_states"`
		Tasks          int64 `json:"tasks"`
		Corrections    int64 `json:"corrections"`
		Skills         int64 `json:"skills"`
	} `json:"counts"`
}

// SessionRow is one row in the sessions table view.
type SessionRow struct {
	ID         string  `json:"id"`
	SessionKey string  `json:"session_key"`
	Source     *string `json:"source,omitempty"`
	Status     string  `json:"status"`
	Abstract   *string `json:"abstract"`
	TurnCount  int64   `json:"turn_count"`
	AtomCount  int64   `json:"atom_count"`
	SceneCount int64   `json:"scene_count"`
	T1Status   string  `json:"t1_status,omitempty"`
	T2Status   string  `json:"t2_status,omitempty"`
	T3Status   string  `json:"t3_status,omitempty"`
	URI        string  `json:"uri"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
	LastTurnAt *string `json:"last_turn_at"`
}

// TurnRow is one conversation turn in session detail.
type TurnRow struct {
	ID            string `json:"id"`
	TurnIndex     int    `json:"turn_index"`
	URI           string `json:"uri"`
	MessagesJSONL string `json:"messages_jsonl"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// PipelineStateJSON mirrors the cloud API field names (t1/t2/t3).
type PipelineStateJSON struct {
	SessionID            string  `json:"session_id"`
	T1Status             string  `json:"t1_status"`
	T2Status             string  `json:"t2_status"`
	T3Status             string  `json:"t3_status"`
	T1AdvancedAt         *string `json:"t1_advanced_at,omitempty"`
	T2AdvancedAt         *string `json:"t2_advanced_at,omitempty"`
	T3AdvancedAt         *string `json:"t3_advanced_at,omitempty"`
	T1TurnsSinceAdvanced int     `json:"t1_turns_since_advanced"`
	WarmupThreshold      int     `json:"warmup_threshold"`
	UpdatedAt            string  `json:"updated_at"`
}

// AtomJSON is an L1 atom for list/detail views.
type AtomJSON struct {
	ID            string   `json:"id"`
	SessionID     string   `json:"session_id"`
	Category      string   `json:"category"`
	Priority      int      `json:"priority"`
	SceneName     *string  `json:"scene_name,omitempty"`
	Slug          *string  `json:"slug,omitempty"`
	Content       string   `json:"content"`
	SourceTurnIDs []string `json:"source_turn_ids"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
	URI           string   `json:"uri,omitempty"`
}

// SceneJSON is an L2 scene for list/detail views.
type SceneJSON struct {
	ID          string   `json:"id"`
	SessionID   string   `json:"session_id"`
	DisplayName *string  `json:"display_name,omitempty"`
	Abstract    *string  `json:"abstract,omitempty"`
	Body        *string  `json:"body,omitempty"`
	SourceAtoms []string `json:"source_atoms"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	URI         string   `json:"uri,omitempty"`
}

// MemoryJSON is an active L3 memory row.
type MemoryJSON struct {
	ID                   string   `json:"id"`
	URI                  string   `json:"uri"`
	Category             string   `json:"category"`
	Slug                 *string  `json:"slug,omitempty"`
	Version              int      `json:"version"`
	Abstract             *string  `json:"abstract,omitempty"`
	Body                 *string  `json:"body,omitempty"`
	SourceSceneURIs      []string `json:"source_scene_uris"`
	SourceCorrectionURIs []string `json:"source_correction_uris"`
	CreatedAt            string   `json:"created_at"`
	UpdatedAt            string   `json:"updated_at"`
}

// PipelineStateRow joins pipeline state with session key.
type PipelineStateRow struct {
	PipelineStateJSON
	SessionKey string `json:"session_key"`
	SessionURI string `json:"session_uri"`
}

// SessionDetail is the full session drill-down payload.
type SessionDetail struct {
	Session       SessionRow         `json:"session"`
	Turns         []TurnRow          `json:"turns"`
	PipelineState *PipelineStateJSON `json:"pipeline_state"`
	Atoms         []AtomJSON         `json:"atoms"`
	Scenes        []SceneJSON        `json:"scenes"`
}

// ListParams carries pagination and search for browse lists.
type ListParams struct {
	Limit    int
	Offset   int
	Query    string
	Category string
	Sort     string
	Order    string
}

// Page is the standard list response envelope.
type Page[T any] struct {
	Items  []T   `json:"items"`
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}
