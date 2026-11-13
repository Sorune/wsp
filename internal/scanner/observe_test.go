package scanner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

func TestObserveExplicitDirectoryExclusionsAreOpaqueAndDeterministic(t *testing.T) {
	r := fixture(t)
	if err := os.MkdirAll(filepath.Join(r, "node_modules", "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r, "node_modules", "nested", "package.json"), []byte(`{"name":"hidden"}`), 0600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(r, "node_modules", "nested", "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	config := Config{ExcludeDirNames: []string{"node_modules"}}
	b1, err := Observe(Request{Path: r, SubjectID: "exclusion-fixture", Config: config})
	if err != nil {
		t.Fatal(err)
	}
	b2, err := Observe(Request{Path: r, SubjectID: "exclusion-fixture", Config: config})
	if err != nil {
		t.Fatal(err)
	}
	s1, err := BuildSnapshot(b1, config)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := BuildSnapshot(b2, config)
	if err != nil {
		t.Fatal(err)
	}
	j1, _ := SerializeSnapshot(s1)
	j2, _ := SerializeSnapshot(s2)
	if !bytes.Equal(j1, j2) || s1.Digests != s2.Digests {
		t.Fatal("excluded observation is not deterministic")
	}
	if _, ok := findNode(b1, "directory", "node_modules"); !ok {
		t.Fatal("opaque excluded boundary is not visible")
	}
	if _, ok := findNode(b1, "file", "node_modules/nested/package.json"); ok {
		t.Fatal("excluded descendant was observed")
	}
	if hasUnknownAt(b1, "node_modules/nested/link") {
		t.Fatal("symlink inside excluded boundary was inspected")
	}
	filesystem := coverageState(b1, "filesystem")
	if filesystem != "COMPLETE" {
		t.Fatalf("filesystem coverage within declared scope = %s", filesystem)
	}
	if s1.Config.ExcludeDirNames == nil || len(s1.Config.ExcludeDirNames) != 1 {
		t.Fatalf("config exclusions not serialized: %#v", s1.Config)
	}
	candidates, err := Derive(s1)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates.Candidates) != 0 {
		t.Fatalf("excluded directory became a candidate: %#v", candidates.Candidates)
	}
	for _, edge := range s1.Edges {
		for _, node := range s1.Nodes {
			if node.Attributes["opaque"] == "configured_exclusion" && edge.From == node.ID {
				t.Fatalf("excluded directory has a containment relation: %#v", edge)
			}
		}
	}
	raw, err := SerializeSnapshot(s1)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip PhysicalSnapshot
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip.Digests.Config != s1.Digests.Config || len(roundTrip.Config.ExcludeDirNames) != 1 {
		t.Fatal("exclusion config did not survive artifact round trip")
	}
	noOptions, err := BuildSnapshot(b1, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if noOptions.Digests.Config == s1.Digests.Config {
		t.Fatal("exclusions did not affect config digest")
	}
	caseRoot := fixture(t)
	if err := os.Mkdir(filepath.Join(caseRoot, "Node_Modules"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, "Node_Modules", "visible.txt"), []byte("visible"), 0600); err != nil {
		t.Fatal(err)
	}
	caseBatch, err := Observe(Request{Path: caseRoot, SubjectID: "case-sensitive-exclusion", Config: config})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(caseBatch, "file", "Node_Modules/visible.txt"); !ok {
		t.Fatal("mixed-case directory was excluded by case-sensitive match")
	}
}

func TestObserveExcludedSourceRootRemainsPartial(t *testing.T) {
	r := fixture(t)
	_ = os.MkdirAll(filepath.Join(r, "node_modules", "pkg"), 0755)
	cfg := Config{SourceRoots: []string{"node_modules/pkg"}, ExcludeDirNames: []string{"node_modules"}}
	b, err := Observe(Request{Path: r, SubjectID: "excluded-source-root", Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if coverageState(b, "filesystem") != "PARTIAL" || !hasUnknownAt(b, "node_modules/pkg") {
		t.Fatalf("excluded source root did not fail closed: coverage=%#v unknowns=%#v", b.Coverage, b.Unknowns)
	}
}

func TestObserveUnrelatedAndBrokenSymlinksRemainUnknown(t *testing.T) {
	r := fixture(t)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"link": outside, "broken": filepath.Join(r, "missing-target")} {
		if err := os.Symlink(target, filepath.Join(r, name)); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
	}
	b, err := Observe(Request{Path: r, SubjectID: "symlink-controls", Config: Config{ExcludeDirNames: []string{"node_modules"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"link", "broken"} {
		found := false
		for _, u := range b.Unknowns {
			if u.Kind == "symlink" && u.Coordinate == name && u.Reason == "symlink_not_followed" {
				found = true
			}
		}
		if !found {
			t.Fatalf("unrelated symlink %q was not kept UNKNOWN: %#v", name, b.Unknowns)
		}
	}
}

func findNode(b ObservationBatch, kind, coordinate string) (Node, bool) {
	for _, n := range b.Nodes {
		if n.Kind == kind && n.Coordinate == coordinate {
			return n, true
		}
	}
	return Node{}, false
}
func hasUnknownAt(b ObservationBatch, coordinate string) bool {
	for _, u := range b.Unknowns {
		if u.Coordinate == coordinate {
			return true
		}
	}
	return false
}
func coverageState(b ObservationBatch, capability string) string {
	for _, c := range b.Coverage {
		if c.Capability == capability {
			return c.State
		}
	}
	return ""
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

func TestObserveGitMetadataRespectsDeclaredExclusionScope(t *testing.T) {
	r := fixture(t)
	initGitFixture(t, r)
	for p, data := range map[string]string{
		"node_modules/pkg/a.go":         "package pkg\n",
		"node_modules/pkg/package.json": `{"name":"excluded-pkg"}`,
		"outside.txt":                   "outside\n",
	} {
		full := filepath.Join(r, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, r, "add", ".")
	gitRun(t, r, "commit", "-qm", "fixture")
	config := Config{ExcludeDirNames: []string{"node_modules"}}
	clean, err := Observe(Request{Path: r, SubjectID: "excluded-git", Config: config})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range clean.Unknowns {
		if u.Coordinate == "node_modules/pkg/a.go" || u.Coordinate == "node_modules/pkg/package.json" {
			t.Fatalf("excluded tracked path reported UNKNOWN: %#v", u)
		}
	}
	for _, n := range clean.Nodes {
		if (n.Kind == "git.change" || n.Kind == "git.ignored") && strings.HasPrefix(n.Coordinate, "node_modules/") {
			t.Fatalf("excluded path emitted Git path node: %#v", n)
		}
	}
	if err := os.WriteFile(filepath.Join(r, "node_modules/pkg/a.go"), []byte("package pkg\n// changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r, "outside.txt"), []byte("outside changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := Observe(Request{Path: r, SubjectID: "excluded-git", Config: config})
	if err != nil {
		t.Fatal(err)
	}
	outsideChange := false
	for _, n := range changed.Nodes {
		if n.Kind == "git.change" && strings.HasPrefix(n.Coordinate, "node_modules/") {
			t.Fatalf("excluded modified path emitted git.change: %#v", n)
		}
		if n.Kind == "git.change" && n.Coordinate == "outside.txt" {
			outsideChange = true
		}
	}
	if !outsideChange {
		t.Fatal("outside-scope Git change was not retained")
	}
	for _, u := range changed.Unknowns {
		if u.Kind == "git_tracked_path" && strings.HasPrefix(u.Coordinate, "node_modules/") {
			t.Fatalf("excluded tracked path emitted UNKNOWN: %#v", u)
		}
	}
}

func TestObserveReadFailureOutsideExcludedScopeRemainsFailClosed(t *testing.T) {
	r := fixture(t)
	if err := os.WriteFile(filepath.Join(r, "unreadable.txt"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 {
		t.Skip("root can read mode-zero files")
	}
	if err := os.Chmod(filepath.Join(r, "unreadable.txt"), 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(r, "unreadable.txt"), 0600) })
	b, err := Observe(Request{Path: r, SubjectID: "read-failure-outside-exclusion", Config: Config{ExcludeDirNames: []string{"node_modules"}}})
	if err != nil {
		t.Fatal(err)
	}
	if coverageState(b, "filesystem") != "PARTIAL" || !hasUnknownAt(b, "unreadable.txt") {
		t.Fatalf("outside-scope read failure did not remain fail-closed: coverage=%#v unknowns=%#v", b.Coverage, b.Unknowns)
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
	for _, k := range []string{"npm_manifest", "go_module", "go_workspace", "pnpm_workspace"} {
		if !seen[k] {
			t.Fatalf("missing unknown for %s: %#v", k, b.Unknowns)
		}
	}
	if _, e := BuildSnapshot(b, Config{}); e != nil {
		t.Fatalf("unsupported syntax must remain a valid partial observation: %v", e)
	}
}

func TestObservePnpmWorkspaceLiteralPatterns(t *testing.T) {
	r := fixture(t)
	if err := os.WriteFile(filepath.Join(r, "package.json"), []byte(`{"name":"workspace-root"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"apps/studio", "packages/stable/core", "packages/internal/tool", "packages/public/content", "tools/checker", "outside/ignored"} {
		if err := os.MkdirAll(filepath.Join(r, dir), 0755); err != nil {
			t.Fatal(err)
		}
		name := strings.ReplaceAll(dir, "/", "-")
		if err := os.WriteFile(filepath.Join(r, dir, "package.json"), []byte(`{"name":"`+name+`"}`), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(r, "pnpm-workspace.yaml"), []byte("packages:\n  - '.'\n  - 'apps/*'\n  - 'packages/stable/*'\n  - 'packages/internal/*'\n  - 'packages/public/*'\n  - 'tools/*'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b, err := Observe(Request{Path: r, SubjectID: "pnpm-fixture"})
	if err != nil {
		t.Fatal(err)
	}
	members := map[string]bool{}
	for _, edge := range b.Edges {
		if edge.Kind != "workspace.use" {
			continue
		}
		for _, node := range b.Nodes {
			if node.ID == edge.To {
				members[node.Attributes["directory"]] = true
			}
		}
	}
	for _, dir := range []string{"apps/studio", "packages/stable/core", "packages/internal/tool", "packages/public/content", "tools/checker"} {
		if !members[dir] {
			t.Errorf("expected pnpm membership for %s", dir)
		}
	}
	if !members["."] {
		t.Fatal("literal root package membership was not observed")
	}
	if members["outside/ignored"] {
		t.Fatal("non-matching package was included")
	}
	for _, coverage := range b.Coverage {
		if coverage.Capability == "pnpm_workspaces" && coverage.State != "COMPLETE" {
			t.Fatalf("pnpm coverage = %s", coverage.State)
		}
	}
	first, _ := SerializeSnapshot(mustSnapshot(t, b))
	b2, err := Observe(Request{Path: r, SubjectID: "pnpm-fixture"})
	if err != nil {
		t.Fatal(err)
	}
	second, _ := SerializeSnapshot(mustSnapshot(t, b2))
	if string(first) != string(second) {
		t.Fatal("pnpm observation was not deterministic")
	}
}

func TestObservePnpmMalformedAndUnmatchedStayPartial(t *testing.T) {
	for _, content := range []string{"packages:\n", "packages: ['apps/*']\n", "packages:\n  - '!apps/*'\n", "packages:\n  - 'apps/*'\n  - 'missing/*'\n"} {
		r := fixture(t)
		_ = os.MkdirAll(filepath.Join(r, "apps", "studio"), 0755)
		_ = os.WriteFile(filepath.Join(r, "apps", "studio", "package.json"), []byte(`{"name":"studio"}`), 0600)
		_ = os.WriteFile(filepath.Join(r, "pnpm-workspace.yaml"), []byte(content), 0600)
		b, err := Observe(Request{Path: r, SubjectID: "pnpm-unknown"})
		if err != nil {
			t.Fatal(err)
		}
		state := ""
		for _, c := range b.Coverage {
			if c.Capability == "pnpm_workspaces" {
				state = c.State
			}
		}
		if state != "PARTIAL" {
			t.Fatalf("input %q coverage = %q, want PARTIAL", content, state)
		}
		if len(b.Unknowns) == 0 {
			t.Fatalf("input %q emitted no unknown", content)
		}
	}
}

func mustSnapshot(t *testing.T, b ObservationBatch) PhysicalSnapshot {
	t.Helper()
	s, err := BuildSnapshot(b, Config{})
	if err != nil {
		t.Fatal(err)
	}
	return s
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
