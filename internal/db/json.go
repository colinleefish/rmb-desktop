package db

import (
	"encoding/json"
	"fmt"
)

// MarshalStringArray JSON-encodes a string slice for SQLite TEXT columns.
func MarshalStringArray(items []string) (string, error) {
	if items == nil {
		items = []string{}
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("marshal string array: %w", err)
	}
	return string(raw), nil
}

// UnmarshalStringArray decodes a JSON string array from SQLite.
func UnmarshalStringArray(raw string) ([]string, error) {
	if raw == "" {
		return []string{}, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("unmarshal string array: %w", err)
	}
	return out, nil
}
