package model

// Atom is an L1 extracted fact.
type Atom struct {
	ID            string
	SessionID     string
	Category      string
	Priority      int
	SceneName     *string
	Slug          *string
	Content       string
	SourceTurnIDs []string
	CreatedAt     int64
	UpdatedAt     int64
}

// Scene is an L2 per-session summary segment.
type Scene struct {
	ID          string
	SessionID   string
	DisplayName *string
	Abstract    *string
	Body        *string
	SourceAtoms []string
	CreatedAt   int64
	UpdatedAt   int64
}

// Memory is an L3 versioned long-term fact.
type Memory struct {
	ID                   string
	URI                  string
	Category             string
	Slug                 *string
	Version              int
	SupersededAt         *int64
	Abstract             *string
	Body                 *string
	SourceSceneURIs      []string
	SourceCorrectionURIs []string
	CreatedAt            int64
	UpdatedAt            int64
}

// SessionTurn is a raw conversation exchange.
type SessionTurn struct {
	ID            string
	SessionID     string
	MessagesJSON  string
	CreatedAt     int64
	L1ExtractedAt *int64
}

// PipelineState tracks per-session distillation progress.
type PipelineState struct {
	SessionID            string
	L1Status             string
	L2Status             string
	L3Status             string
	L1AdvancedAt         *int64
	L2AdvancedAt         *int64
	L3AdvancedAt         *int64
	L1TurnsSinceAdvanced int
	WarmupThreshold      int
	UpdatedAt            int64
}
