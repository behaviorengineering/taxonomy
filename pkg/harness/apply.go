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
	case DraftKindBreadcrumb:
		parent := strings.TrimSpace(d.Parent)
		kind := strings.TrimSpace(d.Alias)
		if kind == "" && res.Essence != nil {
			kind = strings.TrimSpace(res.Essence.Kind)
		}
		segs := catalog.ParseKindSegments(kind)
		if len(segs) == 0 {
			return catalog.Vocabulary{}, newErr("Apply", CodeInvalidDraft, "breadcrumb kind required", nil)
		}
		about := strings.TrimSpace(d.Description)
		if about == "" {
			return catalog.Vocabulary{}, newErr("Apply", CodeInvalidDraft, "breadcrumb about required", nil)
		}
		shape := ""
		if res.Essence != nil {
			shape = strings.TrimSpace(res.Essence.Shape)
		}
		updated, _, err := catalog.EnsurePath(out, catalog.EnsurePathInput{
			Segments:    segs,
			UnderParent: parent,
			About:       about,
			Shape:       shape,
		})
		if err != nil {
			return catalog.Vocabulary{}, newErr("Apply", CodeInvalidDraft, "breadcrumb path", err)
		}
		return updated, nil
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
			return catalog.Vocabulary{}, newErr("Apply", CodeInvalidDraft, fmt.Sprintf("leaf %q not found", leafID), nil)
		}
	default:
		return catalog.Vocabulary{}, newErr("Apply", CodeInvalidDraft, fmt.Sprintf("unsupported draft kind %q", d.Kind), nil)
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
