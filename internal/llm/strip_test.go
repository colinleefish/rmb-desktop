package llm

import "testing"

func TestStripCodeFence(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"plain json", `{"a":1}`, `{"a":1}`},
		{"surrounding whitespace", "\n  {\"a\":1}  \n", `{"a":1}`},
		{"fenced with language", "```json\n{\"a\":1}\n```", `{"a":1}`},
		{"fenced without language", "```\n{\"a\":1}\n```", `{"a":1}`},
		{"fenced missing closing", "```json\n{\"a\":1}", `{"a":1}`},
		{"fence only one line", "```json", "```json"},
		{"inner blank lines", "```json\n{\n\"a\": 1\n}\n```", "{\n\"a\": 1\n}"},
	}
	for _, tc := range cases {
		if got, err := StripCodeFence(tc.raw); err != nil || got != tc.want {
			t.Errorf("%s: StripCodeFence(%q) = (%q, %v), want (%q, nil)", tc.name, tc.raw, got, err, tc.want)
		}
	}
	if _, err := StripCodeFence("   "); err == nil || err.Error() != "empty llm response" {
		t.Errorf("whitespace-only: got err %v, want 'empty llm response'", err)
	}
	if _, err := StripCodeFence(""); err == nil || err.Error() != "empty llm response" {
		t.Errorf("empty: got err %v, want 'empty llm response'", err)
	}
}
