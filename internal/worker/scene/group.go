package scene

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/model"
	"github.com/colinleefish/rmb-desktop/internal/uri"
	"github.com/google/uuid"
)

var sceneNamespace = uuid.MustParse("b6f6e2c2-7c1a-4e2b-9c3d-7a1f0d2e4b88")

func sceneIDForName(sessionID string, displayName string, dup int) string {
	name := strings.ToLower(strings.TrimSpace(displayName))
	seed := sessionID + "\x00" + name
	if dup > 1 {
		seed += "\x00" + strconv.Itoa(dup)
	}
	return uuid.NewSHA1(sceneNamespace, []byte(seed)).String()
}

type atomGroup struct {
	DisplayName string
	Atoms       []model.Atom
}

type atomInput struct {
	URI       string  `json:"uri"`
	Category  string  `json:"category"`
	Priority  int     `json:"priority"`
	SceneName string  `json:"scene_name"`
	Slug      *string `json:"slug,omitempty"`
	Content   string  `json:"content"`
}

func groupAtomsBySceneName(atoms []model.Atom) []atomGroup {
	byName := make(map[string][]model.Atom)
	order := make([]string, 0)
	for _, atom := range atoms {
		name := defaultSceneName
		if atom.SceneName != nil {
			if trimmed := strings.TrimSpace(*atom.SceneName); trimmed != "" {
				name = trimmed
			}
		}
		if _, ok := byName[name]; !ok {
			order = append(order, name)
		}
		byName[name] = append(byName[name], atom)
	}
	sort.Strings(order)

	out := make([]atomGroup, 0, len(order))
	for _, name := range order {
		out = append(out, atomGroup{DisplayName: name, Atoms: byName[name]})
	}
	return out
}

func chunkGroups(groups []atomGroup, maxAtoms int) [][]atomGroup {
	if maxAtoms <= 0 || len(groups) == 0 {
		return [][]atomGroup{groups}
	}
	var chunks [][]atomGroup
	var cur []atomGroup
	curCount := 0
	for _, g := range groups {
		n := len(g.Atoms)
		if len(cur) > 0 && curCount+n > maxAtoms {
			chunks = append(chunks, cur)
			cur = nil
			curCount = 0
		}
		cur = append(cur, g)
		curCount += n
	}
	if len(cur) > 0 {
		chunks = append(chunks, cur)
	}
	return chunks
}

func serializeAtomsForLLM(groups []atomGroup) (string, error) {
	inputs := make([]atomInput, 0)
	for _, group := range groups {
		for _, atom := range group.Atoms {
			sceneName := defaultSceneName
			if atom.SceneName != nil {
				sceneName = strings.TrimSpace(*atom.SceneName)
			}
			inputs = append(inputs, atomInput{
				URI:       uri.BuildAtom(atom.ID),
				Category:  atom.Category,
				Priority:  atom.Priority,
				SceneName: sceneName,
				Slug:      atom.Slug,
				Content:   atom.Content,
			})
		}
	}
	raw, err := json.Marshal(map[string]any{"atoms": inputs})
	if err != nil {
		return "", fmt.Errorf("marshal atoms for llm: %w", err)
	}
	return string(raw), nil
}

func atomURISet(atoms []model.Atom) map[string]struct{} {
	out := make(map[string]struct{}, len(atoms))
	for _, atom := range atoms {
		out[uri.BuildAtom(atom.ID)] = struct{}{}
	}
	return out
}
