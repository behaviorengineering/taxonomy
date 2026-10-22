package catalog

import (
	"fmt"
	"strings"
)

// DeriveSegment infers the semantic KIND segment from parent-prefixed storage ids.
func DeriveSegment(parentID, id string) string {
	parentID = strings.TrimSpace(parentID)
	id = strings.TrimSpace(id)
	if parentID == "" {
		return id
	}
	prefix := parentID + "-"
	if strings.HasPrefix(id, prefix) {
		return id[len(prefix):]
	}
	return id
}

// ResolveChild finds an active child of parentID whose Segment matches seg.
func ResolveChild(cat *Catalog, parentID, seg string) (string, bool) {
	if cat == nil {
		return "", false
	}
	id, err := ResolveChildTerms(cat.Vocab.Terms, parentID, seg)
	if err != nil || id == "" {
		return "", false
	}
	return id, true
}

// ResolveChildTerms finds a child under parentID in terms by semantic Segment.
func ResolveChildTerms(terms []Term, parentID, seg string) (string, error) {
	const op = "ResolveChildTerms"
	parentID = strings.TrimSpace(parentID)
	seg = strings.TrimSpace(seg)
	want := normID(seg)
	if want == "" {
		return "", newErr(op, CodeInvalidVocab, "segment required", nil)
	}
	var found string
	for _, t := range terms {
		if strings.TrimSpace(t.Parent) != parentID {
			continue
		}
		if strings.TrimSpace(t.Status) == StatusDeprecated {
			continue
		}
		childSeg := strings.TrimSpace(t.Segment)
		if childSeg == "" {
			childSeg = DeriveSegment(parentID, t.ID)
		}
		if normID(childSeg) != want {
			continue
		}
		id := strings.TrimSpace(t.ID)
		if found != "" && found != id {
			return "", newErr(op, CodeDuplicateSegment, fmt.Sprintf("segment %q used by %s and %s", seg, found, id), nil)
		}
		found = id
	}
	return found, nil
}

func segmentForResolved(rt *ResolvedTerm) string {
	if rt == nil {
		return ""
	}
	seg := strings.TrimSpace(rt.Segment)
	if seg != "" {
		return seg
	}
	return DeriveSegment(rt.Parent, rt.ID)
}

func segmentsAlong(byID map[string]*ResolvedTerm, path []string) []string {
	out := make([]string, len(path))
	for i, id := range path {
		rt := byID[id]
		if rt == nil {
			out[i] = id
			continue
		}
		out[i] = segmentForResolved(rt)
	}
	return out
}
