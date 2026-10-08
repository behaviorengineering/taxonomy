package catalog

import (
	"fmt"
	"regexp"
	"strings"
)

var kebabRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Breadcrumb returns term ids from root to ref joined with " > ".
func Breadcrumb(c *Catalog, ref string) string {
	if c == nil {
		return ""
	}
	rt, ok := c.Lookup(ref)
	if !ok || len(rt.Path) == 0 {
		return ""
	}
	return strings.Join(rt.Path, " > ")
}

// AssignableLeafIDs returns assignable leaf ids in vocabulary term order.
func (c *Catalog) AssignableLeafIDs() []string {
	if c == nil {
		return nil
	}
	var out []string
	for _, id := range c.termOrder {
		rt := c.ByID[id]
		if rt == nil || rt.Status == StatusDeprecated {
			continue
		}
		if rt.Leaf {
			out = append(out, id)
		}
	}
	return out
}

// EnsurePathInput builds missing terms along a KIND breadcrumb.
type EnsurePathInput struct {
	Segments    []string
	UnderParent string
	About       string
	Shape       string
}

// EnsurePath appends terms to vocab for each segment; returns updated vocab and final leaf id.
func EnsurePath(vocab Vocabulary, in EnsurePathInput) (Vocabulary, string, error) {
	const op = "catalog.EnsurePath"
	if len(in.Segments) == 0 {
		return vocab, "", newErr(op, CodeInvalidVocab, "segments required", nil)
	}
	about := strings.TrimSpace(in.About)
	if about == "" {
		return vocab, "", newErr(op, CodeInvalidVocab, "about required", nil)
	}
	out := vocab
	out.Terms = append([]Term{}, vocab.Terms...)
	byID := map[string]Term{}
	for _, t := range out.Terms {
		byID[strings.TrimSpace(t.ID)] = t
	}
	parent := strings.TrimSpace(in.UnderParent)
	var lastID string
	for i, seg := range in.Segments {
		seg = strings.TrimSpace(seg)
		if !kebabRE.MatchString(seg) {
			return vocab, "", newErr(op, CodeInvalidVocab, fmt.Sprintf("segment %q not kebab-case", seg), nil)
		}
		id := seg
		if existing, ok := byID[id]; ok {
			expParent := parent
			if i == 0 && parent == "" {
				expParent = ""
			}
			if strings.TrimSpace(existing.Parent) != expParent {
				id = mintSegmentID(parent, seg, byID)
			}
		}
		if _, ok := byID[id]; !ok {
			label := titleFromKebab(seg)
			desc := about
			if i < len(in.Segments)-1 {
				desc = strings.TrimSpace(in.Shape)
				if desc == "" {
					desc = about
				}
			}
			term := Term{
				ID:          id,
				Label:       label,
				Parent:      parent,
				Description: desc,
				Status:      StatusActive,
			}
			out.Terms = append(out.Terms, term)
			byID[id] = term
		}
		lastID = id
		parent = id
	}
	return out, lastID, nil
}

func mintSegmentID(parent, seg string, byID map[string]Term) string {
	base := seg
	if parent != "" {
		base = parent + "-" + seg
	}
	if _, ok := byID[base]; !ok {
		return base
	}
	for n := 2; n < 1000; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if _, ok := byID[candidate]; !ok {
			return candidate
		}
	}
	return base + "-x"
}

func titleFromKebab(id string) string {
	parts := strings.Split(id, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// ParseKindSegments splits KIND on ">".
func ParseKindSegments(kind string) []string {
	var out []string
	for _, p := range strings.Split(kind, ">") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
