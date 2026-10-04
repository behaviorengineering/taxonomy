package catalog

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseYAML decodes one vocabulary document.
func ParseYAML(raw []byte) (Vocabulary, error) {
	var file fileShape
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return Vocabulary{}, newErr("Parse", CodeParse, "decode yaml", err)
	}
	return file.Vocabulary, nil
}

// SaveYAML encodes a vocabulary document.
func SaveYAML(vocab Vocabulary) ([]byte, error) {
	out, err := yaml.Marshal(fileShape{Vocabulary: vocab})
	if err != nil {
		return nil, newErr("Save", CodeParse, "encode yaml", err)
	}
	return out, nil
}

// Merge appends overlay terms onto base. Term id collisions fail closed.
func Merge(base, overlay Vocabulary) (Vocabulary, error) {
	seen := map[string]struct{}{}
	out := Vocabulary{
		ID:          base.ID,
		Label:       base.Label,
		Description: base.Description,
		Aliases:     append([]string{}, base.Aliases...),
		Terms:       append([]Term{}, base.Terms...),
	}
	for _, t := range base.Terms {
		seen[strings.TrimSpace(t.ID)] = struct{}{}
	}
	for _, t := range overlay.Terms {
		id := strings.TrimSpace(t.ID)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			return Vocabulary{}, newErr("Merge", CodeMergeCollision, fmt.Sprintf("term %q collides with base vocabulary", id), nil)
		}
		seen[id] = struct{}{}
		out.Terms = append(out.Terms, t)
	}
	if strings.TrimSpace(overlay.Label) != "" && out.Label == "" {
		out.Label = overlay.Label
	}
	if strings.TrimSpace(overlay.Description) != "" && out.Description == "" {
		out.Description = overlay.Description
	}
	for _, a := range overlay.Aliases {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		out.Aliases = uniqueNonEmpty(append(out.Aliases, a))
	}
	return out, nil
}
