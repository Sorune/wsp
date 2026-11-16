package scanner

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryEvidencePortableAcrossCheckoutRoots(t *testing.T) {
	r := fixture(t)
	initGitFixture(t, r)
	if err := os.WriteFile(filepath.Join(r, "go.mod"), []byte("module example.invalid/demo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, r, "add", ".")
	gitRun(t, r, "commit", "-qm", "fixture")
	other := fixture(t)
	gitRun(t, other, "clone", "-q", r, ".")
	var previous []byte
	for _, root := range []string{r, other} {
		b, err := Observe(Request{Path: root, SubjectID: "portable-evidence"})
		if err != nil {
			t.Fatal(err)
		}
		s, err := BuildSnapshot(b, Config{})
		if err != nil {
			t.Fatal(err)
		}
		data, err := SerializeSnapshot(s)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range s.Evidence {
			if !validCoordinate(e.Path) {
				t.Fatalf("nonportable observed evidence path: %q", e.Path)
			}
		}
		if previous != nil && !bytes.Equal(previous, data) {
			t.Fatal("identical checkout changed serialized snapshot")
		}
		previous = data
	}
}

func TestUnsupportedManifestsRecognizedWithoutParsingOrPromotion(t *testing.T) {
	r := fixture(t)
	expected := map[string]string{
		"setup.py":           "python_declaration_not_supported",
		"setup.cfg":          "python_declaration_not_supported",
		"Makefile":           "build_manifest_not_supported",
		"pnpm-workspace.yml": "workspace_manifest_not_supported",
	}
	files := map[string]string{"node_modules/requirements.txt": "DEMO_SECRET_NOT_REAL", "tsconfig.json": "{}", "Dockerfile": "FROM scratch", "compose.yaml": "services: {}", "Cargo.lock": "not parsed", "package.json": "{\"name\":\"demo\"}"}
	for n := range expected {
		files[n] = "DEMO_SECRET_NOT_REAL"
	}
	for n, v := range files {
		p := filepath.Join(r, n)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(v), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := Config{ExcludeDirNames: []string{"node_modules"}}
	b, err := Observe(Request{Path: r, SubjectID: "unsupported-manifests", Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	s, err := BuildSnapshot(b, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if coverageState(b, "unsupported_manifest") != "UNSUPPORTED" {
		t.Fatal("unsupported coverage not reported")
	}
	seen := map[string]bool{}
	for _, u := range s.Unknowns {
		if u.Kind != "unsupported_manifest" {
			continue
		}
		if expected[u.Coordinate] != u.Reason || len(u.EvidenceRefs) != 1 {
			t.Fatalf("wrong unsupported classification: %+v", u)
		}
		seen[u.Coordinate] = true
	}
	if len(seen) != len(expected) {
		t.Fatalf("unsupported count=%d, want %d", len(seen), len(expected))
	}
	for _, n := range s.Nodes {
		if n.Kind == "unsupported_manifest" || n.Kind == "python.project" || n.Kind == "gradle.project" {
			t.Fatal("unsupported content became a parsed declaration")
		}
	}
	raw, err := SerializeSnapshot(s)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("DEMO_SECRET_NOT_REAL")) {
		t.Fatal("unsupported raw content leaked")
	}
}
func TestUnsupportedManifestAbsentInSupportedOnlyScope(t *testing.T) {
	r := fixture(t)
	if err := os.WriteFile(filepath.Join(r, "package.json"), []byte("{\"name\":\"demo\"}"), 0600); err != nil {
		t.Fatal(err)
	}
	b, err := Observe(Request{Path: r, SubjectID: "supported-only"})
	if err != nil {
		t.Fatal(err)
	}
	if coverageState(b, "unsupported_manifest") != "NOT_APPLICABLE" || hasUnknownKind(&b, "unsupported_manifest") {
		t.Fatal("false unsupported classification")
	}
}
