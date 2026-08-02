package correction

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// EnqueueSessionsForMemoryTargets marks L3 pending for sessions that own the
// targeted memories so the next rollup re-distills those buckets.
func EnqueueSessionsForMemoryTargets(ctx context.Context, database *sql.DB, targetURIs []string) (int, error) {
	if len(targetURIs) == 0 {
		return 0, nil
	}

	nowMS := time.Now().UTC().UnixMilli()
	seen := make(map[string]struct{})
	for _, target := range targetURIs {
		rows, err := database.QueryContext(ctx, `
			SELECT DISTINCT sc.session_id
			FROM memories m
			JOIN json_each(m.source_scene_uris) je
			JOIN scenes sc ON ('rmb://scenes/' || lower(sc.id)) = je.value
			WHERE m.superseded_at IS NULL AND m.uri = ?`, target)
		if err != nil {
			return 0, fmt.Errorf("resolve correction target sessions: %w", err)
		}
		for rows.Next() {
			var sessionID string
			if err := rows.Scan(&sessionID); err != nil {
				rows.Close()
				return 0, err
			}
			seen[sessionID] = struct{}{}
		}
		if err := rows.Close(); err != nil {
			return 0, err
		}
	}

	for sessionID := range seen {
		_, err := database.ExecContext(ctx, `
			UPDATE pipeline_state SET l3_status = 'pending', updated_at = ?
			WHERE session_id = ?`, nowMS, sessionID)
		if err != nil {
			return 0, fmt.Errorf("enqueue session %s: %w", sessionID, err)
		}
	}
	return len(seen), nil
}
