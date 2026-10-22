package harness

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
)

var essenceKebabRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func (h *Harness) attach(ctx context.Context, op Op) (Result, error) {
	ess, err := h.resolveEssence(ctx, op)
	if err != nil {
		return Result{}, err
	}
	if err := validateEssence(ess); err != nil {
		return Result{}, err
	}
	segments := catalog.ParseKindSegments(ess.Kind)
	res := Result{
		Strategy: StrategyAttach,
		Kind:     ess.Kind,
		About:    ess.About,
		Shape:    ess.Shape,
		Essence:  &ess,
	}
	cat := op.Catalog

	parentID, path, consumed := longestKindPrefix(cat, segments)
	if consumed == len(segments) && consumed > 0 {
		rt := cat.ByID[parentID]
		if rt != nil && rt.Leaf {
			return aliasAttachResult(res, cat, ess, parentID, 0, false)
		}
		if rt != nil && !rt.Leaf {
			return h.attachCosineUnder(ctx, op, res, ess, cat, parentID, path)
		}
	}
	if consumed > 0 && consumed < len(segments) {
		remaining := segments[consumed:]
		essRem := ess
		essRem.Kind = strings.Join(remaining, " > ")
		return finishBreadcrumb(cat, res, essRem, parentID, path, 0, false)
	}

	leaves := cat.AssignableLeafIDs()
	if len(leaves) == 0 {
		return finishBreadcrumb(cat, res, ess, "", nil, 0, false)
	}
	pick, err := h.embedBestLeaf(ctx, ess.Kind, leaves, cat)
	if err != nil {
		return Result{}, err
	}
	bestLeaf, bestCos := pick.leafID, pick.cosine
	res.Cosine = bestCos
	res.CanonicalID = bestLeaf

	if bestCos >= h.attachMinCosine {
		return aliasAttachResult(res, cat, ess, bestLeaf, bestCos, false)
	}
	if bestCos >= h.walkReinforceMin {
		startParent, path, _ := longestKindPrefix(cat, segments)
		if len(activeChildIDs(cat, startParent)) == 0 {
			return finishBreadcrumb(cat, res, ess, startParent, path, bestCos, true)
		}
		walkRes, walkErr := h.walkFrom(ctx, cat, walkConfig{
			judgeText: formatEssence(ess),
			worldBase: op.WorldContext,
			parentID:  startParent,
			path:      path,
			reinforce: true,
		})
		if walkErr == nil {
			walkRes.Strategy = StrategyAttach
			walkRes.Kind = ess.Kind
			walkRes.About = ess.About
			walkRes.Shape = ess.Shape
			walkRes.Essence = &ess
			walkRes.Cosine = bestCos
			walkRes.Reinforced = true
			if len(walkRes.Assigned) > 0 {
				leafID := walkRes.Assigned[0].TermID
				walkRes.CanonicalID = leafID
				draft := DraftOut{Kind: DraftKindAlias, LeafID: leafID, Alias: ess.Kind}
				walkRes.Draft = &draft
				walkRes.DraftAccepted = true
			}
			return walkRes, nil
		}
		var stop reinforceWalkStop
		if errors.As(walkErr, &stop) {
			remaining := segmentsRemainingAfterPath(cat, segments, stop.path)
			essRem := ess
			if len(remaining) == 0 {
				remaining = []string{fallbackSegment(ess)}
			}
			essRem.Kind = strings.Join(remaining, " > ")
			return finishBreadcrumb(cat, res, essRem, stop.parentID, stop.path, bestCos, true)
		}
		return Result{}, walkErr
	}
	return finishBreadcrumb(cat, res, ess, "", nil, bestCos, false)
}

func checkEmbedVectors(vecs [][]float64, want int) error {
	if len(vecs) != want {
		return newErr("Operate", CodeEmbed, fmt.Sprintf("expected %d vectors got %d", want, len(vecs)), nil)
	}
	dim := 0
	for i, v := range vecs {
		if len(v) == 0 {
			return newErr("Operate", CodeEmbed, fmt.Sprintf("empty vector at %d", i), nil)
		}
		if dim == 0 {
			dim = len(v)
		} else if len(v) != dim {
			return newErr("Operate", CodeEmbed, "embedding dimension mismatch", nil)
		}
		for j, x := range v {
			if math.IsNaN(x) || math.IsInf(x, 0) {
				return newErr("Operate", CodeEmbed, fmt.Sprintf("non-finite value at vector %d index %d", i, j), nil)
			}
		}
	}
	return nil
}

// segmentsRemainingAfterPath returns KIND segments not yet covered by termPath ids.
func segmentsRemainingAfterPath(cat *catalog.Catalog, kindSegments []string, termPath []string) []string {
	parent := ""
	consumed := 0
	for i := 0; i < len(kindSegments) && i < len(termPath); i++ {
		child := findChildSegment(cat, parent, kindSegments[i])
		if child == "" || child != termPath[i] {
			break
		}
		parent = child
		consumed++
	}
	if consumed >= len(kindSegments) {
		return nil
	}
	return kindSegments[consumed:]
}

func longestKindPrefix(cat *catalog.Catalog, segments []string) (parentID string, path []string, consumed int) {
	parentID = ""
	path = nil
	for _, seg := range segments {
		child := findChildSegment(cat, parentID, seg)
		if child == "" {
			break
		}
		path = append(path, child)
		parentID = child
		consumed++
	}
	return parentID, path, consumed
}

func findChildSegment(cat *catalog.Catalog, parentID, seg string) string {
	id, ok := catalog.ResolveChild(cat, parentID, seg)
	if ok {
		return id
	}
	return ""
}

func assignableLeavesUnder(cat *catalog.Catalog, ancestorID string) []string {
	var out []string
	for _, id := range cat.AssignableLeafIDs() {
		rt := cat.ByID[id]
		if rt == nil {
			continue
		}
		for _, p := range rt.Path {
			if p == ancestorID {
				out = append(out, id)
				break
			}
		}
	}
	return out
}

func (h *Harness) attachCosineUnder(ctx context.Context, _ Op, res Result, ess EssenceOut, cat *catalog.Catalog, ancestorID string, path []string) (Result, error) {
	leaves := assignableLeavesUnder(cat, ancestorID)
	if len(leaves) == 0 {
		return Result{}, newErr("Operate", CodeInvalidDraft, "kind ended on non-leaf", nil)
	}
	pick, err := h.embedBestLeaf(ctx, ess.Kind, leaves, cat)
	if err != nil {
		return Result{}, err
	}
	bestLeaf, bestCos := pick.leafID, pick.cosine
	res.Cosine = bestCos
	res.Path = append([]string{}, path...)
	if bestCos >= h.attachMinCosine {
		return aliasAttachResult(res, cat, ess, bestLeaf, bestCos, false)
	}
	return finishBreadcrumb(cat, res, ess, ancestorID, path, bestCos, false)
}

type leafCosinePick struct {
	leafID string
	cosine float64
}

func (h *Harness) embedBestLeaf(ctx context.Context, kind string, leaves []string, cat *catalog.Catalog) (leafCosinePick, error) {
	texts := make([]string, 1+len(leaves))
	texts[0] = kind
	for i, id := range leaves {
		texts[i+1] = catalog.Breadcrumb(cat, id)
	}
	vecs, err := h.embedder.Embed(ctx, texts)
	if err != nil {
		return leafCosinePick{}, newErr("Operate", CodeEmbed, "embed", err)
	}
	if err := checkEmbedVectors(vecs, len(texts)); err != nil {
		return leafCosinePick{}, err
	}
	query := vecs[0]
	bestLeaf := leaves[0]
	bestCos := h.cosine(query, vecs[1])
	for i := 1; i < len(leaves); i++ {
		c := h.cosine(query, vecs[i+1])
		if c > bestCos {
			bestCos = c
			bestLeaf = leaves[i]
		}
	}
	return leafCosinePick{leafID: bestLeaf, cosine: bestCos}, nil
}

func aliasAttachResult(res Result, cat *catalog.Catalog, ess EssenceOut, leafID string, cos float64, reinforced bool) (Result, error) {
	rt, ok := cat.PreferLeaf(leafID)
	if !ok {
		return Result{}, newErr("Operate", CodeInvalidDraft, "leaf missing", nil)
	}
	res.Cosine = cos
	res.CanonicalID = rt.ID
	res.Reinforced = reinforced
	res.Path = append([]string{}, rt.Path...)
	draft := DraftOut{Kind: DraftKindAlias, LeafID: rt.ID, Alias: ess.Kind}
	res.Draft = &draft
	res.DraftAccepted = true
	res.Assigned = []catalog.Assignment{{
		Vocab:  cat.Vocab.ID,
		TermID: rt.ID,
		Label:  rt.Label,
		Source: ess.Kind,
	}}
	return res, nil
}

func finishBreadcrumb(cat *catalog.Catalog, res Result, ess EssenceOut, parentID string, path []string, cos float64, reinforced bool) (Result, error) {
	segments := catalog.ParseKindSegments(ess.Kind)
	if len(segments) == 0 {
		return Result{}, newErr("Operate", CodeInvalidDraft, "breadcrumb segments required", nil)
	}
	_, leafID, err := catalog.EnsurePath(cat.Vocab, catalog.EnsurePathInput{
		Segments:    segments,
		UnderParent: parentID,
		About:       ess.About,
		Shape:       ess.Shape,
	})
	if err != nil {
		return Result{}, newErr("Operate", CodeInvalidDraft, "breadcrumb", err)
	}
	res.Cosine = cos
	res.Reinforced = reinforced
	res.Path = append([]string{}, path...)
	res.CanonicalID = leafID
	draft := DraftOut{
		Kind:        DraftKindBreadcrumb,
		Alias:       ess.Kind,
		Description: ess.About,
		Parent:      parentID,
	}
	res.Draft = &draft
	res.DraftAccepted = true
	res.Assigned = []catalog.Assignment{{
		Vocab:  cat.Vocab.ID,
		TermID: leafID,
		Label:  segments[len(segments)-1],
	}}
	return res, nil
}

func fallbackSegment(ess EssenceOut) string {
	segs := catalog.ParseKindSegments(ess.Kind)
	if len(segs) > 0 {
		return segs[len(segs)-1]
	}
	return "message"
}

func validateEssence(e EssenceOut) error {
	for i, w := range e.Why {
		if strings.TrimSpace(w) == "" {
			return newErr("Operate", CodeEssence, fmt.Sprintf("why%d required", i+1), nil)
		}
	}
	if strings.TrimSpace(e.About) == "" || strings.TrimSpace(e.Shape) == "" {
		return newErr("Operate", CodeEssence, "about and shape required", nil)
	}
	segs := catalog.ParseKindSegments(e.Kind)
	if len(segs) < 3 {
		return newErr("Operate", CodeEssence, "kind requires at least 3 segments", nil)
	}
	for _, seg := range segs {
		if !essenceKebabRE.MatchString(seg) {
			return newErr("Operate", CodeEssence, fmt.Sprintf("kind segment %q not kebab-case", seg), nil)
		}
	}
	return nil
}

func formatEssence(e EssenceOut) string {
	var b strings.Builder
	for i, w := range e.Why {
		fmt.Fprintf(&b, "WHY%d: %s\n", i+1, w)
	}
	b.WriteString("ABOUT: " + e.About + "\n")
	b.WriteString("SHAPE: " + e.Shape + "\n")
	b.WriteString("KIND: " + e.Kind + "\n")
	return b.String()
}

func (h *Harness) resolveEssence(ctx context.Context, op Op) (EssenceOut, error) {
	if op.Essence != nil {
		return *op.Essence, nil
	}
	key := strings.TrimSpace(op.EssenceKey)
	if key != "" && h.essenceCache != nil {
		cached, hit, getErr := h.essenceCache.Get(ctx, key)
		if getErr != nil {
			return EssenceOut{}, newErr("Operate", CodeEssence, "cache get", getErr)
		}
		if hit {
			return cached, nil
		}
	}
	if h.essencer == nil {
		return EssenceOut{}, newErr("Operate", CodeEssence, "essencer required", nil)
	}
	got, err := h.essencer.Essence(ctx, EssenceIn{WorldContext: op.WorldContext, Text: op.Text})
	if err != nil {
		return EssenceOut{}, newErr("Operate", CodeEssence, "essence", err)
	}
	if key != "" && h.essenceCache != nil {
		if putErr := h.essenceCache.Put(ctx, key, got); putErr != nil {
			return EssenceOut{}, newErr("Operate", CodeEssence, "cache put", putErr)
		}
	}
	return got, nil
}

func preflightAttachCatalog(cat *catalog.Catalog) error {
	if len(cat.ByID) == 0 {
		return nil
	}
	return preflightCatalog(cat)
}
