package appshell

import (
	"fmt"
	"os"
	"time"
)

func stderrPrintf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
}

func sleep(seconds int) {
	time.Sleep(time.Duration(seconds) * time.Second)
}

// truncate cuts s to at most max runes, appending an ellipsis when cut.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
