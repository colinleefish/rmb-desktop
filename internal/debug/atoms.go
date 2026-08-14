package debug

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/model"
	"github.com/colinleefish/rmb-desktop/internal/uri"
)

// SerializeSessionAtoms builds the atoms JSON payload used by the T2 worker.
func SerializeSessionAtoms(ctx context.Context, database *sql.DB, sessionID string, cfg config.PipelineConfig) (string, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT id, session_id, category, priority, scene_name, slug, content, source_turn_ids, created_at, updated_at
		FROM atoms WHERE session_id = ? ORDER BY created_at ASC, id ASC`, sessionID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	type atomInput struct {
		URI       string  `json:"uri"`
		Category  string  `json:"category"`
		Priority  int     `json:"priority"`
		SceneName string  `json:"scene_name"`
		Slug      *string `json:"slug,omitempty"`
		Content   string  `json:"content"`
	}

	inputs := make([]atomInput, 0)
	for rows.Next() {
		var a model.Atom
		var sceneName, slug sql.NullString
		var sourceJSON string
		if err := rows.Scan(&a.ID, &a.SessionID, &a.Category, &a.Priority, &sceneName, &slug, &a.Content, &sourceJSON, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return "", err
		}
		name := "General"
		if sceneName.Valid && strings.TrimSpace(sceneName.String) != "" {
			name = strings.TrimSpace(sceneName.String)
		}
		var slugPtr *string
		if slug.Valid {
			slugPtr = &slug.String
		}
		inputs = append(inputs, atomInput{
			URI:       uri.BuildAtom(a.ID),
			Category:  a.Category,
			Priority:  a.Priority,
			SceneName: name,
			Slug:      slugPtr,
			Content:   a.Content,
		})
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(inputs) == 0 {
		return "", fmt.Errorf("session has no atoms")
	}

	maxAtoms := cfg.L2MaxAtoms
	if maxAtoms <= 0 {
		maxAtoms = 60
	}
	if len(inputs) > maxAtoms {
		inputs = inputs[:maxAtoms]
	}

	raw, err := json.Marshal(map[string]any{"atoms": inputs})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
