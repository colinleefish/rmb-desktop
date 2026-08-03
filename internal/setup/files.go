package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func readFile(path string) (string, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(b), true, nil
}

func writeFileWithBackup(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		backup := path + ".rmb.bak"
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if writeErr := os.WriteFile(backup, data, 0o644); writeErr != nil {
			return writeErr
		}
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func prettyJSON(raw string) (string, error) {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return "", err
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}

func changeTypeFor(current, proposed string, exists bool) ChangeType {
	if current == proposed {
		return ChangeUnchanged
	}
	if !exists {
		return ChangeCreate
	}
	return ChangeModify
}

func changeTypeForJSON(rawCurrent, rawProposed string, exists bool) ChangeType {
	if jsonEquivalent(rawCurrent, rawProposed) {
		return ChangeUnchanged
	}
	if !exists || strings.TrimSpace(rawCurrent) == "" {
		return ChangeCreate
	}
	return ChangeModify
}

func jsonEquivalent(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == b {
		return true
	}
	var va, vb any
	if err := json.Unmarshal([]byte(a), &va); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(b), &vb); err != nil {
		return false
	}
	ja, errA := json.Marshal(va)
	jb, errB := json.Marshal(vb)
	return errA == nil && errB == nil && string(ja) == string(jb)
}

func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func anyPathExists(paths []string) bool {
	for _, p := range paths {
		if pathExists(p) {
			return true
		}
	}
	return false
}

func artifactFromStrings(id, title, path, desc, current, proposed string, exists bool, apply ApplyMode, warnings []string, lang string) Artifact {
	change := changeTypeFor(current, proposed, exists)
	return Artifact{
		ID:          id,
		Title:       title,
		Path:        displayPath(path),
		Description: desc,
		Exists:      exists,
		Current:     current,
		Proposed:    proposed,
		ChangeType:  change,
		ApplyMode:   apply,
		Warnings:    warnings,
		Language:    lang,
	}
}

func artifactFromRaw(id, title, path, desc, rawCurrent, rawProposed, displayCurrent, displayProposed string, exists bool, apply ApplyMode, warnings []string, lang string) Artifact {
	change := changeTypeFor(rawCurrent, rawProposed, exists)
	if lang == "json" {
		change = changeTypeForJSON(rawCurrent, rawProposed, exists)
	}
	return Artifact{
		ID:          id,
		Title:       title,
		Path:        displayPath(path),
		Description: desc,
		Exists:      exists,
		Current:     displayCurrent,
		Proposed:    displayProposed,
		ChangeType:  change,
		ApplyMode:   apply,
		Warnings:    warnings,
		Language:    lang,
	}
}

func hookStatus(configured bool) string {
	if configured {
		return "configured"
	}
	return "none"
}

func recallStatus(configured bool) string {
	if configured {
		return "configured"
	}
	return "none"
}

func applyErrorCopyOnly(id string) error {
	return fmt.Errorf("artifact %q is copy-only; paste the proposed content manually", id)
}
