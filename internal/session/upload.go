package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Message is one chat message in an upload batch.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// UploadInput is the service-layer upload request.
type UploadInput struct {
	SessionKey string
	Source     string
	Messages   []Message
}

// UploadResult summarizes a stored turn.
type UploadResult struct {
	SessionID string
	TurnID    string
	TurnURI   string
	CreatedAt time.Time
}

// Service stores session turns.
type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Upload inserts or creates a session and appends a turn.
func (s *Service) Upload(ctx context.Context, in UploadInput) (UploadResult, error) {
	sessionKey := strings.ToLower(strings.TrimSpace(in.SessionKey))
	source := strings.ToLower(strings.TrimSpace(in.Source))
	if sessionKey == "" {
		return UploadResult{}, fmt.Errorf("session key required")
	}
	if len(in.Messages) == 0 {
		return UploadResult{}, fmt.Errorf("messages required")
	}

	now := time.Now().UTC()
	nowMS := now.UnixMilli()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return UploadResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var sessionID string
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM sessions WHERE session_key = ?`, sessionKey,
	).Scan(&sessionID)
	if err == sql.ErrNoRows {
		sessionID = uuid.NewString()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO sessions (id, session_key, source, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)`,
			sessionID, sessionKey, nullIfEmpty(source), nowMS, nowMS,
		)
		if err != nil {
			return UploadResult{}, fmt.Errorf("insert session: %w", err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO pipeline_state (session_id, l1_status, updated_at)
			VALUES (?, 'pending', ?)`,
			sessionID, nowMS,
		)
		if err != nil {
			return UploadResult{}, fmt.Errorf("insert pipeline_state: %w", err)
		}
	} else if err != nil {
		return UploadResult{}, fmt.Errorf("lookup session: %w", err)
	} else {
		if source != "" {
			_, err = tx.ExecContext(ctx,
				`UPDATE sessions SET updated_at = ?, source = ? WHERE id = ?`, nowMS, source, sessionID,
			)
		} else {
			_, err = tx.ExecContext(ctx,
				`UPDATE sessions SET updated_at = ? WHERE id = ?`, nowMS, sessionID,
			)
		}
		if err != nil {
			return UploadResult{}, fmt.Errorf("touch session: %w", err)
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE pipeline_state SET
				l1_status = 'pending',
				l1_turns_since_advanced = l1_turns_since_advanced + 1,
				updated_at = ?
			WHERE session_id = ?`,
			nowMS, sessionID,
		)
		if err != nil {
			return UploadResult{}, fmt.Errorf("queue l1: %w", err)
		}
	}

	messagesJSON, err := json.Marshal(in.Messages)
	if err != nil {
		return UploadResult{}, err
	}

	turnID := uuid.NewString()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO session_turns (id, session_id, messages_json, created_at, l1_status)
		VALUES (?, ?, ?, ?, 'pending')`,
		turnID, sessionID, string(messagesJSON), nowMS,
	)
	if err != nil {
		return UploadResult{}, fmt.Errorf("insert turn: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return UploadResult{}, err
	}

	return UploadResult{
		SessionID: sessionID,
		TurnID:    turnID,
		TurnURI:   "rmb://turns/" + turnID,
		CreatedAt: now,
	}, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
