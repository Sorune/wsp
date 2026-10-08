package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sorune/wsp/internal/config"
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
