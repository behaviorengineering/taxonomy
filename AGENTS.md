# Agents

Portable taxonomy library for hierarchical vocabularies and the Operate harness.

## Packages

- `pkg/catalog`: YAML vocabularies, `BuildCatalog`, `Term.Segment` (semantic KIND segment vs storage `ID`), `RepairAliases`, `MatchFields`, `LearnExact`, `EnsurePath`, `Breadcrumb`
- `pkg/harness`: `CreateHarness`, `Operate` (Walk or Attach; Attach walks KIND by Segment before global cosine), `Apply` (alias uniqueness on `DraftKindAlias` / `DraftKindNewLeaf`)
- `pkg/tags`: flat tag vocabulary (`CreateCatalog`, `FilterStoredShape`, `CreateCurator` with injected `Objective`, `Deny`, and `Ledger`)

## Operate strategies

| Strategy | Required seats |
|----------|----------------|
| `walk` (default) | Judge, Author |
| `attach` | Judge, Author, Embedder, Essencer |

Load `ai-copilots/skills/taxonomy-ops/SKILL.md` before changing Operate, Apply, or catalog path helpers.

## Release

Tag minors manually (`v0.4.0` for Attach). Consumers pin `go.mod` and submodule gitlinks together.
