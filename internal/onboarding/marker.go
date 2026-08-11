package onboarding

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/platform"
)

// Marker is stored in onboarding.complete when setup finishes.
type Marker struct {
	CompletedAt   string `json:"completed_at"`
	SkippedAgents bool   `json:"skipped_agents"`
}

// Status reports whether the onboarding marker file exists.
func Status() (bool, Marker, string, error) {
	path, err := platform.OnboardingCompletePath()
	if err != nil {
		return false, Marker{}, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, Marker{}, path, nil
		}
		return false, Marker{}, path, fmt.Errorf("read onboarding marker: %w", err)
	}
	if len(data) == 0 {
		return true, Marker{}, path, nil
	}
	var marker Marker
	if err := json.Unmarshal(data, &marker); err != nil {
		return true, Marker{}, path, nil
	}
	return true, marker, path, nil
}

// MarkComplete writes the onboarding marker file.
func MarkComplete(skippedAgents bool) (string, error) {
	path, err := platform.OnboardingCompletePath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, fmt.Errorf("mkdir onboarding dir: %w", err)
	}
	marker := Marker{
		CompletedAt:   time.Now().Format(time.RFC3339),
		SkippedAgents: skippedAgents,
	}
	data, err := json.Marshal(marker)
	if err != nil {
		return path, fmt.Errorf("marshal onboarding marker: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return path, fmt.Errorf("write onboarding marker: %w", err)
	}
	return path, nil
}

// Reset removes the onboarding marker file.
func Reset() error {
	path, err := platform.OnboardingCompletePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove onboarding marker: %w", err)
	}
	return nil
}
