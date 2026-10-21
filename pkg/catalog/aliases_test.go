package catalog

import (
	"reflect"
	"testing"
)

func ciFailureFixture() Vocabulary {
	return Vocabulary{
		ID: "demo",
		Terms: []Term{
			{ID: "notification", Label: "N", Description: "d"},
			{ID: "ci", Label: "CI", Parent: "notification", Description: "d"},
			{
				ID:          "ci-failure",
				Label:       "CI failure",
				Parent:      "ci",
				Description: "d",
				Aliases:     []string{"notification > ci > ci-failure"},
			},
			{
				ID:          "refactor",
				Label:       "Refactor",
				Parent:      "ci",
				Description: "d",
				Aliases:     []string{"notification > ci > ci-failure"},
			},
		},
	}
}

func TestRepairAliases_lastSegmentKeepsMatchingID(t *testing.T) {
	in := ciFailureFixture()
	out, drops, err := RepairAliases(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildCatalog(out); err != nil {
		t.Fatalf("build after repair: %v", err)
	}
	for _, d := range drops {
		if d.TermID == "refactor" {
			return
		}
	}
	t.Fatalf("expected drop from refactor, got %v", drops)
}

func TestRepairAliases_breadcrumbAliasExactPath(t *testing.T) {
	in := Vocabulary{
		ID: "demo",
		Terms: []Term{
			{ID: "a", Label: "A", Description: "d"},
			{ID: "b", Label: "B", Parent: "a", Description: "d", Aliases: []string{"a > b"}},
			{ID: "c", Label: "C", Parent: "a", Description: "d", Aliases: []string{"a > b"}},
		},
	}
	out, _, err := RepairAliases(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildCatalog(out); err != nil {
		t.Fatal(err)
	}
	keep, _ := out.Terms[1].ID, out.Terms[1].Aliases
	if len(out.Terms[1].Aliases) == 0 {
		t.Fatal("winner b should keep alias")
	}
	if len(out.Terms[2].Aliases) != 0 {
		t.Fatalf("loser c should drop alias, has %v", out.Terms[2].Aliases)
	}
	_ = keep
}

func TestRepairAliases_depthThenYAMLIndex(t *testing.T) {
	in := Vocabulary{
		ID: "demo",
		Terms: []Term{
			{ID: "root", Label: "R", Description: "d"},
			{ID: "shallow", Label: "S", Parent: "root", Description: "d", Aliases: []string{"root > leaf"}},
			{ID: "deep", Label: "D", Parent: "shallow", Description: "d", Aliases: []string{"root > leaf"}},
			{ID: "leaf", Label: "L", Parent: "deep", Description: "d", Aliases: []string{"root > leaf"}},
		},
	}
	out, _, err := RepairAliases(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildCatalog(out); err != nil {
		t.Fatal(err)
	}
	if len(out.Terms[3].Aliases) == 0 {
		t.Fatal("deepest leaf should win")
	}
}

func TestRepairAliases_idBeatsAlias(t *testing.T) {
	in := Vocabulary{
		ID: "demo",
		Terms: []Term{
			{ID: "target-id", Label: "T", Description: "d"},
			{ID: "other", Label: "O", Description: "d", Aliases: []string{"target-id"}},
		},
	}
	out, drops, err := RepairAliases(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildCatalog(out); err != nil {
		t.Fatal(err)
	}
	if len(drops) == 0 {
		t.Fatal("expected alias drop")
	}
	if len(out.Terms[1].Aliases) != 0 {
		t.Fatalf("alias removed from other: %v", out.Terms[1].Aliases)
	}
}

func TestRepairAliases_doesNotMutateInput(t *testing.T) {
	in := ciFailureFixture()
	before := append([]string{}, in.Terms[3].Aliases...)
	out, _, err := RepairAliases(in)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in.Terms[3].Aliases, before) {
		t.Fatal("input mutated")
	}
	if reflect.DeepEqual(out.Terms[3].Aliases, before) {
		t.Fatal("expected repair to change loser")
	}
}

func TestRepairAliases_cycle(t *testing.T) {
	in := Vocabulary{
		ID: "demo",
		Terms: []Term{
			{ID: "a", Label: "A", Parent: "b", Description: "d", Aliases: []string{"x"}},
			{ID: "b", Label: "B", Parent: "a", Description: "d", Aliases: []string{"x"}},
		},
	}
	_, _, err := RepairAliases(in)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if CodeOf(err) != CodeCycle {
		t.Fatalf("code %v", CodeOf(err))
	}
}

func TestRepairAliases_intraTermDuplicate(t *testing.T) {
	in := Vocabulary{
		ID: "demo",
		Terms: []Term{
			{ID: "leaf", Label: "L", Description: "d", Aliases: []string{"Foo", "foo"}},
		},
	}
	out, _, err := RepairAliases(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Terms[0].Aliases) != 1 {
		t.Fatalf("collapsed: %v", out.Terms[0].Aliases)
	}
	if out.Terms[0].Aliases[0] != "Foo" {
		t.Fatalf("keep first spelling: %q", out.Terms[0].Aliases[0])
	}
}
