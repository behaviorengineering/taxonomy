package harness

import (
	"context"
	"strings"
	"sync"
)

const defaultEssenceParallel = 8

// PrefetchEssence fills Op.Essence for attach ops using Essencer and optional EssenceCache.
func (h *Harness) PrefetchEssence(ctx context.Context, ops []Op) error {
	const op = "PrefetchEssence"
	if h == nil {
		return newErr(op, CodeConfig, "harness is nil", nil)
	}
	if ctx == nil {
		return newErr(op, CodeConfig, "context required", nil)
	}
	if err := requireDeadline(ctx); err != nil {
		return err
	}
	if h.essencer == nil {
		return nil
	}
	par := h.essenceParallel
	for start := 0; start < len(ops); start += par {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := start + par
		if end > len(ops) {
			end = len(ops)
		}
		type work struct {
			idx int
		}
		var batch []work
		for i := start; i < end; i++ {
			if ops[i].Essence != nil {
				continue
			}
			key := strings.TrimSpace(ops[i].EssenceKey)
			if key == "" {
				continue
			}
			if h.essenceCache != nil {
				cached, hit, getErr := h.essenceCache.Get(ctx, key)
				if getErr != nil {
					return newErr(op, CodeEssence, "cache get", getErr)
				}
				if hit {
					stored := cached
					ops[i].Essence = &stored
					continue
				}
			}
			batch = append(batch, work{idx: i})
		}
		if len(batch) == 0 {
			continue
		}
		results := make([]struct {
			out EssenceOut
			err error
		}, len(batch))
		var wg sync.WaitGroup
		sem := make(chan struct{}, par)
		for j, w := range batch {
			wg.Add(1)
			go func(j int, idx int) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					results[j].err = ctx.Err()
					return
				}
				if err := ctx.Err(); err != nil {
					results[j].err = err
					return
				}
				got, err := h.essencer.Essence(ctx, EssenceIn{
					WorldContext: ops[idx].WorldContext,
					Text:         ops[idx].Text,
				})
				results[j].out = got
				results[j].err = err
			}(j, w.idx)
		}
		wg.Wait()
		for j, w := range batch {
			if results[j].err != nil {
				continue
			}
			stored := results[j].out
			ops[w.idx].Essence = &stored
			if h.essenceCache == nil {
				continue
			}
			key := strings.TrimSpace(ops[w.idx].EssenceKey)
			if putErr := h.essenceCache.Put(ctx, key, stored); putErr != nil {
				return newErr(op, CodeEssence, "cache put", putErr)
			}
		}
	}
	return nil
}
