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

type countJudge struct {
	stub    stubJudge
	decides int
}

func (c *countJudge) Decide(_ context.Context, in harness.DecideIn) (harness.DecideOut, error) {
	c.decides++
	return c.stub.Decide(context.Background(), in)
}

type countLedger struct {
	inner *memLedger
	listN int
}

func (c *countLedger) ListComplete(ctx context.Context) ([]Row, error) {
	c.listN++
	return c.inner.ListComplete(ctx)
}

func (c *countLedger) Rewrite(ctx context.Context, id, shape string) error {
	return c.inner.Rewrite(ctx, id, shape)
}

func TestCurator_keepOwnWritesKeptAndSecondRunSkipsDecide(t *testing.T) {
	ledger := &memLedger{rows: map[string]string{"h1": "custom-tag fyi"}}
	cat, dep := testCurateCatalog(t)
	judge := &countJudge{stub: stubJudge{
		hop1: map[string]harness.DecideOut{
			"custom-tag": {Choice: choiceKeep, Score: 0.9},
		},
		hop2: map[string]harness.DecideOut{
			"custom-tag": {Choice: choiceKeep, Score: 0.9},
		},
	}}
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
	_, err = cur.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dep)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "[[kept]]") || !strings.Contains(string(b), "custom-tag") {
		t.Fatalf("dep %q", b)
	}
	first := judge.decides
	cat2, err := CreateCatalog(cat.Config())
	if err != nil {
		t.Fatal(err)
	}
	cur2, err := CreateCurator(CuratorConfig{
		Ledger: ledger, Catalog: cat2, Objective: testObjective(), MinScore: 0.8, Judge: judge,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = cur2.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if judge.decides != first {
		t.Fatalf("expected no new Decide calls got %d then %d", first, judge.decides)
	}
}

func TestCurator_belowMinScoreWritesSkippedAndSecondRunSkipsDecide(t *testing.T) {
	ledger := &memLedger{rows: map[string]string{"h1": "agenda fyi"}}
	cat, dep := testCurateCatalog(t)
	judge := &countJudge{stub: stubJudge{
		hop1: map[string]harness.DecideOut{
			"agenda": {Choice: choiceDrop, Score: 0.2},
		},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cur, err := CreateCurator(CuratorConfig{
		Ledger: ledger, Catalog: cat, Objective: testObjective(), MinScore: 0.8, Judge: judge,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = cur.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dep)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "[[skipped]]") || !strings.Contains(string(b), "agenda") {
		t.Fatalf("dep %q", b)
	}
	first := judge.decides
	cat2, _ := CreateCatalog(cat.Config())
	cur2, _ := CreateCurator(CuratorConfig{
		Ledger: ledger, Catalog: cat2, Objective: testObjective(), MinScore: 0.8, Judge: judge,
	})
	_, err = cur2.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if judge.decides != first {
		t.Fatalf("decides %d want %d", judge.decides, first)
	}
}

func TestCurator_newLeftoverStillDecides(t *testing.T) {
	ledger := &memLedger{rows: map[string]string{"h1": "brand-new fyi"}}
	cat, _ := testCurateCatalog(t)
	judge := &countJudge{stub: stubJudge{
		hop1: map[string]harness.DecideOut{
			"brand-new": {Choice: choiceKeep, Score: 0.9},
		},
		hop2: map[string]harness.DecideOut{
			"brand-new": {Choice: choiceKeep, Score: 0.9},
		},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cur, err := CreateCurator(CuratorConfig{
		Ledger: ledger, Catalog: cat, Objective: testObjective(), MinScore: 0.8, Judge: judge,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = cur.Run(ctx)
	if err != nil || judge.decides == 0 {
		t.Fatalf("run err=%v decides=%d", err, judge.decides)
	}
}

func TestCurator_listCompleteOncePerStorePhase(t *testing.T) {
	inner := &memLedger{rows: map[string]string{"h1": "leftover-token fyi"}}
	ledger := &countLedger{inner: inner}
	cat, _ := testCurateCatalog(t)
	judge := &stubJudge{
		hop1: map[string]harness.DecideOut{
			"leftover-token": {Choice: choiceKeep, Score: 0.9},
		},
		hop2: map[string]harness.DecideOut{
			"leftover-token": {Choice: choiceKeep, Score: 0.9},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cur, err := CreateCurator(CuratorConfig{
		Ledger: ledger, Catalog: cat, Objective: testObjective(), MinScore: 0.8, Judge: judge,
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := cur.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.listN != 2 {
		t.Fatalf("list calls %d want 2", ledger.listN)
	}
	if res.RegisterPath == "" {
		t.Fatal("empty register path")
	}
	ledger.listN = 0
	curNoJudge, err := CreateCurator(CuratorConfig{Ledger: ledger, Catalog: cat, Objective: testObjective()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = curNoJudge.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.listN != 1 {
		t.Fatalf("judge-nil list calls %d want 1", ledger.listN)
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
