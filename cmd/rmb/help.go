package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/client"
	"github.com/colinleefish/rmb-desktop/internal/uri"
)

const helpSectionRule = "════════════════════════════════════════"

func printBootstrap() int {
	out := os.Stdout
	fmt.Fprintln(out, "rmb - long-term memory for AI agents")
	fmt.Fprintln(out)

	cl, err := apiClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "rmb: %v\n", err)
		fmt.Fprintln(os.Stderr)
		printUsage()
		return 1
	}

	ctx := context.Background()
	sections := []struct {
		marker string
		title  string
		target string
		blurb  string
	}{
		{
			marker: "profile",
			title:  "Profile",
			target: uri.BuildProfile(),
			blurb:  "Who the user is — stable identity, work context, and preferences.",
		},
		{
			marker: "agent",
			title:  "Agent guide",
			target: uri.BuildAgent(),
			blurb:  "How to use rmb — recall rules, URI shapes, and skill discovery.",
		},
	}
	for _, section := range sections {
		printHelpSectionHeader(out, section.marker, section.title, section.target, section.blurb)
		body, err := cl.Inspect(ctx, "cat", section.target)
		if err != nil {
			fmt.Fprintf(out, "  (unavailable: %v)\n", err)
		} else if strings.TrimSpace(body) == "" {
			fmt.Fprintln(out, "  (empty)")
		} else {
			fmt.Fprintln(out, strings.TrimRight(body, "\n"))
		}
		fmt.Fprintln(out)
	}

	if err := printSkillsCatalog(ctx, cl, out); err != nil {
		printHelpSectionHeader(out, "skills", "Skills", uri.BuildSkill(""), "Curated playbooks. Read SKILL.md with rmb cat; run scripts via rmb skill pull.")
		fmt.Fprintf(out, "  (unavailable: %v)\n", err)
	}

	printHelpUsageDivider(out)
	fmt.Fprint(out, usageText())
	return 0
}

func printHelpSectionHeader(out io.Writer, marker, title, targetURI, blurb string) {
	fmt.Fprintln(out, helpSectionRule)
	if marker = strings.TrimSpace(marker); marker != "" {
		fmt.Fprintf(out, "[%s]\n", marker)
	}
	fmt.Fprintf(out, "%s  %s\n", title, targetURI)
	if blurb = strings.TrimSpace(blurb); blurb != "" {
		fmt.Fprintln(out, blurb)
	}
	fmt.Fprintln(out)
}

func printHelpUsageDivider(out io.Writer) {
	fmt.Fprintln(out, helpSectionRule)
	fmt.Fprintln(out, "[usage]")
	fmt.Fprintln(out)
}

const (
	helpSkillCatalogLimit = 20
	helpSkillDescMaxLen   = 120
)

func printSkillsCatalog(ctx context.Context, cl *client.Client, out io.Writer) error {
	items, err := cl.ListSkills(ctx)
	if err != nil {
		return err
	}
	printHelpSectionHeader(out, "skills", "Skills", uri.BuildSkill(""), "Curated playbooks. Read SKILL.md with rmb cat; run scripts via rmb skill pull.")
	if len(items) == 0 {
		fmt.Fprintln(out, "  (no skills)")
		return nil
	}

	sort.Slice(items, func(i, j int) bool { return items[i].URI < items[j].URI })

	limit := len(items)
	if limit > helpSkillCatalogLimit {
		limit = helpSkillCatalogLimit
	}
	for i := 0; i < limit; i++ {
		printHelpSkillEntry(out, items[i])
		if i < limit-1 {
			fmt.Fprintln(out)
		}
	}
	if len(items) > helpSkillCatalogLimit {
		fmt.Fprintf(out, "\n  ... %d more — rmb tree rmb://skills/\n", len(items)-helpSkillCatalogLimit)
	}
	return nil
}

func printHelpSkillEntry(out io.Writer, it client.SkillSummary) {
	fmt.Fprintln(out, it.URI)
	fmt.Fprintf(out, "  [desc] %s\n", formatHelpSkillDescription(it.Description))
	if len(it.Tags) > 0 {
		fmt.Fprintf(out, "  [tags] %s\n", strings.Join(it.Tags, ", "))
	}
}

func formatHelpSkillDescription(desc string) string {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return "(no description)"
	}
	desc = strings.Join(strings.Fields(strings.ReplaceAll(desc, "\n", " ")), " ")
	if len(desc) <= helpSkillDescMaxLen {
		return desc
	}
	return desc[:helpSkillDescMaxLen-1] + "…"
}
