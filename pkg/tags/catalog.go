package tags

import (
	"bufio"
	"os"
	"slices"
	"strings"
)

// Config holds paths and optional deny list for the tag catalog.
type Config struct {
	RegisterPath     string
	DeprecationsPath string
	SeedPath         string
	Deny             []string
}

// Catalog holds active tag ids, deprecations, seed ids, and deny set for validation.
type Catalog struct {
	active   []string
	deprec   map[string]string
	depRows  []Deprecation
	dropped  map[string]struct{}
	kept     map[string]struct{}
	skipped  map[string]struct{}
	seedIDs  []string
	seedSet  map[string]struct{}
	deny     map[string]struct{}
	register string
	seed     string
	depPath  string
	cfg      Config
}

// CreateCatalog loads register (or seed), deprecations, and seed list.
func CreateCatalog(cfg Config) (*Catalog, error) {
	const op = "tags.CreateCatalog"
	seedPath := strings.TrimSpace(cfg.SeedPath)
	if seedPath == "" {
		return nil, newErr(op, CodeConfig, "empty seed path", nil)
	}
	seedIDs, err := LoadSeedIDs(seedPath)
	if err != nil {
		return nil, err
	}
	maps, err := LoadMaps(cfg.DeprecationsPath)
	if err != nil {
		return nil, err
	}
	active, err := loadActiveIDs(cfg.RegisterPath, seedIDs)
	if err != nil {
		return nil, err
	}
	deny := denySet(cfg.Deny)
	return &Catalog{
		active:   active,
		deprec:   deprecationMap(maps.Deprecated),
		depRows:  maps.Deprecated,
		dropped:  droppedSet(maps.Dropped),
		kept:     idSet(maps.Kept),
		skipped:  idSet(maps.Skipped),
		seedIDs:  seedIDs,
		seedSet:  seedIDSet(seedIDs),
		deny:     deny,
		register: strings.TrimSpace(cfg.RegisterPath),
		seed:     seedPath,
		depPath:  strings.TrimSpace(cfg.DeprecationsPath),
		cfg:      cfg,
	}, nil
}

// Config returns the catalog config used at load (for curator reload).
func (c *Catalog) Config() Config {
	if c == nil {
		return Config{}
	}
	return c.cfg
}

// ActiveIDs returns tag ids for the prefer list.
func (c *Catalog) ActiveIDs() []string {
	if c == nil {
		return nil
	}
	return slices.Clone(c.active)
}

// SeedIDs returns seed tag ids.
func (c *Catalog) SeedIDs() []string {
	if c == nil {
		return nil
	}
	return slices.Clone(c.seedIDs)
}

// Deprecations returns deprecation rows for register rendering.
func (c *Catalog) Deprecations() []Deprecation {
	if c == nil {
		return nil
	}
	return slices.Clone(c.depRows)
}

// RegisterPath returns the operator register path from config.
func (c *Catalog) RegisterPath() string {
	if c == nil {
		return ""
	}
	return c.register
}

// DeprecationsPath returns the operator deprecations TOML path from config.
func (c *Catalog) DeprecationsPath() string {
	if c == nil {
		return ""
	}
	return c.depPath
}

// SeedPath returns the bundled seed list path from config.
func (c *Catalog) SeedPath() string {
	if c == nil {
		return ""
	}
	return c.seed
}

// DenySet returns the loaded deny map for token validation.
func (c *Catalog) DenySet() map[string]struct{} {
	if c == nil || c.deny == nil {
		return nil
	}
	return c.deny
}

// FilterStoredShape normalizes ledger shape text without failing on legacy junk tokens.
func FilterStoredShape(raw string, cat *Catalog) string {
	tokens := SplitTags(raw)
	if len(tokens) == 0 {
		return ""
	}
	var deny map[string]struct{}
	if cat != nil {
		deny = cat.deny
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		tok = normalizeToken(tok)
		if tok == "" {
			continue
		}
		if cat != nil && cat.deprec != nil {
			if to, ok := cat.deprec[tok]; ok {
				tok = to
			}
		}
		if cat != nil && cat.dropped != nil {
			if _, drop := cat.dropped[tok]; drop {
				continue
			}
		}
		if isStopword(tok) && !isSeedToken(cat, tok) {
			continue
		}
		if !IsValidStoredToken(tok, deny) {
			continue
		}
		if _, ok := seen[tok]; ok {
			continue
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
	}
	return JoinTags(out)
}

func isSeedToken(cat *Catalog, token string) bool {
	if cat == nil || cat.seedSet == nil {
		return false
	}
	_, ok := cat.seedSet[token]
	return ok
}

func seedIDSet(seedIDs []string) map[string]struct{} {
	out := make(map[string]struct{}, len(seedIDs))
	for _, id := range seedIDs {
		id = normalizeToken(id)
		if id != "" {
			out[id] = struct{}{}
		}
	}
	return out
}

// NormalizeTokens applies deprecations, validates each token, and returns sorted unique ids.
func (c *Catalog) NormalizeTokens(tokens []string) ([]string, error) {
	const op = "tags.Catalog.NormalizeTokens"
	if len(tokens) == 0 {
		return nil, newErr(op, CodeGate, ErrTokenInvalid, nil)
	}
	deny := c.DenySet()
	seen := make(map[string]struct{})
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		tok = normalizeToken(tok)
		if tok == "" {
			continue
		}
		if isStopword(tok) && !isSeedToken(c, tok) {
			continue
		}
		if c != nil && c.deprec != nil {
			if to, ok := c.deprec[tok]; ok {
				tok = to
			}
		}
		if c != nil && c.dropped != nil {
			if _, drop := c.dropped[tok]; drop {
				continue
			}
		}
		if err := ValidateToken(tok, deny); err != nil {
			return nil, err
		}
		if _, ok := seen[tok]; ok {
			continue
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
	}
	if len(out) == 0 {
		return nil, newErr(op, CodeGate, ErrTokenInvalid, nil)
	}
	slices.Sort(out)
	return out, nil
}

// NormalizeField splits raw tag text, normalizes, and joins for stored shape.
func (c *Catalog) NormalizeField(raw string) (string, error) {
	tokens := SplitTags(raw)
	ids, err := c.NormalizeTokens(tokens)
	if err != nil {
		return "", err
	}
	return JoinTags(ids), nil
}

// NormalizeShape is an alias for NormalizeField.
func (c *Catalog) NormalizeShape(raw string) (string, error) {
	return c.NormalizeField(raw)
}

// LoadSeedIDs reads one kebab id per line from the seed file.
func LoadSeedIDs(seedPath string) ([]string, error) {
	const op = "tags.LoadSeedIDs"
	seedPath = strings.TrimSpace(seedPath)
	if seedPath == "" {
		return nil, newErr(op, CodeInvalid, "empty seed path", nil)
	}
	f, err := os.Open(seedPath)
	if err != nil {
		return nil, newErr(op, CodeFailed, "open seed", err)
	}
	defer func() { _ = f.Close() }()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = normalizeToken(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, newErr(op, CodeFailed, "read seed", err)
	}
	if len(out) == 0 {
		return nil, newErr(op, CodeFailed, "seed file empty after trim", nil)
	}
	return out, nil
}

func loadActiveIDs(registerPath string, seedIDs []string) ([]string, error) {
	registerPath = strings.TrimSpace(registerPath)
	if registerPath != "" {
		ids, err := parseRegisterIDs(registerPath)
		if err != nil {
			return nil, err
		}
		if len(ids) > 0 {
			return ids, nil
		}
	}
	return slices.Clone(seedIDs), nil
}

func parseRegisterIDs(path string) ([]string, error) {
	const op = "tags.parseRegisterIDs"
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, newErr(op, CodeFailed, "read register", err)
	}
	var out []string
	inActive := false
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "## Active tags" {
			inActive = true
			continue
		}
		if strings.HasPrefix(line, "## ") {
			inActive = false
			continue
		}
		if !inActive || line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tag, _, _ := strings.Cut(line, "\t")
		tag = normalizeToken(tag)
		if tag != "" {
			out = append(out, tag)
		}
	}
	slices.Sort(out)
	return out, nil
}
