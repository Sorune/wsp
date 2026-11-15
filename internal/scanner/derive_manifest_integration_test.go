package scanner

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDeriveStaticManifestBoundariesStayUnresolved(t *testing.T) {
	root := fixture(t)
	initGitFixture(t, root)
	files := map[string]string{
		"settings.gradle":    "rootProject.name = 'demo'\ninclude ':missing'\n",
		"build.gradle":       "group = 'example'\nversion = '1'\n",
		"java/pom.xml":       "<project><artifactId>demo</artifactId><modules><module>missing</module></modules></project>",
		"rust/Cargo.toml":    "[package]\nname = \"demo\"\nversion = \"1\"\n[workspace]\nmembers = [\"missing\"]\n",
		"py/pyproject.toml":  "[project]\nname = \"demo\"\n",
		"ignored/pom.xml":    "<project><artifactId>ignored</artifactId></project>",
		"py/ignored/pom.xml": "<project><artifactId>ignored</artifactId></project>",
		".gitignore":         "ignored/\n",
	}
	for name, src := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0600); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "-qm", "fixture")
	b, e := Observe(Request{Path: root, SubjectID: "static-manifest-test"})
	if e != nil {
		t.Fatal(e)
	}
	snap, e := BuildSnapshot(b, Config{})
	if e != nil {
		t.Fatal(e)
	}
	a, e := Derive(snap)
	if e != nil {
		t.Fatal(e)
	}
	seen := map[string]bool{}
	for _, c := range a.Candidates {
		for _, sig := range c.Signals {
			if sig.Kind != "manifest_boundary" {
				continue
			}
			if len(sig.NodeIDs) == 0 || len(sig.EvidenceRefs) == 0 {
				t.Fatal("missing provenance")
			}
			if c.Confidence != ConfidenceUnresolved || !c.ReviewRequired || !contains(c.ReviewReasons, "static_manifest_partial") {
				t.Fatalf("static declaration incorrectly promoted: %+v", c)
			}
			seen[c.Coordinate] = true
		}
	}
	if len(seen) != 4 || !seen["."] || !seen["java"] || !seen["rust"] || !seen["py"] {
		t.Fatalf("unexpected candidate boundaries: %v", seen)
	}
	if seen["ignored"] || seen["py/ignored"] || seen["missing"] {
		t.Fatal("invented or ignored boundary")
	}
	again, e := Derive(snap)
	if e != nil {
		t.Fatal(e)
	}
	j1, _ := SerializeCandidates(a)
	j2, _ := SerializeCandidates(again)
	if !bytes.Equal(j1, j2) {
		t.Fatal("candidate output not deterministic")
	}
}

func TestDeriveNewManifestRequiresExactSourceEvidence(t *testing.T) {
	f := newEligibilityFixture("invalid-static-boundary")
	f.addDirectory(".")
	f.addDirectory("other")
	ref := f.addEvidence("maven_pom", "pom.xml", "project")
	id := NodeID(f.subject, "maven.project", "pom.xml#project")
	f.batch.Nodes = append(f.batch.Nodes, Node{ID: id, Kind: "maven.project", Coordinate: "pom.xml#project", Name: "demo", Attributes: map[string]string{"directory": "other"}, EvidenceRefs: []string{ref}})
	out, err := Derive(f.snapshot(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range out.Candidates {
		for _, sig := range c.Signals {
			if sig.Kind == "manifest_boundary" {
				t.Fatal("mismatched declaring coordinate became a boundary")
			}
		}
	}
}
