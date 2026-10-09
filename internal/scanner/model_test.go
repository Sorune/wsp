package scanner

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func testBatch() ObservationBatch {
	sub := "repo:one"
	addEvidence := func(kind, path string, line int) Evidence {
		e := Evidence{Kind: kind, Path: path, Line: line, Column: 2, Detail: "observed"}
		e.ID = StableID("evidence", e.Kind, e.Path, itoa(e.Line), itoa(e.Column), e.Detail)
		return e
	}
	e1 := addEvidence("source", "/tmp/checkout/src/a.go", 3)
	e2 := addEvidence("source", "/tmp/checkout/src/b.go", 5)
	e3 := addEvidence("git", "src/b.go", 0)
	n1 := Node{ID: NodeID(sub, "file", "src/a.go"), Kind: "file", Coordinate: "src/a.go", Name: "a.go", Attributes: map[string]string{"package": "app", "hash": "aaa"}, EvidenceRefs: []string{e1.ID}}
	n2 := Node{ID: NodeID(sub, "file", "src/b.go"), Kind: "file", Coordinate: "src/b.go", Name: "b.go", Attributes: map[string]string{"package": "app", "hash": "bbb"}, EvidenceRefs: []string{e2.ID, e3.ID}}
	n3 := Node{ID: NodeID(sub, "directory", "src"), Kind: "directory", Coordinate: "src", Name: "src", Attributes: map[string]string{}, EvidenceRefs: []string{}}
	edge := Edge{Kind: "contains", From: n3.ID, To: n1.ID, Resolution: "RESOLVED", EvidenceRefs: []string{e1.ID}}
	edge.ID = EdgeID(edge.Kind, edge.From, edge.To)
	u := Unknown{Kind: "parse", Coordinate: "src/c.go", Reason: "unsupported_syntax"}
	u.ID = StableID("unknown", sub, u.Kind, u.Coordinate)
	u.EvidenceRefs = []string{e3.ID}
	return ObservationBatch{Producer: Producer{ID: "scanner", Version: ScannerVersion, ModelVersion: ObservationModelVersion, DerivationModelVersion: DerivationModelVersion}, Subject: Subject{ID: sub, Revision: "abc", Branch: "main", WorkingTree: "clean", Root: "/tmp/checkout"}, Nodes: []Node{n1, n2, n3}, Edges: []Edge{edge}, Evidence: []Evidence{e1, e2, e3}, Coverage: []Coverage{{Capability: "go", State: "PARTIAL"}, {Capability: "filesystem", State: "COMPLETE"}}, Unknowns: []Unknown{u}, Diagnostics: []Diagnostic{{Code: "PARSE", Severity: "WARNING", Coordinate: "src/c.go", Message: "unknown syntax"}, {Code: "READ", Severity: "INFO", Coordinate: "src/b.go", Message: "read"}}}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	return string(rune('0' + n))
}

func TestSnapshotOrderingAndLocationDigests(t *testing.T) {
	b := testBatch()
	a, err := BuildSnapshot(b, Config{SourceRoots: []string{"src", "."}})
	if err != nil {
		t.Fatal(err)
	}
	sh := b
	sh.Nodes = append([]Node(nil), b.Nodes...)
	sh.Edges = append([]Edge(nil), b.Edges...)
	sh.Evidence = append([]Evidence(nil), b.Evidence...)
	sh.Coverage = append([]Coverage(nil), b.Coverage...)
	sh.Unknowns = append([]Unknown(nil), b.Unknowns...)
	sh.Diagnostics = append([]Diagnostic(nil), b.Diagnostics...)
	reverseNodes := func() {
		for i, j := 0, len(sh.Nodes)-1; i < j; i, j = i+1, j-1 {
			sh.Nodes[i], sh.Nodes[j] = sh.Nodes[j], sh.Nodes[i]
		}
	}
	reverseNodes()
	for i := range sh.Nodes {
		for j, k := 0, len(sh.Nodes[i].EvidenceRefs)-1; j < k; j, k = j+1, k-1 {
			sh.Nodes[i].EvidenceRefs[j], sh.Nodes[i].EvidenceRefs[k] = sh.Nodes[i].EvidenceRefs[k], sh.Nodes[i].EvidenceRefs[j]
		}
	}
	for _, xs := range [][]Evidence{sh.Evidence} {
		for i, j := 0, len(xs)-1; i < j; i, j = i+1, j-1 {
			xs[i], xs[j] = xs[j], xs[i]
		}
	}
	for i, j := 0, len(sh.Edges)-1; i < j; i, j = i+1, j-1 {
		sh.Edges[i], sh.Edges[j] = sh.Edges[j], sh.Edges[i]
	}
	for i, j := 0, len(sh.Coverage)-1; i < j; i, j = i+1, j-1 {
		sh.Coverage[i], sh.Coverage[j] = sh.Coverage[j], sh.Coverage[i]
	}
	for i, j := 0, len(sh.Unknowns)-1; i < j; i, j = i+1, j-1 {
		sh.Unknowns[i], sh.Unknowns[j] = sh.Unknowns[j], sh.Unknowns[i]
	}
	for i, j := 0, len(sh.Diagnostics)-1; i < j; i, j = i+1, j-1 {
		sh.Diagnostics[i], sh.Diagnostics[j] = sh.Diagnostics[j], sh.Diagnostics[i]
	}
	c, err := BuildSnapshot(sh, Config{SourceRoots: []string{".", "src"}})
	if err != nil {
		t.Fatal(err)
	}
	aj, _ := SerializeSnapshot(a)
	cj, _ := SerializeSnapshot(c)
	if !bytes.Equal(aj, cj) {
		t.Fatal("equivalent normalized snapshots serialized differently")
	}
	if a.Digests != c.Digests {
		t.Fatal("shuffled input changed a digest")
	}
	changed := testBatch()
	changed.Subject.Revision = "def"
	changed.Subject.Root = "/different/location"
	changed.Evidence[0].Path = "/different/location/src/a.go"
	changed.Evidence[0].Line++
	e := &changed.Evidence[0]
	e.ID = StableID("evidence", e.Kind, e.Path, itoa(e.Line), itoa(e.Column), e.Detail)
	oldID := StableID("evidence", "source", "/tmp/checkout/src/a.go", "3", "2", "observed")
	for i := range changed.Nodes {
		for j, r := range changed.Nodes[i].EvidenceRefs {
			if r == oldID {
				changed.Nodes[i].EvidenceRefs[j] = e.ID
			}
		}
	}
	changed.Edges[0].EvidenceRefs = []string{e.ID}
	d, err := BuildSnapshot(changed, Config{SourceRoots: []string{".", "src"}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Digests.Structure != a.Digests.Structure {
		t.Fatal("location, line, or revision changed structural digest")
	}
	if d.Digests.Evidence == a.Digests.Evidence {
		t.Fatal("evidence location did not change evidence digest")
	}
	if d.Digests.Artifact == a.Digests.Artifact {
		t.Fatal("evidence change did not change artifact digest")
	}
	changed.Nodes[0].Name = "renamed.go"
	f, err := BuildSnapshot(changed, Config{SourceRoots: []string{".", "src"}})
	if err != nil {
		t.Fatal(err)
	}
	if f.Digests.Structure == d.Digests.Structure {
		t.Fatal("structural content change did not change structure digest")
	}
}

func TestNormalizeRejectsInvalidCoordinatesAndReferences(t *testing.T) {
	b := testBatch()
	b.Nodes[0].Coordinate = "../escape"
	b.Nodes[0].ID = NodeID(b.Subject.ID, b.Nodes[0].Kind, b.Nodes[0].Coordinate)
	if _, err := Normalize(b); err == nil {
		t.Fatal("accepted traversal coordinate")
	}
	b = testBatch()
	b.Nodes[0].EvidenceRefs = []string{"missing"}
	if _, err := Normalize(b); err == nil {
		t.Fatal("accepted dangling evidence reference")
	}
	b = testBatch()
	b.Coverage = append(b.Coverage, b.Coverage[0])
	if _, err := Normalize(b); err == nil {
		t.Fatal("accepted duplicate coverage")
	}
	b = testBatch()
	b.Edges[0].Resolution = "MAYBE"
	if _, err := Normalize(b); err == nil {
		t.Fatal("accepted unsupported resolution")
	}
	b = testBatch()
	b.Unknowns = append(b.Unknowns, b.Unknowns[0])
	if _, err := Normalize(b); err == nil {
		t.Fatal("accepted duplicate unknown")
	}
	for _, c := range []string{`C:/absolute`, "src\\..\\secret", "src/../secret", "src\x00bad"} {
		if validCoordinate(c) {
			t.Fatalf("accepted invalid coordinate %q", c)
		}
	}
	if _, err := BuildSnapshot(testBatch(), Config{SourceRoots: []string{"src/../package"}}); err == nil {
		t.Fatal("accepted traversing source root")
	}
	if _, err := BuildSnapshot(testBatch(), Config{NamespacePrefixes: []string{"example"}}); err == nil {
		t.Fatal("accepted unsupported namespace prefixes")
	}
}

func TestObserveAndNormalizeUnicodeAndHashCoordinates(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "pkg#one")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "élan.go"), []byte("package café\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := Observe(Request{Path: root, SubjectID: "unicode-fixture"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := BuildSnapshot(b, Config{SourceRoots: []string{"pkg#one"}})
	if err != nil {
		t.Fatalf("normalization rejected valid hash/Unicode coordinates: %v", err)
	}
	foundFile, foundPackage := false, false
	for _, n := range s.Nodes {
		if n.Coordinate == "pkg#one/élan.go" && n.Kind == "file" {
			foundFile = true
		}
		if n.Kind == "go.package" && n.Name == "café" {
			foundPackage = true
		}
	}
	if !foundFile || !foundPackage {
		t.Fatalf("missing observed hash/Unicode facts: file=%v package=%v", foundFile, foundPackage)
	}
}

func TestNormalizeDoesNotMutateInput(t *testing.T) {
	b := testBatch()
	b.Nodes[0].EvidenceRefs = []string{b.Evidence[1].ID, b.Evidence[0].ID}
	before := append([]string(nil), b.Nodes[0].EvidenceRefs...)
	m := map[string]string{"z": "last", "a": "first"}
	b.Nodes[0].Attributes = m
	_, err := Normalize(b)
	if err != nil {
		t.Fatal(err)
	}
	if b.Nodes[0].EvidenceRefs[0] != before[0] || b.Nodes[0].EvidenceRefs[1] != before[1] {
		t.Fatal("input reference order changed")
	}
	if len(m) != 2 || m["z"] != "last" {
		t.Fatal("input attributes map changed")
	}
}
