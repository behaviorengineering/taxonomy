package catalog

import (
	"strings"
	"testing"
)

func TestLearnExact_appendsQuoted(t *testing.T) {
	vocab := Vocabulary{
		ID:    "v",
		Terms: []Term{{ID: "t1", Label: "T1"}},
	}
	out, err := LearnExact(vocab, "t1", map[string]string{"source": "alerts@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Terms[0].Patterns) != 1 {
		t.Fatalf("patterns: %+v", out.Terms[0].Patterns)
	}
	p := out.Terms[0].Patterns[0]
	if p.Field != "source" || p.Re != "^alerts@example\\.com$" {
		t.Fatalf("pattern: %+v", p)
	}
	cat, err := BuildCatalog(out)
	if err != nil {
		t.Fatal(err)
	}
	rt, ok, _ := cat.MatchFields(map[string]string{"source": "alerts@example.com"})
	if !ok || rt.ID != "t1" {
		t.Fatalf("learned match: ok=%v", ok)
	}
}

func TestLearnExact_noDuplicate(t *testing.T) {
	vocab := Vocabulary{
		ID:    "v",
		Terms: []Term{{ID: "t1", Label: "T1"}},
	}
	fields := map[string]string{"source": "x"}
	v1, err := LearnExact(vocab, "t1", fields)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := LearnExact(v1, "t1", fields)
	if err != nil {
		t.Fatal(err)
	}
	if len(v2.Terms[0].Patterns) != 1 {
		t.Fatalf("expected one pattern: %+v", v2.Terms[0].Patterns)
	}
}

func TestLearnExact_unknownTerm(t *testing.T) {
	_, err := LearnExact(Vocabulary{ID: "v", Terms: []Term{{ID: "a", Label: "A"}}}, "missing", map[string]string{"source": "x"})
	if err == nil || CodeOf(err) != CodeInvalidVocab {
		t.Fatalf("expected unknown term: %v", err)
	}
}

func TestLearnExact_emptyValueSkipped(t *testing.T) {
	vocab := Vocabulary{ID: "v", Terms: []Term{{ID: "t1", Label: "T1"}}}
	out, err := LearnExact(vocab, "t1", map[string]string{"source": "  "})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Terms[0].Patterns) != 0 {
		t.Fatal("empty value should not add pattern")
	}
}

func TestParseSaveRoundTrip_patternsMapsTo(t *testing.T) {
	raw := []byte(`vocabulary:
  id: demo
  label: Demo
  terms:
    - id: item
      label: Item
      maps_to: other-vocab-term
      patterns:
        - field: source
          re: ^foo$
        - field: title
          re: ^Bar$
`)
	v, err := ParseYAML(raw)
	if err != nil {
		t.Fatal(err)
	}
	out, err := SaveYAML(v)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "maps_to") || !strings.Contains(string(out), "patterns") {
		t.Fatalf("yaml missing fields: %s", out)
	}
	v2, err := ParseYAML(out)
	if err != nil {
		t.Fatal(err)
	}
	term := v2.Terms[0]
	if term.MapsTo != "other-vocab-term" || len(term.Patterns) != 2 {
		t.Fatalf("round trip: %+v", term)
	}
}
