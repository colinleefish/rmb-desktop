package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/client"
	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/hook"
	"github.com/colinleefish/rmb-desktop/internal/recall"
	"github.com/colinleefish/rmb-desktop/internal/version"
)

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
	case "ls":
		return inspectCmd("ls", os.Args[2:])
	case "meta":
		return inspectCmd("meta", os.Args[2:])
	case "pull":
		return pullCmd(os.Args[2:])
	case "put":
		return putCmd(os.Args[2:])
	case "setup":
		return setupCmd(os.Args[2:])
	case "doctor":
		return doctorCmd(os.Args[2:])
	case "backfill-provenance":
		return backfillCmd(os.Args[2:])
	case "version":
		if c := version.Commit; c != "" && c != "dev" {
			fmt.Printf("%s (%s)\n", version.Version, c)
		} else {
			fmt.Println(version.Version)
		}
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
		fmt.Fprintf(os.Stderr, `usage: rmb search "<query>" [--scope=memory,scene,skill,atom] [--k=n] [--since=<date|Nd>] [--until=<date|Nd>] [--no-boost]`)
		return 2
	}
	k, err := parseK(rest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "search: %v\n", err)
		return 2
	}
	scopes := parseScopes(rest)
	since, err := parseTimeFlag(rest, "--since=")
	if err != nil {
		fmt.Fprintf(os.Stderr, "search: %v\n", err)
		return 2
	}
	until, err := parseTimeFlag(rest, "--until=")
	if err != nil {
		fmt.Fprintf(os.Stderr, "search: %v\n", err)
		return 2
	}
	noBoost := hasFlag(rest, "--no-boost")

	cl, err := apiClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "search: %v\n", err)
		return 1
	}
	matches, err := cl.Search(context.Background(), query, k, scopes, since, until, noBoost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "search: %v\n", err)
		return 1
	}
	printMatches(matches)
	return 0
}

func inspectCmd(kind string, args []string) int {
	var (
		uri   string
		extra url.Values
		err   error
	)
	if kind == "ls" {
		uri, extra, err = parseLsArgs(args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: %v\n", err)
			return 2
		}
	} else {
		uri = strings.TrimSpace(strings.Join(args, " "))
	}
	if uri == "" {
		fmt.Fprintf(os.Stderr, "usage: rmb %s <uri>%s\n", kind, lsUsageHint(kind))
		return 2
	}
	cl, err := apiClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", kind, err)
		return 1
	}
	out, err := cl.InspectWith(context.Background(), kind, uri, extra)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", kind, err)
		return 1
	}
	fmt.Print(out)
	return 0
}

func lsUsageHint(kind string) string {
	if kind != "ls" {
		return ""
	}
	return " [--limit N] [--offset N] [--since <date|7d>] [--until <date|7d>] [--count]"
}

// parseLsArgs splits a uri from ls flags, accepting both --flag=value and
// --flag value forms. Numeric flags are validated here; time filters are
// passed through for the daemon to parse (inspect.ParseTimeFilter) so CLI
// and API semantics stay in sync.
func parseLsArgs(args []string) (string, url.Values, error) {
	var positional []string
	extra := url.Values{}
	i := 0
	for i < len(args) {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			positional = append(positional, a)
			i++
			continue
		}
		name, val, hasVal := strings.Cut(a, "=")
		consume := func() (string, error) {
			if hasVal {
				return val, nil
			}
			if i+1 < len(args) {
				i++
				return args[i], nil
			}
			return "", fmt.Errorf("missing value for %s", name)
		}
		switch name {
		case "--count":
			v := "true"
			if hasVal {
				v = val
			}
			extra.Set("count", v)
		case "--limit":
			v, err := consume()
			if err != nil {
				return "", nil, err
			}
			if _, err := strconv.Atoi(v); err != nil {
				return "", nil, fmt.Errorf("bad --limit %q (want a non-negative integer)", v)
			}
			extra.Set("limit", v)
		case "--offset":
			v, err := consume()
			if err != nil {
				return "", nil, err
			}
			if _, err := strconv.Atoi(v); err != nil {
				return "", nil, fmt.Errorf("bad --offset %q (want a non-negative integer)", v)
			}
			extra.Set("offset", v)
		case "--since":
			v, err := consume()
			if err != nil {
				return "", nil, err
			}
			extra.Set("since", v)
		case "--until":
			v, err := consume()
			if err != nil {
				return "", nil, err
			}
			extra.Set("until", v)
		default:
			return "", nil, fmt.Errorf("unknown ls flag %q", a)
		}
		i++
	}
	return strings.TrimSpace(strings.Join(positional, " ")), extra, nil
}

func printMatches(matches []client.Match) {
	for i, m := range matches {
		ver := ""
		if m.Version > 0 {
			ver = fmt.Sprintf(", v=%d", m.Version)
		}
		fmt.Printf("%2d. [%s] %s (%.4f%s)\n", i+1, m.Tier, m.URI, m.Rank, ver)
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

// hasFlag reports whether the exact flag token is present in args.
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
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

// parseTimeFlag extracts a --since=/--until= value and validates its shape
// (absolute date or relative Nd/Nh/Nm) client-side so typos fail fast.
// The daemon re-parses and is the authority.
func parseTimeFlag(args []string, prefix string) (string, error) {
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			raw := strings.TrimSpace(strings.TrimPrefix(a, prefix))
			if raw == "" {
				return "", fmt.Errorf("%s needs a value (2026-08-01, 15:04, or 7d/12h/30m)", strings.TrimSuffix(prefix, "="))
			}
			if _, err := recall.ParseTimeValue(raw, time.Now()); err != nil {
				return "", err
			}
			return raw, nil
		}
	}
	return "", nil
}

// doctorCmd implements `rmb doctor <subcommand>`.
//
//	rmb doctor archive                      # --dry-run: propose cold memories
//	rmb doctor archive --dry-run [--days=N] # explicit review list
//	rmb doctor archive --apply [--uri=U...] # archive approved list (all if no uri)
//	rmb doctor archive --restore <uri>...    # un-archive specific memories
//	rmb doctor archive --restore-all         # un-archive everything
//	rmb doctor metrics                       # retrieval-health report (#24)
func doctorCmd(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, doctorUsage())
		return 2
	}
	switch args[0] {
	case "archive":
		return doctorArchive(args[1:])
	case "metrics":
		return doctorMetrics()
	default:
		fmt.Fprint(os.Stderr, doctorUsage())
		return 2
	}
}

func doctorUsage() string {
	return `Usage:
  rmb doctor archive                    # --dry-run: propose cold memories (issue #32)
  rmb doctor archive --dry-run [--days=N]
  rmb doctor archive --apply [--uri=rmb://... ...]  # archive approved list (bulk-all if no --uri)
  rmb doctor archive --restore <uri>...              # restore specific uri(s)
  rmb doctor archive --restore-all                  # restore everything
  rmb doctor metrics                                # retrieval-health report (#24)
`
}

func doctorArchive(args []string) int {
	apply := false
	restore := false
	restoreAll := false
	days := 0
	var uris []string
	for _, a := range args {
		switch {
		case a == "--dry-run":
			// explicit no-op (default path is the review list)
		case a == "--apply":
			apply = true
		case a == "--restore-all" || a == "--restore_all":
			restoreAll = true
		case strings.HasPrefix(a, "--days="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--days="))
			if err != nil || n < 0 {
				fmt.Fprintf(os.Stderr, "doctor archive: bad --days\n")
				return 2
			}
			days = n
		case strings.HasPrefix(a, "--uri="):
			uris = append(uris, strings.TrimPrefix(a, "--uri="))
		case strings.HasPrefix(a, "--restore="):
			restore = true
			if v := strings.TrimPrefix(a, "--restore="); v != "" {
				uris = append(uris, v)
			}
		case a == "-h" || a == "--help":
			fmt.Fprint(os.Stderr, doctorUsage())
			return 2
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "doctor archive: unknown flag %q\n", a)
			return 2
		default:
			// positional uri(s), valid for --restore
			uris = append(uris, a)
		}
	}

	cl, err := apiClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor archive: %v\n", err)
		return 1
	}

	if restoreAll {
		return runArchiveAction(cl, "restore", nil, true)
	}
	if restore {
		if len(uris) == 0 {
			fmt.Fprint(os.Stderr, "doctor archive: --restore needs at least one uri\n")
			return 2
		}
		return runArchiveAction(cl, "restore", uris, false)
	}
	if apply {
		return runArchiveAction(cl, "archive", uris, false)
	}
	// dry-run (default): propose the reviewable list.
	cands, err := cl.DoctorArchiveCandidates(context.Background(), days)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor archive: %v\n", err)
		return 1
	}
	if len(cands) == 0 {
		fmt.Println("no cold memories proposed for archival (dry-run, 90-day window)")
		return 0
	}
	fmt.Printf("%d memory(ies) proposed for archival (dry-run, %d-day window)\n\n", len(cands), max(days, 90))
	for _, c := range cands {
		fmt.Printf("  %s\n    category=%s version=%d heat=%.3f updated=%d\n", c.URI, c.Category, c.Version, c.Heat, c.UpdatedAt)
		if c.Abstract != "" {
			fmt.Printf("    %s\n", truncate(c.Abstract, 120))
		}
	}
	fmt.Println("\nReview the list, then run:  rmb doctor archive --apply   (or --restore later)")
	return 0
}

func runArchiveAction(cl *client.Client, action string, uris []string, all bool) int {
	n, err := cl.DoctorArchiveAction(context.Background(), action, uris, all)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor archive: %v\n", err)
		return 1
	}
	verb := map[string]string{"archive": "archived", "restore": "restored"}[action]
	switch {
	case all:
		fmt.Printf("%d memory(ies) %s\n", n, verb)
	case len(uris) == 0:
		fmt.Printf("%d cold memory(ies) %s (bulk apply of the proposed list; nothing deleted)\n", n, verb)
	default:
		for _, u := range uris {
			fmt.Printf("  %s\n", u)
		}
		fmt.Printf("%d memory(ies) %s\n", n, verb)
	}
	return 0
}

func doctorMetrics() int {
	cl, err := apiClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor metrics: %v\n", err)
		return 1
	}
	m, err := cl.DoctorMetrics(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor metrics: %v\n", err)
		return 1
	}
	fmt.Printf("window_days=%d searches=%d zero_cat_rate=%.2f heat_concentration=%.2f alarm=%v\n",
		m.WindowDays, m.Searches, m.ZeroCatRate, m.HeatConcentration, m.HeatAlarm)
	return 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func printUsage() {
	fmt.Fprint(os.Stderr, usageText())
}

func usageText() string {
	return `Usage:
  rmb hook-submit --source=<cursor> [--url=http://127.0.0.1:19019]
  rmb search "<query>" [--scope=memory,scene,skill,atom] [--k=n] [--since=<date|Nd>] [--until=<date|Nd>] [--no-boost]
  rmb ls <uri-prefix>            # list container contents (e.g. rmb://events/)
  rmb ls <uri-prefix> [--limit=N] [--offset=N] [--since=<date|7d>] [--until=<date|7d>] [--count]
  rmb cat <uri>
  rmb meta <uri>
  rmb pull <uri> [--out=<dir>]   # rmb://skills/<name> | rmb://skills/ (all)
  rmb put rmb://skills/<name> [--dir=<path>]
  rmb setup status [--json] [--agent=<name>]
  rmb setup --agent=<name> [--dry-run] [--apply=<ids>]
  rmb version

`
}
