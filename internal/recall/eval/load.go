package eval

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadGolden parses golden.yaml.
func LoadGolden(path string) (*GoldenSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read golden %s: %w", path, err)
	}
	var g GoldenSet
	if err := yaml.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("parse golden %s: %w", path, err)
	}
	return &g, nil
}

// LoadFixture parses a fixture.json snapshot.
func LoadFixture(path string) (*Fixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixture %s: %w", path, err)
	}
	var f Fixture
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse fixture %s: %w", path, err)
	}
	return &f, nil
}
