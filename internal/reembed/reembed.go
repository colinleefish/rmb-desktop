package reembed

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/config"
)

// SettingsChanged reports whether embed provider settings changed in a way that
// invalidates stored vectors (model, base URL, or dimensions).
func SettingsChanged(before, after config.EmbedConfig) bool {
	return fingerprint(before) != fingerprint(after)
}

func fingerprint(c config.EmbedConfig) string {
	return strings.TrimSpace(c.APIBase) + "\x00" +
		strings.TrimSpace(c.Model) + "\x00" +
		fmt.Sprintf("%d", c.Dimensions)
}

// ClearAll nulls embeddings so the embed worker re-embeds every row.
func ClearAll(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`UPDATE atoms SET embedding = NULL WHERE embedding IS NOT NULL`,
		`UPDATE scenes SET embedding = NULL WHERE embedding IS NOT NULL`,
		`UPDATE memories SET embedding = NULL WHERE embedding IS NOT NULL AND superseded_at IS NULL`,
		`UPDATE skills SET embedding = NULL WHERE embedding IS NOT NULL AND superseded_at IS NULL`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("clear embeddings: %w", err)
		}
	}
	return nil
}
