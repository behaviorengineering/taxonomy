package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// BuildCatalog validates terms and derives depth, leaf, path, and alias maps.
func BuildCatalog(vocab Vocabulary) (*Catalog, error) {
	vocab.ID = strings.TrimSpace(vocab.ID)
	vocab.Label = strings.TrimSpace(vocab.Label)
	if vocab.ID == "" {
		return nil, newErr("Build", CodeInvalidVocab, "vocabulary id required", nil)
	}
	if vocab.Label == "" {
		vocab.Label = vocab.ID
	}

	byID := map[string]*ResolvedTerm{}
	aliasToID := map[string]string{}
	termOrder := make([]string, 0, len(vocab.Terms))
	compiledPatterns := map[string][]compiledFieldPattern{}

	for i, term := range vocab.Terms {
		id := strings.TrimSpace(term.ID)
		if id == "" {
			return nil, newErr("Build", CodeInvalidVocab, fmt.Sprintf("term %d: id required", i+1), nil)
		}
		if _, dup := byID[id]; dup {
			return nil, newErr("Build", CodeDuplicateID, fmt.Sprintf("duplicate term id %q", id), nil)
		}
		label := strings.TrimSpace(term.Label)
		if label == "" {
			return nil, newErr("Build", CodeInvalidVocab, fmt.Sprintf("term %s: label required", id), nil)
		}
		status := strings.TrimSpace(term.Status)
		if status == "" {
			status = StatusActive
		}
		if status != StatusActive && status != StatusDeprecated {
			return nil, newErr("Build", CodeInvalidVocab, fmt.Sprintf("term %s: status must be %s or %s", id, StatusActive, StatusDeprecated), nil)
		}
		patterns, compiled, err := compileTermPatterns(id, term.Patterns)
		if err != nil {
			return nil, err
		}
		rt := &ResolvedTerm{
			Term: Term{
				ID:           id,
				Label:        label,
				Parent:       strings.TrimSpace(term.Parent),
				Aliases:      uniqueNonEmpty(term.Aliases),
				Description:  strings.TrimSpace(term.Description),
				Status:       status,
				DeprecatedBy: strings.TrimSpace(term.DeprecatedBy),
				Patterns:     patterns,
				MapsTo:       strings.TrimSpace(term.MapsTo),
			},
		}
		byID[id] = rt
		termOrder = append(termOrder, id)
		if len(compiled) > 0 {
			compiledPatterns[id] = compiled
		}

		key := normID(id)
		if other, ok := aliasToID[key]; ok && other != id {
			return nil, newErr("Build", CodeAliasCollision, fmt.Sprintf("id %q collides with alias of %s", id, other), nil)
		}
		aliasToID[key] = id
		for _, a := range rt.Aliases {
			akey := normID(a)
			if akey == "" {
				continue
			}
			if other, ok := aliasToID[akey]; ok && other != id {
				return nil, newErr("Build", CodeAliasCollision, fmt.Sprintf("alias %q used by %s and %s", a, other, id), nil)
			}
			aliasToID[akey] = id
		}
	}

	for id, rt := range byID {
		if rt.Parent == "" {
			continue
		}
		if _, ok := byID[rt.Parent]; !ok {
			return nil, newErr("Build", CodeMissingParent, fmt.Sprintf("term %s: missing parent %q", id, rt.Parent), nil)
		}
		if rt.Parent == id {
			return nil, newErr("Build", CodeInvalidVocab, fmt.Sprintf("term %s: parent cannot be self", id), nil)
		}
	}

	for id, rt := range byID {
		if rt.DeprecatedBy == "" {
			continue
		}
		if _, ok := byID[rt.DeprecatedBy]; !ok {
			return nil, newErr("Build", CodeInvalidVocab, fmt.Sprintf("term %s: deprecated_by missing %q", id, rt.DeprecatedBy), nil)
		}
	}

	for id := range byID {
		path, err := ancestorsIncludingSelf(byID, id)
		if err != nil {
			return nil, newErr("Build", CodeCycle, fmt.Sprintf("term %s: %v", id, err), err)
		}
		rt := byID[id]
		rt.Path = path
		rt.Depth = len(path) - 1
	}

	for id, rt := range byID {
		if rt.Parent == "" {
			continue
		}
		parent := byID[rt.Parent]
		parent.Children = append(parent.Children, id)
	}
	for _, rt := range byID {
		sort.Strings(rt.Children)
		rt.Leaf = len(rt.Children) == 0
	}

	roots := make([]string, 0)
	for id, rt := range byID {
		if rt.Parent == "" {
			roots = append(roots, id)
		}
	}
	sort.Strings(roots)

	return &Catalog{
		Vocab:            vocab,
		ByID:             byID,
		AliasToID:        aliasToID,
		Roots:            roots,
		termOrder:        termOrder,
		compiledPatterns: compiledPatterns,
	}, nil
}

func ancestorsIncludingSelf(byID map[string]*ResolvedTerm, id string) ([]string, error) {
	seen := map[string]struct{}{}
	var rev []string
	cur := id
	for cur != "" {
		if _, loop := seen[cur]; loop {
			return nil, fmt.Errorf("cycle involving %q", id)
		}
		seen[cur] = struct{}{}
		rt, ok := byID[cur]
		if !ok {
			return nil, fmt.Errorf("missing term %q", cur)
		}
		rev = append(rev, cur)
		cur = rt.Parent
	}
	out := make([]string, len(rev))
	for i := range rev {
		out[i] = rev[len(rev)-1-i]
	}
	return out, nil
}

// Lookup resolves an id or alias to the canonical term.
func (c *Catalog) Lookup(ref string) (*ResolvedTerm, bool) {
	if c == nil {
		return nil, false
	}
	id, ok := c.AliasToID[normID(ref)]
	if !ok {
		return nil, false
	}
	rt, ok := c.ByID[id]
	return rt, ok
}

// PreferLeaf follows deprecated_by when present so assignments land on the preferred leaf.
func (c *Catalog) PreferLeaf(ref string) (*ResolvedTerm, bool) {
	rt, ok := c.Lookup(ref)
	if !ok {
		return nil, false
	}
	seen := map[string]struct{}{}
	for rt.DeprecatedBy != "" {
		if _, loop := seen[rt.ID]; loop {
			break
		}
		seen[rt.ID] = struct{}{}
		next, ok := c.ByID[rt.DeprecatedBy]
		if !ok {
			break
		}
		rt = next
	}
	return rt, true
}

// IsLeaf reports whether the resolved term has no children.
func (c *Catalog) IsLeaf(ref string) bool {
	rt, ok := c.Lookup(ref)
	return ok && rt.Leaf
}

// AncestorIDs returns parent chain excluding self (root-first).
func (c *Catalog) AncestorIDs(ref string) []string {
	rt, ok := c.Lookup(ref)
	if !ok || len(rt.Path) < 2 {
		return nil
	}
	return append([]string{}, rt.Path[:len(rt.Path)-1]...)
}

// DescendantLeaves returns all leaf ids under ref (ref itself when already a leaf).
func (c *Catalog) DescendantLeaves(ref string) []string {
	rt, ok := c.Lookup(ref)
	if !ok {
		return nil
	}
	if rt.Leaf {
		return []string{rt.ID}
	}
	var out []string
	var walk func(string)
	walk = func(id string) {
		node := c.ByID[id]
		if node == nil {
			return
		}
		if node.Leaf {
			out = append(out, id)
			return
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(rt.ID)
	sort.Strings(out)
	return out
}

// TermsSorted returns all terms ordered by path then id.
func (c *Catalog) TermsSorted() []*ResolvedTerm {
	if c == nil {
		return nil
	}
	out := make([]*ResolvedTerm, 0, len(c.ByID))
	for _, rt := range c.ByID {
		out = append(out, rt)
	}
	sort.Slice(out, func(i, j int) bool {
		pi := strings.Join(out[i].Path, "/")
		pj := strings.Join(out[j].Path, "/")
		if pi != pj {
			return pi < pj
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// LeafTerms returns active leaf terms sorted by id.
func (c *Catalog) LeafTerms() []*ResolvedTerm {
	if c == nil {
		return nil
	}
	var out []*ResolvedTerm
	for _, rt := range c.ByID {
		if !rt.Leaf || rt.Status == StatusDeprecated {
			continue
		}
		out = append(out, rt)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
