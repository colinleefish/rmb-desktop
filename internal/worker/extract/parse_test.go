package extract

import (
	"testing"
)

func TestParseExtractResponse(t *testing.T) {
	raw := `{"atoms":[{"category":"profile","priority":80,"content":"User prefers Go","source_turn_indices":[0]}]}`
	atoms, err := parseExtractResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(atoms) != 1 {
		t.Fatalf("expected 1 atom, got %d", len(atoms))
	}
	if atoms[0].Category != "profile" {
		t.Fatalf("category=%s", atoms[0].Category)
	}
}

func TestParseExtractResponse_emptyAtoms(t *testing.T) {
	atoms, err := parseExtractResponse(`{"atoms":[]}`)
	if err != nil {
		t.Fatal(err)
	}
	if atoms != nil {
		t.Fatalf("expected nil, got %v", atoms)
	}
}
