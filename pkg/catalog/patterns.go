package catalog

import (
	"fmt"
	"regexp"
	"strings"
)

type compiledFieldPattern struct {
	field string
	re    *regexp.Regexp
}

func compileTermPatterns(termID string, raw []FieldPattern) ([]FieldPattern, []compiledFieldPattern, error) {
	seen := map[string]struct{}{}
	patterns := make([]FieldPattern, 0, len(raw))
	compiled := make([]compiledFieldPattern, 0, len(raw))
	for _, p := range raw {
		field := strings.TrimSpace(p.Field)
		reStr := strings.TrimSpace(p.Re)
		if field == "" || reStr == "" {
			return nil, nil, newErr("Build", CodeInvalidPattern, fmt.Sprintf("term %s: pattern field and re required", termID), nil)
		}
		key := field + "\x00" + reStr
		if _, dup := seen[key]; dup {
			return nil, nil, newErr("Build", CodeInvalidPattern, fmt.Sprintf("term %s: duplicate pattern on field %q", termID, field), nil)
		}
		seen[key] = struct{}{}
		re, err := regexp.Compile(reStr)
		if err != nil {
			return nil, nil, newErr("Build", CodeInvalidPattern, fmt.Sprintf("term %s: invalid pattern on field %q: %v", termID, field, err), err)
		}
		patterns = append(patterns, FieldPattern{Field: field, Re: reStr})
		compiled = append(compiled, compiledFieldPattern{field: field, re: re})
	}
	return patterns, compiled, nil
}
