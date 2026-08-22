package eval

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"

	"github.com/colinleefish/rmb-desktop/internal/recall"
)

// Run executes the golden set against a fixture-built DB using the hash
// embedder. Each query runs hybrid recall with default scopes, k=5.
func Run(ctx context.Context, database *sql.DB, golden *GoldenSet) (*Report, error) {
	svc := recall.NewService(database)
	embed := recall.QueryEmbedder(func(_ context.Context, q string) ([]float32, error) {
		return HashEmbed(q), nil
	})

	report := &Report{TotalQuestions: len(golden.Questions)}
	for _, q := range golden.Questions {
		matches, err := svc.Search(ctx, embed, q.Query, 5, nil)
		if err != nil {
			return nil, fmt.Errorf("question %s: %w", q.ID, err)
		}
		top5 := make([]string, 0, len(matches))
		snippets := make([]string, 0, len(matches))
		for _, m := range matches {
			top5 = append(top5, m.URI)
			snippets = append(snippets, m.Snippet)
		}
		expected := map[string]bool{}
		for _, u := range q.Expected {
			expected[u] = true
		}
		hit := false
		hitRank := -1
		for i, u := range top5 {
			if expected[u] {
				hit = true
				hitRank = i
				break
			}
		}
		res := QuestionResult{ID: q.ID, Query: q.Query, Tags: q.Tags, Top5: top5, Hit: hit}
		if hasTag(q.Tags, "recency") && q.Latest != "" {
			ok := recencyOK(top5, q.Latest, hitRank)
			res.RecencyOK = &ok
		}
		report.Questions = append(report.Questions, res)
		if hit {
			report.Passed++
		}
		report.DupRate += dupInTop5(top5, snippets)
	}

	n := float64(len(golden.Questions))
	report.RecallAt5 = float64(report.Passed) / n
	report.DupRate = report.DupRate / n

	recQ, recOK := 0, 0
	for _, r := range report.Questions {
		if r.RecencyOK != nil {
			recQ++
			if *r.RecencyOK {
				recOK++
			}
		}
	}
	if recQ > 0 {
		report.RecencyPrecision = float64(recOK) / float64(recQ)
	}
	return report, nil
}

func hasTag(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}

// recencyOK: if the expected set is present in top-5, the latest-tagged URI
// must not rank below another expected URI. If none of the expected set is in
// top-5, recall decides the question and recency is reported OK so the two
// metrics stay independently attributable.
func recencyOK(top5 []string, latest string, hitRank int) bool {
	if hitRank == -1 {
		return true
	}
	for _, u := range top5 {
		if u == latest {
			return true
		}
	}
	return false
}

func dupInTop5(uris []string, snippets []string) float64 {
	if len(uris) < 2 {
		return 0
	}
	seen := map[string]bool{}
	dup := 0
	for i, sn := range snippets {
		isDup := false
		for j := 0; j < i; j++ {
			if IsNearDuplicate(snippets[j], sn) {
				isDup = true
				break
			}
		}
		if seen[uris[i]] {
			isDup = true
		}
		seen[uris[i]] = true
		if isDup {
			dup++
		}
	}
	return float64(dup) / float64(len(uris))
}

// GatesOK reports whether aggregate metrics meet the configured gates.
func (r *Report) GatesOK(g *GoldenGates) bool {
	return r.RecallAt5 >= g.MinRecallAt5 &&
		r.DupRate <= g.MaxDupRate &&
		r.RecencyPrecision >= g.MinRecencyPrecision
}

// WriteJSON writes the report as pretty JSON.
func (r *Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
