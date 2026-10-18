package tags

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeprecationRemap(t *testing.T) {
	dir := t.TempDir()
	dep := filepath.Join(dir, "tag-deprecations.toml")
	if err := os.WriteFile(dep, []byte(`[[deprecated]]
from = "promo"
to = "promotional"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	seed := filepath.Join(dir, "seed.txt")
	if err := os.WriteFile(seed, []byte("promotional\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cat, err := CreateCatalog(Config{SeedPath: seed, DeprecationsPath: dep})
	if err != nil {
		t.Fatal(err)
	}
	out, err := cat.NormalizeField("promo")
	if err != nil || out != "promotional" {
		t.Fatalf("got %q err %v", out, err)
	}
}

func TestLoadDeprecations_duplicateFrom(t *testing.T) {
	dir := t.TempDir()
	dep := filepath.Join(dir, "d.toml")
	body := `[[deprecated]]
from = "a"
to = "b"
[[deprecated]]
from = "a"
to = "c"
`
	if err := os.WriteFile(dep, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDeprecations(dep); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestRenderRegister_golden(t *testing.T) {
	body := RenderRegister(map[string]int{"marketing-pitch": 3}, []string{"fyi"}, nil)
	if !strings.Contains(body, "marketing-pitch\t3") {
		t.Fatalf("%q", body)
	}
	if !strings.Contains(body, "fyi\t0") {
		t.Fatalf("seed at zero count: %q", body)
	}
}

func TestNormalizeField_stripsStopwords(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.txt")
	if err := os.WriteFile(seed, []byte("fyi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cat, err := CreateCatalog(Config{SeedPath: seed})
	if err != nil {
		t.Fatal(err)
	}
	out, err := cat.NormalizeField("a with fyi")
	if err != nil || out != "fyi" {
		t.Fatalf("got %q err %v", out, err)
	}
}

func TestFilterStoredShape(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.txt")
	if err := os.WriteFile(seed, []byte("fyi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cat, err := CreateCatalog(Config{
		SeedPath: seed,
		Deny:     []string{"rectangular"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := FilterStoredShape("a with rectangular notification fyi", cat)
	want := "fyi notification"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if FilterStoredShape("", cat) != "" {
		t.Fatal("empty allowed")
	}
}

func TestCountTagsFromShapes_skipsStopwords(t *testing.T) {
	counts := CountTagsFromShapes([]string{"a notification with fyi"})
	if counts["a"] != 0 || counts["with"] != 0 {
		t.Fatalf("counts %v", counts)
	}
	if counts["fyi"] != 1 || counts["notification"] != 1 {
		t.Fatalf("counts %v", counts)
	}
}

func TestSaveMaps_roundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dep.toml")
	if err := SaveMaps(path, Maps{
		Deprecated: []Deprecation{{From: "promo", To: "promotional"}},
		Dropped:    []string{"notification"},
	}); err != nil {
		t.Fatal(err)
	}
	deps, err := LoadDeprecations(path)
	if err != nil || len(deps) != 1 {
		t.Fatalf("deps %v err %v", deps, err)
	}
	dropped, err := LoadDropped(path)
	if err != nil || len(dropped) != 1 || dropped[0] != "notification" {
		t.Fatalf("dropped %v err %v", dropped, err)
	}
}

func TestSaveMaps_duplicateDropped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dep.toml")
	err := SaveMaps(path, Maps{Dropped: []string{"notification", "notification"}})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestEnsureFiles_noOverwriteRegister(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.txt")
	reg := filepath.Join(dir, "register.txt")
	dep := filepath.Join(dir, "dep.toml")
	if err := os.WriteFile(seed, []byte("fyi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reg, []byte("custom\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureFiles(reg, dep, seed); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(reg)
	if err != nil || string(b) != "custom\n" {
		t.Fatalf("register overwritten: %q err %v", b, err)
	}
}
