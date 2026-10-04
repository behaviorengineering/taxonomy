package harness

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
)

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

// Operate classifies text against one merged catalog inside the game in play.
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

	options, err := packLeafOptions(op.Catalog)
	if err != nil {
		return Result{}, err
	}
	choiceSet := map[string]struct{}{ChoiceSkip: {}}
	for _, o := range options {
		choiceSet[o.Choice] = struct{}{}
	}

	decideIn := DecideIn{WorldContext: world, Text: text, Options: options}
	decideOut, err := h.judge.Decide(ctx, decideIn)
	if err != nil {
		return Result{}, newErr("Operate", CodeConfig, "judge", err)
	}
	if math.IsNaN(decideOut.Score) || math.IsInf(decideOut.Score, 0) {
		return Result{}, newErr("Operate", CodeInvalidScore, "judge score must be finite", nil)
	}
	if _, ok := choiceSet[decideOut.Choice]; !ok {
		return Result{}, newErr("Operate", CodeUnknownChoice, fmt.Sprintf("choice %q not in packed set", decideOut.Choice), nil)
	}

	res := Result{JudgeScore: decideOut.Score}
	if strings.HasPrefix(decideOut.Choice, ChoicePrefixUse) && decideOut.Score >= h.minJudgeScore {
		leafID := strings.TrimPrefix(decideOut.Choice, ChoicePrefixUse)
		rt, ok := op.Catalog.Lookup(leafID)
		if !ok || !rt.Leaf {
			return Result{}, newErr("Operate", CodeUnknownChoice, fmt.Sprintf("leaf %q missing from catalog", leafID), nil)
		}
		res.Assigned = []catalog.Assignment{{
			Vocab:  op.Catalog.Vocab.ID,
			TermID: rt.ID,
			Label:  rt.Label,
		}}
		return res, nil
	}

	reason := "skip"
	if decideOut.Choice != ChoiceSkip {
		reason = fmt.Sprintf("low_score:%.3f", decideOut.Score)
	}
	parents := packParentOptions(op.Catalog)
	draftIn := DraftIn{WorldContext: world, Text: text, Parents: parents, Reason: reason}
	draftOut, err := h.author.Draft(ctx, draftIn)
	if err != nil {
		return Result{}, newErr("Operate", CodeConfig, "author", err)
	}
	if err := validateDraft(op.Catalog, draftOut); err != nil {
		return Result{}, err
	}
	res.Draft = &draftOut

	gateOptions := packGateOptions(draftOut)
	gateOut, err := h.judge.Decide(ctx, DecideIn{
		WorldContext: world,
		Text:         text,
		Options:      gateOptions,
	})
	if err != nil {
		return Result{}, newErr("Operate", CodeConfig, "gate", err)
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
			Vocab:  op.Catalog.Vocab.ID,
			TermID: strings.TrimSpace(draftOut.ID),
			Label:  strings.TrimSpace(draftOut.Label),
		}}
	case DraftKindAlias:
		rt, ok := op.Catalog.Lookup(draftOut.LeafID)
		if !ok || !rt.Leaf {
			return Result{}, newErr("Operate", CodeInvalidDraft, "alias leaf missing", nil)
		}
		res.Assigned = []catalog.Assignment{{
			Vocab:  op.Catalog.Vocab.ID,
			TermID: rt.ID,
			Label:  rt.Label,
			Source: draftOut.Alias,
		}}
	}
	return res, nil
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

func packLeafOptions(cat *catalog.Catalog) ([]PackedOption, error) {
	var out []PackedOption
	for _, rt := range cat.LeafTerms() {
		if strings.TrimSpace(rt.Description) == "" {
			return nil, newErr("Operate", CodeEmptyDescription, fmt.Sprintf("leaf %q missing description", rt.ID), nil)
		}
		out = append(out, PackedOption{
			Choice:      ChoicePrefixUse + rt.ID,
			Label:       rt.Label,
			Description: rt.Description,
			ParentID:    rt.Parent,
			LeafID:      rt.ID,
		})
	}
	out = append(out, PackedOption{
		Choice:      ChoiceSkip,
		Label:       "Skip",
		Description: "No existing leaf fits; propose a new term or alias.",
	})
	return out, nil
}

func packParentOptions(cat *catalog.Catalog) []PackedOption {
	var out []PackedOption
	for _, rt := range cat.TermsSorted() {
		if rt.Leaf {
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
		if !ok || p.Leaf {
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
		if !ok || !rt.Leaf {
			return newErr("Operate", CodeInvalidDraft, "alias leaf must exist", nil)
		}
	default:
		return newErr("Operate", CodeInvalidDraft, fmt.Sprintf("unknown draft kind %q", d.Kind), nil)
	}
	return nil
}
