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

func TestInitMissingWorkspaceDoesNotCreateDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	err := initCommand([]string{root})
	if err == nil {
		t.Fatal("init unexpectedly created a missing workspace root")
	}
	if got := errorCategory(err); got != "TARGET_NOT_FOUND" {
		t.Fatalf("unexpected error category: %s (%v)", got, err)
	}
	if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
		t.Fatalf("missing workspace root was mutated: %v", statErr)
	}
}
