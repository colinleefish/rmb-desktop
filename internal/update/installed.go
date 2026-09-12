package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// sidecarStampFile is written under installDir after a successful sidecar swap.
const sidecarStampFile = ".sidecar-version.json"

type sidecarStamp struct {
	Version string `json:"version"`
}

// WriteSidecarStamp records the installed sidecar release version.
func WriteSidecarStamp(installDir, version string) error {
	version = strings.TrimSpace(version)
	if installDir == "" || version == "" {
		return nil
	}
	data, err := json.Marshal(sidecarStamp{Version: version})
	if err != nil {
		return err
	}
	path := filepath.Join(installDir, sidecarStampFile)
	return os.WriteFile(path, data, 0o644)
}

// ReadSidecarStamp returns the stamped version, or "" if missing/invalid.
func ReadSidecarStamp(installDir string) string {
	if installDir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(installDir, sidecarStampFile))
	if err != nil {
		return ""
	}
	var st sidecarStamp
	if err := json.Unmarshal(data, &st); err != nil {
		return ""
	}
	return strings.TrimSpace(st.Version)
}
