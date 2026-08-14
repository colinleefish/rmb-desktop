package scene

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/model"
)

// DryRunStep is one step in a T2 dry-run trace.
type DryRunStep struct {
	Name        string `json:"name"`
	MS          int64  `json:"ms"`
	OK          bool   `json:"ok"`
	Detail      string `json:"detail,omitempty"`
	RawPreview  string `json:"raw_preview,omitempty"`
	Error       string `json:"error,omitempty"`
}

// DryRunResult is returned by T2 dry-run endpoints.
type DryRunResult struct {
	SessionKey  string       `json:"session_key"`
	SessionID   string       `json:"session_id"`
	AtomCount   int          `json:"atom_count"`
	SceneCount  int          `json:"scene_count"`
	Steps       []DryRunStep `json:"steps"`
	Error       string       `json:"error,omitempty"`
}

// DryRunT2 runs the T2 LLM pipeline for one session without persisting scenes.
func DryRunT2(
	ctx context.Context,
	database *sql.DB,
	llm SceneBuilder,
	cfg config.PipelineConfig,
	sessionID string,
) (*DryRunResult, error) {
	var sessionKey string
	err := database.QueryRowContext(ctx, `SELECT session_key FROM sessions WHERE id = ?`, sessionID).Scan(&sessionKey)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found")
	}
	if err != nil {
		return nil, err
	}

	result := &DryRunResult{
		SessionKey: sessionKey,
		SessionID:  sessionID,
		Steps:      []DryRunStep{},
	}

	step := func(name string, fn func() (string, error)) error {
		start := time.Now()
		detail, err := fn()
		s := DryRunStep{
			Name: name,
			MS:   time.Since(start).Milliseconds(),
			OK:   err == nil,
		}
		if detail != "" {
			s.Detail = detail
		}
		if err != nil {
			s.Error = err.Error()
			result.Steps = append(result.Steps, s)
			result.Error = err.Error()
			return err
		}
		result.Steps = append(result.Steps, s)
		return nil
	}

	var atoms []model.Atom
	if err := step("load_atoms", func() (string, error) {
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return "", err
		}
		defer func() { _ = tx.Rollback() }()
		atoms, err = loadSessionAtoms(ctx, tx, sessionID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d atoms", len(atoms)), nil
	}); err != nil {
		return result, nil
	}
	atoms = trimSessionAtoms(atoms, cfg.L2MaxAtoms)
	result.AtomCount = len(atoms)
	if len(atoms) == 0 {
		result.Error = "no atoms to build scenes from"
		return result, nil
	}

	groups := groupAtomsBySceneName(atoms)
	chunks := chunkGroups(groups, cfg.L2MaxAtoms, cfg.L2MaxScenes)
	validURIs := atomURISet(atoms)

	var parsed []ParsedScene
	for i, chunk := range chunks {
		chunkName := fmt.Sprintf("chunk_%d", i+1)
		if err := step("serialize_"+chunkName, func() (string, error) {
			raw, err := serializeAtomsForLLM(chunk)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d bytes", len(raw)), nil
		}); err != nil {
			return result, nil
		}

		var llmRaw string
		if err := step("llm.build_scenes_"+chunkName, func() (string, error) {
			atomsJSON, err := serializeAtomsForLLM(chunk)
			if err != nil {
				return "", err
			}
			raw, err := llm.BuildScenes(ctx, atomsJSON)
			if err != nil {
				return "", err
			}
			llmRaw = raw
			return fmt.Sprintf("%d bytes response", len(raw)), nil
		}); err != nil {
			if n := len(result.Steps); n > 0 {
				result.Steps[n-1].RawPreview = preview(llmRaw, 800)
			}
			return result, nil
		}

		if err := step("parse_"+chunkName, func() (string, error) {
			chunkScenes, err := parseBuildScenesResponse(llmRaw, validURIs)
			if err != nil {
				result.Steps[len(result.Steps)-2].RawPreview = preview(llmRaw, 800)
				return "", err
			}
			parsed = append(parsed, chunkScenes...)
			return fmt.Sprintf("%d scenes", len(chunkScenes)), nil
		}); err != nil {
			return result, nil
		}
	}

	if err := step("llm.session_abstract", func() (string, error) {
		abstract, err := llm.SummarizeSessionAbstract(ctx, joinSceneAbstracts(parsed))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d chars", len(abstract)), nil
	}); err != nil {
		return result, nil
	}

	result.SceneCount = len(parsed)
	return result, nil
}

// BuildScenesProbe runs BuildScenes against inline or session atoms JSON.
func BuildScenesProbe(ctx context.Context, llm SceneBuilder, atomsJSON string) (raw string, scenes []ParsedScene, steps []DryRunStep, err error) {
	start := time.Now()
	raw, err = llm.BuildScenes(ctx, atomsJSON)
	steps = append(steps, DryRunStep{
		Name:       "llm.build_scenes",
		MS:         time.Since(start).Milliseconds(),
		OK:         err == nil,
		Detail:     fmt.Sprintf("%d bytes response", len(raw)),
		RawPreview: preview(raw, 1200),
	})
	if err != nil {
		steps[len(steps)-1].Error = err.Error()
		return "", nil, steps, err
	}

	// Validate atom_uris against the input payload when present.
	valid := validURIsFromAtomsJSON(atomsJSON)
	start = time.Now()
	scenes, err = parseBuildScenesResponse(raw, valid)
	step := DryRunStep{
		Name: "parse",
		MS:   time.Since(start).Milliseconds(),
		OK:   err == nil,
	}
	if err != nil {
		step.Error = err.Error()
	} else {
		step.Detail = fmt.Sprintf("%d scenes", len(scenes))
	}
	steps = append(steps, step)
	return raw, scenes, steps, err
}

func validURIsFromAtomsJSON(atomsJSON string) map[string]struct{} {
	var payload struct {
		Atoms []struct {
			URI string `json:"uri"`
		} `json:"atoms"`
	}
	if err := json.Unmarshal([]byte(atomsJSON), &payload); err != nil {
		return map[string]struct{}{}
	}
	out := make(map[string]struct{}, len(payload.Atoms))
	for _, a := range payload.Atoms {
		if u := strings.TrimSpace(a.URI); u != "" {
			out[u] = struct{}{}
		}
	}
	return out
}

func preview(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
