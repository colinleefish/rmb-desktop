package textsim

import (
	"strings"
	"testing"
)

func TestTokens_MixedLanguageAndStopwords(t *testing.T) {
	toks := Tokens("The user prefers 文档 written in Chinese")
	// CJK text tokenizes per ideograph (see Tokens): 文档 -> 文, 档.
	for _, want := range []string{"prefers", "文", "档", "written", "chinese"} {
		if _, ok := toks[want]; !ok {
			t.Errorf("missing token %q in %v", want, toks)
		}
	}
	for _, drop := range []string{"the", "user", "in"} {
		if _, ok := toks[drop]; ok {
			t.Errorf("stopword %q must be dropped", drop)
		}
	}
}

func TestJaccard(t *testing.T) {
	// Near-verbatim restatement: high.
	restated := Jaccard("The user prefers documentation written in Chinese.",
		"Documentation written in Chinese is what the user prefers.")
	if restated < 0.6 {
		t.Errorf("near-verbatim restatement should score high, got %.2f", restated)
	}
	// Distinct facts: low.
	distinct := Jaccard("Jenkins home directory is /var/lib/jenkins.",
		"The user prefers documentation written in Chinese.")
	if distinct > 0.2 {
		t.Errorf("distinct facts should score low, got %.2f", distinct)
	}
	// Paraphrase-with-one-synonym-swap on a >=12-token body: the body-gate
	// fixture (calibration for bodyUnchangedJaccard = 0.80).
	original := "Speaks Chinese at home in Beijing works as an infrastructure engineer at HungryStudio uses a MacBook Pro and an iPhone daily prefers concise technical answers"
	paraphrase := strings.Replace(original, "Beijing", "based", 1)
	j := Jaccard(original, paraphrase)
	if j < 0.80 {
		t.Errorf("reworded body fixture must stay >= 0.80 (gate), got %.2f", j)
	}
	// Genuine update (one new fact bullet): below the gate.
	updated := original + " also brews tea every morning while reviewing logs"
	ju := Jaccard(original, updated)
	if ju >= 0.80 {
		t.Errorf("updated body fixture must fall below 0.80 (gate), got %.2f", ju)
	}
}

func TestSlugEqual(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"doc-language", "docs-language", true},
		{"redis-credentials", "redis-credential", true},
		{"redis-credentials", "redis-creds", false},
		{"bbc-deploy", "bbc-build", false},
		{"jenkins", "jenkins", true},
		{"", "x", false},
	}
	for _, c := range cases {
		if got := SlugEqual(c.a, c.b); got != c.want {
			t.Errorf("SlugEqual(%q,%q)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestHashIDs_OrderIndependent(t *testing.T) {
	a := HashIDs([]string{"x1", "x2", "x3"})
	b := HashIDs([]string{"x3", "x1", "x2"})
	if a == "" || a != b {
		t.Errorf("hash must be order-independent and non-empty: %q vs %q", a, b)
	}
	if HashIDs([]string{"x1", "x2"}) == a {
		t.Error("different ID sets must hash differently")
	}
	if HashIDs(nil) != "" {
		t.Error("empty input must hash to empty string")
	}
}
