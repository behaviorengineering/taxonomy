package catalog

import "strings"

const (
	// StatusActive is a normal term.
	StatusActive = "active"
	// StatusDeprecated remains resolvable via aliases but should not be newly assigned.
	StatusDeprecated = "deprecated"
)

// FieldPattern is a field-scoped regex matcher on catalog terms.
type FieldPattern struct {
	Field string `json:"field" yaml:"field"`
	Re    string `json:"re" yaml:"re"`
}

// Term is one node in a vocabulary tree.
type Term struct {
	ID          string   `json:"id" yaml:"id"`
	Label       string   `json:"label" yaml:"label"`
	Parent      string   `json:"parent,omitempty" yaml:"parent,omitempty"`
	Aliases     []string `json:"aliases,omitempty" yaml:"aliases,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Status      string   `json:"status,omitempty" yaml:"status,omitempty"`
	// DeprecatedBy points to the preferred leaf when Status is deprecated.
	DeprecatedBy string `json:"deprecated_by,omitempty" yaml:"deprecated_by,omitempty"`
	// Patterns match caller field values at catalog build time (compiled regex).
	Patterns []FieldPattern `json:"patterns,omitempty" yaml:"patterns,omitempty"`
	// MapsTo is an opaque related term id in another vocabulary; not resolved here.
	MapsTo string `json:"maps_to,omitempty" yaml:"maps_to,omitempty"`
}

// Vocabulary is one named tag strategy.
type Vocabulary struct {
	ID          string `json:"id" yaml:"id"`
	Label       string `json:"label" yaml:"label"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Aliases are alternate vocabulary ids accepted by hosts during migration.
	Aliases []string `json:"aliases,omitempty" yaml:"aliases,omitempty"`
	Terms   []Term   `json:"terms" yaml:"terms"`
}

type fileShape struct {
	Vocabulary Vocabulary `yaml:"vocabulary"`
}

// ResolvedTerm is a term with derived tree fields.
type ResolvedTerm struct {
	Term
	Depth    int      `json:"depth"`
	Leaf     bool     `json:"leaf"`
	Path     []string `json:"path"` // root → self
	Children []string `json:"children,omitempty"`
}

// Catalog is the loaded, validated tree for one vocabulary.
type Catalog struct {
	Vocab Vocabulary
	ByID  map[string]*ResolvedTerm
	// AliasToID maps lower-case alias or id → canonical term id.
	AliasToID map[string]string
	Roots     []string
	// termOrder preserves vocabulary YAML term order for MatchFields first-fit.
	termOrder []string
	// compiledPatterns holds compiled regexes per term id (build time only).
	compiledPatterns map[string][]compiledFieldPattern
}

// Assignment is one leaf tagging on a content item.
type Assignment struct {
	Vocab  string `json:"vocab"`
	TermID string `json:"termId"`
	Label  string `json:"label"`
	// Source is the original string from frontmatter (may be an alias).
	Source string `json:"source,omitempty"`
}

// Projection is resolved metadata for one content item.
type Projection struct {
	Vocab      string       `json:"vocab"`
	Assigned   []Assignment `json:"assigned"`
	Ancestors  []string     `json:"ancestors,omitempty"`
	Descendant []string     `json:"descendant,omitempty"`
	Unknown    []string     `json:"unknown,omitempty"`
	NonLeaf    []string     `json:"nonLeaf,omitempty"`
}

// Issue is one validation or migration finding.
type Issue struct {
	Severity string `json:"severity"` // error | warn
	Path     string `json:"path,omitempty"`
	TermID   string `json:"termId,omitempty"`
	Message  string `json:"message"`
}

func normID(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

func uniqueNonEmpty(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
