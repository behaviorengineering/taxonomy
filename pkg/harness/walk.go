package harness

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
)

type walkConfig struct {
	judgeText string
	worldBase string
	parentID  string
	path      []string
	reinforce bool
}

type reinforceWalkStop struct {
	parentID string
	path     []string
	score    float64
}

func (e reinforceWalkStop) Error() string {
	return "reinforce walk stop"
}

func worldWithWalkPath(worldBase string, path []string) string {
	if len(path) == 0 {
		return worldBase
	}
	return worldBase + "\nWalk path: " + strings.Join(path, " > ")
}

func (h *Harness) walkFrom(ctx context.Context, cat *catalog.Catalog, cfg walkConfig) (Result, error) {
	cap := len(cat.ByID) + 1
	if hopCapForTest > 0 {
		cap = hopCapForTest
	}
	parentID := cfg.parentID
	path := append([]string{}, cfg.path...)
	hops := 0
	lastScore := 0.0
	judgeText := cfg.judgeText
	worldBase := cfg.worldBase

	for {
		hops++
		if hops > cap {
			return Result{}, newErr("Operate", CodeHopLimit, "hop cap exceeded", nil)
		}
		if err := ctx.Err(); err != nil {
			return Result{}, newErr("Operate", CodeConfig, "context", err)
		}

		world := worldWithWalkPath(worldBase, path)
		options, choiceSet, err := packChildren(cat, parentID)
		if err != nil {
			return Result{}, err
		}
		if len(options) == 1 && options[0].Choice == ChoiceSkip {
			if cfg.reinforce {
				return Result{}, reinforceWalkStop{parentID: parentID, path: path, score: lastScore}
			}
			return Result{}, newErr("Operate", CodeConfig, "no packable children", nil)
		}

		decideOut, err := h.judge.Decide(ctx, DecideIn{
			WorldContext: world,
			Text:         judgeText,
			Options:      options,
			Path:         append([]string{}, path...),
		})
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
			rt, ok := cat.Lookup(id)
			if !ok {
				return Result{}, newErr("Operate", CodeUnknownChoice, fmt.Sprintf("term %q missing from catalog", id), nil)
			}
			path = append(path, rt.ID)
			if isAssignableLeaf(cat, rt) {
				assignRT, ok := cat.PreferLeaf(rt.ID)
				if !ok || !isAssignableLeaf(cat, assignRT) {
					return Result{}, newErr("Operate", CodeUnknownChoice, fmt.Sprintf("leaf %q not assignable", id), nil)
				}
				return Result{
					Assigned: []catalog.Assignment{{
						Vocab:  cat.Vocab.ID,
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

		if cfg.reinforce {
			return Result{}, reinforceWalkStop{parentID: parentID, path: path, score: lastScore}
		}
		reason := "skip"
		if decideOut.Choice != ChoiceSkip {
			reason = fmt.Sprintf("low_score:%.3f", decideOut.Score)
		}
		return h.authorAndGate(ctx, cat, world, judgeText, parentID, reason, lastScore, path)
	}
}
