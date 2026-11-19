package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sorune/wsp/internal/model"
)

func TestManifestSubsetRejectsAmbiguityAndDuplicateDeclarations(t *testing.T) {
	cases := map[string]string{
		"unterminated quote":      "schema_version: 1\nworkspace_id: \"workspace:unterminated\nrepositories: []\nrelations: []\n",
		"duplicate root":          "schema_version: 1\nworkspace_id: \"workspace:a\"\nworkspace_id: \"workspace:b\"\nrepositories: []\nrelations: []\n",
		"unknown field":           "schema_version: 1\nworkspace_id: \"workspace:a\"\nexternal_authority: ACCEPT\nrepositories: []\nrelations: []\n",
		"duplicate section":       "schema_version: 1\nworkspace_id: \"workspace:a\"\nrepositories: []\nrepositories: []\nrelations: []\n",
		"duplicate item field":    "schema_version: 1\nworkspace_id: \"workspace:a\"\nrepositories:\n  - id: \"repository:one\"\n    id: \"repository:two\"\n    path: \"/tmp/example\"\nrelations: []\n",
		"nested unsupported yaml": "schema_version: 1\nworkspace_id: \"workspace:a\"\nrepositories:\n  - id: \"repository:a\"\n    path: \"/tmp/a\"\n      arbitrary: value\nrelations: []\n",
		"unknown item field":      "schema_version: 1\nworkspace_id: \"workspace:a\"\nrepositories:\n  - id: \"repository:a\"\n    accepted: true\n    path: \"/tmp/a\"\nrelations: []\n",
		"invalid escape":          "schema_version: 1\nworkspace_id: \"workspace:\\q\"\nrepositories: []\nrelations: []\n",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "workspace.yaml")
			if err := os.WriteFile(file, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadFile(file); err == nil {
				t.Fatalf("malformed manifest was accepted: %s", raw)
			}
		})
	}
}

func TestManifestGlobalUniqueIDsAndCopySingleParent(t *testing.T) {
	base := Manifest{SchemaVersion: 1, WorkspaceID: "workspace:fixture",
		Repositories: []Item{
			{ID: "repository:a", Path: "/tmp/a"},
			{ID: "repository:b", Path: "/tmp/b"},
		},
		Entities: []Item{{ID: "workspace-copy:one", Kind: "WORKSPACE_COPY",
			Path: "/tmp/copy", Origin: "github.com/example/a"}},
		Relations: []model.Relation{{ID: "relation:a", Type: "contains", Axis: "logical",
			From: "repository:a", To: "workspace-copy:one"}},
	}
	if err := Validate(base); err != nil {
		t.Fatal("valid physical copy rejected:", err)
	}
	conflict := base
	conflict.Entities = []Item{{ID: "repository:a", Kind: "WORKSPACE_COPY"}}
	if err := Validate(conflict); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("cross-kind ID collision accepted: %v", err)
	}
	twice := base
	twice.Repositories = append(append([]Item{}, base.Repositories...), Item{ID: "repository:a", Path: "/tmp/other"})
	if err := Validate(twice); err == nil {
		t.Fatal("duplicate Repository ID accepted")
	}
	doubleParent := base
	doubleParent.Relations = append(append([]model.Relation{}, base.Relations...),
		model.Relation{ID: "relation:b", Type: "contains", Axis: "logical",
			From: "repository:b", To: "workspace-copy:one"})
	if err := Validate(doubleParent); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("multiple Copy parents accepted: %v", err)
	}
	legacy := base
	legacy.Entities = []Item{{ID: "workspace-copy:one", Kind: "WORKSPACE_COPY"}}
	legacy.Relations = nil
	if err := Validate(legacy); err != nil {
		t.Fatalf("legacy Copy placeholder rejected: %v", err)
	}
}

func TestManifestWriteRejectsSymlinkBoundaryAndPreservesExternalContent(t *testing.T) {
	temp := t.TempDir()
	external := filepath.Join(temp, "external")
	if err := os.Mkdir(external, 0700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(temp, "workspace")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, ".wsp")); err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{SchemaVersion: 1, WorkspaceID: "workspace:fixture"}
	if err := Write(root, manifest); err == nil {
		t.Fatal("Write followed .wsp symlink")
	}
	if _, err := os.Lstat(filepath.Join(external, "workspace.yaml")); !os.IsNotExist(err) {
		t.Fatalf("wrote outside explicit Workspace root: %v", err)
	}
	physical := filepath.Join(temp, "physical")
	if err := os.Mkdir(physical, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(physical, ".wsp"), 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(external, "workspace.yaml")
	if err := os.WriteFile(target, []byte("MARKER"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, Path(physical)); err != nil {
		t.Fatal(err)
	}
	if err := Write(physical, manifest); err == nil {
		t.Fatal("Write followed existing manifest symlink")
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "MARKER" {
		t.Fatalf("foreign manifest changed: %q %v", data, err)
	}
}
