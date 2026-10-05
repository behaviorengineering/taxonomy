package catalog

import "testing"

func TestBuildCatalog_patterns(t *testing.T) {
	cat, err := BuildCatalog(Vocabulary{
		ID:    "sources",
		Label: "Sources",
		Terms: []Term{{
			ID:    "alerts",
			Label: "Alerts",
			Patterns: []FieldPattern{
				{Field: "source", Re: "^alerts@example\\.com$"},
			},
			MapsTo: "upstream-alerts",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	rt := cat.ByID["alerts"]
	if len(rt.Patterns) != 1 || rt.MapsTo != "upstream-alerts" {
		t.Fatalf("patterns/maps_to: %+v", rt.Term)
	}
}

func TestBuildCatalog_invalidPattern(t *testing.T) {
	_, err := BuildCatalog(Vocabulary{
		ID: "x",
		Terms: []Term{{
			ID: "a", Label: "A",
			Patterns: []FieldPattern{{Field: "source", Re: "("}},
		}},
	})
	if err == nil || CodeOf(err) != CodeInvalidPattern {
		t.Fatalf("expected invalid pattern: %v", err)
	}
}

func TestBuildCatalog_emptyPatternField(t *testing.T) {
	_, err := BuildCatalog(Vocabulary{
		ID: "x",
		Terms: []Term{{
			ID: "a", Label: "A",
			Patterns: []FieldPattern{{Field: "", Re: "x"}},
		}},
	})
	if err == nil || CodeOf(err) != CodeInvalidPattern {
		t.Fatalf("expected invalid pattern: %v", err)
	}
}

func TestBuildCatalog_duplicatePattern(t *testing.T) {
	_, err := BuildCatalog(Vocabulary{
		ID: "x",
		Terms: []Term{{
			ID: "a", Label: "A",
			Patterns: []FieldPattern{
				{Field: "source", Re: "^a$"},
				{Field: "source", Re: "^a$"},
			},
		}},
	})
	if err == nil || CodeOf(err) != CodeInvalidPattern {
		t.Fatalf("expected duplicate pattern: %v", err)
	}
}
