package tags

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/harness"
)

const (
	defaultMinScore      = 0.80
	defaultStoreTimeout  = 30 * time.Second
	defaultJudgeTimeout  = 2 * time.Minute
	defaultMaxHopOptions = 8

	choiceDrop = "drop"
	choiceKeep = "keep"

	labelDrop = "drop leftover token"
	labelKeep = "keep as tag"
)

// DefaultMaxHopOptions is the max packed options per Judge call (including keep).
const DefaultMaxHopOptions = defaultMaxHopOptions

// Objective configures curator hop copy and world context.
type Objective struct {
	WorldContext string
	Purpose      string
	NotPurpose   string
	TokenHint    string
	KeepOwn      string
}

// Row is one ledger row with stable id and shape text.
type Row struct {
	ID    string
	Shape string
}

// Ledger reads and rewrites stored shape fields.
type Ledger interface {
	ListComplete(ctx context.Context) ([]Row, error)
	Rewrite(ctx context.Context, id, shape string) error
}

// CuratorConfig configures ledger tag unify and optional Judge curation.
type CuratorConfig struct {
	Ledger       Ledger
	Catalog      *Catalog
	Objective    Objective
	Judge        harness.Judge
	MinScore     float64
	StoreTimeout time.Duration
	JudgeTimeout time.Duration
}

// Result summarizes a tag curation run.
type Result struct {
	RegisterPath string
	Rewrote      int
	Dropped      []string
	Merged       map[string]string
	Skipped      []string
	JudgeSkipped bool
	Error        string
}

// Curator rewrites stored shapes and optional Judge cleanup.
type Curator struct {
	ledger       Ledger
	catalog      *Catalog
	catalogCfg   Config
	depPath      string
	register     string
	objective    Objective
	judge        harness.Judge
	minScore     float64
	storeTimeout time.Duration
	judgeTimeout time.Duration
}

// CreateCurator builds a curator after nil checks and objective validation.
func CreateCurator(cfg CuratorConfig) (*Curator, error) {
	const op = "tags.CreateCurator"
	if cfg.Ledger == nil {
		return nil, newErr(op, CodeInvalid, "nil ledger", nil)
	}
	if cfg.Catalog == nil {
		return nil, newErr(op, CodeInvalid, "nil catalog", nil)
	}
	if strings.TrimSpace(cfg.Objective.WorldContext) == "" ||
		strings.TrimSpace(cfg.Objective.Purpose) == "" ||
		strings.TrimSpace(cfg.Objective.NotPurpose) == "" {
		return nil, newErr(op, CodeConfig, "objective WorldContext, Purpose, and NotPurpose required", nil)
	}
	depPath := strings.TrimSpace(cfg.Catalog.DeprecationsPath())
	if depPath == "" {
		return nil, newErr(op, CodeInvalid, "empty deprecations path", nil)
	}
	min := cfg.MinScore
	if min == 0 {
		min = defaultMinScore
	}
	storeTO := cfg.StoreTimeout
	if storeTO == 0 {
		storeTO = defaultStoreTimeout
	}
	judgeTO := cfg.JudgeTimeout
	if judgeTO == 0 {
		judgeTO = defaultJudgeTimeout
	}
	return &Curator{
		ledger:       cfg.Ledger,
		catalog:      cfg.Catalog,
		catalogCfg:   cfg.Catalog.Config(),
		depPath:      depPath,
		register:     cfg.Catalog.RegisterPath(),
		objective:    cfg.Objective,
		judge:        cfg.Judge,
		minScore:     min,
		storeTimeout: storeTO,
		judgeTimeout: judgeTO,
	}, nil
}

// Catalog returns the catalog last loaded by the curator.
func (c *Curator) Catalog() *Catalog {
	if c == nil {
		return nil
	}
	return c.catalog
}

// Run applies deterministic filter rewrite, optional Judge maps, and register regeneration.
func (c *Curator) Run(parent context.Context) (Result, error) {
	const op = "tags.Curator.Run"
	if c == nil || c.ledger == nil || c.catalog == nil {
		return Result{}, newErr(op, CodeInvalid, "nil curator", nil)
	}
	if parent == nil {
		return Result{}, newErr(op, CodeInvalid, "nil context", nil)
	}
	if _, ok := parent.Deadline(); !ok {
		return Result{}, newErr(op, CodeNoDeadline, "context must have deadline", nil)
	}
	res := Result{Merged: map[string]string{}}
	storeCtx, cancel := context.WithTimeout(parent, c.storeTimeout)
	defer cancel()
	rewrote, err := c.rewriteAll(storeCtx, c.catalog)
	if err != nil {
		return res, err
	}
	res.Rewrote = rewrote
	regPath, regErr := RegenerateRegister(storeCtx, c.ledger, c.catalog)
	if regErr != nil {
		return res, newErr(op, CodeFailed, "regenerate register", regErr)
	}
	res.RegisterPath = regPath
	if c.judge == nil {
		res.JudgeSkipped = true
		return res, nil
	}
	left := c.collectLeftovers(storeCtx, c.catalog)
	if len(left) == 0 {
		return res, nil
	}
	judgeCtx, jevCancel := context.WithTimeout(parent, c.judgeTimeout)
	defer jevCancel()
	deps := slices.Clone(c.catalog.Deprecations())
	dropped, _ := LoadDropped(c.depPath)
	dropped = slices.Clone(dropped)
	depMap := deprecationMap(deps)
	dropSet := droppedIDSet(dropped)
	seeds := c.catalog.SeedIDs()
	seedSuffix := formatSeedSuffix(seeds)
	skipped := make([]string, 0)
	keepAfterHop1 := make([]string, 0)
	for _, token := range left {
		dec, err := c.judge.Decide(judgeCtx, harness.DecideIn{
			WorldContext: c.objective.WorldContext,
			Text:         tokenState(token, c.objective.TokenHint, seedSuffix),
			Options:      hop1Options(c.objective),
		})
		if err != nil {
			return res, newErr(op, CodeJudge, "judge hop1", err)
		}
		choice, score, ok := applyDecision(dec, c.minScore)
		if !ok || choice == choiceDrop {
			if ok && choice == choiceDrop {
				if _, exists := dropSet[token]; !exists {
					dropped = append(dropped, token)
					dropSet[token] = struct{}{}
					res.Dropped = append(res.Dropped, token)
				}
			} else {
				skipped = append(skipped, token)
			}
			continue
		}
		keepAfterHop1 = append(keepAfterHop1, token)
		_ = score
	}
	for _, token := range keepAfterHop1 {
		mergeChoice, mergeOK := c.mergeHop(judgeCtx, token, seeds, seedSuffix)
		if !mergeOK {
			skipped = append(skipped, token)
			continue
		}
		if mergeChoice == choiceKeep {
			continue
		}
		if _, exists := depMap[token]; exists {
			continue
		}
		if !slices.Contains(seeds, mergeChoice) {
			skipped = append(skipped, token)
			continue
		}
		deps = append(deps, Deprecation{From: token, To: mergeChoice})
		depMap[token] = mergeChoice
		res.Merged[token] = mergeChoice
	}
	res.Skipped = skipped
	if err := SaveMaps(c.depPath, deps, dropped); err != nil {
		return res, newErr(op, CodeFailed, "save tag maps", err)
	}
	cat, err := CreateCatalog(c.catalogCfg)
	if err != nil {
		return res, newErr(op, CodeFailed, "reload catalog", err)
	}
	c.catalog = cat
	storeCtx2, cancel2 := context.WithTimeout(parent, c.storeTimeout)
	defer cancel2()
	rewrote2, err := c.rewriteAll(storeCtx2, c.catalog)
	if err != nil {
		return res, err
	}
	res.Rewrote += rewrote2
	regPath2, regErr := RegenerateRegister(storeCtx2, c.ledger, c.catalog)
	if regErr != nil {
		return res, newErr(op, CodeFailed, "regenerate register after judge", regErr)
	}
	res.RegisterPath = regPath2
	return res, nil
}

func (c *Curator) mergeHop(ctx context.Context, token string, seeds []string, seedSuffix string) (string, bool) {
	chunk := defaultMaxHopOptions - 1
	if chunk < 1 {
		chunk = 1
	}
	bestChoice := choiceKeep
	bestScore := -1.0
	keepOwn := c.objective.KeepOwn
	if strings.TrimSpace(keepOwn) == "" {
		keepOwn = c.objective.Purpose
	}
	for i := 0; i < len(seeds); i += chunk {
		end := i + chunk
		if end > len(seeds) {
			end = len(seeds)
		}
		opts := hop2Options(seeds[i:end], keepOwn, c.objective.Purpose)
		dec, err := c.judge.Decide(ctx, harness.DecideIn{
			WorldContext: c.objective.WorldContext,
			Text:         tokenState(token, c.objective.TokenHint, seedSuffix),
			Options:      opts,
		})
		if err != nil || dec.Choice == "" {
			return "", false
		}
		choice, score, ok := applyDecision(dec, c.minScore)
		if !ok {
			continue
		}
		if score > bestScore {
			bestScore = score
			bestChoice = choice
		}
	}
	if bestScore < 0 {
		return "", false
	}
	return bestChoice, true
}

func hop1Options(obj Objective) []harness.PackedOption {
	return []harness.PackedOption{
		{Choice: choiceDrop, Label: labelDrop, Description: obj.NotPurpose},
		{Choice: choiceKeep, Label: labelKeep, Description: obj.Purpose},
	}
}

func hop2Options(seeds []string, keepOwn, purpose string) []harness.PackedOption {
	opts := []harness.PackedOption{{
		Choice:      choiceKeep,
		Label:       labelKeep,
		Description: keepOwn,
	}}
	for _, s := range seeds {
		opts = append(opts, harness.PackedOption{
			Choice:      s,
			Label:       "merge into seed " + s,
			Description: "The token is a synonym or near-synonym of seed " + s + ". Both would be used for: " + purpose,
		})
	}
	return opts
}

func tokenState(token, hint, seedSuffix string) string {
	var b strings.Builder
	b.WriteString("Token: ")
	b.WriteString(token)
	b.WriteByte('\n')
	if strings.TrimSpace(hint) != "" {
		b.WriteString(strings.TrimSpace(hint))
	}
	b.WriteString(seedSuffix)
	return b.String()
}

func applyDecision(dec harness.DecideOut, minScore float64) (string, float64, bool) {
	if dec.Score < minScore {
		return "", 0, false
	}
	choice := strings.TrimSpace(dec.Choice)
	if choice == "" {
		return "", 0, false
	}
	return choice, dec.Score, true
}

func (c *Curator) rewriteAll(ctx context.Context, cat *Catalog) (int, error) {
	const op = "tags.Curator.rewriteAll"
	rows, err := c.ledger.ListComplete(ctx)
	if err != nil {
		return 0, newErr(op, CodeFailed, "list complete", err)
	}
	var rewrote int
	for _, row := range rows {
		filtered := FilterStoredShape(row.Shape, cat)
		if filtered == strings.TrimSpace(row.Shape) {
			continue
		}
		if err := c.ledger.Rewrite(ctx, row.ID, filtered); err != nil {
			return rewrote, newErr(op, CodeFailed, "rewrite shape", err)
		}
		rewrote++
	}
	return rewrote, nil
}

func (c *Curator) collectLeftovers(ctx context.Context, cat *Catalog) []string {
	rows, err := c.ledger.ListComplete(ctx)
	if err != nil {
		return nil
	}
	depKeys := deprecationMap(cat.Deprecations())
	seedSet := seedIDSet(cat.SeedIDs())
	seen := map[string]struct{}{}
	for _, row := range rows {
		filtered := FilterStoredShape(row.Shape, cat)
		for _, tok := range strings.Fields(filtered) {
			if _, seed := seedSet[tok]; seed {
				continue
			}
			if _, inDep := depKeys[tok]; inDep {
				continue
			}
			seen[tok] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for tok := range seen {
		out = append(out, tok)
	}
	slices.Sort(out)
	return out
}

func formatSeedSuffix(seeds []string) string {
	if len(seeds) == 0 {
		return ""
	}
	const max = 12
	show := seeds
	if len(show) > max {
		show = show[:max]
	}
	return "\nActive seeds: " + strings.Join(show, ", ")
}

func droppedIDSet(ids []string) map[string]struct{} {
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}
