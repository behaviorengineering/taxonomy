package catalog

import (
	"fmt"
	"strings"
)

// AliasDrop records one alias removed during repair.
type AliasDrop struct {
	TermID string
	Alias  string
}

type aliasClaim struct {
	TermID    string
	Path      []string
	YAMLIndex int
	Spelling  string
}

// aliasKeyOwner returns the first term that owns normKey as an alias (not term id).
func aliasKeyOwner(vocab Vocabulary, normKey string) (termID string, ok bool) {
	if normKey == "" {
		return "", false
	}
	for _, term := range vocab.Terms {
		id := strings.TrimSpace(term.ID)
		for _, a := range term.Aliases {
			if normID(a) == normKey {
				return id, true
			}
		}
	}
	return "", false
}

// AliasOwner returns the first YAML-order occupant of normID(alias) among term ids and aliases.
func AliasOwner(vocab Vocabulary, alias string) (termID string, ok bool) {
	key := normID(alias)
	if key == "" {
		return "", false
	}
	for _, term := range vocab.Terms {
		id := strings.TrimSpace(term.ID)
		if id == "" {
			continue
		}
		if normID(id) == key {
			return id, true
		}
		for _, a := range term.Aliases {
			if normID(a) == key {
				return id, true
			}
		}
	}
	return "", false
}

// RepairAliases drops duplicate alias keys using the catalog winner rule.
func RepairAliases(vocab Vocabulary) (Vocabulary, []AliasDrop, error) {
	const op = "RepairAliases"
	out := vocab
	out.Terms = append([]Term{}, vocab.Terms...)

	if len(out.Terms) == 0 {
		return out, nil, nil
	}

	byID := map[string]int{}
	for i, term := range out.Terms {
		id := strings.TrimSpace(term.ID)
		if id == "" {
			return vocab, nil, newErr(op, CodeInvalidVocab, fmt.Sprintf("term %d: id required", i+1), nil)
		}
		if _, dup := byID[id]; dup {
			return vocab, nil, newErr(op, CodeDuplicateID, fmt.Sprintf("duplicate term id %q", id), nil)
		}
		byID[id] = i
	}

	parentOf := map[string]string{}
	for id, idx := range byID {
		parent := strings.TrimSpace(out.Terms[idx].Parent)
		if parent == "" {
			continue
		}
		if parent == id {
			return vocab, nil, newErr(op, CodeInvalidVocab, fmt.Sprintf("term %s: parent cannot be self", id), nil)
		}
		if _, ok := byID[parent]; !ok {
			return vocab, nil, newErr(op, CodeMissingParent, fmt.Sprintf("term %s: missing parent %q", id, parent), nil)
		}
		parentOf[id] = parent
	}

	paths := map[string][]string{}
	for id := range byID {
		path, err := repairAncestors(parentOf, id)
		if err != nil {
			if strings.Contains(err.Error(), "cycle") {
				return vocab, nil, newErr(op, CodeCycle, fmt.Sprintf("term %s: %v", id, err), err)
			}
			return vocab, nil, newErr(op, CodeMissingParent, fmt.Sprintf("term %s: %v", id, err), err)
		}
		paths[id] = path
	}

	for i := range out.Terms {
		out.Terms[i].Aliases = uniqueAliasesNorm(out.Terms[i].Aliases)
	}

	claimsByKey := map[string][]aliasClaim{}
	for i, term := range out.Terms {
		id := strings.TrimSpace(term.ID)
		for _, a := range term.Aliases {
			key := normID(a)
			if key == "" {
				continue
			}
			claimsByKey[key] = append(claimsByKey[key], aliasClaim{
				TermID:    id,
				Path:      paths[id],
				YAMLIndex: i,
				Spelling:  a,
			})
		}
	}

	var drops []AliasDrop
	for key, claims := range claimsByKey {
		if len(claims) <= 1 {
			continue
		}
		alias := claims[0].Spelling
		for _, c := range claims[1:] {
			if c.Spelling != "" {
				alias = c.Spelling
				break
			}
		}
		winner := chooseAliasOwner(alias, claims, out.Terms)
		if winner == "" {
			continue
		}
		for _, c := range claims {
			if c.TermID == winner {
				continue
			}
			idx := byID[c.TermID]
			newAliases, dropped := removeAliasNorm(out.Terms[idx].Aliases, key)
			for _, d := range dropped {
				drops = append(drops, AliasDrop{TermID: c.TermID, Alias: d})
			}
			out.Terms[idx].Aliases = newAliases
		}
	}

	for i, term := range out.Terms {
		id := strings.TrimSpace(term.ID)
		key := normID(id)
		if key == "" {
			continue
		}
		for j, other := range out.Terms {
			if i == j {
				continue
			}
			oid := strings.TrimSpace(other.ID)
			for _, a := range other.Aliases {
				if normID(a) != key {
					continue
				}
				newAliases, dropped := removeAliasNorm(out.Terms[j].Aliases, key)
				for _, d := range dropped {
					drops = append(drops, AliasDrop{TermID: oid, Alias: d})
				}
				out.Terms[j].Aliases = newAliases
			}
		}
	}

	return out, drops, nil
}

func repairAncestors(parentOf map[string]string, id string) ([]string, error) {
	seen := map[string]struct{}{}
	var rev []string
	cur := id
	for cur != "" {
		if _, loop := seen[cur]; loop {
			return nil, fmt.Errorf("cycle involving %q", id)
		}
		seen[cur] = struct{}{}
		rev = append(rev, cur)
		cur = parentOf[cur]
	}
	out := make([]string, len(rev))
	for i := range rev {
		out[i] = rev[len(rev)-1-i]
	}
	return out, nil
}

func uniqueAliasesNorm(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, a := range in {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		key := normID(a)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, a)
	}
	return out
}

func removeAliasNorm(aliases []string, key string) ([]string, []string) {
	var out []string
	var dropped []string
	for _, a := range aliases {
		if normID(a) == key {
			dropped = append(dropped, a)
			continue
		}
		out = append(out, a)
	}
	return out, dropped
}

func chooseAliasOwner(alias string, claims []aliasClaim, terms []Term) string {
	if len(claims) == 0 {
		return ""
	}
	key := normID(alias)
	for _, term := range terms {
		id := strings.TrimSpace(term.ID)
		if normID(id) == key {
			return id
		}
	}
	winner := claims[0]
	for i := 1; i < len(claims); i++ {
		if aliasClaimBeats(alias, claims[i], winner) {
			winner = claims[i]
		}
	}
	return winner.TermID
}

func aliasClaimBeats(alias string, a, b aliasClaim) bool {
	S := kindSegmentsForAlias(alias)
	aExact, bExact := aliasExactPath(alias, a.Path), aliasExactPath(alias, b.Path)
	if aExact != bExact {
		return aExact
	}
	aLast, bLast := aliasLastSegMatch(S, a.Path), aliasLastSegMatch(S, b.Path)
	if aLast != bLast {
		return aLast
	}
	aCov, bCov := aliasSegmentCoverage(S, a.Path), aliasSegmentCoverage(S, b.Path)
	if aCov != bCov {
		return aCov > bCov
	}
	if len(a.Path) != len(b.Path) {
		return len(a.Path) > len(b.Path)
	}
	return a.YAMLIndex < b.YAMLIndex
}

func kindSegmentsForAlias(alias string) []string {
	S := ParseKindSegments(alias)
	if len(S) == 0 {
		a := strings.TrimSpace(alias)
		if a == "" {
			return nil
		}
		return []string{a}
	}
	return S
}

func aliasExactPath(alias string, path []string) bool {
	if len(path) == 0 {
		return false
	}
	return normID(strings.Join(path, " > ")) == normID(alias)
}

func aliasLastSegMatch(S []string, path []string) bool {
	if len(S) == 0 || len(path) == 0 {
		return false
	}
	return normID(path[len(path)-1]) == normID(S[len(S)-1])
}

func aliasSegmentCoverage(S []string, path []string) int {
	if len(S) == 0 || len(path) == 0 {
		return 0
	}
	pathSet := map[string]struct{}{}
	for _, p := range path {
		pathSet[normID(p)] = struct{}{}
	}
	n := 0
	for _, s := range S {
		if _, ok := pathSet[normID(s)]; ok {
			n++
		}
	}
	return n
}
