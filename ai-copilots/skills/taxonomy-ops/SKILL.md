# taxonomy-ops

Operate hierarchical classification catalogs and the Judge + Author harness.

## MUST

- Build merged catalogs with `catalog.BuildCatalog` after `catalog.Merge` for overlays.
- Pack Judge options from **active children of the current catalog node** plus `skip` (tree walk). Every **active** term (branch or leaf) needs a non-empty description before `Operate`.
- Expect **multiple** `Judge.Decide` calls per `Operate` (one per hop). Author `Parents` is the skip node (one branch) or root branches when skip at the top.
- Require non-empty `WorldContext` from the host; the library MUST NOT invent case prose.
- Require `context.Context` with a deadline before `harness.Operate`.
- Inject both `Judge` and `Author` via `harness.CreateHarness`; nil seats MUST fail at create time.
- Map walk `Decide` failures to `CodeJudge`, `Draft` failures to `CodeAuthor`, and gate `Decide` failures to `CodeGate`, wrapping the seat error. Hosts unwrap that cause for provider-specific unavailable.
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
