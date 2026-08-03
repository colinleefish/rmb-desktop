package setup

import "strings"

const recallStart = "<!-- rmb:recall:start -->"
const recallEnd = "<!-- rmb:recall:end -->"

func recallBlock() string {
	return recallStart + `
ALWAYS RUN ` + "`rmb`" + ` cli at the start of each conversation.
Search before asking the user. Follow rmb://agent for recall rules.
` + recallEnd
}

func mergeRecallMarkdown(current string) (proposed string, change ChangeType) {
	block := strings.TrimSpace(recallBlock())
	cur := strings.TrimRight(current, "\n")
	if cur == "" {
		return block + "\n", ChangeCreate
	}
	if strings.Contains(cur, recallStart) && strings.Contains(cur, recallEnd) {
		before, _, ok := strings.Cut(cur, recallStart)
		if !ok {
			return cur + "\n\n" + block + "\n", ChangeAppend
		}
		_, after, ok := strings.Cut(cur, recallEnd)
		if !ok {
			return cur + "\n\n" + block + "\n", ChangeAppend
		}
		proposed = strings.TrimRight(before, "\n") + "\n\n" + block + strings.TrimLeft(after, "\n")
		if !strings.HasSuffix(proposed, "\n") {
			proposed += "\n"
		}
		if proposed == cur+"\n" || proposed == cur {
			return cur, ChangeUnchanged
		}
		return proposed, ChangeModify
	}
	if strings.TrimSpace(cur) == strings.TrimSpace(block) {
		return cur, ChangeUnchanged
	}
	if strings.Contains(cur, "rmb://agent") || strings.Contains(cur, "ALWAYS RUN `rmb`") {
		return cur + "\n\n" + block + "\n", ChangeAppend
	}
	return cur + "\n\n" + block + "\n", ChangeAppend
}

func hasRecallBlock(content string) bool {
	return strings.Contains(content, recallStart) || strings.Contains(content, "rmb://agent")
}
