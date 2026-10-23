package catalog

import (
	"reflect"
	"testing"
)

func TestTransferAlias_dropsLoserKeepsWinner(t *testing.T) {
	in := ciFailureFixture()
	out, err := TransferAlias(in, "notification > ci > ci-failure", "refactor")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Terms[2].Aliases) != 0 {
		t.Fatalf("loser ci-failure aliases: %v", out.Terms[2].Aliases)
	}
	if len(out.Terms[3].Aliases) != 1 || out.Terms[3].Aliases[0] != "notification > ci > ci-failure" {
		t.Fatalf("winner refactor aliases: %v", out.Terms[3].Aliases)
	}
}

func TestTransferAlias_idempotentWhenAlreadyOnWinner(t *testing.T) {
	in := Vocabulary{
		ID: "demo",
		Terms: []Term{
			{ID: "a", Label: "A", Description: "d"},
			{ID: "b", Label: "B", Parent: "a", Description: "d", Aliases: []string{"kind-path"}},
		},
	}
	out1, err := TransferAlias(in, "kind-path", "b")
	if err != nil {
		t.Fatal(err)
	}
	out2, err := TransferAlias(out1, "kind-path", "b")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out1.Terms[1].Aliases, out2.Terms[1].Aliases) {
		t.Fatalf("idempotent: %v vs %v", out1.Terms[1].Aliases, out2.Terms[1].Aliases)
	}
}

func TestTransferAlias_doesNotMutateInput(t *testing.T) {
	in := ciFailureFixture()
	before := append([]string{}, in.Terms[2].Aliases...)
	out, err := TransferAlias(in, "notification > ci > ci-failure", "refactor")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in.Terms[2].Aliases, before) {
		t.Fatal("input mutated")
	}
	if reflect.DeepEqual(out.Terms[2].Aliases, before) {
		t.Fatal("expected transfer to change loser")
	}
}

func TestTransferAlias_missingWinner(t *testing.T) {
	in := ciFailureFixture()
	_, err := TransferAlias(in, "notification > ci > ci-failure", "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if CodeOf(err) != CodeInvalidVocab {
		t.Fatalf("code %v", CodeOf(err))
	}
}

func TestBuildCatalog_afterTransferAlias(t *testing.T) {
	in := ciFailureFixture()
	out, err := TransferAlias(in, "notification > ci > ci-failure", "refactor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildCatalog(out); err != nil {
		t.Fatal(err)
	}
}
