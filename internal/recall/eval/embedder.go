// Package eval provides a golden-set retrieval evaluation harness for the
// recall pipeline. It exists so every ranking/pipeline change has a
// regression gate (issue #22).
//
// Determinism: the harness runs fully in-process against a fixture DB built
// from a read-only snapshot of the live store. Query and document embeddings
// are produced by HashEmbed — a deterministic, hash-based bag-of-tokens
// embedder — so the eval runs offline and reproducibly in CI.
//
// Accuracy tradeoff: HashEmbed approximates semantic similarity with lexical
// overlap (shared tokens => similar vectors). Cross-lingual and paraphrase
// recall therefore measures mostly the FTS leg plus exact-token overlap, and
// numbers are a lower bound compared to the live embedder. For a full-fidelity
// local run, snapshot the live DB and re-embed documents with the production
// embedder (out of scope for v1).
package eval

import (
	"math"
	"math/rand"
	"strings"
	"unicode"
)

// EmbedDim is the dimension of HashEmbed vectors. Any consistent dimension
// works because the eval DB is re-embedded wholesale; 256 keeps the fixture
// vectors cheap while giving tokens enough room to decorrelate.
const EmbedDim = 256

// HashEmbed deterministically embeds text into a unit-normalized vector.
// Each token seeds an RNG (FNV-derived) and contributes a random direction;
// texts sharing tokens end up cosine-similar. It never calls the network.
func HashEmbed(text string) []float32 {
	tokens := Tokenize(text)
	vec := make([]float32, EmbedDim)
	for _, tok := range tokens {
		h := fnv64(tok)
		seed := int64(h>>1) ^ int64(len(tok))
		rng := rand.New(rand.NewSource(seed))
		for i := range vec {
			vec[i] += rng.Float32()*2 - 1
		}
	}
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if norm == 0 {
		return vec
	}
	s := float32(1 / math.Sqrt(norm))
	for i := range vec {
		vec[i] *= s
	}
	return vec
}

// Tokenize splits text into lowercase alphanumeric words, individual CJK
// runes, and CJK bigrams (so Chinese queries overlap stored Chinese text
// beyond single characters).
func Tokenize(text string) []string {
	var out []string
	var runes []rune
	flushWord := func() {
		if len(runes) > 0 {
			out = append(out, strings.ToLower(string(runes)))
			runes = runes[:0]
		}
	}
	var cjk []rune
	flushCJK := func() {
		for _, r := range cjk {
			out = append(out, string(r))
		}
		for i := 0; i+1 < len(cjk); i++ {
			out = append(out, string(cjk[i:i+2]))
		}
		cjk = cjk[:0]
	}
	for _, r := range text {
		switch {
		case isCJK(r):
			flushWord()
			cjk = append(cjk, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			flushCJK()
			runes = append(runes, r)
		default:
			flushWord()
			flushCJK()
		}
	}
	flushWord()
	flushCJK()
	return out
}

func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0xF900 && r <= 0xFAFF) || (r >= 0x3040 && r <= 0x30FF)
}

func fnv64(s string) uint64 {
	h := uint64(0xcbf29ce484222325)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 0x100000001b3
	}
	return h
}
