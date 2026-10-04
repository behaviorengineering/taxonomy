# BOOTSTRAP: taxonomy ai-copilots

**Module path:** `github.com/behaviorengineering/taxonomy`

```bash
MOD="$(go list -m -f '{{.Dir}}' github.com/behaviorengineering/taxonomy)"
mkdir -p .cursor/skills
ln -snf "$MOD/ai-copilots/skills/taxonomy-ops" .cursor/skills/taxonomy-ops
test -f .cursor/skills/taxonomy-ops/SKILL.md
```
