package catalog

import "strings"

// MatchFields returns the first vocabulary term that fits the field map (YAML term order).
func (c *Catalog) MatchFields(fields map[string]string) (ResolvedTerm, bool, error) {
	if c == nil || fields == nil {
		return ResolvedTerm{}, false, nil
	}
	for _, id := range c.termOrder {
		rt := c.ByID[id]
		if rt == nil || rt.Status == StatusDeprecated {
			continue
		}
		if termFitsFields(c, rt, fields) {
			pref, ok := c.PreferLeaf(rt.ID)
			if !ok {
				return ResolvedTerm{}, false, nil
			}
			return *pref, true, nil
		}
	}
	return ResolvedTerm{}, false, nil
}

func termFitsFields(c *Catalog, rt *ResolvedTerm, fields map[string]string) bool {
	if fieldValuesMatchTerm(c, rt, fields) {
		return true
	}
	compiled := c.compiledPatterns[rt.ID]
	for _, p := range compiled {
		val := strings.TrimSpace(fields[p.field])
		if val == "" {
			continue
		}
		if p.re.MatchString(val) {
			return true
		}
	}
	return false
}

func fieldValuesMatchTerm(_ *Catalog, rt *ResolvedTerm, fields map[string]string) bool {
	idKey := normID(rt.ID)
	for _, val := range fields {
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}
		n := normID(val)
		if n == idKey {
			return true
		}
		for _, a := range rt.Aliases {
			if n == normID(a) {
				return true
			}
		}
	}
	return false
}
