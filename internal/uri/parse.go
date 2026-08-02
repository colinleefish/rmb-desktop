package uri

import (
	"fmt"
	"strings"
)

const (
	ScopeRoot     = ""
	ScopeSessions = "sessions"
	ScopeTurns    = "turns"
	ScopeAgent    = "agent"
	ScopeSkills     = "skills"
	ScopeCorrections = "corrections"
)

type URI struct {
	Scope     string
	Segments  []string
	Container bool
}

func Parse(raw string) (URI, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return URI{}, fmt.Errorf("%w: empty", ErrInvalidURI)
	}
	if strings.Contains(s, "{") || strings.Contains(s, "}") {
		return URI{}, fmt.Errorf("%w: reserved template syntax", ErrInvalidURI)
	}

	switch {
	case strings.HasPrefix(s, Scheme+"://"):
		s = strings.TrimPrefix(s, Scheme+"://")
	case strings.HasPrefix(s, "/"):
		s = strings.TrimPrefix(s, "/")
	case strings.HasPrefix(s, Scheme+":"):
		return URI{}, fmt.Errorf("%w: missing // after scheme", ErrInvalidURI)
	}

	container := strings.HasSuffix(s, "/")
	s = strings.TrimSuffix(s, "/")

	parts := splitSegments(s)
	if len(parts) == 0 {
		return URI{Scope: ScopeRoot, Container: container}, nil
	}

	scope := parts[0]
	if err := validateScope(scope); err != nil {
		return URI{}, err
	}

	segments := parts[1:]
	for _, seg := range segments {
		if strings.TrimSpace(seg) == "" {
			return URI{}, fmt.Errorf("%w: empty path segment", ErrInvalidURI)
		}
		if len(seg) > MaxSegment {
			return URI{}, fmt.Errorf("%w: segment too long", ErrInvalidURI)
		}
	}

	return URI{Scope: scope, Segments: segments, Container: container}, nil
}

func (u URI) String() string {
	if u.Scope == ScopeRoot && len(u.Segments) == 0 {
		return Scheme + "://"
	}
	var b strings.Builder
	b.WriteString(Scheme)
	b.WriteString("://")
	b.WriteString(u.Scope)
	for _, seg := range u.Segments {
		b.WriteByte('/')
		b.WriteString(seg)
	}
	if u.Container {
		b.WriteByte('/')
	}
	return b.String()
}

func (u URI) IsRoot() bool {
	return u.Scope == ScopeRoot && len(u.Segments) == 0
}

func (u URI) IsContainer() bool {
	return u.Container
}

func BuildSession(sessionKey string) string {
	return Scheme + "://" + ScopeSessions + "/" + strings.ToLower(sessionKey)
}

func BuildTurn(turnID string) string {
	return Scheme + "://" + ScopeTurns + "/" + strings.ToLower(turnID)
}

func BuildAgent() string {
	return Scheme + "://" + ScopeAgent
}

func splitSegments(path string) []string {
	if path == "" {
		return nil
	}
	raw := strings.Split(path, "/")
	out := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func validateScope(scope string) error {
	switch scope {
	case ScopeSessions, ScopeTurns, ScopeAtoms, ScopeScenes, ScopeProfile, ScopeAgent,
		ScopePrefs, ScopeEntities, ScopeEvents, ScopeSkills, ScopeCorrections:
		return nil
	default:
		return fmt.Errorf("%w: unknown scope %q", ErrInvalidURI, scope)
	}
}
