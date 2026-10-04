package catalog

import (
	"sort"
	"strings"
)

// ResolveAssignments maps free-form tags onto leaf terms for one vocabulary.
func (c *Catalog) ResolveAssignments(sources []string) Projection {
	proj := Projection{Vocab: ""}
	if c != nil {
		proj.Vocab = c.Vocab.ID
	}
	if c == nil {
		for _, s := range sources {
			s = strings.TrimSpace(s)
			if s != "" {
				proj.Unknown = append(proj.Unknown, s)
			}
		}
		return proj
	}

	assigned := map[string]Assignment{}
	ancestorSet := map[string]struct{}{}

	for _, src := range sources {
		src = strings.TrimSpace(src)
		if src == "" {
			continue
		}
		rt, ok := c.PreferLeaf(src)
		if !ok {
			proj.Unknown = append(proj.Unknown, src)
			continue
		}
		if !rt.Leaf {
			proj.NonLeaf = append(proj.NonLeaf, src)
			continue
		}
		assigned[rt.ID] = Assignment{
			Vocab:  c.Vocab.ID,
			TermID: rt.ID,
			Label:  rt.Label,
			Source: src,
		}
		for _, a := range c.AncestorIDs(rt.ID) {
			ancestorSet[a] = struct{}{}
		}
	}

	ids := make([]string, 0, len(assigned))
	for id := range assigned {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		proj.Assigned = append(proj.Assigned, assigned[id])
		proj.Descendant = append(proj.Descendant, id)
	}
	for a := range ancestorSet {
		proj.Ancestors = append(proj.Ancestors, a)
	}
	sort.Strings(proj.Ancestors)
	proj.Unknown = uniqueNonEmpty(proj.Unknown)
	proj.NonLeaf = uniqueNonEmpty(proj.NonLeaf)
	return proj
}

// ExpandFilterRefs turns selected filter refs into matching leaf ids.
func (c *Catalog) ExpandFilterRefs(refs []string) (leaves []string, stale []string) {
	if c == nil {
		return nil, append([]string{}, refs...)
	}
	seen := map[string]struct{}{}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if i := strings.IndexByte(ref, ':'); i > 0 {
			prefix := ref[:i]
			rest := ref[i+1:]
			if prefix == c.Vocab.ID || prefix == "pattern" || prefix == "concept" {
				ref = rest
			}
		}
		if _, ok := c.Lookup(ref); !ok {
			stale = append(stale, ref)
			continue
		}
		for _, leaf := range c.DescendantLeaves(ref) {
			if _, ok := seen[leaf]; ok {
				continue
			}
			seen[leaf] = struct{}{}
			leaves = append(leaves, leaf)
		}
	}
	sort.Strings(leaves)
	stale = uniqueNonEmpty(stale)
	return leaves, stale
}

// MatchAny reports whether assigned leaf ids intersect the expanded filter leaves.
func MatchAny(assignedLeafIDs, filterLeafIDs []string) bool {
	if len(filterLeafIDs) == 0 {
		return true
	}
	want := map[string]struct{}{}
	for _, id := range filterLeafIDs {
		want[id] = struct{}{}
	}
	for _, id := range assignedLeafIDs {
		if _, ok := want[id]; ok {
			return true
		}
	}
	return false
}

// TreeNode is a JSON-friendly hierarchy node.
type TreeNode struct {
	ID       string     `json:"id"`
	Label    string     `json:"label"`
	Depth    int        `json:"depth"`
	Leaf     bool       `json:"leaf"`
	Status   string     `json:"status,omitempty"`
	Count    int        `json:"count,omitempty"`
	Children []TreeNode `json:"children,omitempty"`
}

// Tree builds a nested tree from catalog roots.
func (c *Catalog) Tree(counts map[string]int) []TreeNode {
	if c == nil {
		return nil
	}
	var build func(id string) TreeNode
	build = func(id string) TreeNode {
		rt := c.ByID[id]
		n := TreeNode{
			ID:     rt.ID,
			Label:  rt.Label,
			Depth:  rt.Depth,
			Leaf:   rt.Leaf,
			Status: rt.Status,
		}
		if rt.Leaf {
			n.Count = counts[rt.ID]
		}
		for _, child := range rt.Children {
			cn := build(child)
			n.Children = append(n.Children, cn)
			n.Count += cn.Count
		}
		return n
	}
	out := make([]TreeNode, 0, len(c.Roots))
	for _, root := range c.Roots {
		out = append(out, build(root))
	}
	return out
}
