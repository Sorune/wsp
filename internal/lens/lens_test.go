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

	if len(logical.Roots) != 3 || logical.Roots[0].ID != "repository:demo" || logical.Roots[1].ID != "session:demo" || logical.Roots[2].ID != "workspace:demo" {
		t.Fatalf("logical projection did not preserve axis-specific roots: %#v", logical)
	}
	if len(session.Roots) != 2 || session.Roots[0].ID != "project:demo" || session.Roots[1].ID != "workspace:demo" {
		t.Fatalf("session projection did not preserve axis-specific roots: %#v", session)
	}
	if len(logical.Roots[2].Children) != 1 || logical.Roots[2].Children[0].ID != "project:demo" {
		t.Fatalf("logical relation missing: %#v", logical)
	}
	if len(session.Roots[1].Children) != 1 || session.Roots[1].Children[0].ID != "session:demo" {
		t.Fatalf("session relation missing: %#v", session)
	}
	if len(session.Roots[1].Children[0].Children) != 1 || session.Roots[1].Children[0].Children[0].ID != "repository:demo" {
		t.Fatalf("session traversal missing: %#v", session)
	}
}

func TestTreeProjectionDoesNotInferAxis(t *testing.T) {
	doc := model.Document{
		Entities:  []model.Entity{{ID: "a", Kind: "A", Name: "A"}, {ID: "b", Kind: "B", Name: "B"}},
		Relations: []model.Relation{{ID: "named-session", Type: "session", From: "a", To: "b"}},
	}
	tree := TreeProjection(doc, "session")
	if len(tree.Roots) != 2 || tree.Roots[0].ID != "a" || tree.Roots[1].ID != "b" {
		t.Fatalf("unscoped relation was inferred into axis: %#v", tree)
	}
}
