package llm

import (
	"strings"
	"testing"
)

func TestBuildDistillMemoryPromptV2EventFormatAndRelated(t *testing.T) {
	related := []RelatedEvent{
		{URI: "rmb://events/2026-07-13-starlink-hs99-vip-500-bug", Snippet: "get_diff_tag_from_tag_base_editer dict-vs-list bug"},
		{URI: "rmb://events/2026-07-15-tag-audit", Snippet: ""},
	}
	out := buildDistillMemoryPromptV(2, "events", "2026-07-16-soft-delete-one-tag-solutions", `{"facts":[]}`, nil, related)

	for _, want := range []string{
		"EVENT FORMAT",
		"**Decision:**",
		"**Rationale:**",
		"**Outcome:**",
		"**Related:**",
		"**Refs:**",
		"RELATED ACTIVE EVENTS",
		"rmb://events/2026-07-13-starlink-hs99-vip-500-bug",
		"get_diff_tag_from_tag_base_editer dict-vs-list bug",
		"resolves rmb://events/",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("v2 event prompt missing %q", want)
		}
	}
}

func TestBuildDistillMemoryPromptV2NoRelated(t *testing.T) {
	out := buildDistillMemoryPromptV(2, "events", "2026-08-21-cluster-admin-toolbox-removal", `{"facts":[]}`, nil, nil)
	if !strings.Contains(out, "Related active events: none found.") {
		t.Error("v2 event prompt should state that no related events were found")
	}
}

func TestBuildDistillMemoryPromptV1HasNoEventInjection(t *testing.T) {
	related := []RelatedEvent{{URI: "rmb://events/2026-07-13-problem", Snippet: "snippet"}}
	out := buildDistillMemoryPromptV(1, "events", "2026-07-16-fix", `{"facts":[]}`, nil, related)
	for _, unwanted := range []string{"EVENT FORMAT", "RELATED ACTIVE EVENTS", "Rationale"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("v1 prompt must not contain %q (kept for A/B replay)", unwanted)
		}
	}
	if !strings.Contains(out, "category: events") {
		t.Error("v1 prompt should still carry the category")
	}
}

func TestBuildDistillMemoryPromptNonEventHasNoEventFormat(t *testing.T) {
	out := buildDistillMemoryPromptV(2, "entities", "starlink", `{"facts":[]}`, nil,
		[]RelatedEvent{{URI: "rmb://events/2026-07-13-problem"}})
	if strings.Contains(out, "EVENT FORMAT") || strings.Contains(out, "RELATED ACTIVE EVENTS") {
		t.Error("event format must only apply to the events category")
	}
}

func TestBuildDistillMemoryPromptProfileFilterStillApplies(t *testing.T) {
	out := buildDistillMemoryPromptV(2, "profile", "", `{"facts":[]}`, nil, nil)
	if !strings.Contains(out, "Keep ONLY durable first-person facts") {
		t.Error("profile filter missing")
	}
	if strings.Contains(out, "EVENT FORMAT") {
		t.Error("profile prompt should not carry the event format")
	}
}

func TestExtractPromptPairVersions(t *testing.T) {
	sys2, _, err := ExtractPromptPair(2, "batch")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sys2, "RATIONALE") {
		t.Error("v2 extraction system prompt should carry the rationale/outcome guidance")
	}

	sys1, _, err := ExtractPromptPair(1, "batch")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sys1, "RATIONALE") {
		t.Error("v1 extraction system prompt must stay pre-P2.2 for A/B replay")
	}

	if _, _, err := ExtractPromptPair(99, ""); err == nil {
		t.Error("unknown version must error")
	}
	if _, _, err := ExtractPromptPair(0, ""); err != nil {
		t.Errorf("version 0 must resolve to latest: %v", err)
	}
}

func TestPromptVersionConstants(t *testing.T) {
	if ExtractAtomsPromptLatest < 2 {
		t.Error("extraction prompt should be at generation 2 after P2.2")
	}
	if DistillMemoryPromptLatest < 2 {
		t.Error("distill prompt should be at generation 2 after P2.2")
	}
}
