# taxonomy-ops

Operate hierarchical classification catalogs and the Judge + Author harness.

## MUST

- Build merged catalogs with `catalog.BuildCatalog` after `catalog.Merge` for overlays.
- Pack Judge options only from the merged catalog with id, label, parent, and **non-empty** description on leaves.
- Require non-empty `WorldContext` from the host; the library MUST NOT invent case prose.
- Require `context.Context` with a deadline before `harness.Operate`.
- Inject both `Judge` and `Author` via `harness.CreateHarness`; nil seats MUST fail at create time.
- Persist vocabulary bytes with `catalog.SaveYAML` on host-owned paths only.

## MUST NOT

- Mint `cn_` ontology concept ids or Accept proposals from taxonomy operate.
- Treat taxonomy parentage as proof or graph edges.
- Classify against a generic dictionary or another flavour's catalog.
- Call `Operate` without both seats or without in-world context.

## CORRECT

```text
merged := catalog.Merge(pack, overlay)
cat, err := catalog.BuildCatalog(merged)
h, err := harness.CreateHarness(Config{Judge: j, Author: a})
res, err := h.Operate(ctx, harness.Op{WorldContext: brief, Text: body, Catalog: cat})
```

## PROHIBITED

```text
harness.Operate(context.Background(), Op{Catalog: cat})  // no deadline
Judge options built from bare ids without descriptions
```
