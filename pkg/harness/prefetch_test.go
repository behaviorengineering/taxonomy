package harness

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type countingEssencer struct {
	calls atomic.Int32
	delay time.Duration
	out   EssenceOut
	err   error
}

func (c *countingEssencer) Essence(_ context.Context, _ EssenceIn) (EssenceOut, error) {
	c.calls.Add(1)
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	if c.err != nil {
		return EssenceOut{}, c.err
	}
	if c.out.Kind != "" {
		return c.out, nil
	}
	return sampleEssence(), nil
}

type memEssenceCache struct {
	mu       sync.Mutex
	data     map[string]EssenceOut
	getErr   error
	putErr   error
	putAfter chan struct{}
}

func (m *memEssenceCache) Get(_ context.Context, key string) (EssenceOut, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getErr != nil {
		return EssenceOut{}, false, m.getErr
	}
	if v, ok := m.data[key]; ok {
		return v, true, nil
	}
	return EssenceOut{}, false, nil
}

func (m *memEssenceCache) Put(_ context.Context, key string, out EssenceOut) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.putErr != nil {
		return m.putErr
	}
	if m.data == nil {
		m.data = map[string]EssenceOut{}
	}
	m.data[key] = out
	if m.putAfter != nil {
		m.putAfter <- struct{}{}
	}
	return nil
}

func attachPrefetchHarness(t *testing.T, ess Essencer, cache EssenceCache, par int) *Harness {
	h, err := CreateHarness(Config{
		Judge:           &fakeJudge{},
		Author:          &fakeAuthor{},
		Embedder:        &fakeEmbedder{},
		Essencer:        ess,
		EssenceCache:    cache,
		EssenceParallel: par,
		Strategy:        StrategyAttach,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestPrefetchEssence_cacheHitSkipsSecondEssenceCalls(t *testing.T) {
	ess := &countingEssencer{}
	cache := &memEssenceCache{}
	h := attachPrefetchHarness(t, ess, cache, 8)
	ctx := ctxWithDeadline(t)
	ops := []Op{
		{EssenceKey: "a", WorldContext: "w", Text: "one"},
		{EssenceKey: "b", WorldContext: "w", Text: "two"},
	}
	if err := h.PrefetchEssence(ctx, ops); err != nil {
		t.Fatal(err)
	}
	if ess.calls.Load() != 2 {
		t.Fatalf("first prefetch calls=%d", ess.calls.Load())
	}
	if ops[0].Essence == nil || ops[1].Essence == nil {
		t.Fatal("expected essence filled")
	}
	ops2 := []Op{
		{EssenceKey: "a", WorldContext: "w", Text: "one"},
		{EssenceKey: "b", WorldContext: "w", Text: "two"},
	}
	if err := h.PrefetchEssence(ctx, ops2); err != nil {
		t.Fatal(err)
	}
	if ess.calls.Load() != 2 {
		t.Fatalf("second prefetch calls=%d want 2", ess.calls.Load())
	}
}

func TestOperate_cacheHitSkipsEssencer(t *testing.T) {
	ess := &countingEssencer{}
	cache := &memEssenceCache{data: map[string]EssenceOut{"k": sampleEssence()}}
	h := attachPrefetchHarness(t, ess, cache, 8)
	cat := emptyKindCatalog(t)
	res, err := h.Operate(ctxWithDeadline(t), Op{
		WorldContext: "w",
		Text:         "body",
		Catalog:      cat,
		EssenceKey:   "k",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ess.calls.Load() != 0 {
		t.Fatalf("essencer calls=%d", ess.calls.Load())
	}
	if res.Essence == nil {
		t.Fatal("expected essence on result")
	}
}

func TestPrefetchEssence_parallelOverlap(t *testing.T) {
	ess := &countingEssencer{delay: 50 * time.Millisecond}
	h := attachPrefetchHarness(t, ess, nil, 2)
	ctx := ctxWithDeadline(t)
	ops := []Op{
		{EssenceKey: "a", WorldContext: "w", Text: "1"},
		{EssenceKey: "b", WorldContext: "w", Text: "2"},
	}
	start := time.Now()
	if err := h.PrefetchEssence(ctx, ops); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 50*time.Millisecond {
		t.Fatal("expected parallel overlap")
	}
}

func TestPrefetchEssence_putAfterWorkersDone(t *testing.T) {
	ess := &countingEssencer{delay: 30 * time.Millisecond}
	putGate := make(chan struct{}, 2)
	cache := &memEssenceCache{putAfter: putGate}
	h := attachPrefetchHarness(t, ess, cache, 2)
	ctx := ctxWithDeadline(t)
	ops := []Op{
		{EssenceKey: "a", WorldContext: "w", Text: "1"},
		{EssenceKey: "b", WorldContext: "w", Text: "2"},
	}
	if err := h.PrefetchEssence(ctx, ops); err != nil {
		t.Fatal(err)
	}
	if len(cache.data) != 2 {
		t.Fatalf("puts=%d", len(cache.data))
	}
	close(putGate)
}

func TestPrefetchEssence_nilCacheStillFills(t *testing.T) {
	ess := &countingEssencer{}
	h := attachPrefetchHarness(t, ess, nil, 8)
	ops := []Op{{EssenceKey: "a", WorldContext: "w", Text: "x"}}
	if err := h.PrefetchEssence(ctxWithDeadline(t), ops); err != nil {
		t.Fatal(err)
	}
	if ops[0].Essence == nil {
		t.Fatal("expected in-memory fill")
	}
}

func TestPrefetchEssence_oneEssenceErrorContinues(t *testing.T) {
	ess := &countingEssencer{err: errors.New("boom")}
	ess2 := &countingEssencer{}
	// use essencer that fails on first call only
	mixed := &mixedEssencer{first: ess, rest: ess2}
	h := attachPrefetchHarness(t, mixed, &memEssenceCache{}, 1)
	ops := []Op{
		{EssenceKey: "bad", WorldContext: "w", Text: "1"},
		{EssenceKey: "ok", WorldContext: "w", Text: "2"},
	}
	if err := h.PrefetchEssence(ctxWithDeadline(t), ops); err != nil {
		t.Fatal(err)
	}
	if ops[0].Essence != nil {
		t.Fatal("bad op should stay nil")
	}
	if ops[1].Essence == nil {
		t.Fatal("ok op should be filled")
	}
}

type mixedEssencer struct {
	first *countingEssencer
	rest  *countingEssencer
	n     atomic.Int32
}

func (m *mixedEssencer) Essence(ctx context.Context, in EssenceIn) (EssenceOut, error) {
	if m.n.Add(1) == 1 {
		return m.first.Essence(ctx, in)
	}
	return m.rest.Essence(ctx, in)
}

func TestPrefetchEssence_canceledCtxStopsNewChunk(t *testing.T) {
	ess := &countingEssencer{delay: 200 * time.Millisecond}
	h := attachPrefetchHarness(t, ess, nil, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var ops []Op
	for i := 0; i < 4; i++ {
		ops = append(ops, Op{EssenceKey: "k" + string(rune('a'+i)), WorldContext: "w", Text: "t"})
	}
	cancel()
	err := h.PrefetchEssence(ctx, ops)
	if err == nil {
		t.Fatal("expected cancel error")
	}
}
