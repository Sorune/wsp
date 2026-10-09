package scanner

import (
	"strings"
	"testing"
)

func remediationSnapshot(t *testing.T, subject, boundary string, files []string, attrs map[string]string) PhysicalSnapshot {
	t.Helper()
	dir := Node{ID: NodeID(subject, "directory", boundary), Kind: "directory", Coordinate: boundary, Name: leaf(boundary)}
	pkgName := attrs["package"]
	pkg := Node{ID: NodeID(subject, "go.package", boundary+"#package/"+pkgName), Kind: "go.package", Coordinate: boundary + "#package/" + pkgName, Name: pkgName, Attributes: map[string]string{"directory": boundary, "package": pkgName}}
	nodes := []Node{dir, pkg}
	edges := []Edge{}
	for i, p := range files {
		n := Node{ID: NodeID(subject, "file", p), Kind: "file", Coordinate: p, Name: leaf(p), Attributes: map[string]string{"content_sha256": StableID("content", string(rune('a'+i)))}}
		nodes = append(nodes, n)
		e := Edge{Kind: "filesystem.contains", From: dir.ID, To: n.ID}
		e.ID = EdgeID(e.Kind, e.From, e.To)
		edges = append(edges, e)
	}
	s, err := BuildSnapshot(ObservationBatch{Producer: candidateProducer(), Subject: Subject{ID: subject}, Nodes: nodes, Edges: edges}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func candidateAt(t *testing.T, s PhysicalSnapshot, coord string) Candidate {
	t.Helper()
	set, err := Derive(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range set.Candidates {
		if c.Coordinate == coord {
			return c
		}
	}
	t.Fatalf("candidate %q not found in %#v", coord, set.Candidates)
	return Candidate{}
}

func TestDeriveReviewGateStrongGenericAndSupportedSuspicion(t *testing.T) {
	strong := remediationSnapshot(t, "review-strong", "pkg", []string{"pkg/a.go"}, map[string]string{"package": "pkg"})
	c := candidateAt(t, strong, "pkg")
	if c.Confidence != ConfidenceStrong || c.ReviewRequired {
		t.Fatalf("non-suspicious strong candidate=%#v", c)
	}
	generic := remediationSnapshot(t, "review-generic", "common", []string{"common/a.go"}, map[string]string{"package": "common"})
	c = candidateAt(t, generic, "common")
	if c.Confidence != ConfidenceStrong || !c.ReviewRequired || !contains(c.ReviewReasons, "generic_name_review") {
		t.Fatalf("strong generic candidate=%#v", c)
	}

	for _, label := range []string{"app", "common"} {
		sub := "supported-" + label
		mod := Node{ID: NodeID(sub, "go.module", label+"/go.mod#module"), Kind: "go.module", Coordinate: label + "/go.mod#module", Name: "example/" + label, Attributes: map[string]string{"directory": label, "module": "example/" + label}}
		ws := Node{ID: NodeID(sub, "go.workspace", "go.work#workspace"), Kind: "go.workspace", Coordinate: "go.work#workspace", Name: "go.work", Attributes: map[string]string{"directory": "."}}
		ref := StableID("evidence", "workspace.use", "go.work", "0", "0", label)
		edge := Edge{ID: EdgeID("workspace.use", ws.ID, mod.ID), Kind: "workspace.use", From: ws.ID, To: mod.ID, Resolution: "RESOLVED", EvidenceRefs: []string{ref}}
		ev := Evidence{ID: ref, Kind: "workspace.use", Path: "go.work", Detail: label}
		s, err := BuildSnapshot(ObservationBatch{Producer: candidateProducer(), Subject: Subject{ID: sub}, Nodes: []Node{mod, ws}, Edges: []Edge{edge}, Evidence: []Evidence{ev}}, Config{})
		if err != nil {
			t.Fatal(err)
		}
		candidate := candidateAt(t, s, label)
		wantReview := label == "common"
		if candidate.Confidence != ConfidenceSupported || candidate.ReviewRequired != wantReview {
			t.Fatalf("supported candidate %s = %#v", label, candidate)
		}
	}
}

func TestDeriveReviewGateIdentityDirectoryAndRepeatedLayerSuspicion(t *testing.T) {
	sub := "directory-disagreement"
	dir := Node{ID: NodeID(sub, "directory", "pkg"), Kind: "directory", Coordinate: "pkg", Name: "pkg"}
	pkg := Node{ID: NodeID(sub, "go.package", "pkg#package/other"), Kind: "go.package", Coordinate: "pkg#package/other", Name: "other", Attributes: map[string]string{"directory": "pkg", "package": "other"}}
	s, err := BuildSnapshot(ObservationBatch{Producer: candidateProducer(), Subject: Subject{ID: sub}, Nodes: []Node{dir, pkg}}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	c := candidateAt(t, s, "pkg")
	if c.Confidence != ConfidenceConflicted || !c.ReviewRequired || !contains(c.ReviewReasons, "package_directory_disagreement") {
		t.Fatalf("directory disagreement=%#v", c)
	}

	sub = "repeated-strong"
	nodes := []Node{}
	for _, p := range []string{"alpha/service", "beta/service"} {
		d := Node{ID: NodeID(sub, "directory", p), Kind: "directory", Coordinate: p, Name: "service"}
		g := Node{ID: NodeID(sub, "go.package", p+"#package/service"), Kind: "go.package", Coordinate: p + "#package/service", Name: "service", Attributes: map[string]string{"directory": p, "package": "service"}}
		nodes = append(nodes, d, g)
	}
	s, err = BuildSnapshot(ObservationBatch{Producer: candidateProducer(), Subject: Subject{ID: sub}, Nodes: nodes}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	set, err := Derive(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"alpha/service", "beta/service"} {
		c := Candidate{}
		for _, x := range set.Candidates {
			if x.Coordinate == p {
				c = x
			}
		}
		if c.ID == "" || c.Confidence != ConfidenceStrong || !c.ReviewRequired || !contains(c.ReviewReasons, "repeated_layer_pattern") {
			t.Fatalf("strong repeated layer %s=%#v", p, c)
		}
	}
}

func TestCandidateIdentityUsesSubjectModelAndBoundaryOnly(t *testing.T) {
	base := remediationSnapshot(t, "identity", "pkg", []string{"pkg/a.go"}, map[string]string{"package": "pkg"})
	first := candidateAt(t, base, "pkg")
	withMember := remediationSnapshot(t, "identity", "pkg", []string{"pkg/a.go", "pkg/b.go"}, map[string]string{"package": "pkg"})
	second := candidateAt(t, withMember, "pkg")
	if first.ID != second.ID {
		t.Fatalf("adding a member changed candidate identity: %s != %s", first.ID, second.ID)
	}
	if len(first.PhysicalNodeIDs) == len(second.PhysicalNodeIDs) {
		t.Fatal("membership fixture did not change physical membership")
	}
	withoutMember := remediationSnapshot(t, "identity", "pkg", nil, map[string]string{"package": "pkg"})
	third := candidateAt(t, withoutMember, "pkg")
	if first.ID != third.ID {
		t.Fatalf("removing a member changed candidate identity: %s != %s", first.ID, third.ID)
	}
	changedBoundary := remediationSnapshot(t, "identity", "pkg/sub", []string{"pkg/sub/a.go"}, map[string]string{"package": "pkg"})
	fourth := candidateAt(t, changedBoundary, "pkg/sub")
	if first.ID == fourth.ID {
		t.Fatal("changing structural boundary preserved candidate identity")
	}
	changedSubject := remediationSnapshot(t, "other-subject", "pkg", nil, map[string]string{"package": "pkg"})
	fifth := candidateAt(t, changedSubject, "pkg")
	if first.ID == fifth.ID {
		t.Fatal("changing subject preserved candidate identity")
	}
}

func TestCandidateDigestTracksLogicalProjectionOnly(t *testing.T) {
	base := remediationSnapshot(t, "digest", "pkg", []string{"pkg/a.go"}, map[string]string{"package": "pkg"})
	source1 := base.Digests.Structure
	set1, err := Derive(base)
	if err != nil {
		t.Fatal(err)
	}
	orig := set1.Candidates[0]
	wantDigest := set1.Digest
	// Content change is physical evidence: structure changes while the logical
	// candidate fields remain the same, so the derived digest stays stable.
	content := remediationSnapshot(t, "digest", "pkg", []string{"pkg/a.go"}, map[string]string{"package": "pkg"})
	for i := range content.Nodes {
		if content.Nodes[i].Kind == "file" {
			content.Nodes[i].Attributes["content_sha256"] = "different-content"
		}
	}
	content, err = BuildSnapshot(ObservationBatch{Producer: content.Producer, Subject: content.Subject, Nodes: content.Nodes, Edges: content.Edges}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if content.Digests.Structure == source1 {
		t.Fatal("physical content mutation did not change source structure digest")
	}
	setContent, err := Derive(content)
	if err != nil {
		t.Fatal(err)
	}
	if setContent.Digest != wantDigest {
		t.Fatal("physical content change changed logical candidate digest")
	}
	// Evidence movement changes evidence/artifact data and serialized candidate
	// provenance, while keeping structure, identity, and logical digest stable.
	evidence := remediationSnapshot(t, "digest", "pkg", []string{"pkg/a.go"}, map[string]string{"package": "pkg"})
	ref := StableID("evidence", "source", "/tmp/a.go", "7", "2", "package")
	e := Evidence{ID: ref, Kind: "source", Path: "/tmp/a.go", Line: 7, Column: 2, Detail: "package"}
	evidence.Evidence = []Evidence{e}
	for i := range evidence.Nodes {
		if evidence.Nodes[i].Kind == "go.package" {
			evidence.Nodes[i].EvidenceRefs = []string{ref}
		}
	}
	evidence, err = BuildSnapshot(ObservationBatch{Producer: evidence.Producer, Subject: evidence.Subject, Nodes: evidence.Nodes, Edges: evidence.Edges, Evidence: evidence.Evidence}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	evSet, err := Derive(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if evSet.Digest != wantDigest || evSet.SourceSnapshotDigest != set1.SourceSnapshotDigest || evidence.Digests.Evidence == base.Digests.Evidence {
		t.Fatal("evidence-only change affected logical candidate digest/structure")
	}
	hasEvidenceRef := false
	for _, signal := range evSet.Candidates[0].Signals {
		if contains(signal.EvidenceRefs, ref) {
			hasEvidenceRef = true
		}
	}
	if !hasEvidenceRef {
		t.Fatal("evidence provenance was not serialized into candidate signals")
	}

	// Unrelated physical inventory, source revision, observer provenance, and
	// unmapped records may change without changing the logical candidate set.
	more := remediationSnapshot(t, "digest", "pkg", []string{"pkg/a.go"}, map[string]string{"package": "pkg"})
	more.Subject.Revision = "new-revision"
	more.Subject.Branch = "topic"
	more.Producer.Version = "new-observer"
	more.Producer.ModelVersion = "new-observation-model"
	more.Nodes = append(more.Nodes, Node{ID: NodeID("digest", "file", "outside.txt"), Kind: "file", Coordinate: "outside.txt", Name: "outside.txt"})
	more, err = BuildSnapshot(ObservationBatch{Producer: more.Producer, Subject: more.Subject, Nodes: more.Nodes, Edges: more.Edges}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	moreSet, err := Derive(more)
	if err != nil {
		t.Fatal(err)
	}
	if more.Digests.Structure == source1 || len(moreSet.UnmappedNodeIDs) == 0 {
		t.Fatal("unrelated physical fixture did not change source inventory")
	}
	if moreSet.Digest != wantDigest {
		t.Fatal("unrelated physical/provenance change changed logical digest")
	}

	// Physical membership, signals, evidence refs, snapshot provenance and
	// observer metadata remain serialized but are outside the digest domain.
	mutateExcluded := func(mut func(*DerivedCandidateSet)) {
		x := set1
		x.Candidates = append([]Candidate(nil), set1.Candidates...)
		x.Candidates[0].PhysicalNodeIDs = append([]string(nil), orig.PhysicalNodeIDs...)
		x.Candidates[0].Signals = append([]Signal(nil), orig.Signals...)
		if len(x.Candidates[0].Signals) > 0 {
			x.Candidates[0].Signals[0].NodeIDs = append([]string(nil), orig.Signals[0].NodeIDs...)
			x.Candidates[0].Signals[0].EvidenceRefs = append([]string(nil), orig.Signals[0].EvidenceRefs...)
		}
		mut(&x)
		if got := candidateDigest(x); got != wantDigest {
			t.Fatalf("excluded field changed digest: %s != %s", got, wantDigest)
		}
	}
	mutateExcluded(func(x *DerivedCandidateSet) {
		x.Candidates[0].PhysicalNodeIDs = []string{"different"}
		x.Candidates[0].Signals = []Signal{{Kind: "other", Value: "other"}}
	})
	mutateExcluded(func(x *DerivedCandidateSet) {
		x.SourceSnapshotDigest = "other"
		x.Subject.Revision = "other"
		x.Subject.Branch = "other"
		x.Producer.ID = "other"
		x.Producer.Version = "other"
		x.Producer.ModelVersion = "other"
		x.Config.SourceRoots = []string{"pkg"}
		x.UnmappedNodeIDs = []string{"outside"}
		x.Coverage = []Coverage{{Capability: "x", State: "PARTIAL"}}
		x.Diagnostics = []Diagnostic{{Code: "other", Severity: "INFO", Message: "x"}}
	})

	// Every logical candidate projection field, subject ID, and derivation model
	// version changes the digest. Review and hierarchy fields are decision data.
	mutations := map[string]func(*DerivedCandidateSet){
		"id":               func(s *DerivedCandidateSet) { s.Candidates[0].ID += "x" },
		"coordinate":       func(s *DerivedCandidateSet) { s.Candidates[0].Coordinate += "/child" },
		"label":            func(s *DerivedCandidateSet) { s.Candidates[0].Label += "x" },
		"parent":           func(s *DerivedCandidateSet) { s.Candidates[0].ParentID = "parent" },
		"confidence":       func(s *DerivedCandidateSet) { s.Candidates[0].Confidence = ConfidenceWeak },
		"review":           func(s *DerivedCandidateSet) { s.Candidates[0].ReviewRequired = true },
		"reason":           func(s *DerivedCandidateSet) { s.Candidates[0].ReviewReasons = []string{"human_review"} },
		"subject":          func(s *DerivedCandidateSet) { s.Subject.ID = "other" },
		"derivation_model": func(s *DerivedCandidateSet) { s.Producer.DerivationModelVersion += "x" },
	}
	for name, mut := range mutations {
		t.Run(name, func(t *testing.T) {
			x := set1
			x.Candidates = append([]Candidate(nil), set1.Candidates...)
			x.Candidates[0].ReviewReasons = append([]string(nil), orig.ReviewReasons...)
			mut(&x)
			if got := candidateDigest(x); got == wantDigest {
				t.Fatalf("logical change %s did not change digest", name)
			}
		})
	}
	// Signals and membership remain present in the artifact despite being
	// excluded from its logical identity digest.
	serialized, err := SerializeCandidates(set1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(serialized), "physical_node_ids") || !strings.Contains(string(serialized), "signals") {
		t.Fatal("candidate artifact dropped physical provenance")
	}
}
