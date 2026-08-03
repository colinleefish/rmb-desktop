package setup

import "fmt"

// Status returns detection and configuration summary for all agents.
func Status() (StatusResponse, error) {
	out := StatusResponse{Agents: make([]AgentSummary, 0, len(agentDefs))}
	for _, def := range agentDefs {
		state, err := Preview(def.ID)
		if err != nil {
			return StatusResponse{}, err
		}
		out.Agents = append(out.Agents, AgentSummary{
			ID:           string(def.ID),
			Name:         def.Label,
			Detected:     state.Detected,
			HookStatus:   state.HookStatus,
			RecallStatus: state.RecallStatus,
		})
	}
	return out, nil
}

// Preview builds the full agent setup state for one agent.
func Preview(id AgentID) (AgentState, error) {
	def, ok := agentDefByID(id)
	if !ok {
		return AgentState{}, fmt.Errorf("unknown agent %q", id)
	}
	switch id {
	case AgentCursor:
		return previewCursor(def)
	case AgentClaudeCode:
		return previewClaude(def)
	case AgentCodex:
		return previewCodex(def)
	case AgentOpenCode:
		return previewOpenCode(def)
	case AgentPi:
		return previewPi(def)
	default:
		return AgentState{}, fmt.Errorf("unknown agent %q", id)
	}
}

// PreviewByName resolves an agent id string (including aliases).
func PreviewByName(raw string) (AgentState, error) {
	id, ok := parseAgentID(raw)
	if !ok {
		return AgentState{}, fmt.Errorf("unknown agent %q", raw)
	}
	return Preview(id)
}

// Apply writes selected artifacts for an agent.
func Apply(raw string, artifactIDs []string) (ApplyResponse, error) {
	id, ok := parseAgentID(raw)
	if !ok {
		return ApplyResponse{}, fmt.Errorf("unknown agent %q", raw)
	}
	applied := make([]string, 0, len(artifactIDs))
	for _, artifactID := range artifactIDs {
		if err := applyArtifact(id, artifactID); err != nil {
			return ApplyResponse{}, err
		}
		applied = append(applied, artifactID)
	}
	state, err := Preview(id)
	if err != nil {
		return ApplyResponse{}, err
	}
	return ApplyResponse{Applied: applied, Agent: state}, nil
}

func applyArtifact(id AgentID, artifactID string) error {
	switch id {
	case AgentCursor:
		return applyCursor(artifactID)
	case AgentClaudeCode:
		return applyClaude(artifactID)
	case AgentCodex:
		return applyCodex(artifactID)
	case AgentOpenCode:
		return applyOpenCode(artifactID)
	case AgentPi:
		return applyPi(artifactID)
	default:
		return fmt.Errorf("unknown agent %q", id)
	}
}
