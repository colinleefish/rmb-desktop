package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/colinleefish/rmb-desktop/internal/hook"
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		printUsage()
		return 2
	}

	switch os.Args[1] {
	case "hook-submit":
		return hookSubmit(os.Args[2:])
	case "version":
		fmt.Println("rmb dev")
		return 0
	case "help", "-h", "--help":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		return 2
	}
}

func hookSubmit(args []string) int {
	fs := flag.NewFlagSet("hook-submit", flag.ExitOnError)
	source := fs.String("source", "", "agent source (cursor)")
	baseURL := fs.String("url", "", "rmbd base URL (default from config)")
	_ = fs.Parse(args)

	if *source == "" {
		fmt.Fprintln(os.Stderr, "hook-submit: --source is required")
		return 2
	}

	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
		return 1
	}

	err = hook.Submit(context.Background(), hook.SubmitInput{
		Source:     *source,
		StdinJSON:  stdin,
		OutputSink: os.Stdout,
		BaseURL:    *baseURL,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "hook-submit: %v\n", err)
		return 1
	}
	return 0
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `rmb - local-first memory CLI

Usage:
  rmb hook-submit --source=<cursor> [--url=http://127.0.0.1:19019]
  rmb version

`)
}
