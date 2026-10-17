package tags

import (
	"regexp"
	"slices"
	"strings"
)

var kebabTagRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// SplitTags splits a tag field on commas, semicolons, or whitespace.
func SplitTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = normalizeToken(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func normalizeToken(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// JoinTags returns sorted unique tags joined with single spaces.
func JoinTags(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	seen := make(map[string]struct{}, len(ids))
	uniq := make([]string, 0, len(ids))
	for _, id := range ids {
		id = normalizeToken(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}
	slices.Sort(uniq)
	return strings.Join(uniq, " ")
}

func denySet(ids []string) map[string]struct{} {
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = normalizeToken(id)
		if id != "" {
			out[id] = struct{}{}
		}
	}
	return out
}

// ValidateToken checks one tag id after normalize (kebab, deny list, not KIND path).
func ValidateToken(token string, deny map[string]struct{}) error {
	const op = "tags.ValidateToken"
	token = normalizeToken(token)
	if token == "" {
		return newErr(op, CodeGate, ErrTokenInvalid, nil)
	}
	if strings.Contains(token, ">") {
		return newErr(op, CodeGate, ErrTokenInvalid, nil)
	}
	if _, denied := deny[token]; denied {
		return newErr(op, CodeGate, ErrTokenInvalid, nil)
	}
	if isStopword(token) {
		return newErr(op, CodeGate, ErrTokenInvalid, nil)
	}
	if !kebabTagRE.MatchString(token) {
		return newErr(op, CodeGate, ErrTokenInvalid, nil)
	}
	return nil
}

// IsValidStoredToken is true for kebab tags allowed in stored shape scans.
func IsValidStoredToken(token string, deny map[string]struct{}) bool {
	token = normalizeToken(token)
	if token == "" {
		return false
	}
	if strings.Contains(token, ">") {
		return false
	}
	if deny != nil {
		if _, denied := deny[token]; denied {
			return false
		}
	}
	if isStopword(token) {
		return false
	}
	return kebabTagRE.MatchString(token)
}
