package setup

import "strings"

const recallStart = "<!-- rmb:recall:start -->"
const recallEnd = "<!-- rmb:recall:end -->"
const recallMarkdownHeading = "# RMB memory"

func recallInstructionBody() string {
	return `ALWAYS RUN ` + "`~/.rmb/bin/rmb`" + ` cli at the start of each conversation.
Search before asking the user. Follow rmb://agent for recall rules.`
}

// CursorRecallRule is the canonical recall text for Cursor rules (copy-only).
func CursorRecallRule() string {
	return `---
description: Use of rmb command line 
alwaysApply: true
---
` + recallInstructionBody() + `
`
}

// recallBlock is appended to agent markdown files (CLAUDE.md, AGENTS.md, etc.).
func recallBlock() string {
	return recallMarkdownHeading + `

` + recallInstructionBody() + `
`
}

func hasRecallBlock(content string) bool {
	if strings.Contains(content, recallStart) && strings.Contains(content, recallEnd) {
		return true
	}
	if !strings.Contains(content, recallMarkdownHeading) {
		return false
	}
	// Accept both the old ("`rmb`") and new ("`~/.rmb/bin/rmb`") recall body so
	// existing installs aren't re-appended.
	return strings.Contains(content, "ALWAYS RUN `rmb` cli") ||
		strings.Contains(content, "ALWAYS RUN `~/.rmb/bin/rmb` cli")
}

func mergeRecallMarkdown(current string) (proposed string, change ChangeType) {
	block := recallBlock()
	cur := strings.TrimRight(current, "\n")
	if cur == "" {
		return block, ChangeCreate
	}
	if hasRecallBlock(cur) {
		return ensureTrailingNewline(cur), ChangeUnchanged
	}
	if strings.Contains(cur, recallStart) && strings.Contains(cur, recallEnd) {
		before, _, ok := strings.Cut(cur, recallStart)
		if !ok {
			return ensureTrailingNewline(cur + "\n\n" + block), ChangeAppend
		}
		_, after, ok := strings.Cut(cur, recallEnd)
		if !ok {
			return ensureTrailingNewline(cur + "\n\n" + block), ChangeAppend
		}
		proposed = strings.TrimRight(before, "\n") + "\n\n" + block + strings.TrimLeft(after, "\n")
		return ensureTrailingNewline(proposed), ChangeModify
	}
	return ensureTrailingNewline(cur + "\n\n" + block), ChangeAppend
}

func ensureTrailingNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}
