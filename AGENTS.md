# Agents

This module is a portable taxonomy catalog and harness. Humans read [README.md](README.md).

**Load:** [ai-copilots/skills/taxonomy-ops/SKILL.md](ai-copilots/skills/taxonomy-ops/SKILL.md)

Wire with [ai-copilots/BOOTSTRAP.md](ai-copilots/BOOTSTRAP.md).

```bash
go list -m -f '{{.Dir}}' github.com/behaviorengineering/taxonomy
```

## Package layout

- Public API lives under `pkg/catalog` and `pkg/harness`.
- MUST NOT import host paths, Polypus, Turno, or mint `cn_` concept ids.
- MUST enforce in-world coherence on `harness.Operate` (deadline, WorldContext, described leaves).
