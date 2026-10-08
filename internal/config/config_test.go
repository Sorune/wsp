package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestWriteEmptyRequiredCollectionsAsArrays(t *testing.T) {
	root := t.TempDir()
	m := Manifest{SchemaVersion: 1, WorkspaceID: "workspace:empty"}
	if err := Write(root, m); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ManifestRelativePath))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "repositories: []\n") {
		t.Fatalf("empty repositories must serialize as []: %q", text)
	}
	if !strings.Contains(text, "relations: []\n") {
		t.Fatalf("empty relations must serialize as []: %q", text)
	}
	if _, err := Load(root); err != nil {
		t.Fatalf("generated empty manifest must load: %v", err)
	}
}
