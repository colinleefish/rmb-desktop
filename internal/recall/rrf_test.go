package recall

import "testing"

func TestFuseRRF_vectorPrimary(t *testing.T) {
	vector := []string{"rmb://entities/k8s", "rmb://profile", "rmb://atoms/a1"}
	fts := []string{"rmb://profile", "rmb://entities/k8s", "rmb://atoms/b2"}

	got := FuseRRF(vector, fts, 3, 0.7, 0.3)
	if len(got) != 3 {
		t.Fatalf("len=%d", len(got))
	}
	// profile and k8s appear high in both lists
	if got[0].URI != "rmb://profile" && got[0].URI != "rmb://entities/k8s" {
		t.Fatalf("top hit: %s", got[0].URI)
	}
}
