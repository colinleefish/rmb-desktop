package agentmemory

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/uri"
	"github.com/google/uuid"
)

//go:embed agent_guide.md
var agentGuideBody string

const agentAbstract = "Agent recall guide"

// AgentGuideBody returns the bundled rmb://agent guide content.
func AgentGuideBody() string {
	return strings.TrimSpace(agentGuideBody)
}

// UpsertAgentGuide creates or updates the curated rmb://agent singleton
// from the bundled guide. When the active row already matches, this is a no-op.
func UpsertAgentGuide(ctx context.Context, db *sql.DB) error {
	return UpsertAgentGuideBody(ctx, db, AgentGuideBody())
}

// UpsertAgentGuideBody creates or updates rmb://agent with the given body.
func UpsertAgentGuideBody(ctx context.Context, db *sql.DB, body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return fmt.Errorf("agent guide body must not be empty")
	}

	targetURI := uri.BuildAgent()
	nowMS := time.Now().UTC().UnixMilli()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin agent guide upsert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var activeID, activeBody string
	var version int
	err = tx.QueryRowContext(ctx, `
		SELECT id, COALESCE(body, ''), version FROM memories
		WHERE uri = ? AND superseded_at IS NULL`, targetURI,
	).Scan(&activeID, &activeBody, &version)
	switch {
	case err == sql.ErrNoRows:
		if err := insertAgentGuide(ctx, tx, body, 1, nowMS); err != nil {
			return err
		}
	case err != nil:
		return fmt.Errorf("load active agent guide: %w", err)
	case activeBody == body:
		return tx.Commit()
	default:
		if _, err := tx.ExecContext(ctx, `UPDATE memories SET superseded_at = ? WHERE id = ?`, nowMS, activeID); err != nil {
			return fmt.Errorf("supersede agent guide: %w", err)
		}
		if err := insertAgentGuide(ctx, tx, body, version+1, nowMS); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit agent guide upsert: %w", err)
	}
	return nil
}

func insertAgentGuide(ctx context.Context, tx *sql.Tx, body string, version int, nowMS int64) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate agent guide id: %w", err)
	}
	targetURI := uri.BuildAgent()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO memories (
			id, uri, category, slug, version, abstract, body,
			source_scene_uris, source_correction_uris, created_at, updated_at
		) VALUES (?, ?, 'agent', NULL, ?, ?, ?, '[]', '[]', ?, ?)`,
		id.String(), targetURI, version, agentAbstract, body, nowMS, nowMS,
	)
	if err != nil {
		return fmt.Errorf("insert agent guide: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO memories_fts (rowid, abstract, body)
		VALUES ((SELECT rowid FROM memories WHERE id = ?), ?, ?)`,
		id.String(), agentAbstract, body,
	)
	if err != nil {
		return fmt.Errorf("index agent guide fts: %w", err)
	}
	return nil
}
