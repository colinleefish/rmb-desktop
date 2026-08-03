package skill

import "testing"

func TestNormalizeBundleRequiresSkillMD(t *testing.T) {
	_, _, _, err := normalizeBundle("demo-skill", []FileInput{
		{Path: "readme.md", Content: "---\nname: demo-skill\ndescription: Demo\n---\n"},
	})
	if err == nil {
		t.Fatal("expected error without SKILL.md")
	}
}

func TestNormalizeBundleValid(t *testing.T) {
	body := "---\nname: demo-skill\ndescription: Demo skill for tests.\ntags: work, demo\n---\n\nDo the thing.\n"
	files, hash, meta, err := normalizeBundle("demo-skill", []FileInput{
		{Path: "SKILL.md", Content: body},
		{Path: "scripts/run.sh", Content: "#!/bin/sh\necho ok\n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files=%d want 2", len(files))
	}
	if hash == "" {
		t.Fatal("empty hash")
	}
	if meta.Name != "demo-skill" || meta.Description == "" {
		t.Fatalf("meta=%+v", meta)
	}
	if len(meta.Tags) != 2 || meta.Tags[0] != "demo" || meta.Tags[1] != "work" {
		t.Fatalf("tags=%v", meta.Tags)
	}
}

func TestParseFrontmatterBlockDescription(t *testing.T) {
	body := "---\nname: demo-skill\ndescription: >-\n  Tell jokes on demand.\n  Use when asked.\ntags: personal\n---\n"
	meta, err := parseFrontmatter(body)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Description != "Tell jokes on demand. Use when asked." {
		t.Fatalf("description=%q", meta.Description)
	}
	if len(meta.Tags) != 1 || meta.Tags[0] != "personal" {
		t.Fatalf("tags=%v", meta.Tags)
	}
}

func TestNormalizeBundleRejectsBinary(t *testing.T) {
	body := "---\nname: demo-skill\ndescription: Demo skill.\n---\n"
	_, _, _, err := normalizeBundle("demo-skill", []FileInput{
		{Path: "SKILL.md", Content: body},
		{Path: "scripts/evil.bin", Content: "bad\x00data"},
	})
	if err == nil {
		t.Fatal("expected binary rejection")
	}
}
