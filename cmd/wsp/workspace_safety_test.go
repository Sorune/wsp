package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sorune/wsp/internal/config"
)

func TestWorkspaceMutationRejectsLinkedConfigDirectory(t *testing.T) {
	base := t.TempDir()
	foreign := filepath.Join(base, "foreign")
	root := filepath.Join(base, "root")
	if err := os.Mkdir(foreign, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(foreign, filepath.Join(root, ".wsp")); err != nil {
		t.Fatal(err)
	}
	if err := initCommand([]string{root}); err == nil {
		t.Fatal("init followed .wsp symlink")
	}
	if _, err := os.Lstat(filepath.Join(foreign, "workspace.yaml")); !os.IsNotExist(err) {
		t.Fatal("init escaped Workspace root")
	}
	repo := filepath.Join(base, "repo")
	setupGit(t, repo, "https://github.com/Example/Safety.git")
	if err := os.WriteFile(filepath.Join(foreign, "workspace.yaml"),
		[]byte("schema_version: 1\nworkspace_id: \"workspace:foreign\"\nrepositories: []\nrelations: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(foreign, "workspace.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := workspaceAdd(root, repo, "", ""); err == nil {
		t.Fatal("workspace add followed .wsp symlink")
	}
	after, err := os.ReadFile(filepath.Join(foreign, "workspace.yaml"))
	if err != nil || string(after) != string(before) {
		t.Fatal("workspace add altered foreign Manifest")
	}
	source := filepath.Join(base, "source")
	if err := config.Write(source, config.Manifest{SchemaVersion: 1, WorkspaceID: "workspace:source"}); err != nil {
		t.Fatal(err)
	}
	if err := initFromCommand(root, source); err == nil {
		t.Fatal("init --from followed .wsp symlink")
	}
	after, err = os.ReadFile(filepath.Join(foreign, "workspace.yaml"))
	if err != nil || string(after) != string(before) {
		t.Fatal("init --from altered foreign Manifest")
	}
}
