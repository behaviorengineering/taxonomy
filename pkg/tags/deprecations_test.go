package tags

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMaps_fourSectionsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dep.toml")
	in := Maps{
		Deprecated: []Deprecation{{From: "promo", To: "promotional"}},
		Dropped:    []string{"notification"},
		Kept:       []string{"legal-notice"},
		Skipped:    []string{"agenda"},
	}
	if err := SaveMaps(path, in); err != nil {
		t.Fatal(err)
	}
	got, err := LoadMaps(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Deprecated) != 1 || got.Deprecated[0].From != "promo" {
		t.Fatalf("deprecated %v", got.Deprecated)
	}
	if len(got.Dropped) != 1 || got.Dropped[0] != "notification" {
		t.Fatalf("dropped %v", got.Dropped)
	}
	if len(got.Kept) != 1 || got.Kept[0] != "legal-notice" {
		t.Fatalf("kept %v", got.Kept)
	}
	if len(got.Skipped) != 1 || got.Skipped[0] != "agenda" {
		t.Fatalf("skipped %v", got.Skipped)
	}
}

func TestLoadMaps_missingFileEmpty(t *testing.T) {
	got, err := LoadMaps(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Deprecated) != 0 || len(got.Dropped) != 0 || len(got.Kept) != 0 || len(got.Skipped) != 0 {
		t.Fatalf("expected empty maps got %v", got)
	}
}

func TestSaveMaps_dropWinsOverKept(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dep.toml")
	if err := SaveMaps(path, Maps{
		Dropped: []string{"agenda"},
		Kept:    []string{"agenda"},
	}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if !strings.Contains(body, "[[dropped]]") {
		t.Fatalf("missing dropped: %s", body)
	}
	if strings.Contains(body, "[[kept]]\nid = \"agenda\"") {
		t.Fatalf("kept should be stripped when dropped: %s", body)
	}
}
