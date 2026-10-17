package tags

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/harness"
)

type memLedger struct {
	rows map[string]string
}

func (m *memLedger) ListComplete(_ context.Context) ([]Row, error) {
	out := make([]Row, 0, len(m.rows))
	for id, shape := range m.rows {
		out = append(out, Row{ID: id, Shape: shape})
	}
	return out, nil
}

func (m *memLedger) Rewrite(_ context.Context, id, shape string) error {
	m.rows[id] = shape
	return nil
}

type stubJudge struct {
	hop1 map[string]harness.DecideOut
	hop2 map[string]harness.DecideOut
}

func (s *stubJudge) Decide(_ context.Context, in harness.DecideIn) (harness.DecideOut, error) {
	hop2 := len(in.Options) > 2
	for tok, dec := range s.hop1 {
		if !strings.Contains(in.Text, "Token: "+tok) {
			continue
		}
		if hop2 {
			if d, ok := s.hop2[tok]; ok {
				return d, nil
			}
			return harness.DecideOut{Choice: choiceKeep, Score: 0.9}, nil
		}
		return dec, nil
	}
	return harness.DecideOut{Choice: choiceKeep, Score: 0.5}, nil
}

func testObjective() Objective {
	return Objective{
		WorldContext: "test world",
		Purpose:      "keep purpose",
		NotPurpose:   "drop purpose",
		TokenHint:    "hint line",
		KeepOwn:      "keep own",
	}
}

func testCurateCatalog(t *testing.T) (*Catalog, string) {
	t.Helper()
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.txt")
	if err := os.WriteFile(seed, []byte("fyi\npromotional\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dep := filepath.Join(dir, "dep.toml")
	reg := filepath.Join(dir, "register.txt")
	cat, err := CreateCatalog(Config{
		RegisterPath:     reg,
		DeprecationsPath: dep,
		SeedPath:         seed,
		Deny:             []string{"rectangular"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return cat, dep
}

func TestCurator_deterministicUnify(t *testing.T) {
	ledger := &memLedger{rows: map[string]string{"h1": "a with rectangular notification fyi"}}
	cat, _ := testCurateCatalog(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cur, err := CreateCurator(CuratorConfig{
		Ledger:    ledger,
		Catalog:   cat,
		Objective: testObjective(),
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := cur.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Rewrote != 1 {
		t.Fatalf("rewrote %d", res.Rewrote)
	}
	if ledger.rows["h1"] != "fyi notification" {
		t.Fatalf("shape %q", ledger.rows["h1"])
	}
}

func TestCurator_judgeDropAndMerge(t *testing.T) {
	ledger := &memLedger{rows: map[string]string{"h1": "notification fyi promo"}}
	cat, dep := testCurateCatalog(t)
	judge := &stubJudge{
		hop1: map[string]harness.DecideOut{
			"notification": {Choice: choiceDrop, Score: 0.9},
			"promo":        {Choice: choiceKeep, Score: 0.9},
		},
		hop2: map[string]harness.DecideOut{
			"promo": {Choice: "promotional", Score: 0.9},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cur, err := CreateCurator(CuratorConfig{
		Ledger:    ledger,
		Catalog:   cat,
		Objective: testObjective(),
		MinScore:  0.8,
		Judge:     judge,
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := cur.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.rows["h1"] != "fyi promotional" {
		t.Fatalf("shape %q", ledger.rows["h1"])
	}
	if len(res.Dropped) != 1 || res.Dropped[0] != "notification" {
		t.Fatalf("dropped %v", res.Dropped)
	}
	b, err := os.ReadFile(dep)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "notification") {
		t.Fatalf("dep %q", b)
	}
}

func TestCreateCurator_requiresDeadline(t *testing.T) {
	ledger := &memLedger{rows: map[string]string{}}
	cat, _ := testCurateCatalog(t)
	cur, err := CreateCurator(CuratorConfig{Ledger: ledger, Catalog: cat, Objective: testObjective()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = cur.Run(context.Background())
	if CodeOf(err) != CodeNoDeadline {
		t.Fatalf("got %v", err)
	}
}
