package harness

import (
	"fmt"
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
)

// Apply mutates a copy of vocab when Operate accepted a draft.
func Apply(vocab catalog.Vocabulary, res Result) (catalog.Vocabulary, error) {
	if !res.DraftAccepted || res.Draft == nil {
		return vocab, nil
	}
	d := *res.Draft
	out := vocab
	out.Terms = append([]catalog.Term{}, vocab.Terms...)
	switch strings.TrimSpace(d.Kind) {
	case DraftKindNewLeaf:
		out.Terms = append(out.Terms, catalog.Term{
			ID:          strings.TrimSpace(d.ID),
			Label:       strings.TrimSpace(d.Label),
			Parent:      strings.TrimSpace(d.Parent),
			Description: strings.TrimSpace(d.Description),
			Status:      catalog.StatusActive,
		})
	case DraftKindAlias:
		leafID := strings.TrimSpace(d.LeafID)
		alias := strings.TrimSpace(d.Alias)
		updated := false
		for i, t := range out.Terms {
			if strings.TrimSpace(t.ID) != leafID {
				continue
			}
			aliases := append([]string{}, t.Aliases...)
			aliases = append(aliases, alias)
			t.Aliases = uniqueAliases(aliases)
			out.Terms[i] = t
			updated = true
			break
		}
		if !updated {
			return catalog.Vocabulary{}, fmt.Errorf("harness.Apply: leaf %q not found", leafID)
		}
	default:
		return catalog.Vocabulary{}, fmt.Errorf("harness.Apply: unsupported draft kind %q", d.Kind)
	}
	return out, nil
}

func uniqueAliases(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, a := range in {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		key := strings.ToLower(a)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, a)
	}
	return out
}
