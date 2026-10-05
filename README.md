# taxonomy

Portable hierarchical classification catalogs and a harness that seats a **Judge** (closed choice) and **Author** (generative draft).

Classification must make sense in the **game in play**: merged catalog, non-empty term descriptions, host-provided `WorldContext`, and a `context.Context` deadline on `Operate`.

## Module

```bash
go get github.com/behaviorengineering/taxonomy@v0.3.0
```

## Packages

- `pkg/catalog`: parse/merge/build YAML vocabularies, resolve leaf assignments, field-scoped `MatchFields`, `LearnExact` pattern learning, tree APIs

### Field patterns (self-evolving vocabularies)

```go
vocab, err := catalog.ParseYAML(raw)
cat, err := catalog.BuildCatalog(vocab)
rt, ok, err := cat.MatchFields(map[string]string{"source": "alerts@example.com", "title": "Digest"})
if !ok {
    // host classifier assigns termID, then:
    vocab, err = catalog.LearnExact(vocab, "alerts-example", map[string]string{"source": "alerts@example.com"})
    raw, err = catalog.SaveYAML(vocab)
}
```

Terms may declare `patterns` (per-field regex, compiled at build) and optional `maps_to` (opaque related id). Hosts choose field keys and normalize values before `LearnExact`.
- `pkg/harness`: `CreateHarness`, `Operate` (n-ary tree walk), `Apply` with mandatory Judge + Author seats. Seat failures keep the cause: `CodeJudge`, `CodeAuthor`, `CodeGate` (not generic `CodeConfig`).

## Naming recommendations (host-driven)

- Vocabulary id should name what you classify and what the labels are.
- Prefer ids like `timeline-event-patterns`, `ticket-priority`.
- Avoid vague ids: `patterns`, `concepts`, `tags`, `types`, `taxonomy`.
- Filename should match `vocabulary.id` + `.yaml`.
- Term ids stay stable kebab-case.

## Agents

See [AGENTS.md](AGENTS.md) and [ai-copilots/](ai-copilots/).
