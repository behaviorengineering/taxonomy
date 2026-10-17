package harness

import "context"

// EssenceCache stores parsed essence by opaque key for skip-on-rerun.
type EssenceCache interface {
	Get(ctx context.Context, key string) (EssenceOut, bool, error)
	Put(ctx context.Context, key string, out EssenceOut) error
}
