package db

import (
	"database/sql"

	"github.com/colinleefish/rmb-desktop/internal/model"
)

// ScanAtomRows scans the full atoms column set (id, session_id, category,
// priority, scene_name, slug, content, source_turn_ids, created_at,
// updated_at) into model.Atom values. It consumes rows and returns rows.Err().
func ScanAtomRows(rows *sql.Rows) ([]model.Atom, error) {
	var out []model.Atom
	for rows.Next() {
		var a model.Atom
		var sceneName, slug sql.NullString
		var sourceJSON string
		if err := rows.Scan(&a.ID, &a.SessionID, &a.Category, &a.Priority, &sceneName, &slug, &a.Content, &sourceJSON, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		if sceneName.Valid {
			a.SceneName = &sceneName.String
		}
		if slug.Valid {
			a.Slug = &slug.String
		}
		var err error
		a.SourceTurnIDs, err = UnmarshalStringArray(sourceJSON)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
