package eval

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/colinleefish/rmb-desktop/internal/db"
)

// Fixture is a portable, committed snapshot of a subset of the memory store.
// It is produced by `rmb-eval snapshot` against a read-only copy of the live
// DB and consumed by the runner to build a scratch rmb.db for in-process eval.
type Fixture struct {
	Version   int         `json:"version"`
	Source    string      `json:"source"`
	CreatedAt string      `json:"created_at"`
	Memories  []MemoryRow `json:"memories"`
	Scenes    []SceneRow  `json:"scenes"`
	Skills    []SkillRow  `json:"skills"`
}

type MemoryRow struct {
	ID           string `json:"id"`
	URI          string `json:"uri"`
	Category     string `json:"category"`
	Slug         string `json:"slug,omitempty"`
	Version      int    `json:"version"`
	SupersededAt *int64 `json:"superseded_at,omitempty"`
	Abstract     string `json:"abstract,omitempty"`
	Body         string `json:"body,omitempty"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

type SceneRow struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	DisplayName string `json:"display_name,omitempty"`
	Abstract    string `json:"abstract,omitempty"`
	Body        string `json:"body,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type SkillRow struct {
	ID           string `json:"id"`
	Slug         string `json:"slug"`
	URI          string `json:"uri"`
	Version      int    `json:"version"`
	SupersededAt *int64 `json:"superseded_at,omitempty"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	FTSText      string `json:"fts_text"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

// BuildDB materializes the fixture into a fresh scratch rmb.db (migrations,
// FTS rows, hash embeddings) and returns the opened handle. Caller closes.
func (f *Fixture) BuildDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	_ = os.Remove(path)
	database, err := db.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open scratch db: %w", err)
	}
	for _, m := range f.Memories {
		if _, err := database.Exec(`
			INSERT INTO memories (id, uri, category, slug, version, superseded_at, abstract, body,
				source_scene_uris, source_correction_uris, embedding, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, '[]', '[]', ?, ?, ?)`,
			m.ID, m.URI, m.Category, m.Slug, m.Version, m.SupersededAt, m.Abstract, m.Body,
			embedBlob(m.Abstract, m.Body), m.CreatedAt, m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("insert memory %s: %w", m.URI, err)
		}
		// memories has no FTS triggers (managed by the distiller); seed manually.
		if _, err := database.Exec(`
			INSERT INTO memories_fts (rowid, abstract, body)
			VALUES ((SELECT rowid FROM memories WHERE id = ?), ?, ?)`, m.ID, m.Abstract, m.Body); err != nil {
			return nil, fmt.Errorf("insert memories_fts %s: %w", m.URI, err)
		}
	}
	for _, s := range f.Scenes {
		if _, err := database.Exec(`INSERT OR IGNORE INTO sessions (id, session_key, abstract, created_at, updated_at)
			VALUES (?, ?, '', ?, ?)`, s.SessionID, "fixture-"+s.SessionID, s.CreatedAt, s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("insert session %s: %w", s.SessionID, err)
		}
		if _, err := database.Exec(`
			INSERT INTO scenes (id, session_id, display_name, abstract, body, source_atoms, embedding, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, '[]', ?, ?, ?)`,
			s.ID, s.SessionID, s.DisplayName, s.Abstract, s.Body, embedBlob(s.Abstract, s.Body), s.CreatedAt, s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("insert scene %s: %w", s.ID, err)
		}
	}
	for _, k := range f.Skills {
		if _, err := database.Exec(`
			INSERT INTO skills (id, slug, uri, version, superseded_at, name, description, tags, bundle_sha256, fts_text, embedding, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, '[]', '', ?, ?, ?, ?)`,
			k.ID, k.Slug, k.URI, k.Version, k.SupersededAt, k.Name, k.Description,
			k.FTSText, embedBlob(k.Description, k.FTSText), k.CreatedAt, k.UpdatedAt); err != nil {
			return nil, fmt.Errorf("insert skill %s: %w", k.URI, err)
		}
	}
	return database, nil
}

func embedBlob(texts ...string) []byte {
	joined := ""
	for _, t := range texts {
		joined += "\n" + t
	}
	blob, err := sqlite_vec.SerializeFloat32(HashEmbed(joined))
	if err != nil {
		panic("hash embed serialize: " + err.Error())
	}
	return blob
}
