package harness

import (
	"context"
	"testing"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
)

type fakeJudge struct {
	choices []string
	scores  []float64
	call    int
}

func (f *fakeJudge) Decide(_ context.Context, in DecideIn) (DecideOut, error) {
	i := f.call
	f.call++
	if i >= len(f.choices) {
		return DecideOut{Choice: ChoiceReject, Score: 1}, nil
	}
	return DecideOut{Choice: f.choices[i], Score: f.scores[i]}, nil
}

type fakeAuthor struct {
	draft DraftOut
}

func (f *fakeAuthor) Draft(_ context.Context, _ DraftIn) (DraftOut, error) {
	return f.draft, nil
}

func testCatalog(t *testing.T) *catalog.Catalog {
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "demo",
		Terms: []catalog.Term{
			{ID: "branch", Label: "Branch", Description: "branch desc"},
			{ID: "leaf-a", Label: "Leaf A", Parent: "branch", Description: "leaf a desc"},
		},
	})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	return cat
}

func ctxWithDeadline(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestOperate_noDeadline(t *testing.T) {
	h, err := CreateHarness(Config{Judge: &fakeJudge{}, Author: &fakeAuthor{}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.Operate(context.Background(), Op{WorldContext: "w", Text: "x", Catalog: testCatalog(t)})
	if CodeOf(err) != CodeNoDeadline {
		t.Fatalf("expected no deadline: %v", err)
	}
}

func TestOperate_emptyWorld(t *testing.T) {
	h, _ := CreateHarness(Config{Judge: &fakeJudge{}, Author: &fakeAuthor{}})
	_, err := h.Operate(ctxWithDeadline(t), Op{Text: "x", Catalog: testCatalog(t)})
	if CodeOf(err) != CodeWorldContext {
		t.Fatalf("world: %v", err)
	}
}

func TestOperate_useLeaf(t *testing.T) {
	h, err := CreateHarness(Config{
		Judge:  &fakeJudge{choices: []string{ChoicePrefixUse + "leaf-a"}, scores: []float64{0.9}},
		Author: &fakeAuthor{},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := h.Operate(ctxWithDeadline(t), Op{
		WorldContext: "case brief",
		Text:         "event body",
		Catalog:      testCatalog(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Assigned) != 1 || res.Assigned[0].TermID != "leaf-a" {
		t.Fatalf("assigned: %+v", res.Assigned)
	}
}

func TestOperate_skipAcceptNewLeaf(t *testing.T) {
	h, err := CreateHarness(Config{
		Judge: &fakeJudge{
			choices: []string{ChoiceSkip, ChoiceAcceptDraft},
			scores:  []float64{0.9, 0.95},
		},
		Author: &fakeAuthor{draft: DraftOut{
			Kind:        DraftKindNewLeaf,
			ID:          "leaf-b",
			Parent:      "branch",
			Label:       "Leaf B",
			Description: "new leaf",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := h.Operate(ctxWithDeadline(t), Op{
		WorldContext: "brief",
		Text:         "text",
		Catalog:      testCatalog(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.DraftAccepted || res.Draft == nil {
		t.Fatal("expected accepted draft")
	}
	if len(res.Assigned) != 1 || res.Assigned[0].TermID != "leaf-b" {
		t.Fatalf("assigned: %+v", res.Assigned)
	}
	vocab, err := Apply(testCatalog(t).Vocab, res)
	if err != nil {
		t.Fatal(err)
	}
	if len(vocab.Terms) != 3 {
		t.Fatalf("terms: %d", len(vocab.Terms))
	}
}

func TestOperate_skipRejectDraft(t *testing.T) {
	h, _ := CreateHarness(Config{
		Judge: &fakeJudge{
			choices: []string{ChoiceSkip, ChoiceReject},
			scores:  []float64{0.2, 0.99},
		},
		Author: &fakeAuthor{draft: DraftOut{
			Kind:        DraftKindNewLeaf,
			ID:          "leaf-b",
			Parent:      "branch",
			Label:       "Leaf B",
			Description: "new leaf",
		}},
	})
	res, err := h.Operate(ctxWithDeadline(t), Op{
		WorldContext: "brief",
		Text:         "text",
		Catalog:      testCatalog(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.DraftAccepted {
		t.Fatal("expected rejected draft")
	}
}

func TestOperate_emptyLeafDescription(t *testing.T) {
	h, err := CreateHarness(Config{Judge: &fakeJudge{}, Author: &fakeAuthor{}})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "demo",
		Terms: []catalog.Term{
			{ID: "branch", Label: "Branch", Description: "branch desc"},
			{ID: "leaf-a", Label: "Leaf A", Parent: "branch"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.Operate(ctxWithDeadline(t), Op{
		WorldContext: "brief",
		Text:         "text",
		Catalog:      cat,
	})
	if CodeOf(err) != CodeEmptyDescription {
		t.Fatalf("expected empty description: %v", err)
	}
}

func TestOperate_collidingNewLeaf(t *testing.T) {
	h, err := CreateHarness(Config{
		Judge: &fakeJudge{
			choices: []string{ChoiceSkip, ChoiceAcceptDraft},
			scores:  []float64{0.9, 0.95},
		},
		Author: &fakeAuthor{draft: DraftOut{
			Kind:        DraftKindNewLeaf,
			ID:          "leaf-a",
			Parent:      "branch",
			Label:       "Dup",
			Description: "collision",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.Operate(ctxWithDeadline(t), Op{
		WorldContext: "brief",
		Text:         "text",
		Catalog:      testCatalog(t),
	})
	if CodeOf(err) != CodeInvalidDraft {
		t.Fatalf("expected invalid draft: %v", err)
	}
}
