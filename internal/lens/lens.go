package lens

import (
	"sort"

	"github.com/Sorune/wsp/internal/model"
	"github.com/Sorune/wsp/internal/relation"
)

type Node struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	Children []Node `json:"children,omitempty"`
	Unknown  bool   `json:"unknown,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type Tree struct {
	Axis  string `json:"axis"`
	Roots []Node `json:"roots"`
}

func TreeProjection(doc model.Document, axis string) Tree {
	entities := map[string]model.Entity{}
	for _, e := range doc.Entities {
		entities[e.ID] = e
	}
	rels := relation.Ordered(doc.Relations)
	children := map[string][]model.Relation{}
	hasParent := map[string]bool{}
	selected := map[string]bool{}
	for _, r := range rels {
		// Axis membership is declared in the semantic document. It is never
		// inferred from relation, entity, path, or identifier names.
		if r.Axis != axis {
			continue
		}
		selected[r.From] = true
		selected[r.To] = true
		children[r.From] = append(children[r.From], r)
		hasParent[r.To] = true
	}
	var makeNode func(string, map[string]bool) Node
	makeNode = func(id string, visiting map[string]bool) Node {
		e, ok := entities[id]
		if !ok {
			return Node{ID: id, Label: "UNKNOWN", Kind: "UNKNOWN", Unknown: true, Reason: "relation endpoint is unavailable"}
		}
		n := Node{ID: e.ID, Label: e.Name, Kind: e.Kind}
		if n.Label == "" {
			n.Label = e.ID
		}
		if len(e.Unknown) > 0 {
			n.Unknown = true
			n.Reason = e.Unknown[0].Reason
		}
		if visiting[id] {
			n.Unknown = true
			n.Reason = "relation cycle"
			return n
		}
		next := map[string]bool{}
		for k, v := range visiting {
			next[k] = v
		}
		next[id] = true
		for _, r := range children[id] {
			n.Children = append(n.Children, makeNode(r.To, next))
		}
		sort.Slice(n.Children, func(i, j int) bool { return n.Children[i].ID < n.Children[j].ID })
		return n
	}
	roots := []string{}
	for id := range selected {
		if !hasParent[id] {
			roots = append(roots, id)
		}
	}
	sort.Strings(roots)
	out := Tree{Axis: axis}
	for _, id := range roots {
		out.Roots = append(out.Roots, makeNode(id, map[string]bool{}))
	}
	if len(out.Roots) == 0 && len(selected) > 0 {
		out.Roots = append(out.Roots, Node{ID: "UNKNOWN", Label: "UNKNOWN", Kind: "UNKNOWN", Unknown: true, Reason: "all declared relations form a cycle"})
	}
	return out
}
