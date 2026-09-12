package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sorune/wsp/internal/model"
)

func TestWriteLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	m := Manifest{SchemaVersion: 1, WorkspaceID: "workspace:demo", Repositories: []Item{{ID: "repository:demo", Name: "demo", Path: root}}, Relations: []model.Relation{{ID: "contains", Type: "contains", From: "workspace:demo", To: "repository:demo", Axis: "logical", Provenance: "CONFIG"}}}
	if err := Write(root, m); err != nil {
		t.Fatal(err)
	}
	got, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.WorkspaceID != m.WorkspaceID || len(got.Relations) != 1 || got.Relations[0].Axis != "logical" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(root, ManifestRelativePath)); err != nil {
		t.Fatal(err)
	}
}
