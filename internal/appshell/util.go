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
