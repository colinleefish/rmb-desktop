package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/setup"
)

func setupCmd(args []string) int {
	if len(args) == 0 {
		printSetupUsage()
		return 2
	}
	switch args[0] {
	case "status":
		return setupStatus(args[1:])
	default:
		return setupAgent(args)
	}
}

func setupStatus(args []string) int {
	fs := flag.NewFlagSet("setup status", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	agent := fs.String("agent", "", "filter to one agent")
	_ = fs.Parse(args)

	if strings.TrimSpace(*agent) != "" {
		state, err := setup.PreviewByName(*agent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "setup status: %v\n", err)
			return 1
		}
		if *jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(state)
			return 0
		}
		fmt.Printf("%s detected=%v hook=%s recall=%s\n", state.Name, state.Detected, state.HookStatus, state.RecallStatus)
		return 0
	}

	status, err := setup.Status()
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup status: %v\n", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(status)
		return 0
	}
	for _, a := range status.Agents {
		fmt.Printf("%s detected=%v hook=%s recall=%s\n", a.Name, a.Detected, a.HookStatus, a.RecallStatus)
	}
	return 0
}

func setupAgent(args []string) int {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	agent := fs.String("agent", "", "agent id (cursor, claude-code, cc, codex, opencode, pi)")
	dryRun := fs.Bool("dry-run", false, "print preview JSON only")
	apply := fs.String("apply", "", "comma-separated artifact ids to write")
	_ = fs.Parse(args)

	if strings.TrimSpace(*agent) == "" {
		printSetupUsage()
		return 2
	}

	if strings.TrimSpace(*apply) != "" {
		ids := splitCSV(*apply)
		res, err := setup.Apply(*agent, ids)
		if err != nil {
			fmt.Fprintf(os.Stderr, "setup: %v\n", err)
			return 1
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return 0
	}

	state, err := setup.PreviewByName(*agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup: %v\n", err)
		return 1
	}
	if *dryRun {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(setup.PreviewResponse{Agent: state})
		return 0
	}

	fmt.Printf("%s detected=%v hook=%s recall=%s\n", state.Name, state.Detected, state.HookStatus, state.RecallStatus)
	for _, a := range state.Artifacts {
		fmt.Printf("  - %s (%s) %s [%s]\n", a.ID, a.ChangeType, a.Path, a.ApplyMode)
	}
	return 0
}

func splitCSV(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func printSetupUsage() {
	fmt.Fprintf(os.Stderr, `setup usage:
  rmb setup status [--json] [--agent=<name>]
  rmb setup --agent=<name> [--dry-run]
  rmb setup --agent=<name> --apply=<artifact-id>[,<id>...]

`)
}
