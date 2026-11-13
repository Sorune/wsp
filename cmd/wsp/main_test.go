package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Sorune/wsp/internal/config"
	"github.com/Sorune/wsp/internal/inspect"
	"github.com/Sorune/wsp/internal/model"
)

func TestInitExistingNonGitWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := initCommand([]string{root}); err != nil {
		t.Fatalf("init existing non-Git workspace: %v", err)
	}
	m, err := config.Load(root)
	if err != nil {
		t.Fatalf("load initialized manifest: %v", err)
	}
	if m.WorkspaceID == "" {
		t.Fatal("workspace identity was not created")
	}
	if len(m.Repositories) != 0 {
		t.Fatalf("non-Git workspace invented repositories: %+v", m.Repositories)
	}
	if len(m.Relations) != 0 {
		t.Fatalf("non-Git workspace invented relations: %+v", m.Relations)
	}
}

func TestInitUsesPortableRepositoryPath(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	for _, args := range [][]string{{"-C", root, "config", "user.name", "WSP Test"}, {"-C", root, "config", "user.email", "wsp@example.invalid"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git config: %v %s", err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", root, "add", "file.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	if out, err := exec.Command("git", "-C", root, "commit", "-qm", "fixture").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v %s", err, out)
	}
	if out, err := exec.Command("git", "-C", root, "remote", "add", "origin", "https://example.invalid/wsp/test.git").CombinedOutput(); err != nil {
		t.Fatalf("git remote: %v %s", err, out)
	}
	if err := initCommand([]string{root}); err != nil {
		t.Fatal(err)
	}
	m, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Repositories) != 1 || m.Repositories[0].Path != "." {
		t.Fatalf("init repository path = %#v, want relative dot", m.Repositories)
	}
}

func TestManifestRepositoryPathIsRelativeToManifestRootAfterRelocation(t *testing.T) {
	first := t.TempDir()
	m := config.Manifest{SchemaVersion: 1, WorkspaceID: "workspace:portable", Repositories: []config.Item{{ID: "repository:self", Name: "self", Path: "."}}}
	if err := config.Write(first, m); err != nil {
		t.Fatal(err)
	}
	copyTree := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(config.Path(copyTree)), 0755); err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile(config.Path(first))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.Path(copyTree), manifestBytes, 0644); err != nil {
		t.Fatal(err)
	}
	copyBytes, err := os.ReadFile(config.Path(copyTree))
	if err != nil {
		t.Fatal(err)
	}
	if string(copyBytes) != string(manifestBytes) {
		t.Fatal("relocated manifest bytes changed")
	}
	doc, err := inspect.Manifest(copyTree)
	if err != nil {
		t.Fatal(err)
	}
	var repo model.Entity
	for _, entity := range doc.Entities {
		if entity.ID == "repository:self" {
			repo = entity
		}
	}
	if repo.Attributes["path"] != copyTree || repo.Attributes["path_configured"] != "." {
		t.Fatalf("resolved relocated path = %#v", repo.Attributes)
	}

	abs := filepath.Join(t.TempDir(), "legacy-repo")
	m.Repositories[0].Path = abs
	legacy := t.TempDir()
	if err := config.Write(legacy, m); err != nil {
		t.Fatal(err)
	}
	doc, err = inspect.Manifest(legacy)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entity := range doc.Entities {
		if entity.ID == "repository:self" && (entity.Attributes["path"] != abs || entity.Attributes["path_configured"] != abs) {
			t.Fatalf("absolute path compatibility lost: %#v", entity.Attributes)
		}
		if entity.ID == "repository:self" {
			found = true
		}
	}
	if !found {
		t.Fatal("absolute repository declaration was not projected")
	}
}

func TestInitMissingWorkspaceCreatesCleanRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "new-workspace")
	if err := initCommand([]string{root}); err != nil {
		t.Fatalf("init clean workspace: %v", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("created workspace root missing: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("created workspace root is not a directory")
	}
	m, err := config.Load(root)
	if err != nil {
		t.Fatalf("load clean workspace manifest: %v", err)
	}
	if len(m.Repositories) != 0 {
		t.Fatalf("clean workspace invented repositories: %+v", m.Repositories)
	}
	if len(m.Relations) != 0 {
		t.Fatalf("clean workspace invented relations: %+v", m.Relations)
	}
}

func TestTargetPathUsesCallerDirectory(t *testing.T) {
	caller := t.TempDir()
	t.Setenv("WSP_CALLER_PWD", caller)
	got := targetPath(filepath.Join("child", "..", "repo"))
	want := filepath.Join(caller, "repo")
	if got != want {
		t.Fatalf("targetPath caller-relative mismatch: got %q want %q", got, want)
	}
	absolute := filepath.Join(t.TempDir(), "absolute")
	if got := targetPath(absolute); got != filepath.Clean(absolute) {
		t.Fatalf("absolute target changed: got %q want %q", got, filepath.Clean(absolute))
	}
}

func TestInitHelpDoesNotMutate(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WSP_CALLER_PWD", root)
	if err := run([]string{"init", "--help"}); err != nil {
		t.Fatalf("init --help: %v", err)
	}
	if _, err := os.Stat(config.Path(root)); !os.IsNotExist(err) {
		t.Fatalf("init --help mutated workspace: stat err=%v", err)
	}
}
