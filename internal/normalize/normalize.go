package normalize

import (
	"path/filepath"

	gitadapter "github.com/Sorune/wsp/internal/adapter/git"
	"github.com/Sorune/wsp/internal/model"
)

func Repository(f gitadapter.Facts) model.Document {
	repoID := "repository:" + f.Remote
	if f.Remote == "UNKNOWN" {
		repoID = "repository:" + filepath.Base(f.Root)
	}
	unknowns := []model.Unknown{}
	if f.Remote == "UNKNOWN" {
		unknowns = append(unknowns, model.UnknownValue("remote_identity", repoID, "origin remote is not configured", model.ProvenanceGit))
	}
	attrs := map[string]string{
		"root": f.Root, "working_tree": f.WorkingTree, "revision": f.Revision,
		"branch": f.Branch, "remote": f.Remote, "upstream": f.Upstream,
		"ahead": f.Ahead, "behind": f.Behind,
	}
	return model.Document{
		SchemaVersion: 1, Status: "OK", Target: f.Root,
		Entities: []model.Entity{
			{ID: repoID, Kind: "REPOSITORY", Name: filepath.Base(f.Root), Attributes: attrs, Provenance: model.ProvenanceGit, Unknown: unknowns},
			{ID: "workspace-copy:" + f.Root, Kind: "WORKSPACE_COPY", Name: f.Root, Attributes: map[string]string{"path": f.Root, "working_tree": f.WorkingTree}, Provenance: model.ProvenanceFilesystem},
			{ID: "revision:" + f.Revision, Kind: "REVISION", Name: f.Revision, Attributes: map[string]string{"sha": f.Revision}, Provenance: model.ProvenanceGit},
			{ID: "machine:local", Kind: "MACHINE", Name: "local", Provenance: model.ProvenanceFilesystem},
		},
		Relations: []model.Relation{
			{ID: "contains:" + repoID + ":workspace-copy", Type: "contains", From: repoID, To: "workspace-copy:" + f.Root, Provenance: model.ProvenanceDerived, Reason: "workspace copy resolved from Git top-level"},
			{ID: "at:" + repoID + ":revision", Type: "at_revision", From: repoID, To: "revision:" + f.Revision, Provenance: model.ProvenanceGit},
		},
		Unknowns: unknowns,
	}
}
