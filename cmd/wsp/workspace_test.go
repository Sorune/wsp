package main

import (
	"github.com/Sorune/wsp/internal/config"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupGit(t *testing.T, dir, origin string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, argv := range [][]string{{"init", "-q", dir}, {"-C", dir, "config", "user.email", "wsp@example.invalid"}, {"-C", dir, "config", "user.name", "WSP Test"}} {
		if b, e := exec.Command("git", argv...).CombinedOutput(); e != nil {
			t.Fatalf("git %v: %s (%v)", argv, b, e)
		}
	}
	if e := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("fixture\n"), 0644); e != nil {
		t.Fatal(e)
	}
	for _, argv := range [][]string{{"-C", dir, "add", "test.txt"}, {"-C", dir, "commit", "-qm", "fixture"}, {"-C", dir, "remote", "add", "origin", origin}} {
		if b, e := exec.Command("git", argv...).CombinedOutput(); e != nil {
			t.Fatalf("git %v: %s (%v)", argv, b, e)
		}
	}
}

func TestWorkspaceAddRelocateAndPreserve(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "workspace")
	if e := initCommand([]string{root}); e != nil {
		t.Fatal(e)
	}
	alpha := filepath.Join(base, "External", "Sources", "alpha")
	beta := filepath.Join(base, "Internal", "beta")
	setupGit(t, alpha, "https://github.com/Example/Alpha.git")
	setupGit(t, beta, "git@github.com:Example/Beta.git")
	original, _ := os.ReadFile(config.Path(root))
	if e := os.WriteFile(config.Path(root), append(original, []byte("# keep human note\n")...), 0644); e != nil {
		t.Fatal(e)
	}
	if e := workspaceAdd(root, alpha, "", ""); e != nil {
		t.Fatal(e)
	}
	if e := workspaceAdd(root, beta, "repository:beta", ""); e != nil {
		t.Fatal(e)
	}
	m, e := config.Load(root)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Repositories) != 2 || len(m.Relations) != 2 {
		t.Fatalf("invalid registered count: %+v", m)
	}
	if m.Repositories[0].Origin != "github.com/example/alpha" || m.Repositories[1].Origin != "github.com/example/beta" {
		t.Fatalf("origin %+v", m.Repositories)
	}
	if e := workspaceList(root, true); e != nil {
		t.Fatal(e)
	}
	if e := workspaceAdd(root, alpha, "repository:another", ""); e == nil {
		t.Fatal("duplicate physical registered")
	}
	clone := filepath.Join(base, "other-copy")
	setupGit(t, clone, "git@github.com:Example/Alpha.git")
	if e := workspaceAdd(root, clone, "repository:another", ""); e == nil {
		t.Fatal("duplicate origin registered")
	}
	snapshot, _ := os.ReadFile(config.Path(root))
	if e := workspaceRelocate(root, m.Repositories[0].ID, beta); e == nil {
		t.Fatal("origin mismatch accepted")
	}
	after, _ := os.ReadFile(config.Path(root))
	if string(snapshot) != string(after) {
		t.Fatal("blocked relocate mutated manifest")
	}
	moved := filepath.Join(base, "External", "Sources", "new-location")
	if e := os.Rename(alpha, moved); e != nil {
		t.Fatal(e)
	}
	if e := workspaceRelocate(root, m.Repositories[0].ID, moved); e != nil {
		t.Fatal(e)
	}
	latest, e := config.Load(root)
	if e != nil {
		t.Fatal(e)
	}
	if got := resolvedRepoPath(root, latest.Repositories[0].Path); canonicalPath(got) != canonicalPath(moved) {
		t.Fatalf("path %q", got)
	}
	if len(latest.Relations) != len(m.Relations) || latest.Repositories[0].ID != m.Repositories[0].ID {
		t.Fatal("logical identity changed")
	}
	raw, _ := os.ReadFile(config.Path(root))
	if !strings.Contains(string(raw), "# keep human note") {
		t.Fatal("comment lost")
	}
	if e := workspaceList(root, true); e != nil {
		t.Fatal(e)
	}
}
func TestWorkspaceMutationGuard(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "workspace")
	if e := initCommand([]string{root}); e != nil {
		t.Fatal(e)
	}
	repo := filepath.Join(base, "repo")
	setupGit(t, repo, "https://github.com/Example/Repo.git")
	if e := workspaceAdd(root, repo, "", "project:invented"); e == nil {
		t.Fatal("invented Project")
	}
	before, _ := os.ReadFile(config.Path(root))
	if e := os.WriteFile(config.Path(root)+".lock", []byte("held"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := workspaceAdd(root, repo, "", ""); e == nil {
		t.Fatal("missing lock block")
	}
	after, _ := os.ReadFile(config.Path(root))
	if string(before) != string(after) {
		t.Fatal("blocked mutation changed manifest")
	}
	if e := os.Remove(config.Path(root) + ".lock"); e != nil {
		t.Fatal(e)
	}
	if e := workspaceAdd(root, repo, "", ""); e != nil {
		t.Fatal(e)
	}
	if e := workspaceAdd(root, repo, "", ""); e == nil {
		t.Fatal("duplicate accepted")
	}
}

func TestInitFromAndAliasBinding(t *testing.T) {
	base := t.TempDir()
	first := filepath.Join(base, "source-workspace")
	second := filepath.Join(base, "other-workspace")
	repo := filepath.Join(base, "External", "Sources", "Alpha")
	alias := filepath.Join(base, "Alpha")
	if e := initCommand([]string{first}); e != nil {
		t.Fatal(e)
	}
	setupGit(t, repo, "https://github.com/Example/Alpha.git")
	if e := os.Symlink(repo, alias); e != nil {
		t.Fatal(e)
	}
	if e := workspaceAdd(first, alias, "", ""); e != nil {
		t.Fatal(e)
	}
	m, e := config.Load(first)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Repositories) != 1 || canonicalPath(resolvedRepoPath(first, m.Repositories[0].Path)) != canonicalPath(repo) {
		t.Fatal("alias was not canonicalized")
	}
	if e := workspaceAdd(first, repo, "repository:duplicate", ""); e == nil {
		t.Fatal("alias counted as second copy")
	}
	before, e := os.ReadFile(config.Path(first))
	if e != nil {
		t.Fatal(e)
	}
	if e := initCommand([]string{second, "--from", first}); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(config.Path(first))
	if string(before) != string(after) {
		t.Fatal("init-from changed source")
	}
	clone, e := config.Load(second)
	if e != nil {
		t.Fatal(e)
	}
	if clone.WorkspaceID != m.WorkspaceID || len(clone.Relations) != len(m.Relations) ||
		canonicalPath(resolvedRepoPath(second, clone.Repositories[0].Path)) != canonicalPath(repo) {
		t.Fatalf("rebased paths or relations lost: %+v", clone)
	}
	if e := workspaceList(second, true); e != nil {
		t.Fatal(e)
	}
	if e := initCommand([]string{second, "--from", first}); e == nil {
		t.Fatal("existing manifest overwritten")
	}
}

func TestCanonicalOriginSeparatesHosts(t *testing.T) {
	cases := map[string]string{
		"https://github.com/Example/Alpha.git": "github.com/example/alpha",
		"git@github.com:Example/Alpha.git":     "github.com/example/alpha",
		"ssh://git@gitlab.example.org/a/b.git": "gitlab.example.org/a/b",
		"git@gitlab.example.org:a/b.git":       "gitlab.example.org/a/b",
		"ssh://git@example.org:2222/a/b.git":   "example.org:2222/a/b",
	}
	for remote, expected := range cases {
		if got := canonicalOrigin(remote); got != expected {
			t.Errorf("canonicalOrigin(%q) = %q; want %q", remote, got, expected)
		}
	}
}
