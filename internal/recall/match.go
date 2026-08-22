package recall

// Match is a single retrieval hit.
type Match struct {
	URI     string  `json:"uri"`
	Tier    string  `json:"tier"`
	Rank    float64 `json:"rank"`
	Snippet string  `json:"snippet"`
	// Version is the memory version count (trust signal: high churn means
	// heavily rewritten — verify via linked scene). Populated for the memory
	// tier only.
	Version int `json:"version,omitempty"`
	// SourceScenes are the memory's evidence scenes (drill-down targets).
	// Populated for the memory tier only; used for link-based suppression of
	// duplicated scene hits.
	SourceScenes []string `json:"source_scene_uris,omitempty"`
}

// DefaultScopes is used when no --scope flag is provided. Scenes are
// intentionally NOT a default tier: they are the evidence store, reachable by
// drill-down through a memory's source_scene_uris or explicitly via
// --scope=scene (plan §9.2, D1). Keeping them out of the default keeps one
// fact from flooding the result list across tiers.
var DefaultScopes = []string{"memory", "skill"}
