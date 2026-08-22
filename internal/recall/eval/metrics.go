package eval

import (
	"strings"
	"unicode"
)

// GoldenQuestion is one fixture in the golden set.
type GoldenQuestion struct {
	ID       string   `json:"id" yaml:"id"`
	Query    string   `json:"query" yaml:"query"`
	Expected []string `json:"expected" yaml:"expected"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Latest   string   `json:"latest,omitempty" yaml:"latest,omitempty"`
}

// GoldenSet is the parsed golden.yaml file.
type GoldenSet struct {
	Gates     GoldenGates      `json:"gates" yaml:"gates"`
	Questions []GoldenQuestion `json:"questions" yaml:"questions"`
}

type GoldenGates struct {
	MinRecallAt5        float64 `json:"min_recall_at_5" yaml:"min_recall_at_5"`
	MaxDupRate          float64 `json:"max_dup_rate" yaml:"max_dup_rate"`
	MinRecencyPrecision float64 `json:"min_recency_precision" yaml:"min_recency_precision"`
}

// QuestionResult is the outcome for a single golden question.
type QuestionResult struct {
	ID        string   `json:"id"`
	Query     string   `json:"query"`
	Tags      []string `json:"tags,omitempty"`
	Top5      []string `json:"top5"`
	Hit       bool     `json:"hit"`
	RecencyOK *bool    `json:"recency_ok,omitempty"`
}

// Report aggregates results across the golden set.
type Report struct {
	Questions        []QuestionResult `json:"questions"`
	RecallAt5        float64          `json:"recall_at_5"`
	DupRate          float64          `json:"dup_rate_top5"`
	RecencyPrecision float64          `json:"recency_precision"`
	TotalQuestions   int              `json:"total_questions"`
	Passed           int              `json:"passed_questions"`
}

// NormKey reduces a snippet to a comparable key (lowercase alphanumerics), so
// near-identical bodies can be detected regardless of punctuation/whitespace.
func NormKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// IsNearDuplicate reports whether b's content is effectively the same as a's
// (equal after normalization, or one fully contained in the other at near-full
// length). Used for dup-rate in the top-5.
func IsNearDuplicate(a, b string) bool {
	na, nb := NormKey(a), NormKey(b)
	if na == "" || nb == "" {
		return false
	}
	if na == nb {
		return true
	}
	if len(na) >= len(nb) {
		return strings.Contains(na, nb) && float64(len(nb))/float64(len(na)) > 0.9
	}
	return strings.Contains(nb, na) && float64(len(na))/float64(len(nb)) > 0.9
}
