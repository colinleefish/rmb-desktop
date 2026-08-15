package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func detectAgent(def agentDef) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	for _, rel := range def.DetectPaths {
		if pathExists(filepath.Join(home, rel)) {
			return true
		}
	}
	return false
}

func redactSettingsJSON(raw string) string {
	var root map[string]any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return raw
	}
	if env, ok := root["env"].(map[string]any); ok {
		for k := range env {
			env[k] = "••••••"
		}
		root["env"] = env
	}
	out, err := json.Marshal(root)
	if err != nil {
		return raw
	}
	return string(out)
}
