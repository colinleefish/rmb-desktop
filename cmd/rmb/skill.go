package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/client"
	"github.com/colinleefish/rmb-desktop/internal/skill"
	"github.com/colinleefish/rmb-desktop/internal/uri"
)

func skillCmd(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: rmb skill <ls|put|pull> ...")
		return 2
	}
	switch args[0] {
	case "ls":
		return skillList()
	case "put":
		return skillPut(args[1:])
	case "pull":
		return skillPull(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown skill action %q (use ls|put|pull)\n", args[0])
		return 2
	}
}

func skillList() int {
	cl, err := apiClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "skill ls: %v\n", err)
		return 1
	}
	items, err := cl.ListSkills(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "skill ls: %v\n", err)
		return 1
	}
	if len(items) == 0 {
		fmt.Println("no skills")
		return 0
	}
	for _, it := range items {
		tags := strings.Join(it.Tags, ", ")
		if tags == "" {
			tags = "-"
		}
		fmt.Printf("%s\t[%s]\t%s\n", it.URI, tags, it.Description)
	}
	return 0
}

func skillPut(args []string) int {
	pos := positionalArgs(args)
	if len(pos) == 0 {
		fmt.Fprintln(os.Stderr, "usage: rmb skill put <name> [--dir=<path>]")
		return 2
	}
	name := pos[0]
	if err := uri.ValidateSkillName(name); err != nil {
		fmt.Fprintf(os.Stderr, "skill put: %v\n", err)
		return 1
	}

	dir := strings.TrimSpace(parseFlagValue(args, "--dir"))
	if dir == "" {
		defaultDir, err := skillDir(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skill put: %v\n", err)
			return 1
		}
		if st, err := os.Stat(defaultDir); err != nil || !st.IsDir() {
			fmt.Fprintf(os.Stderr, "default dir %s not found; pass --dir=<path>\n", defaultDir)
			return 1
		}
		dir = defaultDir
	}

	files, err := walkSkillDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "skill put: %v\n", err)
		return 1
	}

	cl, err := apiClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "skill put: %v\n", err)
		return 1
	}
	result, err := cl.PutSkill(context.Background(), name, files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "skill put: %v\n", err)
		return 1
	}
	if result.NoOp {
		fmt.Printf("unchanged: %s (version %d)\n", result.URI, result.Version)
		return 0
	}
	fmt.Printf("uploaded: %s (version %d)\n", result.URI, result.Version)
	return 0
}

func skillPull(args []string) int {
	all := false
	for _, a := range args {
		if a == "--all" {
			all = true
		}
	}
	pos := positionalArgs(args)
	outBase := strings.TrimSpace(parseFlagValue(args, "--out"))
	if outBase == "" {
		base, err := skillsRoot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "skill pull: %v\n", err)
			return 1
		}
		outBase = base
	}

	cl, err := apiClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "skill pull: %v\n", err)
		return 1
	}

	if all {
		items, err := cl.ListSkills(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "skill pull: %v\n", err)
			return 1
		}
		for _, it := range items {
			parsed, err := uri.Parse(it.URI)
			if err != nil || len(parsed.Segments) == 0 {
				continue
			}
			dest := filepath.Join(outBase, parsed.Segments[0])
			if err := materializeSkill(cl, parsed.Segments[0], dest); err != nil {
				fmt.Fprintf(os.Stderr, "skill pull: %v\n", err)
				return 1
			}
		}
		return 0
	}

	if len(pos) == 0 {
		fmt.Fprintln(os.Stderr, "usage: rmb skill pull <name> [--out=<dir>] | rmb skill pull --all [--out=<base>]")
		return 2
	}
	name := pos[0]
	dest := outBase
	if strings.TrimSpace(parseFlagValue(args, "--out")) == "" {
		var err error
		dest, err = skillDir(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skill pull: %v\n", err)
			return 1
		}
	} else {
		dest = filepath.Join(outBase, name)
	}
	if err := materializeSkill(cl, name, dest); err != nil {
		fmt.Fprintf(os.Stderr, "skill pull: %v\n", err)
		return 1
	}
	return 0
}

func materializeSkill(cl *client.Client, name, dest string) error {
	detail, err := cl.GetSkill(context.Background(), name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	for rel, content := range detail.Files {
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
	}
	fmt.Println(filepath.Join(dest, skill.ManifestPath))
	return nil
}

func skillsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".rmb", "skills"), nil
}

func skillDir(name string) (string, error) {
	root, err := skillsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name), nil
}

func walkSkillDir(dir string) ([]client.SkillFile, error) {
	var files []client.SkillFile
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, client.SkillFile{Path: rel, Content: string(data)})
		return nil
	})
	return files, err
}

func positionalArgs(args []string) []string {
	var out []string
	for _, a := range args {
		if strings.HasPrefix(a, "--") {
			continue
		}
		out = append(out, a)
	}
	return out
}

func parseFlagValue(args []string, name string) string {
	for _, a := range args {
		if strings.HasPrefix(a, name+"=") {
			return strings.TrimPrefix(a, name+"=")
		}
	}
	return ""
}
