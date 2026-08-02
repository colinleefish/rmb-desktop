package uri

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const (
	Scheme        = "rmb"
	MaxSegment    = 50
	ScopeAtoms    = "atoms"
	ScopeScenes   = "scenes"
	ScopeProfile  = "profile"
	ScopePrefs    = "preferences"
	ScopeEntities = "entities"
	ScopeEvents   = "events"
)

var (
	ErrInvalidURI = errors.New("invalid rmb uri")
	uuidSegment   = regexp.MustCompile(
		`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
	)
	reservedSlug = map[string]struct{}{
		"atoms": {}, "scenes": {}, "profile": {}, "preferences": {},
		"entities": {}, "events": {}, "sessions": {}, "turns": {},
	}
)

func BuildAtom(atomID string) string {
	return Scheme + "://" + ScopeAtoms + "/" + strings.ToLower(atomID)
}

func BuildScene(sceneID string) string {
	return Scheme + "://" + ScopeScenes + "/" + strings.ToLower(sceneID)
}

func BuildProfile() string {
	return Scheme + "://" + ScopeProfile
}

func BuildMemory(category, segment string) string {
	return Scheme + "://" + category + "/" + segment
}

func BuildCorrection(id string) string {
	return Scheme + "://" + ScopeCorrections + "/" + strings.ToLower(id)
}

func ParseAtomID(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("%w: empty atom reference", ErrInvalidURI)
	}
	if strings.HasPrefix(s, Scheme+"://") {
		s = strings.TrimPrefix(s, Scheme+"://")
		parts := strings.Split(strings.Trim(s, "/"), "/")
		if len(parts) != 2 || parts[0] != ScopeAtoms {
			return "", fmt.Errorf("%w: not an atom uri", ErrInvalidURI)
		}
		s = parts[1]
	}
	if !uuidSegment.MatchString(s) {
		return "", fmt.Errorf("%w: invalid atom id %q", ErrInvalidURI, raw)
	}
	return strings.ToLower(s), nil
}

func SanitizeSlug(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("%w: empty segment", ErrInvalidURI)
	}

	var b strings.Builder
	prevSep := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevSep = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r - 'A' + 'a')
			prevSep = false
		case r == '-':
			if b.Len() > 0 && !prevSep {
				b.WriteByte('-')
				prevSep = true
			}
		case isSlugPreservedRune(r):
			b.WriteRune(r)
			prevSep = false
		default:
			if b.Len() > 0 && !prevSep {
				b.WriteByte('-')
				prevSep = true
			}
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "", fmt.Errorf("%w: segment sanitizes to empty", ErrInvalidURI)
	}
	if len(out) > MaxSegment {
		out = out[:MaxSegment]
		out = strings.TrimRight(out, "-")
	}
	if _, forbidden := reservedSlug[strings.ToLower(out)]; forbidden {
		return "", fmt.Errorf("%w: segment %q is reserved", ErrInvalidURI, out)
	}
	return out, nil
}

func isSlugPreservedRune(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r) ||
		unicode.Is(unicode.Cyrillic, r)
}
