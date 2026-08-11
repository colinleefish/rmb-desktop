package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/client"
	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/hook"
)

const cliVersion = "0.1.6"

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		return printBootstrap()
	}

	switch os.Args[1] {
	case "hook-submit":
		return hookSubmit(os.Args[2:])
	case "search":
		return search(os.Args[2:])
	case "cat":
		return inspectCmd("cat", os.Args[2:])
	case "tree":
		return inspectCmd("tree", os.Args[2:])
	case "meta":
		return inspectCmd("meta", os.Args[2:])
	case "skill":
		return skillCmd(os.Args[2:])
	case "setup":
		return setupCmd(os.Args[2:])
	case "version":
		fmt.Println(cliVersion)
		return 0
	case "help", "-h", "--help":
		return printBootstrap()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		return 2
	}
}

func apiClient() (*client.Client, error) {
	cfg, err := config.LoadDefault()
	if err != nil {
		return nil, err
	}
	return client.FromConfig(cfg), nil
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

func search(args []string) int {
	query, rest := parseQueryAndFlags(args)
	if query == "" {
		fmt.Fprintf(os.Stderr, `usage: rmb search "<query>" [--scope=memory,scene,skill] [--k=n]`)
		return 2
	}
	k, err := parseK(rest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "search: %v\n", err)
		return 2
	}
	scopes := parseScopes(rest)

	cl, err := apiClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "search: %v\n", err)
		return 1
	}
	matches, err := cl.Search(context.Background(), query, k, scopes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "search: %v\n", err)
		return 1
	}
	printMatches(matches)
	return 0
}

func inspectCmd(kind string, args []string) int {
	uri := strings.TrimSpace(strings.Join(args, " "))
	if uri == "" {
		fmt.Fprintf(os.Stderr, "usage: rmb %s <uri>\n", kind)
		return 2
	}
	cl, err := apiClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", kind, err)
		return 1
	}
	out, err := cl.Inspect(context.Background(), kind, uri)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", kind, err)
		return 1
	}
	fmt.Print(out)
	return 0
}

func printMatches(matches []client.Match) {
	for i, m := range matches {
		fmt.Printf("%2d. [%s] %s\n", i+1, m.Tier, m.URI)
		if m.Snippet != "" {
			fmt.Printf("    %s\n", m.Snippet)
		}
	}
}

func parseQueryAndFlags(args []string) (string, []string) {
	var positional []string
	var rest []string
	for _, a := range args {
		if strings.HasPrefix(a, "--") {
			rest = append(rest, a)
		} else {
			positional = append(positional, a)
		}
	}
	return strings.Join(positional, " "), rest
}

func parseK(args []string) (int, error) {
	for _, a := range args {
		if strings.HasPrefix(a, "--k=") {
			return strconv.Atoi(strings.TrimPrefix(a, "--k="))
		}
	}
	return 0, nil
}

func parseScopes(args []string) []string {
	for _, a := range args {
		if strings.HasPrefix(a, "--scope=") {
			raw := strings.TrimPrefix(a, "--scope=")
			if raw == "" {
				return nil
			}
			var out []string
			for _, s := range strings.Split(raw, ",") {
				if s = strings.TrimSpace(s); s != "" {
					out = append(out, s)
				}
			}
			return out
		}
	}
	return nil
}

func printUsage() {
	fmt.Fprint(os.Stderr, usageText())
}

func usageText() string {
	return `Usage:
  rmb hook-submit --source=<cursor> [--url=http://127.0.0.1:19019]
  rmb search "<query>" [--scope=memory,scene,skill] [--k=n]
  rmb cat <uri>
  rmb tree <uri-prefix>
  rmb meta <uri>
  rmb skill ls
  rmb skill put <name> [--dir=<path>]
  rmb skill pull <name> [--out=<dir>]
  rmb skill pull --all [--out=<base>]
  rmb setup status [--json] [--agent=<name>]
  rmb setup --agent=<name> [--dry-run] [--apply=<ids>]
  rmb version

`
}
