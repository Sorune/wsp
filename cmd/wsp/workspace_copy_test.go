package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Sorune/wsp/internal/config"
	"github.com/Sorune/wsp/internal/inspect"
)

func runGitCopy(t *testing.T, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s (%v)", args, out, err)
	}
}

func TestWorkspaceCopyFromSameOriginTracksCleanAndMarksDirty(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "workspace")
	primary := filepath.Join(base, "internal", "alpha")
	copyPath := filepath.Join(base, "external", "alpha-copy")
	if err := initCommand([]string{root}); err != nil {
		t.Fatal(err)
	}
	setupGit(t, primary, "git@github.com:Example/alpha.git")
	if err := workspaceAdd(root, primary, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(copyPath), 0755); err != nil {
		t.Fatal(err)
	}
	runGitCopy(t, "clone", "-q", primary, copyPath)
	runGitCopy(t, "-C", copyPath, "remote", "set-url", "origin", "https://github.com/Example/alpha.git")
	repoID := "repository:github.com/example/alpha"

	m, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	observed := assessCopy(root, m, repoID, copyPath, "")
	if observed.Decision != "ALIGNED" {
		t.Fatalf("clean checkout not aligned: %+v", observed)
	}
	if err := copyAdd(root, repoID, copyPath, "workspace-copy:alpha-copy"); err != nil {
		t.Fatal(err)
	}
	if err := copyAdd(root, repoID, copyPath, "workspace-copy:another"); err == nil {
		t.Fatal("same physical Workspace Copy accepted twice")
	}
	m, err = config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Entities) != 1 || m.Entities[0].Kind != "WORKSPACE_COPY" ||
		m.Entities[0].Path == "" || m.Entities[0].Origin != "github.com/example/alpha" {
		t.Fatalf("copy metadata not preserved: %+v", m.Entities)
	}
	if got := declaredCopyParent(m, "workspace-copy:alpha-copy"); got != repoID {
		t.Fatalf("copy parent = %s", got)
	}
	document, err := inspect.Manifest(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entity := range document.Entities {
		if entity.ID == "workspace-copy:alpha-copy" {
			found = true
			if entity.Attributes["path_physical"] != canonicalPath(copyPath) {
				t.Fatalf("copy physical path is missing: %+v", entity.Attributes)
			}
		}
	}
	if !found {
		t.Fatal("WORKSPACE_COPY missing in Lens model")
	}
	if err := copyList(root, true); err != nil {
		t.Fatal(err)
	}
	// Copy paths are rebased as well as Repository paths when the explicit
	// Workspace root changes. Source manifest and Git checkouts remain intact.
	clonedWorkspace := filepath.Join(base, "new-workspace")
	if err := initFromCommand(clonedWorkspace, root); err != nil {
		t.Fatal(err)
	}
	relocatedManifest, err := config.Load(clonedWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(relocatedManifest.Entities) != 1 ||
		canonicalPath(resolvedRepoPath(clonedWorkspace, relocatedManifest.Entities[0].Path)) != canonicalPath(copyPath) {
		t.Fatalf("Workspace Copy path was not rebased: %+v", relocatedManifest.Entities)
	}
	if err := workspaceList(clonedWorkspace, true); err != nil {
		t.Fatal(err)
	}
	if err := copyDiscover(root, filepath.Join(base, "external"), true); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(copyPath, "untracked.txt"), []byte("local change"), 0644); err != nil {
		t.Fatal(err)
	}
	dirty := assessCopy(root, m, repoID, copyPath, "workspace-copy:alpha-copy")
	if dirty.Decision != "UNTRACKED_DIRTY" {
		t.Fatalf("dirty copy was silently tracked: %+v", dirty)
	}
	if err := copyAdd(root, repoID, copyPath, "workspace-copy:dirty"); err == nil {
		t.Fatal("dirty copy registered")
	}
	if err := copyList(root, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(copyPath, "untracked.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(copyPath, "new.txt"), []byte("new alpha"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitCopy(t, "-C", copyPath, "add", "new.txt")
	runGitCopy(t, "-C", copyPath, "-c", "user.name=WSP Fixture", "-c", "user.email=wsp@example.invalid", "commit", "-qm", "related alpha")
	related := assessCopy(root, m, repoID, copyPath, "workspace-copy:alpha-copy")
	if related.Decision != "RELATED" {
		t.Fatalf("related clean copy not recognized: %+v", related)
	}
	if err := copyList(root, true); err != nil {
		t.Fatal(err)
	}
	// A second clean checkout on a descendant commit is also an explicit
	// eligible WORKSPACE_COPY; no Project semantics are imported.
	relatedPath := filepath.Join(base, "external", "alpha-related")
	runGitCopy(t, "clone", "-q", primary, relatedPath)
	runGitCopy(t, "-C", relatedPath, "remote", "set-url", "origin", "git@github.com:Example/alpha.git")
	runGitCopy(t, "-C", relatedPath, "-c", "user.name=WSP Fixture", "-c", "user.email=wsp@example.invalid", "commit", "-q", "--allow-empty", "-m", "descendant")
	if got := assessCopy(root, m, repoID, relatedPath, ""); got.Decision != "RELATED" {
		t.Fatalf("descendant checkout not eligible: %+v", got)
	}
	if err := copyAdd(root, repoID, relatedPath, "workspace-copy:related"); err != nil {
		t.Fatalf("clean related checkout cannot be registered: %v", err)
	}
	updated, err := config.Load(root)
	if err != nil || len(updated.Entities) != 2 {
		t.Fatalf("two physical copies not retained: %v %+v", err, updated.Entities)
	}

	if err := os.WriteFile(filepath.Join(primary, "primary-untracked.txt"), []byte("local"), 0644); err != nil {
		t.Fatal(err)
	}
	parentDirty := assessCopy(root, m, repoID, copyPath, "workspace-copy:alpha-copy")
	if parentDirty.Decision != "UNTRACKED_DIRTY_PARENT" {
		t.Fatalf("dirty parent not recognized: %+v", parentDirty)
	}
}

func TestWorkspaceCopyDivergenceAndWrongOriginFailClosed(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "workspace")
	primary := filepath.Join(base, "primary")
	other := filepath.Join(base, "other")
	if err := initCommand([]string{root}); err != nil {
		t.Fatal(err)
	}
	setupGit(t, primary, "https://github.com/Example/shared.git")
	setupGit(t, other, "git@github.com:Example/shared.git")
	// Distinct orphan root: identical fixture timestamp/content must not
	// accidentally create a shared initial commit object.
	runGitCopy(t, "-C", other, "checkout", "-q", "--orphan", "isolated")
	runGitCopy(t, "-C", other, "rm", "-q", "-f", "test.txt")
	runGitCopy(t, "-C", other, "commit", "-q", "--allow-empty", "-m", "independent")
	if err := workspaceAdd(root, primary, "", ""); err != nil {
		t.Fatal(err)
	}
	m, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	id := "repository:github.com/example/shared"
	got := assessCopy(root, m, id, other, "")
	if got.Decision != "REVIEW_REQUIRED" {
		t.Fatalf("disjoint object alpha accepted: %+v", got)
	}
	if err := copyAdd(root, id, other, "workspace-copy:unrelated"); err == nil {
		t.Fatal("unrelated copy auto-tracked")
	}
	before, _ := os.ReadFile(config.Path(root))
	runGitCopy(t, "-C", other, "remote", "set-url", "origin", "https://github.com/Other/shared.git")
	got = assessCopy(root, m, id, other, "")
	if got.Decision != "ORIGIN_MISMATCH" {
		t.Fatalf("wrong origin not blocked: %+v", got)
	}
	if err := copyAdd(root, id, other, "workspace-copy:wrong"); err == nil {
		t.Fatal("wrong-origin copy auto-tracked")
	}
	after, _ := os.ReadFile(config.Path(root))
	if string(before) != string(after) {
		t.Fatal("blocked operations changed Manifest")
	}
}

func TestWorkspaceCopyOfflinePersistsDeclaredRelation(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "workspace")
	primary := filepath.Join(base, "primary")
	copyPath := filepath.Join(base, "copy")
	if err := initCommand([]string{root}); err != nil {
		t.Fatal(err)
	}
	setupGit(t, primary, "https://github.com/Example/offline.git")
	runGitCopy(t, "clone", "-q", primary, copyPath)
	runGitCopy(t, "-C", copyPath, "remote", "set-url", "origin", "https://github.com/Example/offline.git")
	if err := workspaceAdd(root, primary, "", ""); err != nil {
		t.Fatal(err)
	}
	id := "repository:github.com/example/offline"
	if err := copyAdd(root, id, copyPath, "workspace-copy:offline"); err != nil {
		t.Fatal(err)
	}
	m, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(copyPath, copyPath+"-missing"); err != nil {
		t.Fatal(err)
	}
	obs := assessCopy(root, m, id, copyPath, "workspace-copy:offline")
	if obs.Decision != "UNAVAILABLE" {
		t.Fatalf("unmounted copy not explicitly unavailable: %+v", obs)
	}
	if err := copyList(root, true); err != nil {
		t.Fatal(err)
	}
	doc, err := inspect.Manifest(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range doc.Entities {
		if e.ID == "workspace-copy:offline" {
			found = true
		}
	}
	if !found {
		t.Fatal("offline physical copy caused semantic deletion")
	}
}

func TestWorkspaceLegacyCopyEntityRemainsReadable(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "legacy")
	copyRoot := filepath.Join(base, "copy-of-workspace")
	m := config.Manifest{SchemaVersion: 1, WorkspaceID: "workspace:legacy",
		Entities: []config.Item{{ID: "workspace-copy:placeholder", Kind: "WORKSPACE_COPY", Name: "legacy placeholder"}}}
	if err := config.Write(root, m); err != nil {
		t.Fatal(err)
	}
	if err := copyList(root, true); err != nil {
		t.Fatal(err)
	}
	rows := observedCopies(root, m)
	if len(rows) != 1 || rows[0].Decision != "UNCONFIGURED_COPY" {
		t.Fatalf("legacy manifest classification: %+v", rows)
	}
	if err := initFromCommand(copyRoot, root); err != nil {
		t.Fatal(err)
	}
	copied, err := config.Load(copyRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(copied.Entities) != 1 || copied.Entities[0].Path != "" {
		t.Fatalf("legacy copy placeholder mutated: %+v", copied.Entities)
	}
	if _, err := inspect.Manifest(copyRoot); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceCopyHealthRollup(t *testing.T) {
	if got := copySummaryStatus(nil); got != "OK" {
		t.Fatalf("empty copy set should be OK: %s", got)
	}
	if got := copySummaryStatus([]copyView{{Decision: "TRACKED"}}); got != "OK" {
		t.Fatalf("tracked copy should be OK: %s", got)
	}
	for _, decision := range []string{"UNTRACKED_DIRTY", "UNTRACKED_DIRTY_PARENT", "REVIEW_REQUIRED", "IDENTITY_MISMATCH", "UNAVAILABLE", "UNCONFIGURED_COPY"} {
		if got := copySummaryStatus([]copyView{{Decision: "TRACKED"}, {Decision: decision}}); got != "PARTIAL" {
			t.Errorf("copy decision %s incorrectly reports %s", decision, got)
		}
	}
}
