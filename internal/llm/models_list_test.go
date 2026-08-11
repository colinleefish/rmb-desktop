package llm

import "testing"

func TestBuildModelsURLCandidates_plainRoot(t *testing.T) {
	got := buildModelsURLCandidates("https://api.deepseek.com")
	want := []string{"https://api.deepseek.com/v1/models"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildModelsURLCandidates_withV1(t *testing.T) {
	got := buildModelsURLCandidates("https://api.openai.com/v1")
	want := []string{"https://api.openai.com/v1/models"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildModelsURLCandidates_zhipuV4(t *testing.T) {
	got := buildModelsURLCandidates("https://open.bigmodel.cn/api/coding/paas/v4")
	want := []string{
		"https://open.bigmodel.cn/api/coding/paas/v4/models",
		"https://open.bigmodel.cn/api/coding/paas/v4/v1/models",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestBuildModelsURLCandidates_codingSuffix(t *testing.T) {
	got := buildModelsURLCandidates("https://ark.cn-beijing.volces.com/api/coding/v3")
	if len(got) < 2 {
		t.Fatalf("expected compat fallbacks, got %v", got)
	}
	if got[0] != "https://ark.cn-beijing.volces.com/api/coding/v3/models" {
		t.Fatalf("first candidate: %s", got[0])
	}
}

func TestModelsContain(t *testing.T) {
	ids := []string{"gpt-4o-mini", "deepseek-chat"}
	if !modelsContain(ids, "deepseek-chat") {
		t.Fatal("expected match")
	}
	if modelsContain(ids, "missing") {
		t.Fatal("expected no match")
	}
	if !modelsContain(ids, "") {
		t.Fatal("empty model should pass")
	}
}
