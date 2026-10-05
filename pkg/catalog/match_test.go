package catalog

import "testing"

func TestMatchFields_firstFitOrder(t *testing.T) {
	cat, err := BuildCatalog(Vocabulary{
		ID: "v",
		Terms: []Term{
			{ID: "first", Label: "First", Patterns: []FieldPattern{{Field: "title", Re: ".*"}}},
			{ID: "second", Label: "Second", Patterns: []FieldPattern{{Field: "title", Re: ".*"}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rt, ok, err := cat.MatchFields(map[string]string{"title": "hello"})
	if err != nil || !ok || rt.ID != "first" {
		t.Fatalf("match: ok=%v id=%s err=%v", ok, rt.ID, err)
	}
}

func TestMatchFields_fieldIsolation(t *testing.T) {
	cat, err := BuildCatalog(Vocabulary{
		ID: "v",
		Terms: []Term{{
			ID: "only-a", Label: "A",
			Patterns: []FieldPattern{{Field: "a", Re: "^x$"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, ok, _ := cat.MatchFields(map[string]string{"b": "x"})
	if ok {
		t.Fatal("pattern on a should not match b")
	}
	rt, ok, _ := cat.MatchFields(map[string]string{"a": "x"})
	if !ok || rt.ID != "only-a" {
		t.Fatalf("expected match on a: ok=%v", ok)
	}
}

func TestMatchFields_aliasViaField(t *testing.T) {
	cat, err := BuildCatalog(Vocabulary{
		ID: "v",
		Terms: []Term{{
			ID: "dual-track", Label: "Dual", Aliases: []string{"dual_track"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	rt, ok, _ := cat.MatchFields(map[string]string{"source": "dual_track"})
	if !ok || rt.ID != "dual-track" {
		t.Fatalf("alias match: ok=%v id=%s", ok, rt.ID)
	}
}

func TestMatchFields_skipsDeprecated(t *testing.T) {
	cat, err := BuildCatalog(Vocabulary{
		ID: "v",
		Terms: []Term{
			{ID: "old", Label: "Old", Status: StatusDeprecated, Patterns: []FieldPattern{{Field: "source", Re: ".*"}}},
			{ID: "new", Label: "New", Patterns: []FieldPattern{{Field: "source", Re: "^same$"}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rt, ok, _ := cat.MatchFields(map[string]string{"source": "same"})
	if !ok || rt.ID != "new" {
		t.Fatalf("deprecated skipped: ok=%v id=%s", ok, rt.ID)
	}
}

func TestMatchFields_nilCatalog(t *testing.T) {
	var cat *Catalog
	_, ok, err := cat.MatchFields(map[string]string{"a": "b"})
	if err != nil || ok {
		t.Fatalf("nil catalog: ok=%v err=%v", ok, err)
	}
}

func TestMatchFields_nilFields(t *testing.T) {
	cat, err := BuildCatalog(Vocabulary{
		ID:    "v",
		Terms: []Term{{ID: "t", Label: "T", Patterns: []FieldPattern{{Field: "a", Re: ".*"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, ok, err := cat.MatchFields(nil)
	if err != nil || ok {
		t.Fatalf("nil fields: ok=%v err=%v", ok, err)
	}
}

func TestMatchFields_preferLeafOnDeprecatedBy(t *testing.T) {
	cat, err := BuildCatalog(Vocabulary{
		ID: "v",
		Terms: []Term{
			{
				ID: "redirect", Label: "Redirect", DeprecatedBy: "target",
				Patterns: []FieldPattern{{Field: "source", Re: "^hit$"}},
			},
			{ID: "target", Label: "Target"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rt, ok, err := cat.MatchFields(map[string]string{"source": "hit"})
	if err != nil || !ok || rt.ID != "target" {
		t.Fatalf("prefer leaf: ok=%v id=%s err=%v", ok, rt.ID, err)
	}
}
