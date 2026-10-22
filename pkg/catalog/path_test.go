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

func TestEnsurePath_idCollidesWithAlias(t *testing.T) {
	vocab := Vocabulary{
		ID: "demo",
		Terms: []Term{
			{ID: "root", Label: "R", Description: "d"},
			{ID: "other", Label: "O", Parent: "root", Description: "d", Aliases: []string{"mint-me"}},
		},
	}
	_, _, err := EnsurePath(vocab, EnsurePathInput{
		Segments:    []string{"mint-me"},
		UnderParent: "root",
		About:       "about text",
	})
	if err == nil {
		t.Fatal("expected alias collision")
	}
	if CodeOf(err) != CodeAliasCollision {
		t.Fatalf("code %v", CodeOf(err))
	}
}

func TestParseKindSegments(t *testing.T) {
	segs := ParseKindSegments("a > b > c")
	if len(segs) != 3 || segs[0] != "a" {
		t.Fatalf("%v", segs)
	}
}

func TestEnsurePath_reusesMintedSegment(t *testing.T) {
	vocab := Vocabulary{
		ID: "demo",
		Terms: []Term{
			{ID: "notification", Label: "N", Segment: "notification", Description: "d"},
			{ID: "notification-ci", Label: "CI", Parent: "notification", Segment: "ci", Description: "d"},
		},
	}
	in := EnsurePathInput{
		Segments:    []string{"notification", "ci", "run-failure"},
		UnderParent: "",
		About:       "about text",
		Shape:       "shape text",
	}
	out1, leaf1, err := EnsurePath(vocab, in)
	if err != nil {
		t.Fatal(err)
	}
	out2, leaf2, err := EnsurePath(out1, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(out2.Terms) != len(out1.Terms) {
		t.Fatalf("terms grew: %d -> %d", len(out1.Terms), len(out2.Terms))
	}
	if leaf1 != leaf2 {
		t.Fatalf("leaves %q vs %q", leaf1, leaf2)
	}
	for _, term := range out2.Terms {
		if term.ID == "notification-ci-2" {
			t.Fatalf("minted duplicate sibling id")
		}
		if term.ID == leaf2 && term.Segment != "run-failure" {
			t.Fatalf("mint segment %q", term.Segment)
		}
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
