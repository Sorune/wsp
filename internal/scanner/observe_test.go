package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func fixture(t *testing.T) string { t.Helper(); return t.TempDir() }
func TestResolveRequiresExplicitIdentityAndRejectsSubdirectory(t *testing.T) {
	r := fixture(t)
	if _, e := Resolve(Request{Path: r}); e == nil {
		t.Fatal("missing subject ID accepted")
	}
	cmd := exec.Command("git", "init", "-q", r)
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("git init: %s", out)
	}
	sub := filepath.Join(r, "child")
	if e := os.Mkdir(sub, 0755); e != nil {
		t.Fatal(e)
	}
	if _, e := Resolve(Request{Path: sub, SubjectID: "s"}); e == nil {
		t.Fatal("subdirectory accepted as repository root")
	}
}
func TestObserveFilesystemOnlyAndDoesNotFollowSymlinks(t *testing.T) {
	r := fixture(t)
	outside := filepath.Join(t.TempDir(), "secret.go")
	if e := os.WriteFile(outside, []byte("package hidden"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(outside, filepath.Join(r, "link.go")); e != nil {
		t.Skipf("symlink unavailable: %v", e)
	}
	if e := os.WriteFile(filepath.Join(r, "main.go"), []byte("package main\n"), 0600); e != nil {
		t.Fatal(e)
	}
	b, e := Observe(Request{Path: r, SubjectID: "fixture"})
	if e != nil {
		t.Fatal(e)
	}
	if b.Subject.Revision != "UNKNOWN" || b.Subject.Root == "" {
		t.Fatalf("unexpected subject: %#v", b.Subject)
	}
	if e := os.WriteFile(filepath.Join(r, "new.go"), []byte("package main\n"), 0600); e != nil {
		t.Fatal(e)
	}
	for _, n := range b.Nodes {
		if strings.Contains(n.Coordinate, "secret") || n.Coordinate == "link.go" {
			t.Fatalf("symlink target observed: %#v", n)
		}
	}
}
func TestObserveGitDirtyAndChangeFacts(t *testing.T) {
	r := fixture(t)
	for _, args := range [][]string{{"init", "-q", r}, {"-C", r, "config", "user.email", "scanner@example.invalid"}, {"-C", r, "config", "user.name", "Scanner"}} {
		if out, e := exec.Command("git", args...).CombinedOutput(); e != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	p := filepath.Join(r, "tracked.txt")
	if e := os.WriteFile(p, []byte("base"), 0600); e != nil {
		t.Fatal(e)
	}
	for _, args := range [][]string{{"-C", r, "add", "tracked.txt"}, {"-C", r, "commit", "-qm", "fixture"}} {
		if out, e := exec.Command("git", args...).CombinedOutput(); e != nil {
			t.Fatalf("git commit: %s", out)
		}
	}
	if e := os.WriteFile(p, []byte("changed"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(r, "new.txt"), []byte("new"), 0600); e != nil {
		t.Fatal(e)
	}
	b, e := Observe(Request{Path: r, SubjectID: "fixture"})
	if e != nil {
		t.Fatal(e)
	}
	if b.Subject.WorkingTree != "DIRTY" {
		t.Fatalf("working tree = %s", b.Subject.WorkingTree)
	}
	changes := 0
	for _, n := range b.Nodes {
		if n.Kind == "git.change" {
			changes++
		}
	}
	if changes < 2 {
		t.Fatalf("expected tracked and untracked facts, got %d", changes)
	}
}
func TestObserveManifestsAndNormalize(t *testing.T) {
	r := fixture(t)
	_ = os.Mkdir(filepath.Join(r, "mod"), 0755)
	_ = os.WriteFile(filepath.Join(r, "go.mod"), []byte("module example.test/root\n"), 0600)
	_ = os.WriteFile(filepath.Join(r, "main.go"), []byte("package main\n"), 0600)
	_ = os.WriteFile(filepath.Join(r, "package.json"), []byte(`{"name":"root","workspaces":["mod","mod"]}`), 0600)
	_ = os.WriteFile(filepath.Join(r, "mod", "package.json"), []byte(`{}`), 0600)
	_ = os.WriteFile(filepath.Join(r, "mod", "go.mod"), []byte("module example.test/mod\n"), 0600)
	_ = os.WriteFile(filepath.Join(r, "go.work"), []byte("go 1.27\nuse (\n .\n ./mod\n)\nuse\t./mod\n"), 0600)
	_ = os.WriteFile(filepath.Join(r, "pkg_test.go"), []byte("package main_test\n"), 0600)
	b, e := Observe(Request{Path: r, SubjectID: "fixture"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Normalize(b); e != nil {
		t.Fatalf("normalization: %v edges=%#v", e, b.Edges)
	}
	pkgs := map[string]bool{}
	for _, n := range b.Nodes {
		if n.Kind == "go.package" {
			pkgs[n.Name] = true
		}
	}
	if !pkgs["main"] || !pkgs["main_test"] {
		t.Fatalf("missing package clause identities: %#v", pkgs)
	}
	b2, e := Observe(Request{Path: r, SubjectID: "fixture"})
	if e != nil {
		t.Fatal(e)
	}
	s1, e := BuildSnapshot(b, Config{})
	if e != nil {
		t.Fatal(e)
	}
	s2, e := BuildSnapshot(b2, Config{})
	if e != nil {
		t.Fatal(e)
	}
	j1, _ := SerializeSnapshot(s1)
	j2, _ := SerializeSnapshot(s2)
	if string(j1) != string(j2) {
		t.Fatal("mixed package / workspace output was not deterministic")
	}
}

func TestObserveUnsupportedManifestSyntaxRemainsUnknown(t *testing.T) {
	r := fixture(t)
	_ = os.Mkdir(filepath.Join(r, "bad"), 0755)
	_ = os.WriteFile(filepath.Join(r, "package.json"), []byte(`null`), 0600)
	_ = os.WriteFile(filepath.Join(r, "bad", "go.mod"), []byte("module example.test/bad\nmodule ${DYNAMIC}\n"), 0600)
	_ = os.WriteFile(filepath.Join(r, "go.work"), []byte("go 1.27\nuse\t\n"), 0600)
	_ = os.WriteFile(filepath.Join(r, "pnpm-workspace.yaml"), []byte("packages: ['*']\n"), 0600)
	b, e := Observe(Request{Path: r, SubjectID: "unsupported-fixture"})
	if e != nil {
		t.Fatal(e)
	}
	seen := map[string]bool{}
	for _, u := range b.Unknowns {
		seen[u.Kind] = true
	}
	for _, k := range []string{"npm_manifest", "go_module", "go_workspace", "unsupported_manifest"} {
		if !seen[k] {
			t.Fatalf("missing unknown for %s: %#v", k, b.Unknowns)
		}
	}
	if _, e := BuildSnapshot(b, Config{}); e != nil {
		t.Fatalf("unsupported syntax must remain a valid partial observation: %v", e)
	}
}

func TestObserveDeterministicReadOnly(t *testing.T) {
	r := fixture(t)
	initGitFixture(t, r)
	_ = os.Mkdir(filepath.Join(r, "pkg"), 0755)
	_ = os.WriteFile(filepath.Join(r, "go.mod"), []byte("module example.test/demo\n"), 0600)
	_ = os.WriteFile(filepath.Join(r, "pkg", "main.go"), []byte("package pkg\n"), 0600)
	_ = os.WriteFile(filepath.Join(r, "README.md"), []byte("fixture\n"), 0600)
	gitRun(t, r, "add", ".")
	gitRun(t, r, "commit", "-qm", "fixture")
	before := treeFingerprint(t, r)
	statusBefore := string(gitRun(t, r, "status", "--porcelain=v1", "-z", "--untracked-files=all"))
	first, e := Observe(Request{Path: r, SubjectID: "e2e-fixture"})
	if e != nil {
		t.Fatal(e)
	}
	s1, e := BuildSnapshot(first, Config{})
	if e != nil {
		t.Fatal(e)
	}
	c1, e := Derive(s1)
	if e != nil {
		t.Fatal(e)
	}
	second, e := Observe(Request{Path: r, SubjectID: "e2e-fixture"})
	if e != nil {
		t.Fatal(e)
	}
	s2, e := BuildSnapshot(second, Config{})
	if e != nil {
		t.Fatal(e)
	}
	c2, e := Derive(s2)
	if e != nil {
		t.Fatal(e)
	}
	a1, _ := SerializeSnapshot(s1)
	a2, _ := SerializeSnapshot(s2)
	d1, _ := SerializeCandidates(c1)
	d2, _ := SerializeCandidates(c2)
	if string(a1) != string(a2) || string(d1) != string(d2) {
		t.Fatal("repeated observation artifacts differ")
	}
	if s1.Digests != s2.Digests || c1.Digest != c2.Digest {
		t.Fatal("repeated digests differ")
	}
	t.Logf("structure=%s artifact=%s candidates=%s", s1.Digests.Structure, s1.Digests.Artifact, c1.Digest)
	if after := treeFingerprint(t, r); after != before {
		t.Fatal("Observe changed a target file, including Git metadata")
	}
	statusAfter := string(gitRun(t, r, "status", "--porcelain=v1", "-z", "--untracked-files=all"))
	if statusAfter != statusBefore {
		t.Fatal("Observe changed Git status")
	}
}

func TestObserveDisablesConfiguredFsmonitorHook(t *testing.T) {
	r := fixture(t)
	initGitFixture(t, r)
	if e := os.WriteFile(filepath.Join(r, "tracked"), []byte("tracked"), 0600); e != nil {
		t.Fatal(e)
	}
	gitRun(t, r, "add", "tracked")
	gitRun(t, r, "commit", "-qm", "fixture")
	marker := filepath.Join(t.TempDir(), "hook-ran")
	hook := filepath.Join(t.TempDir(), "fsmonitor-hook")
	script := "#!/bin/sh\ntouch '" + marker + "'\nprintf 'token\\n\\n'\n"
	if e := os.WriteFile(hook, []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	gitRun(t, r, "config", "core.fsmonitor", hook)
	// Prime and remove any marker that this Git version's hook protocol emits.
	_, _ = exec.Command("git", "-C", r, "status", "--porcelain=v1").CombinedOutput()
	if _, e := os.Stat(marker); e != nil {
		t.Skip("Git did not invoke the configured fsmonitor hook")
	}
	_ = os.Remove(marker)
	if _, e := Observe(Request{Path: r, SubjectID: "fsmonitor-fixture"}); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("Observe ran configured fsmonitor hook")
	}
}

func initGitFixture(t *testing.T, r string) {
	t.Helper()
	gitRun(t, r, "init", "-q")
	gitRun(t, r, "config", "user.email", "scanner@example.invalid")
	gitRun(t, r, "config", "user.name", "Scanner")
}
func gitRun(t *testing.T, r string, args ...string) []byte {
	t.Helper()
	full := append([]string{"-C", r}, args...)
	out, e := exec.Command("git", full...).CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %s", args, out)
	}
	return out
}
func treeFingerprint(t *testing.T, root string) string {
	t.Helper()
	var rows []string
	e := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(root, p)
		h := sha256.Sum256(b)
		rows = append(rows, filepath.ToSlash(rel)+":"+hex.EncodeToString(h[:]))
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	sort.Strings(rows)
	h := sha256.Sum256([]byte(strings.Join(rows, "\n")))
	return fmt.Sprintf("%x", h)
}
