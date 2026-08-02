package model

const (
	PipelineStatusIdle    = "idle"
	PipelineStatusPending = "pending"
	PipelineStatusRunning = "running"
	PipelineStatusFailed  = "failed"
)

const (
	AtomCategoryProfile     = "profile"
	AtomCategoryPreferences = "preferences"
	AtomCategoryEntities    = "entities"
	AtomCategoryEvents      = "events"
)

var ValidAtomCategories = map[string]struct{}{
	AtomCategoryProfile:     {},
	AtomCategoryPreferences: {},
	AtomCategoryEntities:    {},
	AtomCategoryEvents:      {},
}
