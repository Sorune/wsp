package scanner

import (
	"strings"
	"testing"
)

func candidateSnapshot(subject string, nodes []Node) PhysicalSnapshot {
	return PhysicalSnapshot{Subject: Subject{ID: subject}, Producer: Producer{ID: "scanner", Version: ScannerVersion, ModelVersion: ObservationModelVersion, DerivationModelVersion: DerivationModelVersion}, Nodes: nodes}
}

func TestDeriveStructuralCandidatesDeterministicAndConfidence(t *testing.T) {
	s := candidateSnapshot("repo", []Node{
		{ID: NodeID("repo", "directory", "pkg"), Kind: "directory", Coordinate: "pkg", Name: "pkg"},
		{ID: NodeID("repo", "file", "pkg/a.go"), Kind: "file", Coordinate: "pkg/a.go", Name: "a.go"},
		{ID: NodeID("repo", "go.package", "pkg#package"), Kind: "go.package", Coordinate: "pkg#package", Name: "pkg", Attributes: map[string]string{"directory": "pkg", "package": "pkg"}},
	})
	s, err := BuildSnapshot(ObservationBatch{Producer: s.Producer, Subject: s.Subject, Nodes: s.Nodes}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := Derive(s)
	if err != nil {
		t.Fatal(err)
	}
	s.Nodes[0], s.Nodes[2] = s.Nodes[2], s.Nodes[0]
	b, err := Derive(s)
	if err != nil {
		t.Fatal(err)
	}
	if a.Digest != b.Digest || len(a.Candidates) != 1 || a.Candidates[0].Confidence != ConfidenceStrong {
		t.Fatalf("unstable or wrong result: %#v / %#v", a, b)
	}
	if a.Candidates[0].ReviewRequired {
		t.Fatalf("well-supported candidate without suspicion should not require review: %v", a.Candidates[0].ReviewReasons)
	}
}

func TestDeriveDoesNotCountDuplicateSignalsAsIndependent(t *testing.T) {
	s := candidateSnapshot("r", []Node{{ID: NodeID("r", "directory", "common"), Kind: "directory", Coordinate: "common", Name: "common"}, {ID: NodeID("r", "file", "common/x.go"), Kind: "file", Coordinate: "common/x.go", Name: "x.go"}})
	s, _ = BuildSnapshot(ObservationBatch{Producer: s.Producer, Subject: s.Subject, Nodes: s.Nodes}, Config{})
	r, err := Derive(s)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Candidates[0].Confidence; got != ConfidenceWeak {
		t.Fatalf("confidence=%s, want WEAK", got)
	}
	if !contains(r.Candidates[0].ReviewReasons, "generic_name_review") {
		t.Fatalf("generic label not flagged: %v", r.Candidates[0].ReviewReasons)
	}
	if !r.Candidates[0].ReviewRequired {
		t.Fatal("generic WEAK candidate must require review")
	}
}

func TestDerivePackageConflictAndUnmappedFiles(t *testing.T) {
	s := candidateSnapshot("r", []Node{
		{ID: NodeID("r", "directory", "x"), Kind: "directory", Coordinate: "x", Name: "x"},
		{ID: NodeID("r", "go.package", "x#package-a"), Kind: "go.package", Coordinate: "x#package-a", Name: "a", Attributes: map[string]string{"directory": "x", "package": "a"}},
		{ID: NodeID("r", "go.package", "x#package-b"), Kind: "go.package", Coordinate: "x#package-b", Name: "b", Attributes: map[string]string{"directory": "x", "package": "b"}},
		{ID: NodeID("r", "file", "solo.go"), Kind: "file", Coordinate: "solo.go", Name: "solo.go"},
	})
	s, _ = BuildSnapshot(ObservationBatch{Producer: s.Producer, Subject: s.Subject, Nodes: s.Nodes}, Config{})
	r, err := Derive(s)
	if err != nil {
		t.Fatal(err)
	}
	if r.Candidates[0].Confidence != ConfidenceConflicted {
		t.Fatalf("confidence=%s", r.Candidates[0].Confidence)
	}
	if !r.Candidates[0].ReviewRequired {
		t.Fatal("identity conflict must require review")
	}
	if len(r.UnmappedNodeIDs) != 1 || r.UnmappedNodeIDs[0] != NodeID("r", "file", "solo.go") {
		t.Fatalf("unmapped=%v", r.UnmappedNodeIDs)
	}
}

func TestDeriveGoExternalTestPackageFamilyIsNotConflict(t *testing.T) {
	sub := "go-family"
	dir := Node{ID: NodeID(sub, "directory", "foo"), Kind: "directory", Coordinate: "foo", Name: "foo"}
	p1 := Node{ID: NodeID(sub, "go.package", "foo#package"), Kind: "go.package", Coordinate: "foo#package", Name: "foo", Attributes: map[string]string{"directory": "foo", "package": "foo"}}
	p2 := Node{ID: NodeID(sub, "go.package", "foo#package-test"), Kind: "go.package", Coordinate: "foo#package-test", Name: "foo_test", Attributes: map[string]string{"directory": "foo", "package": "foo_test"}}
	s, err := BuildSnapshot(ObservationBatch{Producer: candidateProducer(), Subject: Subject{ID: sub}, Nodes: []Node{dir, p1, p2}}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := Derive(s)
	if err != nil {
		t.Fatal(err)
	}
	if r.Candidates[0].Confidence != ConfidenceStrong {
		t.Fatalf("foo/foo_test family confidence=%s reasons=%v", r.Candidates[0].Confidence, r.Candidates[0].ReviewReasons)
	}
}

func TestDeriveAllConfidenceStatesAndNoSyntheticFileGroup(t *testing.T) {
	// A workspace edge may support a module boundary, but cannot make it STRONG.
	sub := "support"
	mod := Node{ID: NodeID(sub, "go.module", "app/go.mod#module"), Kind: "go.module", Coordinate: "app/go.mod#module", Name: "example/app", Attributes: map[string]string{"directory": "app", "module": "example/app"}}
	workspace := Node{ID: NodeID(sub, "go.workspace", "go.work#workspace"), Kind: "go.workspace", Coordinate: "go.work#workspace", Name: "go.work", Attributes: map[string]string{"directory": "."}}
	ref := StableID("evidence", "workspace.use", "go.work", "0", "0", "app")
	edge := Edge{Kind: "workspace.use", From: workspace.ID, To: mod.ID, Resolution: "RESOLVED", EvidenceRefs: []string{ref}}
	edge.ID = EdgeID(edge.Kind, edge.From, edge.To)
	ev := Evidence{ID: ref, Kind: "workspace.use", Path: "go.work", Detail: "app"}
	snap, err := BuildSnapshot(ObservationBatch{Producer: candidateProducer(), Subject: Subject{ID: sub}, Nodes: []Node{mod, workspace}, Edges: []Edge{edge}, Evidence: []Evidence{ev}}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Derive(snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 1 || got.Candidates[0].Confidence != ConfidenceSupported {
		t.Fatalf("supported candidate: %#v", got.Candidates)
	}
	if got.Candidates[0].ReviewRequired {
		t.Fatalf("supported candidate without suspicion requires review: %#v", got.Candidates[0])
	}
	for _, id := range got.Candidates[0].PhysicalNodeIDs {
		if id == edge.ID {
			t.Fatal("edge id leaked into physical node membership")
		}
	}
	foundSupport := false
	for _, sig := range got.Candidates[0].Signals {
		if sig.Kind == "workspace_use_support" {
			foundSupport = true
			if len(sig.NodeIDs) == 0 || len(sig.EvidenceRefs) != 1 || sig.EvidenceRefs[0] != ref {
				t.Fatalf("support provenance=%#v", sig)
			}
		}
	}
	if !foundSupport {
		t.Fatal("support signal was dropped")
	}

	// A directly matching unknown leaves a relevant hypothesis unresolved.
	sub = "unknown"
	d := Node{ID: NodeID(sub, "directory", "pkg"), Kind: "directory", Coordinate: "pkg", Name: "pkg"}
	u := Unknown{ID: StableID("unknown", sub, "go_source", "pkg/bad.go"), Kind: "go_source", Coordinate: "pkg/bad.go", Reason: "parse_failed"}
	snap, err = BuildSnapshot(ObservationBatch{Producer: candidateProducer(), Subject: Subject{ID: sub}, Nodes: []Node{d}, Unknowns: []Unknown{u}}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	got, err = Derive(snap)
	if err != nil {
		t.Fatal(err)
	}
	if got.Candidates[0].Confidence != ConfidenceUnresolved {
		t.Fatalf("gap confidence=%s", got.Candidates[0].Confidence)
	}
	if !got.Candidates[0].ReviewRequired {
		t.Fatal("relevant coverage gap must require review")
	}

	// Root-level files do not manufacture a project-wide logical group.
	sub = "files"
	f := Node{ID: NodeID(sub, "file", "main.go"), Kind: "file", Coordinate: "main.go", Name: "main.go"}
	snap, err = BuildSnapshot(ObservationBatch{Producer: candidateProducer(), Subject: Subject{ID: sub}, Nodes: []Node{f}}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	got, err = Derive(snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 0 || len(got.UnmappedNodeIDs) != 1 {
		t.Fatalf("file-only result=%#v", got)
	}

	// A partial filesystem scan only weakens the subtree named by its gap.
	sub = "localized-gap"
	good := Node{ID: NodeID(sub, "directory", "healthy"), Kind: "directory", Coordinate: "healthy", Name: "healthy"}
	bad := Node{ID: NodeID(sub, "directory", "affected"), Kind: "directory", Coordinate: "affected", Name: "affected"}
	u = Unknown{ID: StableID("unknown", sub, "symlink", "affected/link"), Kind: "symlink", Coordinate: "affected/link", Reason: "not_followed"}
	snap, err = BuildSnapshot(ObservationBatch{Producer: candidateProducer(), Subject: Subject{ID: sub}, Nodes: []Node{good, bad}, Unknowns: []Unknown{u}, Coverage: []Coverage{{Capability: "filesystem", State: "PARTIAL"}}}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	got, err = Derive(snap)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Confidence{}
	for _, c := range got.Candidates {
		by[c.Coordinate] = c.Confidence
	}
	if by["healthy"] != ConfidenceWeak || by["affected"] != ConfidenceUnresolved {
		t.Fatalf("localized gaps produced confidences %v", by)
	}
	for _, c := range got.Candidates {
		switch c.Coordinate {
		case "healthy":
			if c.ReviewRequired {
				t.Fatalf("ordinary WEAK candidate with only single_structural_signal requires review: %#v", c)
			}
			if !contains(c.ReviewReasons, "single_structural_signal") {
				t.Fatalf("WEAK explanation was dropped: %#v", c)
			}
		case "affected":
			if c.Confidence != ConfidenceUnresolved || !c.ReviewRequired {
				t.Fatalf("affected candidate must remain UNRESOLVED and review-required: %#v", c)
			}
		}
	}
}

func TestDeriveConfiguredTransportAndObservedRepeatedHierarchy(t *testing.T) {
	sub := "hierarchy"
	dirs := []string{".", "src", "src/main", "src/main/go", "src/main/go/alpha", "src/main/go/alpha/service", "src/main/go/alpha/service/impl", "src/main/go/beta", "src/main/go/beta/service", "src/main/go/beta/service/impl"}
	nodes := []Node{}
	for _, p := range dirs {
		name := p
		if p == "." {
			name = "root"
		} else {
			name = leaf(p)
		}
		nodes = append(nodes, Node{ID: NodeID(sub, "directory", p), Kind: "directory", Coordinate: p, Name: name})
	}
	for _, p := range []string{"src/main/go/alpha/service/impl/a.go", "src/main/go/beta/service/impl/b.go"} {
		nodes = append(nodes, Node{ID: NodeID(sub, "file", p), Kind: "file", Coordinate: p, Name: leaf(p)})
	}
	edges := []Edge{}
	for i, p := range dirs {
		if p == "." {
			continue
		}
		par := pathDir(p)
		e := Edge{Kind: "filesystem.contains", From: NodeID(sub, "directory", par), To: NodeID(sub, "directory", p)}
		e.ID = EdgeID(e.Kind, e.From, e.To)
		edges = append(edges, e)
		_ = i
	}
	for _, p := range []string{"src/main/go/alpha/service/impl/a.go", "src/main/go/beta/service/impl/b.go"} {
		dir := pathDir(p)
		e := Edge{Kind: "filesystem.contains", From: NodeID(sub, "directory", dir), To: NodeID(sub, "file", p)}
		e.ID = EdgeID(e.Kind, e.From, e.To)
		edges = append(edges, e)
	}
	snap, err := BuildSnapshot(ObservationBatch{Producer: candidateProducer(), Subject: Subject{ID: sub}, Nodes: nodes, Edges: edges}, Config{SourceRoots: []string{"src/main/go"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Derive(snap)
	if err != nil {
		t.Fatal(err)
	}
	byCoord := map[string]Candidate{}
	for _, c := range got.Candidates {
		byCoord[c.Coordinate] = c
	}
	for _, p := range []string{".", "src", "src/main", "src/main/go"} {
		if _, ok := byCoord[p]; ok {
			t.Fatalf("transport candidate %s was retained", p)
		}
	}
	for _, p := range []string{"src/main/go/alpha/service", "src/main/go/beta/service"} {
		c, ok := byCoord[p]
		if !ok {
			t.Fatalf("missing %s candidate", p)
		}
		if !contains(c.ReviewReasons, "repeated_layer_pattern") {
			t.Fatalf("%s reasons=%v", p, c.ReviewReasons)
		}
		if !c.ReviewRequired {
			t.Fatalf("repeated layer %s should require review", p)
		}
	}
	for _, p := range []string{"src/main/go/alpha/service/impl", "src/main/go/beta/service/impl"} {
		c := byCoord[p]
		if c.ParentID != byCoord[pathDir(p)].ID {
			t.Fatalf("%s parent=%s want observed %s", p, c.ParentID, byCoord[pathDir(p)].ID)
		}
	}
}

func TestDeriveEvidenceMovePreservesIdentityAndDigestAndNormalizesOrder(t *testing.T) {
	sub := "evidence"
	path1 := "pkg/a.go"
	ref1 := StableID("evidence", "source", path1, "0", "0", "code")
	node := Node{ID: NodeID(sub, "directory", "pkg"), Kind: "directory", Coordinate: "pkg", Name: "pkg", EvidenceRefs: []string{ref1}}
	evidence := Evidence{ID: ref1, Kind: "source", Path: path1, Detail: "code"}
	d := Diagnostic{Code: "scan_gap", Severity: "WARNING", Coordinate: "pkg", Message: "gap"}
	u := Unknown{ID: StableID("unknown", sub, "file", "pkg/b.go"), Kind: "file", Coordinate: "pkg/b.go", Reason: "unreadable"}
	u2 := Unknown{ID: StableID("unknown", sub, "file", "pkg/c.go"), Kind: "file", Coordinate: "pkg/c.go", Reason: "unreadable"}
	d2 := Diagnostic{Code: "observer_note", Severity: "INFO", Message: "note"}
	batch := ObservationBatch{Producer: candidateProducer(), Subject: Subject{ID: sub}, Nodes: []Node{node}, Evidence: []Evidence{evidence}, Coverage: []Coverage{{Capability: "go", State: "COMPLETE"}, {Capability: "filesystem", State: "PARTIAL"}, {Capability: "git", State: "COMPLETE"}}, Unknowns: []Unknown{u, u2}, Diagnostics: []Diagnostic{d, d2}}
	aSnap, err := BuildSnapshot(batch, Config{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := Derive(aSnap)
	if err != nil {
		t.Fatal(err)
	}
	path2 := "elsewhere/a.go"
	ref2 := StableID("evidence", "source", path2, "0", "0", "code")
	batch.Evidence[0].ID = ref2
	batch.Evidence[0].Path = path2
	batch.Nodes[0].EvidenceRefs = []string{ref2}
	bSnap, err := BuildSnapshot(batch, Config{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Derive(bSnap)
	if err != nil {
		t.Fatal(err)
	}
	if a.Candidates[0].ID != b.Candidates[0].ID || a.Digest != b.Digest {
		t.Fatal("evidence movement changed structural identity/digest")
	}
	batch.Nodes = append([]Node(nil), batch.Nodes...)
	batch.Edges = []Edge{}
	batch.Coverage[0], batch.Coverage[2] = batch.Coverage[2], batch.Coverage[0]
	batch.Unknowns[0], batch.Unknowns[1] = batch.Unknowns[1], batch.Unknowns[0]
	batch.Diagnostics[0], batch.Diagnostics[1] = batch.Diagnostics[1], batch.Diagnostics[0]
	cSnap, err := BuildSnapshot(batch, Config{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := Derive(cSnap)
	if err != nil {
		t.Fatal(err)
	}
	aj, _ := SerializeCandidates(b)
	cj, _ := SerializeCandidates(c)
	if string(aj) != string(cj) {
		t.Fatal("normalized shuffled metadata changed serialized candidate output")
	}
}

func TestDeriveRejectsInvalidContainmentTopology(t *testing.T) {
	sub := "invalid-parent"
	dir := Node{ID: NodeID(sub, "directory", "pkg"), Kind: "directory", Coordinate: "pkg", Name: "pkg"}
	alt := Node{ID: NodeID(sub, "filesystem.directory", "pkg"), Kind: "filesystem.directory", Coordinate: "pkg", Name: "pkg"}
	file := Node{ID: NodeID(sub, "file", "pkg/a.go"), Kind: "file", Coordinate: "pkg/a.go", Name: "a.go"}
	e1 := Edge{Kind: "filesystem.contains", From: dir.ID, To: file.ID}
	e1.ID = EdgeID(e1.Kind, e1.From, e1.To)
	e2 := Edge{Kind: "filesystem.contains", From: alt.ID, To: file.ID}
	e2.ID = EdgeID(e2.Kind, e2.From, e2.To)
	s, err := BuildSnapshot(ObservationBatch{Producer: candidateProducer(), Subject: Subject{ID: sub}, Nodes: []Node{dir, alt, file}, Edges: []Edge{e1, e2}}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Derive(s); err == nil {
		t.Fatal("multiple physical parents must fail closed")
	}
	sub = "cycle"
	a := Node{ID: NodeID(sub, "directory", "a"), Kind: "directory", Coordinate: "a", Name: "a"}
	b := Node{ID: NodeID(sub, "directory", "a/b"), Kind: "directory", Coordinate: "a/b", Name: "b"}
	e1 = Edge{Kind: "filesystem.contains", From: a.ID, To: b.ID}
	e1.ID = EdgeID(e1.Kind, e1.From, e1.To)
	e2 = Edge{Kind: "filesystem.contains", From: b.ID, To: a.ID}
	e2.ID = EdgeID(e2.Kind, e2.From, e2.To)
	s, err = BuildSnapshot(ObservationBatch{Producer: candidateProducer(), Subject: Subject{ID: sub}, Nodes: []Node{a, b}, Edges: []Edge{e1, e2}}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Derive(s); err == nil {
		t.Fatal("invalid containment cycle must fail closed")
	}
}

func candidateProducer() Producer {
	return Producer{ID: "scanner", Version: ScannerVersion, ModelVersion: ObservationModelVersion, DerivationModelVersion: DerivationModelVersion}
}
func pathDir(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return "."
	}
	return p[:i]
}

func contains(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}
