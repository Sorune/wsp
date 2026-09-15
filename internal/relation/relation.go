package relation

import (
	"fmt"
	"sort"

	"github.com/Sorune/wsp/internal/model"
)

func Validate(relations []model.Relation, entities []model.Entity) error {
	known := map[string]bool{}
	for _, e := range entities {
		known[e.ID] = true
	}
	seen := map[string]bool{}
	for _, r := range relations {
		if r.ID == "" || r.Type == "" || r.From == "" || r.To == "" {
			return fmt.Errorf("INVALID_RELATION: relation requires id, type, from, and to")
		}
		if r.Axis == "" {
			return fmt.Errorf("INVALID_RELATION: relation %q requires an observation axis", r.ID)
		}
		if r.Axis != "logical" && r.Axis != "session" {
			return fmt.Errorf("INVALID_RELATION: unsupported observation axis %q", r.Axis)
		}
		if seen[r.ID] {
			return fmt.Errorf("INVALID_RELATION: duplicate relation %q", r.ID)
		}
		seen[r.ID] = true
		if !known[r.From] || !known[r.To] {
			return fmt.Errorf("INVALID_RELATION: unresolved endpoint in %q", r.ID)
		}
	}
	return nil
}

func Ordered(relations []model.Relation) []model.Relation {
	out := append([]model.Relation(nil), relations...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].To < out[j].To
	})
	return out
}
