package scene

import (
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/model"
)

func mkGroup(name string, n int) atomGroup {
	atoms := make([]model.Atom, n)
	for i := range atoms {
		atoms[i] = model.Atom{ID: name}
	}
	return atomGroup{DisplayName: name, Atoms: atoms}
}

func chunkSceneCounts(chunks [][]atomGroup) []int {
	out := make([]int, 0, len(chunks))
	for _, c := range chunks {
		out = append(out, len(c))
	}
	return out
}

func chunkAtomCounts(chunks [][]atomGroup) []int {
	out := make([]int, 0, len(chunks))
	for _, c := range chunks {
		n := 0
		for _, g := range c {
			n += len(g.Atoms)
		}
		out = append(out, n)
	}
	return out
}

func TestChunkGroupsSplitsBySceneCount(t *testing.T) {
	// 21 distinct scene groups, one atom each: scene limit (8) must dominate.
	var groups []atomGroup
	for i := 0; i < 21; i++ {
		groups = append(groups, mkGroup("s", 1))
	}
	chunks := chunkGroups(groups, 60, 8)
	if got := chunkSceneCounts(chunks); len(got) != 3 || got[0] != 8 || got[1] != 8 || got[2] != 5 {
		t.Fatalf("expected scene split [8 8 5], got %v", got)
	}
}

func TestChunkGroupsSplitsByAtomCount(t *testing.T) {
	// 3 groups of 30 atoms: atom limit (60) must dominate scene limit (8).
	groups := []atomGroup{mkGroup("a", 30), mkGroup("b", 30), mkGroup("c", 30)}
	chunks := chunkGroups(groups, 60, 8)
	if got := chunkAtomCounts(chunks); len(got) != 2 || got[0] != 60 || got[1] != 30 {
		t.Fatalf("expected atom split [60 30], got %v", got)
	}
}

func TestChunkGroupsBothLimits(t *testing.T) {
	// 4 groups of 3 atoms, maxScenes=2: split by scene even though atoms fit.
	groups := []atomGroup{
		mkGroup("a", 3), mkGroup("b", 3), mkGroup("c", 3), mkGroup("d", 3),
	}
	chunks := chunkGroups(groups, 100, 2)
	if got := chunkSceneCounts(chunks); len(got) != 2 || got[0] != 2 || got[1] != 2 {
		t.Fatalf("expected scene split [2 2], got %v", got)
	}
}

func TestChunkGroupsEmpty(t *testing.T) {
	chunks := chunkGroups(nil, 60, 8)
	if len(chunks) != 1 || len(chunks[0]) != 0 {
		t.Fatalf("expected single empty chunk, got %#v", chunks)
	}
}

func TestChunkGroupsOversizedGroupKeepsItsOwnChunk(t *testing.T) {
	// A single group larger than maxAtoms must not be dropped or split.
	groups := []atomGroup{mkGroup("big", 100), mkGroup("small", 1)}
	chunks := chunkGroups(groups, 60, 8)
	if got := chunkSceneCounts(chunks); len(got) != 2 || got[0] != 1 || got[1] != 1 {
		t.Fatalf("expected [1 1] (big group keeps its own chunk), got %v", got)
	}
	if len(chunks[0][0].Atoms) != 100 {
		t.Fatalf("big group lost atoms: %d", len(chunks[0][0].Atoms))
	}
}

func TestChunkGroupsZeroLimitsFallBack(t *testing.T) {
	// Zero limits fall back to defaults, so a 21-group session still splits.
	var groups []atomGroup
	for i := 0; i < 21; i++ {
		groups = append(groups, mkGroup("s", 1))
	}
	chunks := chunkGroups(groups, 0, 0)
	if got := chunkSceneCounts(chunks); len(got) != 3 {
		t.Fatalf("expected default scene split into 3 chunks, got %v", got)
	}
}
