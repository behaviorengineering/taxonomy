package tags

import "testing"

func TestSplitTags_andJoin(t *testing.T) {
	got := SplitTags("marketing-pitch, promotional; time-critical\nfyi")
	want := []string{"marketing-pitch", "promotional", "time-critical", "fyi"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v", got)
		}
	}
	joined := JoinTags([]string{"b", "a", "b"})
	if joined != "a b" {
		t.Fatalf("join %q", joined)
	}
}

func TestValidateToken_rejectsGeometryAndPath(t *testing.T) {
	deny := denySet([]string{"rectangular"})
	for _, tok := range []string{"rectangular", "a > b", "Bad Case"} {
		if err := ValidateToken(tok, deny); err == nil {
			t.Fatalf("expected reject %q", tok)
		}
	}
	if err := ValidateToken("marketing-pitch", nil); err != nil {
		t.Fatal(err)
	}
}

func TestStopwords_rejectGlueWords(t *testing.T) {
	for _, tok := range []string{"a", "and", "with", "the", "for", "or", "is", "an"} {
		if !isStopword(tok) {
			t.Fatalf("expected stopword %q", tok)
		}
		if err := ValidateToken(tok, nil); err == nil {
			t.Fatalf("ValidateToken should reject %q", tok)
		}
		if IsValidStoredToken(tok, nil) {
			t.Fatalf("IsValidStoredToken should reject %q", tok)
		}
	}
	for _, tok := range []string{"fyi", "marketing-pitch", "personal", "receipt"} {
		if isStopword(tok) {
			t.Fatalf("expected not stopword %q", tok)
		}
		if err := ValidateToken(tok, nil); err != nil {
			t.Fatalf("ValidateToken %q: %v", tok, err)
		}
	}
}
