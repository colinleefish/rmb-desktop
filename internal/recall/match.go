package recall

// Match is a single retrieval hit.
type Match struct {
	URI     string  `json:"uri"`
	Tier    string  `json:"tier"`
	Rank    float64 `json:"rank"`
	Snippet string  `json:"snippet"`
}

// DefaultScopes is used when no --scope flag is provided.
var DefaultScopes = []string{"memory", "scene"}
