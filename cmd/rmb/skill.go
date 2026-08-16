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

// pullCmd implements `rmb pull <uri> [--out=<dir>]`.
//
// Supported uris (skills are the only pullable container for now):
//
//	rmb://skills/            → pull every skill into --out/<name>
//	rmb://skills/<name>      → pull one skill into --out/<name>
//
// --out is a base directory (default ~/.rmb/skills); each skill lands in
// <out>/<name>. `rmb pull rmb://skills/` with no extra flag pulls all skills.
func pullCmd(args []string) int {
	pos := positionalArgs(args)
	if len(pos) == 0 {
		fmt.Fprintln(os.Stderr, `usage: rmb pull <uri> [--out=<dir>]
  rmb pull rmb://skills/<name>   one skill → <out>/<name>
  rmb pull rmb://skills/         every skill → <out>/<name>`)
		return 2
	}
	target, err := uri.Parse(pos[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "pull: %v\n", err)
		return 2
	}
	if target.Scope != uri.ScopeSkills || len(target.Segments) > 1 {
		fmt.Fprintln(os.Stderr, "pull: only rmb://skills/<name> and rmb://skills/ are supported")
		return 2
	}

	outBase := strings.TrimSpace(parseFlagValue(args, "--out"))
	if outBase == "" {
		base, err := skillsRoot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pull: %v\n", err)
			return 1
		}
		outBase = base
	}

	cl, err := apiClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pull: %v\n", err)
		return 1
	}

	if len(target.Segments) == 0 {
		items, err := cl.ListSkills(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "pull: %v\n", err)
			return 1
		}
		for _, it := range items {
			parsed, err := uri.Parse(it.URI)
			if err != nil || len(parsed.Segments) == 0 {
				continue
			}
			dest := filepath.Join(outBase, parsed.Segments[0])
			if err := materializeSkill(cl, parsed.Segments[0], dest); err != nil {
				fmt.Fprintf(os.Stderr, "pull: %v\n", err)
				return 1
			}
		}
		return 0
	}

	name := target.Segments[0]
	dest := filepath.Join(outBase, name)
	if err := materializeSkill(cl, name, dest); err != nil {
		fmt.Fprintf(os.Stderr, "pull: %v\n", err)
		return 1
	}
	return 0
}

// putCmd implements `rmb put rmb://skills/<name> [--dir=<path>]` — upload a
// local skill directory (default ~/.rmb/skills/<name>) as a new skill version.
func putCmd(args []string) int {
	pos := positionalArgs(args)
	if len(pos) == 0 {
		fmt.Fprintln(os.Stderr, "usage: rmb put rmb://skills/<name> [--dir=<path>]")
		return 2
	}
	target, err := uri.Parse(pos[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "put: %v\n", err)
		return 2
	}
	if target.Scope != uri.ScopeSkills || len(target.Segments) != 1 {
		fmt.Fprintln(os.Stderr, "put: uri must be rmb://skills/<name>")
		return 2
	}
	name := target.Segments[0]

	dir := strings.TrimSpace(parseFlagValue(args, "--dir"))
	if dir == "" {
		defaultDir, err := skillDir(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "put: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "put: %v\n", err)
		return 1
	}

	cl, err := apiClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "put: %v\n", err)
		return 1
	}
	result, err := cl.PutSkill(context.Background(), name, files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "put: %v\n", err)
		return 1
	}
	if result.NoOp {
		fmt.Printf("unchanged: %s (version %d)\n", result.URI, result.Version)
		return 0
	}
	fmt.Printf("uploaded: %s (version %d)\n", result.URI, result.Version)
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
