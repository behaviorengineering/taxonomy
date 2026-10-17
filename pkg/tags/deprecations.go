package tags

import (
	"fmt"
	"os"
	"path/filepath"
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

type deprecationsFile struct {
	Deprecated []Deprecation `toml:"deprecated"`
	Dropped    []Dropped     `toml:"dropped"`
}

const saveMapsHeader = `# Tag deprecations. Edit [[deprecated]] and [[dropped]] entries.
# Generated sections are rewritten by tag curation; operator comments are not preserved.
`

// LoadDeprecations reads TOML deprecations. Missing file returns nil slice, no error.
func LoadDeprecations(path string) ([]Deprecation, error) {
	deps, _, err := loadMaps(path)
	return deps, err
}

// LoadDropped reads TOML dropped ids. Missing file returns nil slice, no error.
func LoadDropped(path string) ([]string, error) {
	_, dropped, err := loadMaps(path)
	return dropped, err
}

func loadMaps(path string) ([]Deprecation, []string, error) {
	const op = "tags.loadMaps"
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, newErr(op, CodeFailed, "read deprecations", err)
	}
	var f deprecationsFile
	if _, err := toml.Decode(string(b), &f); err != nil {
		return nil, nil, newErr(op, CodeFailed, "decode deprecations", err)
	}
	seenFrom := make(map[string]struct{})
	for i, row := range f.Deprecated {
		from := normalizeToken(row.From)
		to := normalizeToken(row.To)
		if from == "" || to == "" {
			return nil, nil, newErr(op, CodeInvalid, fmt.Sprintf("deprecation from and to must be non-empty (row %d)", i+1), nil)
		}
		if err := ValidateToken(to, nil); err != nil {
			return nil, nil, newErr(op, CodeInvalid, "deprecation to must be valid kebab tag", err)
		}
		if _, dup := seenFrom[from]; dup {
			return nil, nil, newErr(op, CodeInvalid, "duplicate deprecated from", nil)
		}
		seenFrom[from] = struct{}{}
		f.Deprecated[i].From = from
		f.Deprecated[i].To = to
	}
	seenDrop := make(map[string]struct{})
	dropped := make([]string, 0, len(f.Dropped))
	for i, row := range f.Dropped {
		id := normalizeToken(row.ID)
		if id == "" {
			return nil, nil, newErr(op, CodeInvalid, fmt.Sprintf("dropped id must be non-empty (row %d)", i+1), nil)
		}
		if !kebabTagRE.MatchString(id) || isStopword(id) {
			return nil, nil, newErr(op, CodeInvalid, "dropped id must be valid kebab tag", nil)
		}
		if _, dup := seenDrop[id]; dup {
			return nil, nil, newErr(op, CodeInvalid, "duplicate dropped id", nil)
		}
		seenDrop[id] = struct{}{}
		dropped = append(dropped, id)
	}
	return f.Deprecated, dropped, nil
}

// SaveMaps atomically writes deprecations and dropped ids to the TOML path.
func SaveMaps(path string, deps []Deprecation, dropped []string) error {
	const op = "tags.SaveMaps"
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return newErr(op, CodeInvalid, "empty deprecations path", nil)
	}
	if _, _, err := validateMaps(deps, dropped); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString(saveMapsHeader)
	b.WriteString("\n")
	for _, d := range deps {
		fmt.Fprintf(&b, "[[deprecated]]\nfrom = \"%s\"\nto = \"%s\"\n\n", d.From, d.To)
	}
	for _, id := range dropped {
		fmt.Fprintf(&b, "[[dropped]]\nid = \"%s\"\n\n", id)
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

func validateMaps(deps []Deprecation, dropped []string) ([]Deprecation, []string, error) {
	const op = "tags.validateMaps"
	seenFrom := make(map[string]struct{})
	outDeps := make([]Deprecation, 0, len(deps))
	for i, row := range deps {
		from := normalizeToken(row.From)
		to := normalizeToken(row.To)
		if from == "" || to == "" {
			return nil, nil, newErr(op, CodeInvalid, fmt.Sprintf("deprecation from and to must be non-empty (row %d)", i+1), nil)
		}
		if err := ValidateToken(to, nil); err != nil {
			return nil, nil, newErr(op, CodeInvalid, "deprecation to must be valid kebab tag", err)
		}
		if _, dup := seenFrom[from]; dup {
			return nil, nil, newErr(op, CodeInvalid, "duplicate deprecated from", nil)
		}
		seenFrom[from] = struct{}{}
		outDeps = append(outDeps, Deprecation{From: from, To: to})
	}
	seenDrop := make(map[string]struct{})
	outDrop := make([]string, 0, len(dropped))
	for _, id := range dropped {
		id = normalizeToken(id)
		if id == "" {
			return nil, nil, newErr(op, CodeInvalid, "dropped id must be non-empty", nil)
		}
		if !kebabTagRE.MatchString(id) || isStopword(id) {
			return nil, nil, newErr(op, CodeInvalid, "dropped id must be valid kebab tag", nil)
		}
		if _, dup := seenDrop[id]; dup {
			return nil, nil, newErr(op, CodeInvalid, "duplicate dropped id", nil)
		}
		seenDrop[id] = struct{}{}
		outDrop = append(outDrop, id)
	}
	return outDeps, outDrop, nil
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
