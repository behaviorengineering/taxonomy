package tags

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RenderRegister builds the tag-register.txt body (UTF-8, Unix newlines).
func RenderRegister(counts map[string]int, seedIDs []string, deps []Deprecation) string {
	for _, id := range seedIDs {
		id = normalizeToken(id)
		if id == "" {
			continue
		}
		if _, ok := counts[id]; !ok {
			counts[id] = 0
		}
	}
	var b strings.Builder
	b.WriteString("# Tag register (generated). Do not edit by hand.\n")
	b.WriteString("# Active tags from stored shape fields. Deprecations: edit tag-deprecations.toml\n")
	b.WriteString("\n")
	b.WriteString("## Active tags\n")
	b.WriteString("# tag<TAB>count\n")
	type row struct {
		tag   string
		count int
	}
	var active []row
	for tag, n := range counts {
		if tag == "" {
			continue
		}
		active = append(active, row{tag: tag, count: n})
	}
	sort.Slice(active, func(i, j int) bool {
		if active[i].count != active[j].count {
			return active[i].count > active[j].count
		}
		return active[i].tag < active[j].tag
	})
	for _, r := range active {
		fmt.Fprintf(&b, "%s\t%d\n", r.tag, r.count)
	}
	b.WriteString("\n## Deprecated tags\n")
	b.WriteString("# from<TAB>to<TAB>files_still_using_from\n")
	if len(deps) == 0 {
		b.WriteString("# (none)\n")
	} else {
		for _, d := range deps {
			n := counts[d.From]
			fmt.Fprintf(&b, "%s\t%s\t%d\n", d.From, d.To, n)
		}
	}
	return b.String()
}

// CountTagsFromShapes counts valid kebab tokens across stored shape strings (space-separated).
func CountTagsFromShapes(shapes []string) map[string]int {
	counts := make(map[string]int)
	for _, shape := range shapes {
		filtered := FilterStoredShape(shape, nil)
		for _, tok := range strings.Fields(filtered) {
			counts[tok]++
		}
	}
	return counts
}

// WriteRegister atomically writes the generated register file.
func WriteRegister(path string, shapes []string, seedIDs []string, deps []Deprecation) error {
	const op = "tags.WriteRegister"
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return newErr(op, CodeInvalid, "empty register path", nil)
	}
	counts := CountTagsFromShapes(shapes)
	body := RenderRegister(counts, seedIDs, deps)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return newErr(op, CodeFailed, "mkdir register dir", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o600); err != nil {
		return newErr(op, CodeFailed, "write register tmp", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return newErr(op, CodeFailed, "rename register", err)
	}
	return nil
}

// WriteRegisterFromRows writes the register file from ledger rows without listing again.
func WriteRegisterFromRows(cat *Catalog, rows []Row) (string, error) {
	const op = "tags.WriteRegisterFromRows"
	if cat == nil {
		return "", newErr(op, CodeInvalid, "nil catalog", nil)
	}
	shapes := shapesFromRows(rows)
	path := cat.RegisterPath()
	if err := WriteRegister(path, shapes, cat.SeedIDs(), cat.Deprecations()); err != nil {
		return "", err
	}
	return path, nil
}

func shapesFromRows(rows []Row) []string {
	shapes := make([]string, 0, len(rows))
	for _, r := range rows {
		if strings.TrimSpace(r.Shape) != "" {
			shapes = append(shapes, r.Shape)
		}
	}
	return shapes
}

// RegenerateRegister lists shapes from the ledger and writes the register file.
func RegenerateRegister(ctx context.Context, ledger Ledger, cat *Catalog) (string, error) {
	const op = "tags.RegenerateRegister"
	if ledger == nil || cat == nil {
		return "", newErr(op, CodeInvalid, "nil ledger or catalog", nil)
	}
	rows, err := ledger.ListComplete(ctx)
	if err != nil {
		return "", newErr(op, CodeFailed, "list complete", err)
	}
	return WriteRegisterFromRows(cat, rows)
}
