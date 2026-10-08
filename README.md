# taxonomy

Portable hierarchical classification catalogs and a harness that seats a **Judge** (closed choice) and **Author** (generative draft).

Classification must make sense in the **game in play**: merged catalog, non-empty term descriptions, host-provided `WorldContext`, and a `context.Context` deadline on `Operate`.

## Module

```bash
go get github.com/behaviorengineering/taxonomy@v0.4.0
```

## Packages

- `pkg/catalog`: parse/merge/build YAML vocabularies, resolve leaf assignments, field-scoped `MatchFields`, `LearnExact` pattern learning, tree APIs
- `pkg/harness`: `CreateHarness`, `Operate` (**Walk** tree hop or **Attach** essence/embed), `Apply` with mandatory Judge + Author seats. Attach also needs `Embedder` + `Essencer`. Seat failures keep the cause: `CodeJudge`, `CodeAuthor`, `CodeGate`, `CodeEssence`, `CodeEmbed` (not generic `CodeConfig`).

### Strategies

| Strategy | Seats | Catalog |
|----------|-------|---------|
| `walk` (default) | Judge + Author | Non-empty roots, descriptions on active terms |
| `attach` | Judge + Author + Embedder + Essencer | May start empty; descriptions required when terms exist |

Attach compares embedding cosine of essence `KIND` to each assignable leaf breadcrumb. At or above `AttachMinCosine` (default 0.80): alias without gate. Between `WalkReinforceMin` (0.70) and attach min: optional Walk reinforce. Below: create breadcrumb path with `catalog.EnsurePath`.

### Field patterns (self-evolving vocabularies)

```go
vocab, err := catalog.ParseYAML(raw)
if err != nil { /* handle */ }
cat, err := catalog.BuildCatalog(vocab)
if err != nil { /* handle */ }
rt, ok, err := cat.MatchFields(map[string]string{"source": "alerts@example.com", "title": "Digest"})
if err != nil { /* handle */ }
if !ok {
    vocab, err = catalog.LearnExact(vocab, "alerts-example", map[string]string{"source": "alerts@example.com"})
    if err != nil { /* handle */ }
    raw, err = catalog.SaveYAML(vocab)
    if err != nil { /* handle */ }
}
```

Terms may declare `patterns` (per-field regex, compiled at build) and optional `maps_to` (opaque related id). Hosts choose field keys and normalize values before `LearnExact`.

**MatchFields host notes:**

- Term order in YAML is first-fit: put more specific terms before broad ones.
- Id and alias matching scans **every** field value (case-folded via `normID`), not a single designated key.
- A matching non-leaf term is returned unless `deprecated_by` redirects via `PreferLeaf`; use leaf terms or check `rt.Leaf` when you need leaves only.

## Naming recommendations (host-driven)

- Vocabulary id should name what you classify and what the labels are.
- Prefer ids like `timeline-event-patterns`, `ticket-priority`.
- Avoid vague ids: `patterns`, `concepts`, `tags`, `types`, `taxonomy`.
- Filename should match `vocabulary.id` + `.yaml`.
- Term ids stay stable kebab-case.

## Agents

See [AGENTS.md](AGENTS.md) and [ai-copilots/](ai-copilots/).
