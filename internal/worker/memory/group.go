package memory

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/model"
	"github.com/colinleefish/rmb-desktop/internal/uri"
)

type Bucket struct {
	Category string
	Slug     string
	URI      string
	Atoms    []model.Atom
}

type atomLLMInput struct {
	URI      string `json:"uri"`
	Priority int    `json:"priority"`
	Content  string `json:"content"`
}

func groupAtomsIntoBuckets(atoms []model.Atom) ([]Bucket, int) {
	profile := make([]model.Atom, 0)
	type key struct{ category, slug string }
	slugged := make(map[key]*Bucket)
	order := make([]key, 0)
	skipped := 0

	for _, atom := range atoms {
		switch atom.Category {
		case model.AtomCategoryProfile:
			profile = append(profile, atom)
		case model.AtomCategoryPreferences, model.AtomCategoryEntities, model.AtomCategoryEvents:
			rawSlug := ""
			if atom.Slug != nil {
				rawSlug = strings.TrimSpace(*atom.Slug)
			}
			if rawSlug == "" {
				skipped++
				continue
			}
			slug, err := uri.SanitizeSlug(rawSlug)
			if err != nil {
				skipped++
				continue
			}
			k := key{atom.Category, slug}
			b, ok := slugged[k]
			if !ok {
				b = &Bucket{
					Category: atom.Category,
					Slug:     slug,
					URI:      uri.BuildMemory(atom.Category, slug),
				}
				slugged[k] = b
				order = append(order, k)
			}
			b.Atoms = append(b.Atoms, atom)
		default:
			skipped++
		}
	}

	buckets := make([]Bucket, 0, len(order)+1)
	if len(profile) > 0 {
		buckets = append(buckets, Bucket{
			Category: model.AtomCategoryProfile,
			URI:      uri.BuildProfile(),
			Atoms:    profile,
		})
	}

	sort.Slice(order, func(i, j int) bool {
		if order[i].category != order[j].category {
			return order[i].category < order[j].category
		}
		return order[i].slug < order[j].slug
	})
	for _, k := range order {
		buckets = append(buckets, *slugged[k])
	}
	return buckets, skipped
}

func chunkAtoms(atoms []model.Atom, maxAtoms int) [][]model.Atom {
	if maxAtoms <= 0 || len(atoms) <= maxAtoms {
		return [][]model.Atom{atoms}
	}
	var chunks [][]model.Atom
	for i := 0; i < len(atoms); i += maxAtoms {
		end := i + maxAtoms
		if end > len(atoms) {
			end = len(atoms)
		}
		chunks = append(chunks, atoms[i:end])
	}
	return chunks
}

func serializeAtomsForLLM(atoms []model.Atom) (string, error) {
	inputs := make([]atomLLMInput, 0, len(atoms))
	for _, atom := range atoms {
		inputs = append(inputs, atomLLMInput{
			URI:      uri.BuildAtom(atom.ID),
			Priority: atom.Priority,
			Content:  atom.Content,
		})
	}
	raw, err := json.Marshal(map[string]any{"facts": inputs})
	if err != nil {
		return "", fmt.Errorf("marshal atoms for llm: %w", err)
	}
	return string(raw), nil
}

func serializePartialsForLLM(partials []string) (string, error) {
	raw, err := json.Marshal(map[string]any{"facts": partials})
	if err != nil {
		return "", fmt.Errorf("marshal partials for llm: %w", err)
	}
	return string(raw), nil
}

func buildAtomSceneIndex(scenes []model.Scene) map[string][]string {
	index := make(map[string][]string)
	for _, scene := range scenes {
		sceneURI := uri.BuildScene(scene.ID)
		for _, atomID := range scene.SourceAtoms {
			index[atomID] = append(index[atomID], sceneURI)
		}
	}
	return index
}

func sourceSceneURIsFor(bucket Bucket, index map[string][]string) []string {
	seen := make(map[string]struct{})
	for _, atom := range bucket.Atoms {
		for _, sceneURI := range index[atom.ID] {
			seen[sceneURI] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for sceneURI := range seen {
		out = append(out, sceneURI)
	}
	sort.Strings(out)
	return out
}

func equalStringSets(a, b []string) bool {
	counts := make(map[string]int, len(a))
	for _, s := range a {
		counts[s] = 1
	}
	for _, s := range b {
		if _, ok := counts[s]; !ok {
			return false
		}
		counts[s] = 0 // mark matched; duplicates in b just re-mark
	}
	for _, v := range counts {
		if v != 0 {
			return false
		}
	}
	return true
}
