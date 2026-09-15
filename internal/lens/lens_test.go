package lens

import (
	"testing"

	"github.com/Sorune/wsp/internal/model"
)

func TestTreeProjectionUsesDeclaredAxisOnSameDocument(t *testing.T) {
	doc := model.Document{
		Entities: []model.Entity{
			{ID: "workspace:demo", Kind: "WORKSPACE", Name: "Demo"},
			{ID: "project:demo", Kind: "PROJECT", Name: "Project"},
			{ID: "session:demo", Kind: "SESSION", Name: "Session"},
			{ID: "repository:demo", Kind: "REPOSITORY", Name: "Repo"},
		},
		Relations: []model.Relation{
			{ID: "logical-project", Type: "contains", From: "workspace:demo", To: "project:demo", Axis: "logical"},
			{ID: "session-link", Type: "contains", From: "workspace:demo", To: "session:demo", Axis: "session"},
			{ID: "session-repo", Type: "repository", From: "session:demo", To: "repository:demo", Axis: "session"},
		},
	}

	logical := TreeProjection(doc, "logical")
	session := TreeProjection(doc, "session")

	if len(logical.Roots) != 1 || logical.Roots[0].ID != "workspace:demo" {
		t.Fatalf("logical projection leaked unrelated roots: %#v", logical)
	}
	if len(logical.Roots[0].Children) != 1 || logical.Roots[0].Children[0].ID != "project:demo" {
		t.Fatalf("logical relation missing: %#v", logical)
	}
	if containsNodeID(logical.Roots, "session:demo") || containsNodeID(logical.Roots, "repository:demo") {
		t.Fatalf("logical projection leaked session-only entities: %#v", logical)
	}
	if len(session.Roots) != 1 || session.Roots[0].ID != "workspace:demo" {
		t.Fatalf("session projection leaked unrelated roots: %#v", session)
	}
	if len(session.Roots[0].Children) != 1 || session.Roots[0].Children[0].ID != "session:demo" {
		t.Fatalf("session relation missing: %#v", session)
	}
	if len(session.Roots[0].Children[0].Children) != 1 || session.Roots[0].Children[0].Children[0].ID != "repository:demo" {
		t.Fatalf("session traversal missing: %#v", session)
	}
	if containsNodeID(session.Roots, "project:demo") {
		t.Fatalf("session projection leaked logical-only entities: %#v", session)
	}
}

func TestTreeProjectionDoesNotInferAxis(t *testing.T) {
	doc := model.Document{
		Entities:  []model.Entity{{ID: "a", Kind: "A", Name: "A"}, {ID: "b", Kind: "B", Name: "B"}},
		Relations: []model.Relation{{ID: "named-session", Type: "session", From: "a", To: "b"}},
	}
	tree := TreeProjection(doc, "session")
	if len(tree.Roots) != 0 {
		t.Fatalf("unscoped relation was inferred into axis: %#v", tree)
	}
}

func TestTreeProjectionWithEmptyAxisIsEmpty(t *testing.T) {
	doc := model.Document{
		Entities:  []model.Entity{{ID: "a", Kind: "A", Name: "A"}},
		Relations: []model.Relation{{ID: "logical", Type: "contains", From: "a", To: "a", Axis: "logical"}},
	}
	first := TreeProjection(doc, "session")
	second := TreeProjection(doc, "session")
	if len(first.Roots) != 0 || len(second.Roots) != 0 {
		t.Fatalf("empty axis was populated: first=%#v second=%#v", first, second)
	}
}

func containsNodeID(nodes []Node, id string) bool {
	for _, node := range nodes {
		if node.ID == id || containsNodeID(node.Children, id) {
			return true
		}
	}
	return false
}
