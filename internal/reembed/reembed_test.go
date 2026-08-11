package reembed_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/reembed"
)

func TestSettingsChanged(t *testing.T) {
	base := config.EmbedConfig{
		APIBase:    "https://api.openai.com/v1",
		APIKey:     "sk-old",
		Model:      "text-embedding-3-small",
		Dimensions: 1024,
	}
	sameKey := base
	sameKey.APIKey = "sk-new"
	if reembed.SettingsChanged(base, sameKey) {
		t.Fatal("api key change alone should not trigger re-embed")
	}
	otherModel := base
	otherModel.Model = "embedding-3"
	if !reembed.SettingsChanged(base, otherModel) {
		t.Fatal("model change should trigger re-embed")
	}
}

func TestClearAll(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "rmb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx := context.Background()
	if _, err := database.ExecContext(ctx, `
		INSERT INTO sessions (id, session_key, created_at, updated_at)
		VALUES ('s1', 'test-session', 1, 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO atoms (id, session_id, category, content, embedding, created_at, updated_at)
		VALUES ('a1', 's1', 'profile', 'hello', X'01', 1, 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO memories (id, uri, category, body, embedding, created_at, updated_at)
		VALUES ('m1', 'rmb://memories/profile/test', 'profile', 'fact', X'01', 1, 1)`); err != nil {
		t.Fatal(err)
	}

	if err := reembed.ClearAll(ctx, database); err != nil {
		t.Fatal(err)
	}

	var atomEmb []byte
	if err := database.QueryRow(`SELECT embedding FROM atoms WHERE id = 'a1'`).Scan(&atomEmb); err != nil {
		t.Fatal(err)
	}
	if atomEmb != nil {
		t.Fatal("expected atom embedding cleared")
	}
	var memEmb []byte
	if err := database.QueryRow(`SELECT embedding FROM memories WHERE id = 'm1'`).Scan(&memEmb); err != nil {
		t.Fatal(err)
	}
	if memEmb != nil {
		t.Fatal("expected memory embedding cleared")
	}
}
