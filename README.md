# taxonomy

Portable hierarchical classification catalogs and a harness that seats a **Judge** (closed choice) and **Author** (generative draft).

Classification must make sense in the **game in play**: merged catalog, non-empty term descriptions, host-provided `WorldContext`, and a `context.Context` deadline on `Operate`.

## Module

```bash
go get github.com/behaviorengineering/taxonomy@v0.2.0
```

## Packages

- `pkg/catalog`: parse/merge/build YAML vocabularies, resolve leaf assignments, tree APIs
- `pkg/harness`: `CreateHarness`, `Operate` (n-ary tree walk), `Apply` with mandatory Judge + Author seats. Seat failures keep the cause: `CodeJudge`, `CodeAuthor`, `CodeGate` (not generic `CodeConfig`).

## Naming recommendations (host-driven)

- Vocabulary id should name what you classify and what the labels are.
- Prefer ids like `timeline-event-patterns`, `ticket-priority`.
- Avoid vague ids: `patterns`, `concepts`, `tags`, `types`, `taxonomy`.
- Filename should match `vocabulary.id` + `.yaml`.
- Term ids stay stable kebab-case.

## Agents

See [AGENTS.md](AGENTS.md) and [ai-copilots/](ai-copilots/).
