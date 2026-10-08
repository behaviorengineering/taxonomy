package catalog

import "testing"

func TestEnsurePath_mintOnCollision(t *testing.T) {
	vocab := Vocabulary{
		ID: "k",
		Terms: []Term{
			{ID: "software-release", Label: "SR", Parent: "other", Description: "x", Status: StatusActive},
			{ID: "other", Label: "Other", Description: "x", Status: StatusActive},
		},
	}
	out, leaf, err := EnsurePath(vocab, EnsurePathInput{
		Segments:    []string{"notification", "software-release", "changelog"},
		UnderParent: "",
		About:       "about text",
		Shape:       "shape text",
	})
	if err != nil {
		t.Fatal(err)
	}
	if leaf == "software-release" {
		t.Fatalf("expected minted leaf, got existing id %q", leaf)
	}
	found := false
	for _, term := range out.Terms {
		if term.ID == leaf {
			found = true
		}
	}
	if !found {
		t.Fatalf("leaf %q not in vocab", leaf)
	}
}

func TestParseKindSegments(t *testing.T) {
	segs := ParseKindSegments("a > b > c")
	if len(segs) != 3 || segs[0] != "a" {
		t.Fatalf("%v", segs)
	}
}

func TestBreadcrumb(t *testing.T) {
	cat, err := BuildCatalog(Vocabulary{
		ID: "k",
		Terms: []Term{
			{ID: "a", Label: "A", Description: "d"},
			{ID: "b", Label: "B", Parent: "a", Description: "d"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if Breadcrumb(cat, "b") != "a > b" {
		t.Fatalf("got %q", Breadcrumb(cat, "b"))
	}
}
