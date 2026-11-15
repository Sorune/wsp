package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func TestManifestDraftNewStaticProposalsAndExplicitScope(t *testing.T) {
	root := manifestFixture(t, map[string]string{
		"settings.gradle":             "rootProject.name = 'demo'\ninclude ':unobserved'\n",
		"java/pom.xml":                "<project><artifactId>demo</artifactId><modules><module>missing</module></modules></project>",
		"rust/Cargo.toml":             "[package]\nname = \"demo\"\nversion = \"1\"\n[workspace]\nmembers = [\"missing\"]\n",
		"py/pyproject.toml":           "[project]\nname = \"demo\"\n",
		"node_modules/hidden/pom.xml": "<project><artifactId>hidden-sentinel</artifactId></project>",
		"build/pyproject.toml":        "[project]\nname = \"hidden-sentinel\"\n",
	})
	args := []string{"--json", "--exclude-dir-name", "node_modules", "--exclude-dir-name", "build"}
	out, err := draft(t, root, args...)
	if err != nil {
		t.Fatal(err)
	}
	var r manifestDraftReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	if r.Status != "DRAFT" || r.Mutation != "NONE" || r.ProjectCount != 0 || r.RelationCount != 0 || r.ProposalStatus != "PARTIAL" {
		t.Fatal("unsafe proposal status")
	}
	if strings.Join(r.ExcludeDirNames, ",") != "build,node_modules" || strings.Join(r.ExcludedBoundaries, ",") != "build,node_modules" {
		t.Fatalf("scope not retained: %#v %#v", r.ExcludeDirNames, r.ExcludedBoundaries)
	}
	expected := map[string]bool{"gradle.project": false, "gradle.included-project": false, "maven.project": false, "maven.module": false, "cargo.package": false, "cargo.member": false, "python.project": false}
	for _, e := range r.ProposedEntities {
		if _, ok := expected[e.Kind]; ok {
			if e.Status != "PROPOSED" || e.DeclarationPath == "" || len(e.Evidence) == 0 {
				t.Fatal("incomplete declaration provenance")
			}
			expected[e.Kind] = true
		}
		if strings.Contains(e.Path, "node_modules") || strings.Contains(e.Name, "hidden-sentinel") {
			t.Fatal("excluded subtree leaked")
		}
	}
	for k, v := range expected {
		if !v {
			t.Fatalf("missing proposal %s", k)
		}
	}
	if r.Observed["maven.project"] != 1 || r.Observed["python.project"] != 1 {
		t.Fatal("excluded manifests observed")
	}
	count := 0
	for _, c := range r.StructuralCandidates {
		if c.Confidence == "UNRESOLVED" && c.ReviewRequired && strings.Contains(strings.Join(c.ReviewReasons, ","), "static_manifest_partial") {
			count++
		}
	}
	if count != 4 {
		t.Fatalf("new manifest candidate count %d", count)
	}
	again, err := draft(t, root, "--json", "--exclude-dir-name", "build", "--exclude-dir-name", "node_modules")
	if err != nil || out != again {
		t.Fatal("scope normalization not deterministic")
	}
	yaml, err := draft(t, root, "--exclude-dir-name", "build", "--exclude-dir-name", "node_modules")
	if err != nil || !strings.Contains(yaml, "# SCAN_SCOPE:") {
		t.Fatal("scoped YAML missing scope provenance")
	}
	if _, err := os.Stat(config.Path(root)); !os.IsNotExist(err) {
		t.Fatal("draft created target manifest")
	}
	for _, bad := range [][]string{{"--exclude-dir-name"}, {"--exclude-dir-name", "a/b"}, {"--exclude-dir-name", "build", "--exclude-dir-name", "build"}} {
		if _, err := draft(t, root, bad...); err == nil {
			t.Fatal("invalid scope accepted")
		}
	}
}

func TestManifestDraftTrimKeepsRelationEndpoints(t *testing.T) {
	r := manifestDraftReport{Status: "DRAFT", Mutation: "NONE"}
	for i := 0; i < 24; i++ {
		id := fmt.Sprintf("entity-%02d", i)
		r.ProposedEntities = append(r.ProposedEntities, proposalEntity{ID: id, Kind: "directory", Name: strings.Repeat("n", 80), Path: strings.Repeat("p", 80), Status: "PROPOSED"})
		if i > 0 {
			r.ProposedRelations = append(r.ProposedRelations, proposalRelation{ID: fmt.Sprintf("edge-%02d", i), From: "entity-00", To: id, Status: "PROPOSED"})
		}
	}
	r.TotalEntities = len(r.ProposedEntities)
	r.TotalRelations = len(r.ProposedRelations)
	out, err := marshalBoundedDraftReport(&r, 4096)
	if err != nil {
		t.Fatal(err)
	}
	var got manifestDraftReport
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, e := range got.ProposedEntities {
		ids[e.ID] = true
	}
	for _, e := range got.ProposedRelations {
		if !ids[e.From] || !ids[e.To] {
			t.Fatal("dangling relation")
		}
	}
	if got.OmittedEntities == 0 || got.OmittedRelations == 0 || got.TotalRelations != len(got.ProposedRelations)+got.OmittedRelations || got.ProposalStatus != "PARTIAL" {
		t.Fatal("inconsistent trim counts")
	}
}

func TestManifestDraftIgnoredStaticDeclarationIsPhysicalOnly(t *testing.T) {
	root := manifestFixture(t, map[string]string{".gitignore": "android/\n", "package.json": "{\"name\":\"demo\"}"})
	if err := os.MkdirAll(filepath.Join(root, "android"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "android", "settings.gradle"), []byte("rootProject.name = 'demo'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := draft(t, root, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var r manifestDraftReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range r.ProposedEntities {
		if e.Kind == "gradle.project" {
			found = e.GitIgnoredSource && e.DeclarationPath == "android/settings.gradle"
		}
	}
	if !found {
		t.Fatal("ignored physical observation is missing its source classification")
	}
	for _, c := range r.StructuralCandidates {
		if c.Path == "android" || strings.HasPrefix(c.Path, "android/") {
			t.Fatal("ignored source became structural candidate")
		}
	}
}

func TestManifestDraftReportsOmittedCandidateParent(t *testing.T) {
	report := manifestDraftReport{
		Status: "DRAFT", Mutation: "NONE",
		StructuralCandidates: []proposalCandidate{
			{ID: "child", Path: "modules/java", ParentID: "parent", Confidence: "UNRESOLVED", ReviewRequired: true},
			{ID: "root", Path: ".", Confidence: "WEAK"},
			{ID: "other", Path: "modules/other", ParentID: "root", Confidence: "WEAK"},
		},
		TotalCandidates: 4,
	}
	raw, err := marshalBoundedDraftReport(&report, 16384)
	if err != nil {
		t.Fatal(err)
	}
	var got manifestDraftReport
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if !got.StructuralCandidates[0].ParentOmitted || got.StructuralCandidates[0].ParentID != "parent" {
		t.Fatal("omitted parent not preserved and marked")
	}
	if got.StructuralCandidates[1].ParentOmitted || got.StructuralCandidates[2].ParentOmitted {
		t.Fatal("visible or root parent incorrectly marked omitted")
	}
	if got.OmittedCandidates != 1 || got.ProposalStatus != "PARTIAL" {
		t.Fatal("omission status incorrect")
	}
}
