package catalog

import "testing"

func TestDeriveSegment_stripsParentPrefix(t *testing.T) {
	got := DeriveSegment("notification", "notification-ci")
	if got != "ci" {
		t.Fatalf("got %q", got)
	}
}

func TestDeriveSegment_keepsNumericRemainder(t *testing.T) {
	got := DeriveSegment("notification", "notification-ci-2")
	if got != "ci-2" {
		t.Fatalf("got %q", got)
	}
}

func TestDeriveSegment_idWithoutParentPrefix(t *testing.T) {
	got := DeriveSegment("notification-software-development-2", "ci")
	if got != "ci" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildCatalog_duplicateSiblingSegment(t *testing.T) {
	_, err := BuildCatalog(Vocabulary{
		ID: "demo",
		Terms: []Term{
			{ID: "root", Label: "R", Description: "d"},
			{ID: "a-one", Label: "A1", Parent: "root", Segment: "a", Description: "d"},
			{ID: "a-two", Label: "A2", Parent: "root", Segment: "a", Description: "d"},
		},
	})
	if err == nil || CodeOf(err) != CodeDuplicateSegment {
		t.Fatalf("err %v", err)
	}
}

func TestBuildCatalog_doesNotMutateInputSegment(t *testing.T) {
	vocab := Vocabulary{
		ID: "demo",
		Terms: []Term{
			{ID: "notification", Label: "N", Description: "d"},
			{ID: "notification-ci", Label: "CI", Parent: "notification", Description: "d"},
		},
	}
	_, err := BuildCatalog(vocab)
	if err != nil {
		t.Fatal(err)
	}
	if vocab.Terms[1].Segment != "" {
		t.Fatalf("input segment mutated: %q", vocab.Terms[1].Segment)
	}
}

func TestBreadcrumb_usesSegmentNotID(t *testing.T) {
	cat, err := BuildCatalog(Vocabulary{
		ID: "demo",
		Terms: []Term{
			{ID: "notification", Label: "N", Description: "d"},
			{ID: "notification-ci", Label: "CI", Parent: "notification", Description: "d"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := Breadcrumb(cat, "notification-ci")
	if got != "notification > ci" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildCatalog_invalidSegmentNotKebab(t *testing.T) {
	_, err := BuildCatalog(Vocabulary{
		ID: "demo",
		Terms: []Term{
			{ID: "root", Label: "R", Description: "d"},
			{ID: "bad", Label: "B", Parent: "root", Segment: "Not_Kebab", Description: "d"},
		},
	})
	if err == nil || CodeOf(err) != CodeInvalidVocab {
		t.Fatalf("err %v", err)
	}
}

func TestBuildCatalog_deprecatedSiblingSameSegmentAllowed(t *testing.T) {
	cat, err := BuildCatalog(Vocabulary{
		ID: "demo",
		Terms: []Term{
			{ID: "root", Label: "R", Description: "d"},
			{ID: "a-live", Label: "Live", Parent: "root", Segment: "a", Description: "d"},
			{ID: "a-old", Label: "Old", Parent: "root", Segment: "a", Status: StatusDeprecated, Description: "d"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cat.ByID["a-live"].Segment != "a" {
		t.Fatalf("segment %q", cat.ByID["a-live"].Segment)
	}
}
