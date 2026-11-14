package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sorune/wsp/internal/config"
	"github.com/Sorune/wsp/internal/inspect"
)

func manifestFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	gitTestCommand(t, root, "init", "-q")
	gitTestCommand(t, root, "config", "user.name", "WSP Test")
	gitTestCommand(t, root, "config", "user.email", "wsp@example.invalid")
	gitTestCommand(t, root, "remote", "add", "origin", "https://example.invalid/group/repository.git")
	for name, value := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if len(files) == 0 {
		if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("fixture\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	gitTestCommand(t, root, "add", ".")
	gitTestCommand(t, root, "commit", "-qm", "fixture")
	return root
}

func gitTestCommand(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func captureManifestOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	callErr := fn()
	_ = w.Close()
	os.Stdout = old
	data, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	return string(data), callErr
}

func draft(t *testing.T, root string, extra ...string) (string, error) {
	t.Helper()
	args := append([]string{root}, extra...)
	return captureManifestOutput(t, func() error { return manifestDraftCommand(args) })
}

func TestManifestDraftDeterministicReadOnlyAndValid(t *testing.T) {
	root := manifestFixture(t, map[string]string{"go.mod": "module example.invalid/repo\n\ngo 1.21\n", "main.go": "package main\n", "internal/pkg/pkg.go": "package pkg\n"})
	first, err := draft(t, root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := draft(t, root)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("same revision produced different draft bytes")
	}
	if !strings.Contains(first, "STATUS: DRAFT") || !strings.Contains(first, "path: \".\"") {
		t.Fatalf("draft missing review status or portable path: %s", first)
	}
	if strings.Contains(first, "projects:") || strings.Contains(first, "relations:\n  -") || strings.Contains(first, "axis: \"session\"") {
		t.Fatalf("draft invented semantic declarations: %s", first)
	}
	if _, err := os.Stat(config.Path(root)); !os.IsNotExist(err) {
		t.Fatalf("draft wrote target manifest: err=%v", err)
	}
	path := filepath.Join(t.TempDir(), "draft.yaml")
	if err := os.WriteFile(path, []byte(first), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("generated manifest did not parse/validate: %v", err)
	}
	if m.Repositories[0].Path != "." || len(m.Relations) != 0 || len(m.Projects) != 0 {
		t.Fatalf("unsafe draft model: %+v", m)
	}
	if _, err := captureManifestOutput(t, func() error { return manifestValidateCommand([]string{"--input", path}) }); err != nil {
		t.Fatalf("read-only validate: %v", err)
	}
	jsonDraft, err := draft(t, root, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonDraft, `"kind": "go.module"`) || !strings.Contains(jsonDraft, `"path": "."`) || !strings.Contains(jsonDraft, `"proposal_status":`) {
		t.Fatalf("JSON proposal omitted concrete Scanner-grounded structure: %s", jsonDraft)
	}
	if len(jsonDraft) > defaultManifestOutputBudget {
		t.Fatalf("default JSON report exceeded budget: %d", len(jsonDraft))
	}
	// Lens only projects declared relations; a draft does not project Scanner
	// observations into logical or session trees.
	if err := config.Write(root, m); err != nil {
		t.Fatal(err)
	}
	_, logical, err := inspect.Tree(root, "logical")
	if err != nil {
		t.Fatal(err)
	}
	_, session, err := inspect.Tree(root, "session")
	if err != nil {
		t.Fatal(err)
	}
	if len(logical.Roots) != 0 || len(session.Roots) != 0 {
		t.Fatalf("draft projected inferred Lens relations: logical=%+v session=%+v", logical, session)
	}
}

func TestManifestDraftPreservesDirtyRepositoryAndUsesBoundedEvidence(t *testing.T) {
	root := manifestFixture(t, map[string]string{"pom.xml": "<project/>\n", "apps/a/package.json": `{"name":"a"}`, "apps/b/package.json": `{"name":"b"}`, "pnpm-workspace.yaml": "packages:\n  - 'apps/*'\n"})
	if err := os.WriteFile(filepath.Join(root, "local-only.txt"), []byte("dirty\n"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := exec.Command("git", "-C", root, "status", "--porcelain=v1").Output()
	if err != nil {
		t.Fatal(err)
	}
	out, err := draft(t, root, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains([]byte(out), []byte(`"status": "DRAFT"`)) || !bytes.Contains([]byte(out), []byte(`"maven_manifest": 1`)) || !bytes.Contains([]byte(out), []byte(`"npm.package": 2`)) || !bytes.Contains([]byte(out), []byte(`"artifact_id_missing"`)) {
		t.Fatalf("missing truthful bounded Scanner summary: %s", out)
	}
	if strings.Contains(out, "local-only.txt") || strings.Contains(out, "<project/>") {
		t.Fatalf("raw Scanner content leaked to review report: %s", out)
	}
	after, err := exec.Command("git", "-C", root, "status", "--porcelain=v1").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("draft changed dirty repository status: before=%q after=%q", before, after)
	}
	if _, err := os.Stat(config.Path(root)); !os.IsNotExist(err) {
		t.Fatalf("draft created .wsp: err=%v", err)
	}
}

func TestManifestDraftPortableAcrossCheckoutAndBudgetFailsClosed(t *testing.T) {
	root := manifestFixture(t, nil)
	first, err := draft(t, root)
	if err != nil {
		t.Fatal(err)
	}
	clone := t.TempDir()
	if out, err := exec.Command("git", "clone", "-q", root, clone).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v %s", err, out)
	}
	gitTestCommand(t, clone, "remote", "set-url", "origin", "https://example.invalid/group/repository.git")
	second, err := draft(t, clone)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("relocated checkout changed root-relative draft bytes")
	}
	if _, err := draft(t, root, "--json", "--max-output-bytes", "1024"); err == nil || !strings.Contains(err.Error(), "OUTPUT_BUDGET_EXCEEDED") {
		t.Fatalf("expected fail-closed output budget, got %v", err)
	}
}

func TestManifestValidateRejectsInvalidInputWithoutMutation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bad.yaml")
	if err := os.WriteFile(path, []byte("schema_version: 1\nworkspace_id: \"workspace:test\"\nrepositories: []\nrelations:\n  - id: \"bad\"\n    type: \"contains\"\n    from: \"workspace:test\"\n    to: \"repository:missing\"\n    axis: \"logical\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := captureManifestOutput(t, func() error { return manifestValidateCommand([]string{"--input", path}) }); err == nil {
		t.Fatal("invalid endpoint unexpectedly validated")
	}
	if _, err := os.Stat(filepath.Join(root, ".wsp")); !os.IsNotExist(err) {
		t.Fatalf("validation mutated target: %v", err)
	}
}
