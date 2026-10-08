# taxonomy-ops

Operate hierarchical catalogs with **Walk** (Judge tree hop + Author on skip) or **Attach** (Essence, embeddings, alias / Walk reinforce / breadcrumb create).

## Walk (default)

- `CreateHarness` with `Strategy` empty or `walk`. `Embedder` and `Essencer` may be nil.
- `Operate` requires non-empty catalog roots and term descriptions.
- Judge sees `WorldContext` plus optional `Walk path: id > id` suffix each hop.

## Attach

- `Strategy: attach` requires `Embedder` and `Essencer`.
- `AttachMinCosine` default 0.80; `WalkReinforceMin` default 0.70; reinforce min must be `<=` attach min.
- Empty catalog is allowed. Existing terms still need descriptions when present.
- High cosine: alias draft, `DraftAccepted`, no gate Judge.
- Fuzzy band: Walk reinforce from longest KIND prefix; Judge text is formatted essence, not raw message text.
- Low cosine or no leaves: breadcrumb draft via `catalog.EnsurePath`; `Apply` with `DraftKindBreadcrumb`.

## Host integration

- Provide `context.Context` with deadline on every `Operate`.
- Persist vocabulary with `harness.Apply` when `DraftAccepted`.
- Cosine similarity is library-side; embedder returns vectors only.

## Release

Tag library releases as `v0.x.y`. Consumers pin `go.mod` and submodule gitlink together.
