package harness

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
)

// hopCapForTest overrides the Operate hop cap when non-zero (tests only).
var hopCapForTest int

// Config wires mandatory seats for Operate.
type Config struct {
	Judge         Judge
	Author        Author
	MinJudgeScore float64
}

// Harness runs in-world taxonomy classification.
type Harness struct {
	judge         Judge
	author        Author
	minJudgeScore float64
}

// CreateHarness returns a harness with required Judge and Author seats.
func CreateHarness(cfg Config) (*Harness, error) {
	if cfg.Judge == nil || cfg.Author == nil {
		return nil, newErr("CreateHarness", CodeConfig, "Judge and Author are required", nil)
	}
	min := cfg.MinJudgeScore
	if min == 0 {
		min = 0.5
	}
	if math.IsNaN(min) || math.IsInf(min, 0) {
		return nil, newErr("CreateHarness", CodeConfig, "MinJudgeScore must be finite", nil)
	}
	return &Harness{judge: cfg.Judge, author: cfg.Author, minJudgeScore: min}, nil
}

// Operate classifies text by walking the catalog tree (children per hop).
func (h *Harness) Operate(ctx context.Context, op Op) (Result, error) {
	if h == nil {
		return Result{}, newErr("Operate", CodeConfig, "harness is nil", nil)
	}
	if err := requireDeadline(ctx); err != nil {
		return Result{}, err
	}
	world := strings.TrimSpace(op.WorldContext)
	if world == "" {
		return Result{}, newErr("Operate", CodeWorldContext, "WorldContext required", nil)
	}
	text := strings.TrimSpace(op.Text)
	if text == "" {
		return Result{}, newErr("Operate", CodeEmptyText, "Text required", nil)
	}
	if op.Catalog == nil {
		return Result{}, newErr("Operate", CodeNilCatalog, "Catalog required", nil)
	}
	if err := preflightCatalog(op.Catalog); err != nil {
		return Result{}, err
	}

	cap := len(op.Catalog.ByID) + 1
	if hopCapForTest > 0 {
		cap = hopCapForTest
	}
	parentID := ""
	var path []string
	hops := 0
	lastScore := 0.0

	for {
		hops++
		if hops > cap {
			return Result{}, newErr("Operate", CodeHopLimit, "hop cap exceeded", nil)
		}
		if err := ctx.Err(); err != nil {
			return Result{}, newErr("Operate", CodeConfig, "context", err)
		}

		options, choiceSet, err := packChildren(op.Catalog, parentID)
		if err != nil {
			return Result{}, err
		}
		if len(options) == 1 && options[0].Choice == ChoiceSkip {
			return Result{}, newErr("Operate", CodeConfig, "no packable children", nil)
		}

		decideOut, err := h.judge.Decide(ctx, DecideIn{WorldContext: world, Text: text, Options: options})
		if err != nil {
			return Result{}, newErr("Operate", CodeJudge, "judge", err)
		}
		if math.IsNaN(decideOut.Score) || math.IsInf(decideOut.Score, 0) {
			return Result{}, newErr("Operate", CodeInvalidScore, "judge score must be finite", nil)
		}
		if _, ok := choiceSet[decideOut.Choice]; !ok {
			return Result{}, newErr("Operate", CodeUnknownChoice, fmt.Sprintf("choice %q not in packed set", decideOut.Choice), nil)
		}
		lastScore = decideOut.Score

		if strings.HasPrefix(decideOut.Choice, ChoicePrefixUse) && decideOut.Score >= h.minJudgeScore {
			id := strings.TrimPrefix(decideOut.Choice, ChoicePrefixUse)
			rt, ok := op.Catalog.Lookup(id)
			if !ok {
				return Result{}, newErr("Operate", CodeUnknownChoice, fmt.Sprintf("term %q missing from catalog", id), nil)
			}
			path = append(path, rt.ID)
			if isAssignableLeaf(op.Catalog, rt) {
				assignRT, ok := op.Catalog.PreferLeaf(rt.ID)
				if !ok || !isAssignableLeaf(op.Catalog, assignRT) {
					return Result{}, newErr("Operate", CodeUnknownChoice, fmt.Sprintf("leaf %q not assignable", id), nil)
				}
				return Result{
					Assigned: []catalog.Assignment{{
						Vocab:  op.Catalog.Vocab.ID,
						TermID: assignRT.ID,
						Label:  assignRT.Label,
					}},
					JudgeScore: lastScore,
					Path:       path,
				}, nil
			}
			parentID = rt.ID
			continue
		}

		reason := "skip"
		if decideOut.Choice != ChoiceSkip {
			reason = fmt.Sprintf("low_score:%.3f", decideOut.Score)
		}
		return h.authorAndGate(ctx, op.Catalog, world, text, parentID, reason, lastScore, path)
	}
}

func (h *Harness) authorAndGate(ctx context.Context, cat *catalog.Catalog, world, text, parentID, reason string, judgeScore float64, path []string) (Result, error) {
	parents := packAuthorParents(cat, parentID)
	draftIn := DraftIn{WorldContext: world, Text: text, Parents: parents, Reason: reason}
	draftOut, err := h.author.Draft(ctx, draftIn)
	if err != nil {
		return Result{}, newErr("Operate", CodeAuthor, "author", err)
	}
	if err := validateDraft(cat, draftOut); err != nil {
		return Result{}, err
	}
	res := Result{Draft: &draftOut, JudgeScore: judgeScore, Path: path}

	gateOptions := packGateOptions(draftOut)
	gateOut, err := h.judge.Decide(ctx, DecideIn{
		WorldContext: world,
		Text:         text,
		Options:      gateOptions,
	})
	if err != nil {
		return Result{}, newErr("Operate", CodeGate, "gate", err)
	}
	if math.IsNaN(gateOut.Score) || math.IsInf(gateOut.Score, 0) {
		return Result{}, newErr("Operate", CodeInvalidScore, "gate score must be finite", nil)
	}
	res.GateScore = gateOut.Score
	if gateOut.Choice != ChoiceAcceptDraft || gateOut.Score < h.minJudgeScore {
		return res, nil
	}
	res.DraftAccepted = true
	switch draftOut.Kind {
	case DraftKindNewLeaf:
		res.Assigned = []catalog.Assignment{{
			Vocab:  cat.Vocab.ID,
			TermID: strings.TrimSpace(draftOut.ID),
			Label:  strings.TrimSpace(draftOut.Label),
		}}
	case DraftKindAlias:
		rt, ok := cat.Lookup(draftOut.LeafID)
		if !ok || !isAssignableLeaf(cat, rt) {
			return Result{}, newErr("Operate", CodeInvalidDraft, "alias leaf missing", nil)
		}
		res.Assigned = []catalog.Assignment{{
			Vocab:  cat.Vocab.ID,
			TermID: rt.ID,
			Label:  rt.Label,
			Source: draftOut.Alias,
		}}
	}
	return res, nil
}

func preflightCatalog(cat *catalog.Catalog) error {
	if len(cat.Roots) == 0 {
		return newErr("Operate", CodeConfig, "catalog has no roots", nil)
	}
	for _, rt := range cat.ByID {
		if rt.Status == catalog.StatusDeprecated {
			continue
		}
		if strings.TrimSpace(rt.Description) == "" {
			return newErr("Operate", CodeEmptyDescription, fmt.Sprintf("term %q missing description", rt.ID), nil)
		}
	}
	return nil
}

func activeChildIDs(cat *catalog.Catalog, parentID string) []string {
	var ids []string
	if parentID == "" {
		for _, id := range cat.Roots {
			rt := cat.ByID[id]
			if rt == nil || rt.Status == catalog.StatusDeprecated {
				continue
			}
			ids = append(ids, id)
		}
		return ids
	}
	parent := cat.ByID[parentID]
	if parent == nil {
		return nil
	}
	for _, childID := range parent.Children {
		rt := cat.ByID[childID]
		if rt == nil || rt.Status == catalog.StatusDeprecated {
			continue
		}
		ids = append(ids, childID)
	}
	return ids
}

func isAssignableLeaf(cat *catalog.Catalog, rt *catalog.ResolvedTerm) bool {
	if rt == nil || rt.Status == catalog.StatusDeprecated {
		return false
	}
	return len(activeChildIDs(cat, rt.ID)) == 0
}

func packChildren(cat *catalog.Catalog, parentID string) ([]PackedOption, map[string]struct{}, error) {
	childIDs := activeChildIDs(cat, parentID)
	var out []PackedOption
	choiceSet := map[string]struct{}{}
	for _, id := range childIDs {
		rt := cat.ByID[id]
		if rt == nil {
			continue
		}
		desc := strings.TrimSpace(rt.Description)
		if desc == "" {
			return nil, nil, newErr("Operate", CodeEmptyDescription, fmt.Sprintf("term %q missing description", id), nil)
		}
		opt := PackedOption{
			Choice:      ChoicePrefixUse + rt.ID,
			Label:       rt.Label,
			Description: desc,
			ParentID:    parentID,
		}
		if isAssignableLeaf(cat, rt) {
			opt.LeafID = rt.ID
		}
		out = append(out, opt)
		choiceSet[opt.Choice] = struct{}{}
	}
	skip := PackedOption{
		Choice:      ChoiceSkip,
		Label:       "Skip",
		Description: "No existing leaf fits; propose a new term or alias.",
	}
	out = append(out, skip)
	choiceSet[ChoiceSkip] = struct{}{}
	return out, choiceSet, nil
}

func packAuthorParents(cat *catalog.Catalog, parentID string) []PackedOption {
	if parentID != "" {
		rt := cat.ByID[parentID]
		if rt == nil {
			return nil
		}
		desc := strings.TrimSpace(rt.Description)
		if desc == "" {
			desc = rt.Label
		}
		return []PackedOption{{
			Choice:      "parent:" + rt.ID,
			Label:       rt.Label,
			Description: desc,
			ParentID:    rt.ID,
		}}
	}
	var out []PackedOption
	for _, id := range cat.Roots {
		rt := cat.ByID[id]
		if rt == nil || rt.Status == catalog.StatusDeprecated || rt.Leaf {
			continue
		}
		desc := strings.TrimSpace(rt.Description)
		if desc == "" {
			desc = rt.Label
		}
		out = append(out, PackedOption{
			Choice:      "parent:" + rt.ID,
			Label:       rt.Label,
			Description: desc,
			ParentID:    rt.ID,
		})
	}
	return out
}

func requireDeadline(ctx context.Context) error {
	if ctx == nil {
		return newErr("Operate", CodeNoDeadline, "context required", nil)
	}
	if _, ok := ctx.Deadline(); !ok {
		return newErr("Operate", CodeNoDeadline, "context must have a deadline", nil)
	}
	return nil
}

func packGateOptions(d DraftOut) []PackedOption {
	summary := strings.TrimSpace(d.Description)
	if d.Kind == DraftKindAlias {
		summary = fmt.Sprintf("alias %q → leaf %s", d.Alias, d.LeafID)
	}
	return []PackedOption{
		{
			Choice:      ChoiceAcceptDraft,
			Label:       "Accept draft",
			Description: summary,
		},
		{
			Choice:      ChoiceReject,
			Label:       "Reject draft",
			Description: "Do not apply this draft to the vocabulary.",
		},
	}
}

func validateDraft(cat *catalog.Catalog, d DraftOut) error {
	switch strings.TrimSpace(d.Kind) {
	case DraftKindNewLeaf:
		id := strings.TrimSpace(d.ID)
		if id == "" {
			return newErr("Operate", CodeInvalidDraft, "new_leaf id required", nil)
		}
		if _, ok := cat.Lookup(id); ok {
			return newErr("Operate", CodeInvalidDraft, fmt.Sprintf("new_leaf id %q already exists", id), nil)
		}
		parent := strings.TrimSpace(d.Parent)
		if parent == "" {
			return newErr("Operate", CodeInvalidDraft, "new_leaf parent required", nil)
		}
		p, ok := cat.Lookup(parent)
		if !ok || isAssignableLeaf(cat, p) {
			return newErr("Operate", CodeInvalidDraft, "new_leaf parent must be a branch", nil)
		}
		if strings.TrimSpace(d.Label) == "" {
			return newErr("Operate", CodeInvalidDraft, "new_leaf label required", nil)
		}
		if strings.TrimSpace(d.Description) == "" {
			return newErr("Operate", CodeInvalidDraft, "new_leaf description required", nil)
		}
	case DraftKindAlias:
		leaf := strings.TrimSpace(d.LeafID)
		alias := strings.TrimSpace(d.Alias)
		if leaf == "" || alias == "" {
			return newErr("Operate", CodeInvalidDraft, "alias requires leaf and alias text", nil)
		}
		rt, ok := cat.Lookup(leaf)
		if !ok || !isAssignableLeaf(cat, rt) {
			return newErr("Operate", CodeInvalidDraft, "alias leaf must exist", nil)
		}
	default:
		return newErr("Operate", CodeInvalidDraft, fmt.Sprintf("unknown draft kind %q", d.Kind), nil)
	}
	return nil
}
