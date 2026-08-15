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
	MemoryByCategory MemoryCategoryOverview `json:"memory_by_category"`
}

// MemoryCategoryOverview is per-category memory stats for the sidebar.
type MemoryCategoryOverview struct {
	ProfileVersion int   `json:"profile_version"`
	Events         int64 `json:"events"`
	Preferences    int64 `json:"preferences"`
	Entities       int64 `json:"entities"`
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

// PipelineStatusCounts is pending/running/failed/idle/waiting for one tier.
type PipelineStatusCounts struct {
	Pending int64 `json:"pending"`
	Running int64 `json:"running"`
	Failed  int64 `json:"failed"`
	Idle    int64 `json:"idle"`
	Waiting int64 `json:"waiting"` // status idle but stage never advanced (not reached yet)
}

// PipelineFunnel is how many sessions finished each distillation stage.
type PipelineFunnel struct {
	Sessions int64 `json:"sessions"`
	T1Done   int64 `json:"t1_done"`
	T2Done   int64 `json:"t2_done"`
	T3Done   int64 `json:"t3_done"`
}

// PipelineProblem is a failed or long-pending session stage.
type PipelineProblem struct {
	SessionKey string `json:"session_key"`
	SessionURI string `json:"session_uri"`
	Stage      string `json:"stage"`
	Status     string `json:"status"`
	UpdatedAt  string `json:"updated_at"`
	Reason     string `json:"reason,omitempty"`
}

// PipelineHealth is the global distillation dashboard payload.
type PipelineHealth struct {
	DistillationEnabled bool   `json:"distillation_enabled"`
	TrackedSessions     int64  `json:"tracked_sessions"`
	GeneratedAt         string `json:"generated_at"`
	Stages              struct {
		T1 PipelineStatusCounts `json:"t1"`
		T2 PipelineStatusCounts `json:"t2"`
		T3 PipelineStatusCounts `json:"t3"`
	} `json:"stages"`
	Funnel   PipelineFunnel    `json:"funnel"`
	Problems []PipelineProblem `json:"problems"`
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
	T1LastError          *string `json:"t1_last_error,omitempty"`
	T2LastError          *string `json:"t2_last_error,omitempty"`
	T3LastError          *string `json:"t3_last_error,omitempty"`
	T1StartedAt          *string `json:"t1_started_at,omitempty"`
	T2StartedAt          *string `json:"t2_started_at,omitempty"`
	T3StartedAt          *string `json:"t3_started_at,omitempty"`
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
	ID          string       `json:"id"`
	SessionID   string       `json:"session_id"`
	DisplayName *string      `json:"display_name,omitempty"`
	Abstract    *string      `json:"abstract,omitempty"`
	Body        *string      `json:"body,omitempty"`
	SourceAtoms []string     `json:"source_atoms"`
	CreatedAt   string       `json:"created_at"`
	UpdatedAt   string       `json:"updated_at"`
	URI         string       `json:"uri,omitempty"`
	RecallStats *RecallStats `json:"recall_stats,omitempty"`
}

// RecallStats holds per-URI recall counters.
type RecallStats struct {
	URI            string  `json:"uri"`
	SearchCount    int64   `json:"search_count"`
	CatCount       int64   `json:"cat_count"`
	MetaCount      int64   `json:"meta_count"`
	LastSearchedAt *string `json:"last_searched_at,omitempty"`
	LastCatedAt    *string `json:"last_cated_at,omitempty"`
	LastMetaedAt   *string `json:"last_metaed_at,omitempty"`
	UpdatedAt      string  `json:"updated_at"`
}

// MemoryJSON is an active L3 memory row.
type MemoryJSON struct {
	ID                   string       `json:"id"`
	URI                  string       `json:"uri"`
	Category             string       `json:"category"`
	Slug                 *string      `json:"slug,omitempty"`
	Version              int          `json:"version"`
	Abstract             *string      `json:"abstract,omitempty"`
	Body                 *string      `json:"body,omitempty"`
	SourceSceneURIs      []string     `json:"source_scene_uris"`
	SourceCorrectionURIs []string     `json:"source_correction_uris"`
	CreatedAt            string       `json:"created_at"`
	UpdatedAt            string       `json:"updated_at"`
	RecallStats          *RecallStats `json:"recall_stats,omitempty"`
}

// SessionDetail is the full session drill-down payload.
type SessionDetail struct {
	Session       SessionRow         `json:"session"`
	Turns         []TurnRow          `json:"turns"`
	PipelineState *PipelineStateJSON `json:"pipeline_state"`
	Atoms         []AtomJSON         `json:"atoms"`
	Scenes        []SceneJSON        `json:"scenes"`
}

// SkillRow is one row in the skills catalog.
type SkillRow struct {
	Slug        string       `json:"slug"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Tags        []string     `json:"tags"`
	URI         string       `json:"uri"`
	Version     int          `json:"version"`
	UpdatedAt   string       `json:"updated_at"`
	RecallStats *RecallStats `json:"recall_stats,omitempty"`
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
