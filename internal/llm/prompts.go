package llm

import (
	_ "embed"
	"fmt"
	"strings"
)

// Prompt template generations (issue #28). Bump the latest constant when a
// template's behavior changes and keep the previous generation embedded so
// sessions can be A/B replayed via the debug dry-run endpoints
// (POST /api/v1/debug/pipeline/dry-run with prompt_version).
const (
	// ExtractAtomsPromptLatest: v2 additionally extracts rationale,
	// rejected-alternative, and verification/outcome atoms (P2.2).
	ExtractAtomsPromptLatest = 2
	// DistillMemoryPromptLatest: v2 gives event bodies a structured layout
	// (Decision/Rationale/Outcome/Related/Refs) and injects related active
	// events into the prompt for cross-session linking (P2.2).
	DistillMemoryPromptLatest = 2
)

//go:embed prompts/extract_atoms.system.txt
var extractAtomsSystemPrompt string

//go:embed prompts/extract_atoms.system.v1.txt
var extractAtomsSystemPromptV1 string

//go:embed prompts/extract_atoms.user.txt
var extractAtomsUserTmpl string

//go:embed prompts/build_scenes.system.txt
var buildScenesSystemPrompt string

//go:embed prompts/build_scenes.user.txt
var buildScenesUserTmpl string

//go:embed prompts/session_abstract.system.txt
var sessionAbstractSystemPrompt string

//go:embed prompts/session_abstract.user.txt
var sessionAbstractUserTmpl string

//go:embed prompts/distill_memory.system.txt
var distillMemorySystemPrompt string

//go:embed prompts/distill_memory.user.txt
var distillMemoryUserTmpl string

//go:embed prompts/distill_memory.user.v1.txt
var distillMemoryUserTmplV1 string

//go:embed prompts/distill_memory.profile_filter.txt
var distillMemoryProfileFilter string

//go:embed prompts/distill_memory.event_format.txt
var distillMemoryEventFormat string

// RelatedEvent is an active event memory injected into the L3 distill prompt
// (retrieve-then-link, P2.2 / issue #28) so a resolution event can reference
// the earlier problem event it resolves.
type RelatedEvent struct {
	URI     string `json:"uri"`
	Snippet string `json:"snippet"`
}

func orEmptyMarker(s string) string {
	if s = strings.TrimSpace(s); s != "" {
		return s
	}
	return "(empty)"
}

// ExtractPromptPair returns the system+user prompt pair for an extraction
// template generation. Version 1 is retained for A/B replay of sessions
// extracted before P2.2.
func ExtractPromptPair(version int, messagesJSONL string) (system, user string, err error) {
	if version == 0 {
		version = ExtractAtomsPromptLatest
	}
	switch version {
	case 1:
		system = extractAtomsSystemPromptV1
	case 2:
		system = extractAtomsSystemPrompt
	default:
		return "", "", fmt.Errorf("unknown extract prompt version %d (latest %d)", version, ExtractAtomsPromptLatest)
	}
	user = strings.ReplaceAll(extractAtomsUserTmpl, "{{BATCH}}", orEmptyMarker(messagesJSONL))
	return system, strings.TrimSpace(user), nil
}

func buildBuildScenesPrompt(atomsJSON string) string {
	out := strings.ReplaceAll(buildScenesUserTmpl, "{{ATOMS}}", orEmptyMarker(atomsJSON))
	return strings.TrimSpace(out)
}

func buildSessionAbstractPrompt(sceneAbstracts string) string {
	out := strings.ReplaceAll(sessionAbstractUserTmpl, "{{ABSTRACTS}}", orEmptyMarker(sceneAbstracts))
	return strings.TrimSpace(out)
}

// buildDistillMemoryPromptV builds the distill user prompt for a template
// generation. Version 1 predates event linking: it ignores related events and
// the structured event body format (kept for A/B replay).
func buildDistillMemoryPromptV(version int, category, slug, atomsJSON string, corrections []string, related []RelatedEvent) string {
	topic := strings.TrimSpace(slug)
	if topic == "" {
		topic = "(none)"
	}
	filter := ""
	if category == "profile" {
		filter = "\n" + strings.TrimSpace(distillMemoryProfileFilter)
	}
	eventFormat := ""
	if category == "events" && version >= 2 {
		eventFormat = "\n\n" + strings.TrimSpace(distillMemoryEventFormat) + buildRelatedEventsBlock(related)
	}
	tmpl := distillMemoryUserTmpl
	if version == 1 {
		tmpl = distillMemoryUserTmplV1
	}
	out := tmpl
	out = strings.ReplaceAll(out, "{{CATEGORY}}", category)
	out = strings.ReplaceAll(out, "{{SLUG}}", topic)
	out = strings.ReplaceAll(out, "{{FILTER}}", filter)
	out = strings.ReplaceAll(out, "{{EVENT_FORMAT}}", eventFormat)
	out = strings.ReplaceAll(out, "{{CORRECTIONS}}", buildCorrectionsBlock(corrections))
	out = strings.ReplaceAll(out, "{{FACTS}}", orEmptyMarker(atomsJSON))
	return strings.TrimSpace(out)
}

// buildRelatedEventsBlock renders the retrieve-then-link section of the event
// distill prompt: the candidate events the model may link with
// "resolves rmb://events/…" (issue #28).
func buildRelatedEventsBlock(related []RelatedEvent) string {
	if len(related) == 0 {
		return "\n\nRelated active events: none found."
	}
	lines := make([]string, 0, len(related))
	for _, r := range related {
		uri := strings.TrimSpace(r.URI)
		if uri == "" {
			continue
		}
		snippet := strings.TrimSpace(r.Snippet)
		if snippet != "" {
			lines = append(lines, "- "+uri+" — "+snippet)
		} else {
			lines = append(lines, "- "+uri)
		}
	}
	if len(lines) == 0 {
		return "\n\nRelated active events: none found."
	}
	return "\n\nRELATED ACTIVE EVENTS (retrieved for cross-session linking; the current event may resolve, continue, or duplicate one of these — link only what the facts support, using the Related: section):\n" +
		strings.Join(lines, "\n")
}

func buildCorrectionsBlock(corrections []string) string {
	lines := make([]string, 0, len(corrections))
	for _, c := range corrections {
		if c = strings.TrimSpace(c); c != "" {
			lines = append(lines, "- "+c)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n\nAUTHORITATIVE human corrections (highest priority; treat as ground truth and override any conflicting fact below; newest first):\n" +
		strings.Join(lines, "\n")
}
