package harness

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
)

type fakeEssencer struct {
	out EssenceOut
	err error
}

func (f *fakeEssencer) Essence(_ context.Context, _ EssenceIn) (EssenceOut, error) {
	if f.err != nil {
		return EssenceOut{}, f.err
	}
	return f.out, nil
}

type fakeEmbedder struct {
	vecs [][]float64
	err  error
}

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.vecs != nil {
		return f.vecs, nil
	}
	out := make([][]float64, len(texts))
	for i := range texts {
		out[i] = []float64{1, 0}
	}
	return out, nil
}

func sampleEssence() EssenceOut {
	return EssenceOut{
		Why:   [5]string{"w1", "w2", "w3", "w4", "w5"},
		About: "about",
		Shape: "shape",
		Kind:  "notification > software-release > changelog",
	}
}

func attachHarness(t *testing.T, judge *fakeJudge, embed *fakeEmbedder) *Harness {
	h, err := CreateHarness(Config{
		Judge:    judge,
		Author:   &fakeAuthor{},
		Embedder: embed,
		Essencer: &fakeEssencer{out: sampleEssence()},
		Strategy: StrategyAttach,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func emptyKindCatalog(t *testing.T) *catalog.Catalog {
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{ID: "inbox-kind", Label: "Kind"})
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func TestCreateHarness_attachRequiresSeats(t *testing.T) {
	_, err := CreateHarness(Config{
		Judge:    &fakeJudge{},
		Author:   &fakeAuthor{},
		Strategy: StrategyAttach,
	})
	if CodeOf(err) != CodeConfig {
		t.Fatalf("got %v", err)
	}
}

func TestCreateHarness_walkReinforceAboveAttach(t *testing.T) {
	_, err := CreateHarness(Config{
		Judge:            &fakeJudge{},
		Author:           &fakeAuthor{},
		Strategy:         StrategyAttach,
		Embedder:         &fakeEmbedder{},
		Essencer:         &fakeEssencer{},
		AttachMinCosine:  0.8,
		WalkReinforceMin: 0.9,
	})
	if CodeOf(err) != CodeConfig {
		t.Fatalf("got %v", err)
	}
}

func TestAttach_emptyCatalog_breadcrumb(t *testing.T) {
	h := attachHarness(t, &fakeJudge{}, &fakeEmbedder{})
	res, err := h.Operate(ctxWithDeadline(t), Op{
		WorldContext: "world",
		Text:         "body",
		Catalog:      emptyKindCatalog(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Draft == nil || res.Draft.Kind != DraftKindBreadcrumb {
		t.Fatalf("draft %v", res.Draft)
	}
	if res.Cosine != 0 {
		t.Fatalf("cosine %v", res.Cosine)
	}
}

func TestAttach_injectedCosine(t *testing.T) {
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "k",
		Terms: []catalog.Term{
			{ID: "notification", Label: "N", Description: "d"},
			{ID: "leaf", Label: "L", Parent: "notification", Description: "d"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	embed := &fakeEmbedder{
		vecs: [][]float64{
			{0, 1},
			{1, 0},
		},
	}
	h, err := CreateHarness(Config{
		Judge:    &fakeJudge{},
		Author:   &fakeAuthor{},
		Embedder: embed,
		Essencer: &fakeEssencer{out: sampleEssence()},
		Strategy: StrategyAttach,
		Cosine:   func(_ []float64, _ []float64) float64 { return 1.0 },
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := h.Operate(ctxWithDeadline(t), Op{WorldContext: "w", Text: "x", Catalog: cat})
	if err != nil {
		t.Fatal(err)
	}
	if res.Draft == nil || res.Draft.Kind != DraftKindAlias {
		t.Fatalf("draft %+v", res.Draft)
	}
}

func TestAttach_highCosine_aliasNoJudge(t *testing.T) {
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "k",
		Terms: []catalog.Term{
			{ID: "notification", Label: "N", Description: "d"},
			{ID: "leaf", Label: "L", Parent: "notification", Description: "d"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	judge := &fakeJudge{choices: []string{ChoiceSkip}, scores: []float64{0}}
	embed := &fakeEmbedder{
		vecs: [][]float64{
			{1, 0},
			{1, 0},
		},
	}
	h := attachHarness(t, judge, embed)
	res, err := h.Operate(ctxWithDeadline(t), Op{WorldContext: "w", Text: "x", Catalog: cat})
	if err != nil {
		t.Fatal(err)
	}
	if judge.call != 0 {
		t.Fatalf("judge called %d times", judge.call)
	}
	if res.Draft == nil || res.Draft.Kind != DraftKindAlias {
		t.Fatalf("draft %+v", res.Draft)
	}
	if res.Cosine < 0.99 {
		t.Fatalf("cosine %v", res.Cosine)
	}
}

func TestAttach_lowCosine_breadcrumb(t *testing.T) {
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "k",
		Terms: []catalog.Term{
			{ID: "notification", Label: "N", Description: "d"},
			{ID: "leaf", Label: "L", Parent: "notification", Description: "d"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	embed := &fakeEmbedder{
		vecs: [][]float64{
			{1, 0},
			{0, 1},
		},
	}
	h := attachHarness(t, &fakeJudge{}, embed)
	res, err := h.Operate(ctxWithDeadline(t), Op{WorldContext: "w", Text: "x", Catalog: cat})
	if err != nil {
		t.Fatal(err)
	}
	if res.Draft.Kind != DraftKindBreadcrumb {
		t.Fatalf("got %s", res.Draft.Kind)
	}
	if res.Cosine > 0.01 {
		t.Fatalf("cosine %v", res.Cosine)
	}
}

func TestAttach_fuzzyWalk_reinforce(t *testing.T) {
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "k",
		Terms: []catalog.Term{
			{ID: "notification", Label: "N", Description: "d"},
			{ID: "target-leaf", Label: "T", Parent: "notification", Description: "d"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	q := []float64{1, 0}
	leaf := []float64{0.75, math.Sqrt(1 - 0.75*0.75)}
	c := CosineSimilarity(q, leaf)
	if c < 0.7 || c >= 0.8 {
		t.Fatalf("setup cosine %v", c)
	}
	embed := &fakeEmbedder{vecs: [][]float64{q, leaf}}
	judge := &fakeJudge{
		choices: []string{ChoicePrefixUse + "target-leaf"},
		scores:  []float64{0.9},
	}
	h := attachHarness(t, judge, embed)
	res, err := h.Operate(ctxWithDeadline(t), Op{WorldContext: "w", Text: "x", Catalog: cat})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Reinforced {
		t.Fatal("expected reinforced")
	}
	if len(res.Assigned) == 0 || res.Assigned[0].TermID != "target-leaf" {
		t.Fatalf("assigned %+v", res.Assigned)
	}
	if judge.call < 1 {
		t.Fatal("expected walk judge call")
	}
}

func TestCreateHarness_walkNilEmbedderOK(t *testing.T) {
	h, err := CreateHarness(Config{
		Judge:  &fakeJudge{},
		Author: &fakeAuthor{},
	})
	if err != nil || h == nil {
		t.Fatalf("got %v", err)
	}
}

func TestAttach_embedNonFinite(t *testing.T) {
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "k",
		Terms: []catalog.Term{
			{ID: "notification", Label: "N", Description: "d"},
			{ID: "leaf", Label: "L", Parent: "notification", Description: "d"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	embed := &fakeEmbedder{vecs: [][]float64{{1, 0}, {math.NaN(), 0}}}
	h := attachHarness(t, &fakeJudge{}, embed)
	_, err = h.Operate(ctxWithDeadline(t), Op{WorldContext: "w", Text: "x", Catalog: cat})
	if CodeOf(err) != CodeEmbed {
		t.Fatalf("got %v", err)
	}
}

func TestAttach_embedDimensionMismatch(t *testing.T) {
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "k",
		Terms: []catalog.Term{
			{ID: "notification", Label: "N", Description: "d"},
			{ID: "leaf", Label: "L", Parent: "notification", Description: "d"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	embed := &fakeEmbedder{vecs: [][]float64{{1, 0}, {1}}}
	h := attachHarness(t, &fakeJudge{}, embed)
	_, err = h.Operate(ctxWithDeadline(t), Op{WorldContext: "w", Text: "x", Catalog: cat})
	if CodeOf(err) != CodeEmbed {
		t.Fatalf("got %v", err)
	}
}

func TestAttach_termOrderTiePicksFirstLeaf(t *testing.T) {
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "k",
		Terms: []catalog.Term{
			{ID: "root", Label: "R", Description: "d"},
			{ID: "leaf-z", Label: "Z", Parent: "root", Description: "d"},
			{ID: "leaf-a", Label: "A", Parent: "root", Description: "d"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	v := []float64{1, 0}
	embed := &fakeEmbedder{vecs: [][]float64{v, v, v}}
	h := attachHarness(t, &fakeJudge{}, embed)
	res, err := h.Operate(ctxWithDeadline(t), Op{WorldContext: "w", Text: "x", Catalog: cat})
	if err != nil {
		t.Fatal(err)
	}
	if res.CanonicalID != "leaf-z" {
		t.Fatalf("canonical %q", res.CanonicalID)
	}
}

func TestAttach_reinforceStop_remainingKindSegments(t *testing.T) {
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "k",
		Terms: []catalog.Term{
			{ID: "notification", Label: "N", Description: "d"},
			{ID: "software-release", Label: "SR", Parent: "notification", Description: "d"},
			{ID: "target-leaf", Label: "T", Parent: "software-release", Description: "d"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	q := []float64{1, 0}
	leaf := []float64{0.75, math.Sqrt(1 - 0.75*0.75)}
	embed := &fakeEmbedder{vecs: [][]float64{q, leaf}}
	judge := &fakeJudge{
		choices: []string{ChoiceSkip},
		scores:  []float64{0.1},
	}
	h := attachHarness(t, judge, embed)
	res, err := h.Operate(ctxWithDeadline(t), Op{WorldContext: "w", Text: "x", Catalog: cat})
	if err != nil {
		t.Fatal(err)
	}
	if res.Draft == nil || res.Draft.Kind != DraftKindBreadcrumb {
		t.Fatalf("draft %+v", res.Draft)
	}
	if res.Draft.Parent != "software-release" {
		t.Fatalf("parent %q", res.Draft.Parent)
	}
	if res.Draft.Alias != "changelog" {
		t.Fatalf("alias %q want remaining segment only", res.Draft.Alias)
	}
}

func TestAttach_breadcrumb_idCollisionMint(t *testing.T) {
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "k",
		Terms: []catalog.Term{
			{ID: "other", Label: "O", Description: "d"},
			{ID: "software-release", Label: "SR", Parent: "other", Description: "d"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	embed := &fakeEmbedder{vecs: [][]float64{{1, 0}, {0, 1}}}
	h := attachHarness(t, &fakeJudge{}, embed)
	res, err := h.Operate(ctxWithDeadline(t), Op{
		WorldContext: "w",
		Text:         "x",
		Catalog:      cat,
	})
	if err != nil {
		t.Fatal(err)
	}
	vocab, err := Apply(cat.Vocab, res)
	if err != nil {
		t.Fatal(err)
	}
	foundMint := false
	for _, term := range vocab.Terms {
		if strings.HasPrefix(term.ID, "notification-software-release") {
			foundMint = true
		}
	}
	if !foundMint {
		t.Fatalf("terms %+v", vocab.Terms)
	}
}

func TestSegmentsRemainingAfterPath(t *testing.T) {
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "k",
		Terms: []catalog.Term{
			{ID: "notification", Label: "N", Description: "d"},
			{ID: "software-release", Label: "SR", Parent: "notification", Description: "d"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	segs := catalog.ParseKindSegments("notification > software-release > changelog")
	rem := segmentsRemainingAfterPath(cat, segs, []string{"notification", "software-release"})
	if len(rem) != 1 || rem[0] != "changelog" {
		t.Fatalf("%v", rem)
	}
}

func TestAttach_prefilledEssenceSkipsEssencer(t *testing.T) {
	tracker := &countingEssencer{out: sampleEssence()}
	h, err := CreateHarness(Config{
		Judge:    &fakeJudge{choices: []string{ChoiceSkip}, scores: []float64{1}},
		Author:   &fakeAuthor{},
		Embedder: &fakeEmbedder{},
		Essencer: tracker,
		Strategy: StrategyAttach,
	})
	if err != nil {
		t.Fatal(err)
	}
	pre := sampleEssence()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = h.Operate(ctx, Op{
		WorldContext: "w",
		Text:         "msg",
		Catalog:      emptyKindCatalog(t),
		Essence:      &pre,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tracker.calls != 0 {
		t.Fatalf("essencer calls=%d", tracker.calls)
	}
}

func TestAttach_nilEssenceCallsEssencer(t *testing.T) {
	tracker := &countingEssencer{out: sampleEssence()}
	h, err := CreateHarness(Config{
		Judge:    &fakeJudge{choices: []string{ChoiceSkip}, scores: []float64{1}},
		Author:   &fakeAuthor{},
		Embedder: &fakeEmbedder{},
		Essencer: tracker,
		Strategy: StrategyAttach,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = h.Operate(ctx, Op{
		WorldContext: "w",
		Text:         "msg",
		Catalog:      emptyKindCatalog(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if tracker.calls != 1 {
		t.Fatalf("essencer calls=%d", tracker.calls)
	}
}

type countingEssencer struct {
	out   EssenceOut
	calls int
}

func (c *countingEssencer) Essence(_ context.Context, _ EssenceIn) (EssenceOut, error) {
	c.calls++
	return c.out, nil
}

func TestApply_breadcrumb(t *testing.T) {
	vocab := catalog.Vocabulary{ID: "inbox-kind", Label: "Kind"}
	ess := sampleEssence()
	res := Result{
		DraftAccepted: true,
		Essence:       &ess,
		Draft: &DraftOut{
			Kind:        DraftKindBreadcrumb,
			Alias:       ess.Kind,
			Description: ess.About,
		},
	}
	out, err := Apply(vocab, res)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Terms) != 3 {
		t.Fatalf("terms %d", len(out.Terms))
	}
}
