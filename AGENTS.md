# Agents

Portable taxonomy library for hierarchical vocabularies and the Operate harness.

## Packages

- `pkg/catalog`: YAML vocabularies, `BuildCatalog`, `MatchFields`, `LearnExact`, `EnsurePath`, `Breadcrumb`
- `pkg/harness`: `CreateHarness`, `Operate` (Walk or Attach), `Apply`

## Operate strategies

| Strategy | Required seats |
|----------|----------------|
| `walk` (default) | Judge, Author |
| `attach` | Judge, Author, Embedder, Essencer |

Load `ai-copilots/skills/taxonomy-ops/SKILL.md` before changing Operate, Apply, or catalog path helpers.

## Release

Tag minors manually (`v0.4.0` for Attach). Consumers pin `go.mod` and submodule gitlinks together.
