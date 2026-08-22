package memory

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/recall"
)

// eventSlugDatePrefix matches the YYYY-MM-DD- prefix convention for event
// slugs ("2026-07-16-soft-delete-one-tag-solutions").
var eventSlugDatePrefix = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-`)

// relatedEvents retrieves the top relatedEventsTopK active event memories for
// a bucket via FTS (retrieve-then-link, P2.2 / issue #28). The caller injects
// them into the distill prompt so a resolution event can link the earlier
// problem event it resolves.
func (w *Worker) relatedEvents(ctx context.Context, bucket Bucket) ([]llm.RelatedEvent, error) {
	query := relatedEventQuery(bucket)
	if query == "" {
		return nil, nil
	}
	matches, err := recall.FTSEventLinks(ctx, w.db, query, bucket.URI, relatedEventsTopK)
	if err != nil {
		return nil, err
	}
	out := make([]llm.RelatedEvent, 0, len(matches))
	for _, m := range matches {
		if m.URI == "" {
			continue
		}
		out = append(out, llm.RelatedEvent{URI: m.URI, Snippet: m.Snippet})
	}
	return out, nil
}

// relatedEventQuery builds a whitespace-joined token query (OR-matched by
// recall.FTSEventLinks) from the bucket slug — date prefix stripped, since
// "2026-07-16-soft-delete-one-tag-solutions" should match an unrelated
// "2026-07-13-...-tag-..." problem event on subject tokens, not dates. Tokens
// from the atoms' scene names are added for subjects whose slug is thin.
// bm25 down-weights ubiquitous tokens, so short but distinctive terms
// ("tag", "pbp", "500") are kept.
func relatedEventQuery(bucket Bucket) string {
	seen := make(map[string]struct{})
	var tokens []string
	add := func(s string) {
		for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		}) {
			if len(f) < 3 || stopwords[f] || isYear(f) {
				continue
			}
			if _, ok := seen[f]; ok {
				continue
			}
			seen[f] = struct{}{}
			tokens = append(tokens, f)
		}
	}
	add(eventSlugDatePrefix.ReplaceAllString(bucket.Slug, ""))
	for _, a := range bucket.Atoms {
		if a.SceneName != nil {
			add(*a.SceneName)
		}
	}
	if len(tokens) == 0 {
		return ""
	}
	return strings.Join(tokens, " ")
}

var stopwords = map[string]bool{
	"the": true, "and": true, "one": true, "for": true,
}

// isYear suppresses bare 19xx/20xx tokens that survive date-prefix stripping
// (e.g. slugs not following the YYYY-MM-DD convention).
func isYear(f string) bool {
	if len(f) != 4 {
		return false
	}
	for _, r := range f {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	prefix := f[:2]
	return prefix == "19" || prefix == "20"
}
