package recall

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// crossTierDupThreshold is the cosine similarity above which a scene hit in
// the merged candidate list is treated as a cross-tier near-duplicate of a
// memory hit and suppressed (the memory wins; the scene is evidence).
//
// Calibrated 2026-08-22 against the live store (read-only analysis of the
// stored 1024-dim text-embedding-3-small vectors) using the audit's labeled
// duplicate clusters (treasure-hunter-retrieval-stress-test.md finding 6:
// "events and scenes frequently carry byte-identical text") as positives and
// 20,000 random active-memory × scene pairs as negatives:
//
//	positives (byte-identical event+scene text):
//	  events/2026-08-21-cluster-admin-toolbox-removal ↔ scenes/1c572bfd…
//	    cos = 0.9999986
//	related-but-distinct pairs that MUST survive:
//	  events/2026-08-21-reject-cluster-admin-toolbox  ↔ scenes/1c572bfd… cos = 0.7846
//	  events/2026-08-21-starlink-openresty-dynamic-dns ↔ scenes/a391a8fa… cos = 0.9009
//	negatives (random cross-tier pairs): max = 0.9526, p99.9 = 0.7719,
//	  0/20000 above 0.97
//
// 0.98 sits in the empty band between the strongest random negative (0.953)
// and byte-identical duplicates (≈1.0), and well above the related-but-
// distinct pairs, so only true content duplicates are fused.
const crossTierDupThreshold = 0.98

// maxCrossTierPairs bounds the cosine work of the fallback suppression pass.
// The merged candidate list holds ≤2k rows per tier, so pairs grow as 4k²;
// realistic k (≤20) needs ≤1600 cheap dot products, but an exhaustive
// --k=1000 dump would cost ~4G FLOPs. Beyond this budget the fallback is
// skipped — at that k the search is an enumeration, not a precision query.
const maxCrossTierPairs = 65536

// embedFetchChunk bounds the IN-list length of the batched embedding reads
// (SQLite's default variable limit is 999; stay well under it).
const embedFetchChunk = 400

// decodeVecFloat32 decodes a stored embedding BLOB into a float32 slice.
// sqlite-vec serializes float32 vectors as raw little-endian bytes
// (see sqlite_vec.SerializeFloat32), so this is its exact inverse. A
// malformed or empty blob yields nil.
func DecodeVecFloat32(b []byte) []float32 {
	if len(b) == 0 || len(b)%4 != 0 {
		return nil
	}
	out := make([]float32, len(b)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out
}

// cosineSim returns the cosine similarity of two vectors. Vectors of
// mismatched length, zero vectors, or nil inputs return 0 (never a match),
// which makes the suppression pass conservative on malformed rows.
func CosineSim(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		av, bv := float64(a[i]), float64(b[i])
		dot += av * bv
		na += av * av
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// fetchCrossTierEmbeddings loads the stored embeddings for the given memory
// URIs and scene URIs with two batched IN-queries (chunked under SQLite's
// variable limit). The result is keyed by full rmb:// URI for both tiers.
// Rows with no embedding (or an undecodable blob) are simply absent from the
// map, which suppresses nothing for that row.
func fetchCrossTierEmbeddings(ctx context.Context, database *sql.DB, memURIs, sceneURIs []string) (map[string][]float32, error) {
	out := make(map[string][]float32, len(memURIs)+len(sceneURIs))

	read := func(query string, keys []string) error {
		for start := 0; start < len(keys); start += embedFetchChunk {
			end := start + embedFetchChunk
			if end > len(keys) {
				end = len(keys)
			}
			chunk := keys[start:end]
			placeholders := make([]string, len(chunk))
			args := make([]any, len(chunk))
			for i, k := range chunk {
				placeholders[i] = "?"
				args[i] = k
			}
			rows, err := database.QueryContext(ctx, query+strings.Join(placeholders, ",")+")", args...)
			if err != nil {
				return err
			}
			for rows.Next() {
				var uri string
				var blob []byte
				if err := rows.Scan(&uri, &blob); err != nil {
					_ = rows.Close()
					return err
				}
				if vec := DecodeVecFloat32(blob); len(vec) > 0 {
					out[uri] = vec
				}
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return err
			}
			if err := rows.Close(); err != nil {
				return err
			}
		}
		return nil
	}

	if err := read(
		`SELECT uri, embedding FROM memories
		 WHERE superseded_at IS NULL AND embedding IS NOT NULL AND uri IN (`,
		memURIs); err != nil {
		return nil, fmt.Errorf("fetch memory embeddings: %w", err)
	}

	// Scene URIs are 'rmb://scenes/' || lower(id); compare on lower(id).
	sceneIDs := make([]string, len(sceneURIs))
	for i, u := range sceneURIs {
		sceneIDs[i] = strings.TrimPrefix(u, "rmb://scenes/")
	}
	if err := read(
		`SELECT 'rmb://scenes/' || lower(id), embedding FROM scenes
		 WHERE lower(id) IN (`,
		sceneIDs); err != nil {
		return nil, fmt.Errorf("fetch scene embeddings: %w", err)
	}

	return out, nil
}
