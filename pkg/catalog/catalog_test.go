package catalog

import "testing"

func TestBuildCatalog_hierarchyAndLeaves(t *testing.T) {
	cat, err := BuildCatalog(Vocabulary{
		ID:    "patterns",
		Label: "Patterns",
		Terms: []Term{
			{ID: "process", Label: "Process"},
			{ID: "process-failure", Label: "Process failure", Parent: "process"},
			{ID: "dual-track", Label: "Dual track", Parent: "process", Aliases: []string{"dual_track"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	leaf, ok := cat.Lookup("dual_track")
	if !ok || leaf.ID != "dual-track" {
		t.Fatalf("alias resolve: %+v ok=%v", leaf, ok)
	}
	if !leaf.Leaf || leaf.Depth != 1 {
		t.Fatalf("leaf depth: leaf=%v depth=%d", leaf.Leaf, leaf.Depth)
	}
}

func TestBuildCatalog_cycle(t *testing.T) {
	_, err := BuildCatalog(Vocabulary{
		ID: "x",
		Terms: []Term{
			{ID: "a", Label: "A", Parent: "b"},
			{ID: "b", Label: "B", Parent: "a"},
		},
	})
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if CodeOf(err) != CodeCycle {
		t.Fatalf("code: %v", CodeOf(err))
	}
}

func TestMerge_collision(t *testing.T) {
	base := Vocabulary{ID: "v", Terms: []Term{{ID: "a", Label: "A"}}}
	overlay := Vocabulary{Terms: []Term{{ID: "a", Label: "A2"}}}
	_, err := Merge(base, overlay)
	if err == nil || CodeOf(err) != CodeMergeCollision {
		t.Fatalf("merge: %v", err)
	}
}

func TestParseSaveRoundTrip(t *testing.T) {
	raw := []byte(`vocabulary:
  id: demo
  label: Demo
  terms:
    - id: root
      label: Root
`)
	v, err := ParseYAML(raw)
	if err != nil {
		t.Fatal(err)
	}
	out, err := SaveYAML(v)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := ParseYAML(out)
	if err != nil {
		t.Fatal(err)
	}
	if v2.ID != "demo" || len(v2.Terms) != 1 {
		t.Fatalf("round trip: %+v", v2)
	}
}
