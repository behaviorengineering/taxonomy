package catalog

import (
	"fmt"
	"regexp"
	"strings"
)

// LearnExact appends exact-match patterns (QuoteMeta) for non-empty field values on termID.
func LearnExact(vocab Vocabulary, termID string, fields map[string]string) (Vocabulary, error) {
	termID = strings.TrimSpace(termID)
	if termID == "" {
		return vocab, newErr("LearnExact", CodeInvalidVocab, "term id required", nil)
	}
	if fields == nil || len(fields) == 0 {
		return vocab, nil
	}

	idx := -1
	for i, t := range vocab.Terms {
		if strings.TrimSpace(t.ID) == termID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return vocab, newErr("LearnExact", CodeInvalidVocab, fmt.Sprintf("unknown term id %q", termID), nil)
	}

	term := vocab.Terms[idx]
	existing := map[string]struct{}{}
	for _, p := range term.Patterns {
		field := strings.TrimSpace(p.Field)
		reStr := strings.TrimSpace(p.Re)
		if field == "" || reStr == "" {
			continue
		}
		existing[field+"\x00"+reStr] = struct{}{}
	}

	added := false
	for key, val := range fields {
		field := strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if field == "" || val == "" {
			continue
		}
		reStr := "^" + regexp.QuoteMeta(val) + "$"
		pairKey := field + "\x00" + reStr
		if _, dup := existing[pairKey]; dup {
			continue
		}
		existing[pairKey] = struct{}{}
		term.Patterns = append(term.Patterns, FieldPattern{Field: field, Re: reStr})
		added = true
	}
	if !added {
		return vocab, nil
	}

	out := vocab
	out.Terms = append([]Term(nil), vocab.Terms...)
	out.Terms[idx] = term
	return out, nil
}
