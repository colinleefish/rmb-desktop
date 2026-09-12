package memory

import (
	"sync"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/model"
)

// TestB05_incumbentMaterialityReadSkew reproduces issue #72: when a variant
// bucket merges into the incumbent URI before the incumbent bucket's materiality
// SELECT, the incumbent must not re-distill and double-bump the version.
func TestB05_incumbentMaterialityReadSkew(t *testing.T) {
	const incumbentURI = "rmb://preferences/redis-credentials-storage"

	database := openWorkerTestDB(t)
	defer database.Close()

	insertPendingSession(t, database, "s1")
	insertPendingSession(t, database, "s2")
	insertAtom(t, database, "a1", "s1", model.AtomCategoryPreferences, "redis-credentials-storage",
		"Redis credentials live in the ops vault, never in repos.")
	insertAtom(t, database, "a2", "s2", model.AtomCategoryPreferences, "redis-credentials-storage",
		"Redis credentials live in the ops vault, never in repos.")
	w := NewWorker(database, &recordingDistiller{}, nil, testCfg(), nil, nil)
	if err := w.rollup(t.Context()); err != nil {
		t.Fatal(err)
	}

	embedIncumbent(t, database, incumbentURI, []float32{1, 0, 0, 0})

	insertPendingSession(t, database, "s3")
	insertPendingSession(t, database, "s4")
	insertAtom(t, database, "a3", "s3", model.AtomCategoryPreferences, "redis-secrets",
		"Redis credentials live in the ops vault, never in repos.")
	insertAtom(t, database, "a4", "s4", model.AtomCategoryPreferences, "redis-secrets",
		"Redis credentials live in the ops vault, never in repos.")

	incumbentReady := make(chan struct{})
	mergeDone := make(chan struct{})
	var mergeOnce sync.Once

	t.Cleanup(func() { rollupTestHook = nil })
	rollupTestHook = &RollupTestHook{
		BypassURILock: true,
		BeforeBucketUnchangedQuery: func(uri string) {
			if uri != incumbentURI {
				return
			}
			close(incumbentReady)
			<-mergeDone
		},
		BeforeMergeIncumbentCommit: func(uri string) {
			if uri != incumbentURI {
				return
			}
			<-incumbentReady
		},
		AfterMergeIncumbentCommit: func(uri string) {
			if uri != incumbentURI {
				return
			}
			mergeOnce.Do(func() { close(mergeDone) })
		},
	}

	wEmbed := NewWorker(database, &recordingDistiller{}, stubEmbedder{vec: []float32{1, 0, 0, 0}}, testCfg(), nil, nil)
	if err := wEmbed.rollup(t.Context()); err != nil {
		t.Fatal(err)
	}

	var version int
	if err := database.QueryRow(`SELECT version FROM memories WHERE uri=? AND superseded_at IS NULL`, incumbentURI).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("B05: incumbent must stay at v2 after cosine merge, got %d", version)
	}
}
