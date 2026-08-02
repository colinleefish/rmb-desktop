package llm

import (
	_ "embed"
	"strings"
)

//go:embed prompts/extract_atoms.system.txt
var extractAtomsSystemPrompt string

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

//go:embed prompts/distill_memory.profile_filter.txt
var distillMemoryProfileFilter string

func orEmptyMarker(s string) string {
	if s = strings.TrimSpace(s); s != "" {
		return s
	}
	return "(empty)"
}

func buildExtractAtomsPrompt(messagesJSONL string) string {
	out := strings.ReplaceAll(extractAtomsUserTmpl, "{{BATCH}}", orEmptyMarker(messagesJSONL))
	return strings.TrimSpace(out)
}

func buildBuildScenesPrompt(atomsJSON string) string {
	out := strings.ReplaceAll(buildScenesUserTmpl, "{{ATOMS}}", orEmptyMarker(atomsJSON))
	return strings.TrimSpace(out)
}

func buildSessionAbstractPrompt(sceneAbstracts string) string {
	out := strings.ReplaceAll(sessionAbstractUserTmpl, "{{ABSTRACTS}}", orEmptyMarker(sceneAbstracts))
	return strings.TrimSpace(out)
}

func buildDistillMemoryPrompt(category, slug, atomsJSON string, corrections []string) string {
	topic := strings.TrimSpace(slug)
	if topic == "" {
		topic = "(none)"
	}
	filter := ""
	if category == "profile" {
		filter = "\n" + strings.TrimSpace(distillMemoryProfileFilter)
	}
	out := distillMemoryUserTmpl
	out = strings.ReplaceAll(out, "{{CATEGORY}}", category)
	out = strings.ReplaceAll(out, "{{SLUG}}", topic)
	out = strings.ReplaceAll(out, "{{FILTER}}", filter)
	out = strings.ReplaceAll(out, "{{CORRECTIONS}}", buildCorrectionsBlock(corrections))
	out = strings.ReplaceAll(out, "{{FACTS}}", orEmptyMarker(atomsJSON))
	return strings.TrimSpace(out)
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
