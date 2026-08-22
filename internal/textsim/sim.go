// Package textsim provides the token-level similarity and slug-equivalence
// primitives used by the consolidation gates (issue #27, plan §9.3).
//
// Everything here is deterministic and offline: no embeddings, no LLM calls.
// The thresholds that build on these helpers (atom-dedup, body-unchanged) are
// calibrated against the audit's labeled clusters — see the constants in the
// callers — so paraphrase pairs stay above the bar while genuinely distinct
// facts stay below it.
package textsim

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"unicode"
)

// stopwords are dropped from token sets: they survive in almost every
// sentence, so keeping them deflates Jaccard for paraphrase pairs.
var stopwords = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "and": {}, "or": {}, "of": {}, "to": {},
	"in": {}, "on": {}, "for": {}, "is": {}, "are": {}, "was": {}, "were": {},
	"be": {}, "been": {}, "being": {}, "with": {}, "at": {}, "by": {}, "from": {},
	"as": {}, "that": {}, "this": {}, "it": {}, "its": {}, "user": {}, "users": {},
}

// Tokens lowercases s and splits it into a signature token SET: ASCII
// letter/digit words of length >= 2, plus each CJK ideograph as its own token
// (the store holds mixed Chinese/English content, where whitespace tokenizing
// alone would produce one giant token per sentence). Stopwords are dropped so
// "the user prefers docs in Chinese" and "user's documentation language
// preference is Chinese" share most tokens.
func Tokens(s string) map[string]struct{} {
	out := make(map[string]struct{})
	var cur strings.Builder
	flush := func() {
		w := cur.String()
		cur.Reset()
		if len(w) < 2 {
			return
		}
		if _, stop := stopwords[w]; stop {
			return
		}
		out[w] = struct{}{}
	}
	for _, r := range strings.ToLower(s) {
		switch {
		case isCJK(r):
			flush()
			out[string(r)] = struct{}{}
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			cur.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return out
}

func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0xF900 && r <= 0xFAFF) || (r >= 0x3000 && r <= 0x303F && r != 0x3005)
}

// Jaccard returns the Jaccard similarity of the token sets of a and b
// (|A∩B| / |A∪B|); 0 for two empty inputs, 1 for identical sets.
func Jaccard(a, b string) float64 {
	ta, tb := Tokens(a), Tokens(b)
	if len(ta) == 0 && len(tb) == 0 {
		return 0
	}
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	inter := 0
	for t := range ta {
		if _, ok := tb[t]; ok {
			inter++
		}
	}
	union := len(ta) + len(tb) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// slugTokens splits a slug into lowercase word tokens, singularized
// ("docs-language" -> [doc language]). Used for slug equivalence: the audit
// found the same subject extracted as both "doc-language" and "docs-language"
// because buckets are keyed by the LLM-chosen slug.
func slugTokens(slug string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(strings.ToLower(slug), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(f) < 2 {
			continue
		}
		out = append(out, singularize(f))
	}
	sort.Strings(out)
	return out
}

func singularize(w string) string {
	// Conservative English plural strip: only -ses/-ies/-s tails, never
	// short words ("is", "as") or 'ss' ("class", "process").
	if len(w) > 3 && strings.HasSuffix(w, "ies") {
		return w[:len(w)-3] + "y"
	}
	if len(w) > 3 && strings.HasSuffix(w, "ses") {
		return w[:len(w)-2]
	}
	if len(w) > 3 && strings.HasSuffix(w, "ss") {
		return w
	}
	if len(w) > 3 && strings.HasSuffix(w, "s") {
		return w[:len(w)-1]
	}
	return w
}

// SlugEqual reports whether two slugs denote the same subject under
// conservative normalization (exact match after tokenizing/singularizing).
// "docs-language" == "doc-language"; "redis-credentials" == "redis-credential";
// "bbc-deploy" != "bbc-build".
func SlugEqual(a, b string) bool {
	if strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) {
		return true
	}
	ta, tb := slugTokens(a), slugTokens(b)
	if len(ta) == 0 || len(tb) == 0 || len(ta) != len(tb) {
		return false
	}
	for i := range ta {
		if ta[i] != tb[i] {
			return false
		}
	}
	return true
}

// HashIDs returns a stable hex digest of an (unordered) ID set: sorted, joined
// with NUL, SHA-256. Atoms are append-only, so a bucket's atom-ID hash is a
// precise "evidence incorporated so far" fingerprint for the materiality gate
// (issue #27 task 1).
func HashIDs(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\x00")))
	return hex.EncodeToString(sum[:])
}
