package tags

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// Deprecation maps a legacy tag to the preferred replacement.
type Deprecation struct {
	From string `toml:"from"`
	To   string `toml:"to"`
}

// Dropped marks a tag id removed from the ledger inventory.
type Dropped struct {
	ID string `toml:"id"`
}

// Kept marks a leftover token judged as its own mail-form tag (hop2 keep-own).
type Kept struct {
	ID string `toml:"id"`
}

// Skipped marks a leftover token deferred from Judge (below threshold or merge failure).
// Operators may delete a row and run tag curation again to re-judge.
type Skipped struct {
	ID string `toml:"id"`
}

// Maps holds deprecations, dropped, kept, and skipped tag ids from tag-deprecations.toml.
type Maps struct {
	Deprecated []Deprecation
	Dropped    []string
	Kept       []string
	Skipped    []string
}

type deprecationsFile struct {
	Deprecated []Deprecation `toml:"deprecated"`
	Dropped    []Dropped     `toml:"dropped"`
	Kept       []Kept        `toml:"kept"`
	Skipped    []Skipped     `toml:"skipped"`
}

const saveMapsHeader = `# Tag deprecations. Edit [[deprecated]], [[dropped]], [[kept]], and [[skipped]] entries.
# Generated sections are rewritten by tag curation; operator comments are not preserved.
`

// LoadDeprecations reads TOML deprecations. Missing file returns nil slice, no error.
func LoadDeprecations(path string) ([]Deprecation, error) {
	m, err := LoadMaps(path)
	if err != nil {
		return nil, err
	}
	return m.Deprecated, nil
}

// LoadDropped reads TOML dropped ids. Missing file returns nil slice, no error.
func LoadDropped(path string) ([]string, error) {
	m, err := LoadMaps(path)
	if err != nil {
		return nil, err
	}
	return m.Dropped, nil
}

// LoadMaps reads all four map sections from the deprecations TOML path.
func LoadMaps(path string) (Maps, error) {
	const op = "tags.LoadMaps"
	path = strings.TrimSpace(path)
	if path == "" {
		return Maps{}, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Maps{}, nil
		}
		return Maps{}, newErr(op, CodeFailed, "read deprecations", err)
	}
	var f deprecationsFile
	if _, err := toml.Decode(string(b), &f); err != nil {
		return Maps{}, newErr(op, CodeFailed, "decode deprecations", err)
	}
	deps, err := parseDeprecations(f.Deprecated)
	if err != nil {
		return Maps{}, err
	}
	dropped, err := parseDroppedRows(f.Dropped)
	if err != nil {
		return Maps{}, err
	}
	kept, err := parseKeptRows(f.Kept)
	if err != nil {
		return Maps{}, err
	}
	skipped, err := parseSkippedRows(f.Skipped)
	if err != nil {
		return Maps{}, err
	}
	return Maps{
		Deprecated: deps,
		Dropped:    dropped,
		Kept:       kept,
		Skipped:    skipped,
	}, nil
}

func parseDeprecations(rows []Deprecation) ([]Deprecation, error) {
	const op = "tags.parseDeprecations"
	seenFrom := make(map[string]struct{})
	out := make([]Deprecation, 0, len(rows))
	for i, row := range rows {
		from := normalizeToken(row.From)
		to := normalizeToken(row.To)
		if from == "" || to == "" {
			return nil, newErr(op, CodeInvalid, fmt.Sprintf("deprecation from and to must be non-empty (row %d)", i+1), nil)
		}
		if err := ValidateToken(to, nil); err != nil {
			return nil, newErr(op, CodeInvalid, "deprecation to must be valid kebab tag", err)
		}
		if _, dup := seenFrom[from]; dup {
			return nil, newErr(op, CodeInvalid, "duplicate deprecated from", nil)
		}
		seenFrom[from] = struct{}{}
		out = append(out, Deprecation{From: from, To: to})
	}
	return out, nil
}

func parseDroppedRows(rows []Dropped) ([]string, error) {
	const op = "tags.parseDroppedRows"
	seen := make(map[string]struct{})
	out := make([]string, 0, len(rows))
	for i, row := range rows {
		id := normalizeToken(row.ID)
		if id == "" {
			return nil, newErr(op, CodeInvalid, fmt.Sprintf("dropped id must be non-empty (row %d)", i+1), nil)
		}
		if !kebabTagRE.MatchString(id) || isStopword(id) {
			return nil, newErr(op, CodeInvalid, "dropped id must be valid kebab tag", nil)
		}
		if _, dup := seen[id]; dup {
			return nil, newErr(op, CodeInvalid, "duplicate dropped id", nil)
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func parseKeptRows(rows []Kept) ([]string, error) {
	return parseSimpleIDRows(len(rows), "kept", func(i int) string { return rows[i].ID })
}

func parseSkippedRows(rows []Skipped) ([]string, error) {
	return parseSimpleIDRows(len(rows), "skipped", func(i int) string { return rows[i].ID })
}

func parseSimpleIDRows(n int, section string, idAt func(int) string) ([]string, error) {
	const op = "tags.parseSimpleIDRows"
	seen := make(map[string]struct{})
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		id := normalizeToken(idAt(i))
		if id == "" {
			return nil, newErr(op, CodeInvalid, fmt.Sprintf("%s id must be non-empty (row %d)", section, i+1), nil)
		}
		if !kebabTagRE.MatchString(id) || isStopword(id) {
			return nil, newErr(op, CodeInvalid, section+" id must be valid kebab tag", nil)
		}
		if _, dup := seen[id]; dup {
			return nil, newErr(op, CodeInvalid, "duplicate "+section+" id", nil)
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

// SaveMaps atomically writes all map sections to the TOML path.
func SaveMaps(path string, m Maps) error {
	const op = "tags.SaveMaps"
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return newErr(op, CodeInvalid, "empty deprecations path", nil)
	}
	normalized, err := normalizeMaps(m)
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString(saveMapsHeader)
	b.WriteString("\n")
	for _, d := range normalized.Deprecated {
		fmt.Fprintf(&b, "[[deprecated]]\nfrom = \"%s\"\nto = \"%s\"\n\n", d.From, d.To)
	}
	for _, id := range normalized.Dropped {
		fmt.Fprintf(&b, "[[dropped]]\nid = \"%s\"\n\n", id)
	}
	for _, id := range normalized.Kept {
		fmt.Fprintf(&b, "[[kept]]\nid = \"%s\"\n\n", id)
	}
	for _, id := range normalized.Skipped {
		fmt.Fprintf(&b, "[[skipped]]\nid = \"%s\"\n\n", id)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return newErr(op, CodeFailed, "mkdir deprecations dir", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return newErr(op, CodeFailed, "write deprecations tmp", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return newErr(op, CodeFailed, "rename deprecations", err)
	}
	return nil
}

func normalizeMaps(m Maps) (Maps, error) {
	deps, err := parseDeprecations(m.Deprecated)
	if err != nil {
		return Maps{}, err
	}
	dropped, err := parseIDList(m.Dropped, "dropped")
	if err != nil {
		return Maps{}, err
	}
	kept, err := parseIDList(m.Kept, "kept")
	if err != nil {
		return Maps{}, err
	}
	skipped, err := parseIDList(m.Skipped, "skipped")
	if err != nil {
		return Maps{}, err
	}
	dropSet := droppedSet(dropped)
	depFrom := make(map[string]struct{}, len(deps))
	for _, d := range deps {
		depFrom[d.From] = struct{}{}
	}
	kept = filterIDList(kept, dropSet, depFrom)
	skipped = filterIDList(skipped, dropSet, depFrom)
	slices.Sort(kept)
	slices.Sort(skipped)
	return Maps{
		Deprecated: deps,
		Dropped:    dropped,
		Kept:       kept,
		Skipped:    skipped,
	}, nil
}

func parseIDList(ids []string, section string) ([]string, error) {
	const op = "tags.parseIDList"
	seen := make(map[string]struct{})
	out := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := normalizeToken(raw)
		if id == "" {
			return nil, newErr(op, CodeInvalid, section+" id must be non-empty", nil)
		}
		if !kebabTagRE.MatchString(id) || isStopword(id) {
			return nil, newErr(op, CodeInvalid, section+" id must be valid kebab tag", nil)
		}
		if _, dup := seen[id]; dup {
			return nil, newErr(op, CodeInvalid, "duplicate "+section+" id", nil)
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func filterIDList(ids []string, dropSet map[string]struct{}, depFrom map[string]struct{}) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, drop := dropSet[id]; drop {
			continue
		}
		if _, dep := depFrom[id]; dep {
			continue
		}
		out = append(out, id)
	}
	return out
}

func deprecationMap(deps []Deprecation) map[string]string {
	out := make(map[string]string, len(deps))
	for _, d := range deps {
		out[d.From] = d.To
	}
	return out
}

func droppedSet(ids []string) map[string]struct{} {
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = normalizeToken(id)
		if id != "" {
			out[id] = struct{}{}
		}
	}
	return out
}

func idSet(ids []string) map[string]struct{} {
	return droppedSet(ids)
}

func appendUniqueID(ids []string, set map[string]struct{}, id string) ([]string, map[string]struct{}) {
	id = normalizeToken(id)
	if id == "" {
		return ids, set
	}
	if set == nil {
		set = map[string]struct{}{}
	}
	if _, ok := set[id]; ok {
		return ids, set
	}
	set[id] = struct{}{}
	return append(ids, id), set
}
