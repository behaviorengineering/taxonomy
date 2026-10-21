package harness

import (
	"testing"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
)

func aliasFixtureVocab() catalog.Vocabulary {
	return catalog.Vocabulary{
		ID: "demo",
		Terms: []catalog.Term{
			{ID: "notification", Label: "N", Description: "d"},
			{ID: "ci", Label: "CI", Parent: "notification", Description: "d"},
			{
				ID:          "ci-failure",
				Label:       "CI failure",
				Parent:      "ci",
				Description: "d",
				Aliases:     []string{"notification > ci > ci-failure"},
			},
			{ID: "refactor", Label: "Refactor", Parent: "ci", Description: "d"},
		},
	}
}

func TestApply_aliasCollision(t *testing.T) {
	vocab := aliasFixtureVocab()
	before := append([]string{}, vocab.Terms[3].Aliases...)
	_, err := Apply(vocab, Result{
		DraftAccepted: true,
		Draft: &DraftOut{
			Kind:   DraftKindAlias,
			LeafID: "refactor",
			Alias:  "notification > ci > ci-failure",
		},
	})
	if err == nil {
		t.Fatal("expected collision")
	}
	if catalog.CodeOf(err) != catalog.CodeAliasCollision {
		t.Fatalf("code %v", catalog.CodeOf(err))
	}
	if !equalStringSlices(vocab.Terms[3].Aliases, before) {
		t.Fatal("vocab unchanged on collision")
	}
}

func TestApply_aliasIdempotentOnOwner(t *testing.T) {
	vocab := aliasFixtureVocab()
	out, err := Apply(vocab, Result{
		DraftAccepted: true,
		Draft: &DraftOut{
			Kind:   DraftKindAlias,
			LeafID: "ci-failure",
			Alias:  "notification > ci > ci-failure",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Terms[2].Aliases) != 1 {
		t.Fatalf("aliases %v", out.Terms[2].Aliases)
	}
}

func TestApply_emptyAlias(t *testing.T) {
	vocab := aliasFixtureVocab()
	_, err := Apply(vocab, Result{
		DraftAccepted: true,
		Draft: &DraftOut{
			Kind:   DraftKindAlias,
			LeafID: "refactor",
			Alias:  "  ",
		},
	})
	if CodeOf(err) != CodeInvalidDraft {
		t.Fatalf("code %v", CodeOf(err))
	}
}

func TestApply_newLeafIDCollidesWithAlias(t *testing.T) {
	vocab := aliasFixtureVocab()
	_, err := Apply(vocab, Result{
		DraftAccepted: true,
		Draft: &DraftOut{
			Kind:        DraftKindNewLeaf,
			ID:          "notification > ci > ci-failure",
			Label:       "X",
			Parent:      "ci",
			Description: "d",
		},
	})
	if catalog.CodeOf(err) != catalog.CodeAliasCollision {
		t.Fatalf("code %v err=%v", catalog.CodeOf(err), err)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
