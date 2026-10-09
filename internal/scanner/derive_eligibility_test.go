package scanner

import (
	"bytes"
	"encoding/json"
	"path"
	"strings"
	"testing"
)

type eligibilityFixture struct {
	subject string
	batch   ObservationBatch
	dirs    map[string]string
	files   map[string]string
}

func newEligibilityFixture(subject string) *eligibilityFixture {
	return &eligibilityFixture{subject: subject, batch: ObservationBatch{Producer: candidateProducer(), Subject: Subject{ID: subject}}, dirs: map[string]string{}, files: map[string]string{}}
}
func (f *eligibilityFixture) addDirectory(coord string) string {
	if id := f.dirs[coord]; id != "" {
		return id
	}
	id := NodeID(f.subject, "directory", coord)
	f.dirs[coord] = id
	f.batch.Nodes = append(f.batch.Nodes, Node{ID: id, Kind: "directory", Coordinate: coord, Name: leaf(coord), Attributes: map[string]string{}})
	return id
}
func (f *eligibilityFixture) addFile(coord string) string {
	if id := f.files[coord]; id != "" {
		return id
	}
	id := NodeID(f.subject, "file", coord)
	f.files[coord] = id
	f.batch.Nodes = append(f.batch.Nodes, Node{ID: id, Kind: "file", Coordinate: coord, Name: leaf(coord), Attributes: map[string]string{"content_sha256": StableID("fixture", coord)}})
	return id
}
func (f *eligibilityFixture) addContainment(parentID, childID string) {
	e := Edge{Kind: "filesystem.contains", From: parentID, To: childID}
	e.ID = EdgeID(e.Kind, e.From, e.To)
	f.batch.Edges = append(f.batch.Edges, e)
}
func (f *eligibilityFixture) addEvidence(kind, sourcePath, detail string) string {
	e := Evidence{Kind: kind, Path: sourcePath, Detail: detail}
	e.ID = StableID("evidence", kind, sourcePath, "0", "0", detail)
	f.batch.Evidence = append(f.batch.Evidence, e)
	return e.ID
}
func (f *eligibilityFixture) addGoPackage(dir, name string, refs ...string) string {
	coord := dir + "#package/" + name
	id := NodeID(f.subject, "go.package", coord)
	f.batch.Nodes = append(f.batch.Nodes, Node{ID: id, Kind: "go.package", Coordinate: coord, Name: name, Attributes: map[string]string{"directory": dir, "package": name}, EvidenceRefs: append([]string(nil), refs...)})
	return id
}
func (f *eligibilityFixture) addNPMPackage(dir, name, ref string) string {
	coord := path.Join(dir, "package.json") + "#package"
	id := NodeID(f.subject, "npm.package", coord)
	f.batch.Nodes = append(f.batch.Nodes, Node{ID: id, Kind: "npm.package", Coordinate: coord, Name: name, Attributes: map[string]string{"directory": dir, "name": name}, EvidenceRefs: []string{ref}})
	return id
}
func (f *eligibilityFixture) addIgnored(coord string) string {
	id := NodeID(f.subject, "git.ignored", coord)
	f.batch.Nodes = append(f.batch.Nodes, Node{ID: id, Kind: "git.ignored", Coordinate: coord, Name: leaf(coord), Attributes: map[string]string{"ignored": "true"}})
	return id
}
func (f *eligibilityFixture) addUnknown(kind, coord, reason string, refs ...string) string {
	u := Unknown{ID: StableID("unknown", f.subject, kind, coord), Kind: kind, Coordinate: coord, Reason: reason, EvidenceRefs: append([]string(nil), refs...)}
	f.batch.Unknowns = append(f.batch.Unknowns, u)
	return u.ID
}
func (f *eligibilityFixture) snapshot(t *testing.T) PhysicalSnapshot {
	t.Helper()
	// Complete the directory tree with deterministic physical containment.
	for coord, id := range f.dirs {
		if coord == "." {
			continue
		}
		parent := path.Dir(coord)
		if parent == "" {
			parent = "."
		}
		if parentID := f.dirs[parent]; parentID != "" && !hasEdge(f.batch.Edges, parentID, id) {
			f.addContainment(parentID, id)
		}
	}
	for coord, id := range f.files {
		parent := path.Dir(coord)
		if parent == "" {
			parent = "."
		}
		if parentID := f.dirs[parent]; parentID != "" && !hasEdge(f.batch.Edges, parentID, id) {
			f.addContainment(parentID, id)
		}
	}
	s, err := BuildSnapshot(f.batch, Config{})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func hasEdge(edges []Edge, from, to string) bool {
	for _, e := range edges {
		if e.From == from && e.To == to {
			return true
		}
	}
	return false
}
func deriveEligibility(t *testing.T, s PhysicalSnapshot) DerivedCandidateSet {
	t.Helper()
	set, err := Derive(s)
	if err != nil {
		t.Fatal(err)
	}
	return set
}
func candidateByCoordinate(set DerivedCandidateSet, coord string) (Candidate, bool) {
	for _, c := range set.Candidates {
		if c.Coordinate == coord {
			return c, true
		}
	}
	return Candidate{}, false
}
func assertNoPhysicalRefs(t *testing.T, candidates []Candidate, excluded map[string]bool) {
	t.Helper()
	for _, c := range candidates {
		for _, id := range c.PhysicalNodeIDs {
			if excluded[id] {
				t.Errorf("candidate %s includes excluded physical member %s", c.Coordinate, id)
			}
		}
		for _, s := range c.Signals {
			for _, id := range s.NodeIDs {
				if excluded[id] {
					t.Errorf("candidate %s signal %s includes excluded node %s", c.Coordinate, s.Kind, id)
				}
			}
		}
	}
}

func TestDeriveIgnoredSubtreePreservesPhysicalSnapshotAndMetadata(t *testing.T) {
	f := newEligibilityFixture("ignored-subtree")
	root := f.addDirectory(".")
	eligible := f.addDirectory("pkg")
	eligibleFile := f.addFile("pkg/a.go")
	f.addContainment(root, eligible)
	f.addContainment(eligible, eligibleFile)
	ref := f.addEvidence("go_package_clause", "pkg/a.go", "pkg")
	f.addGoPackage("pkg", "pkg", ref)
	ignoredDir := f.addDirectory("pkg/odd-ignored-zone")
	ignoredFile := f.addFile("pkg/odd-ignored-zone/secret.go")
	f.addContainment(eligible, ignoredDir)
	f.addContainment(ignoredDir, ignoredFile)
	// The ignored boundary must win even when semantic evidence is accidentally
	// attributed to another source path.
	secretRef := f.addEvidence("go_package_clause", "elsewhere/secret.go", "secret")
	ignoredPackage := f.addGoPackage("pkg/odd-ignored-zone", "secret", secretRef)
	marker := f.addIgnored("pkg/odd-ignored-zone")
	unknownRef := f.addEvidence("symlink", "pkg/odd-ignored-zone/link", "not_followed")
	f.addUnknown("symlink", "pkg/odd-ignored-zone/link", "symlink_not_followed", unknownRef)
	f.batch.Coverage = []Coverage{{Capability: "filesystem", State: "PARTIAL"}}
	f.batch.Diagnostics = []Diagnostic{{Code: "ignored_symlink", Severity: "WARNING", Coordinate: "pkg/odd-ignored-zone/link", Message: "ignored subtree symlink was not followed"}}
	s := f.snapshot(t)
	before, err := SerializeSnapshot(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{ignoredDir, ignoredFile, ignoredPackage, marker} {
		found := false
		for _, n := range s.Nodes {
			if n.ID == id {
				found = true
			}
		}
		if !found {
			t.Fatalf("physical snapshot omitted ignored fact %s", id)
		}
	}
	if !hasEdge(s.Edges, eligible, ignoredDir) || !hasEdge(s.Edges, ignoredDir, ignoredFile) {
		t.Fatal("physical snapshot omitted ignored directory/file containment")
	}
	set := deriveEligibility(t, s)
	after, err := SerializeSnapshot(s)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("Derive mutated the physical snapshot artifact")
	}
	if _, ok := candidateByCoordinate(set, "pkg/odd-ignored-zone"); ok {
		t.Fatal("ignored subtree produced a candidate")
	}
	if c, ok := candidateByCoordinate(set, "pkg"); !ok || c.Confidence == ConfidenceUnresolved {
		t.Fatalf("ignored symlink + global partial coverage degraded eligible ancestor: %#v present=%v", c, ok)
	}
	assertNoPhysicalRefs(t, set.Candidates, map[string]bool{ignoredDir: true, ignoredFile: true, ignoredPackage: true, marker: true})
	if !containsUnknown(set.Unknowns, "pkg/odd-ignored-zone/link") || len(set.Diagnostics) == 0 {
		t.Fatal("ignored-path unknown/diagnostic metadata was dropped from candidate set")
	}
	if len(set.Coverage) != 1 || set.Coverage[0] != (Coverage{Capability: "filesystem", State: "PARTIAL"}) {
		t.Fatalf("coverage metadata changed during derivation: %#v", set.Coverage)
	}
	serializedCandidates, err := SerializeCandidates(set)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before), "git.ignored") {
		t.Fatal("physical snapshot dropped the ignored marker fact")
	}
	for _, value := range []string{"odd-ignored-zone", "ignored_symlink", "symlink_not_followed"} {
		if !strings.Contains(string(before), value) || !strings.Contains(string(serializedCandidates), value) {
			t.Fatalf("physical or candidate artifact omitted preserved ignored metadata %q", value)
		}
	}
}

func containsUnknown(us []Unknown, coord string) bool {
	for _, u := range us {
		if u.Coordinate == coord {
			return true
		}
	}
	return false
}

func TestDeriveIgnoredPathsAreExactAndDoNotUseNameHeuristics(t *testing.T) {
	f := newEligibilityFixture("ignored-exact")
	for _, coord := range []string{"vendor", "vendor-sibling", "node_modules", "dist", "build", "odd-common", "odd-alpha/service", "odd-beta/service", "arb-ignore", "arb-ignore-sibling", "foo", "foo/bar", "foo#bar"} {
		f.addDirectory(coord)
	}
	f.addIgnored("arb-ignore")
	f.addIgnored("odd-common")
	f.addIgnored("odd-beta")
	f.addIgnored("foo")
	npmRef := f.addEvidence("package_json", "foo#bar/package.json", "name:hash-dir")
	f.addNPMPackage("foo#bar", "hash-dir", npmRef)
	s := f.snapshot(t)
	set := deriveEligibility(t, s)
	for _, coord := range []string{"arb-ignore", "odd-common", "odd-beta/service", "foo/bar"} {
		if _, ok := candidateByCoordinate(set, coord); ok {
			t.Fatalf("ignored path %s produced a candidate", coord)
		}
	}
	for _, coord := range []string{"vendor", "vendor-sibling", "node_modules", "dist", "build", "arb-ignore-sibling", "foo#bar"} {
		if _, ok := candidateByCoordinate(set, coord); !ok {
			t.Fatalf("unignored path %s was name-filtered", coord)
		}
	}
	hashDir, _ := candidateByCoordinate(set, "foo#bar")
	foundManifest := false
	for _, signal := range hashDir.Signals {
		if signal.Kind == "manifest_boundary" {
			foundManifest = true
		}
	}
	if !foundManifest {
		t.Fatal("literal-hash directory lost its eligible npm manifest signal")
	}
	service, ok := candidateByCoordinate(set, "odd-alpha/service")
	if !ok {
		t.Fatal("eligible service candidate disappeared")
	}
	if contains(service.ReviewReasons, "repeated_layer_pattern") {
		t.Fatal("ignored repeated service directory affected eligible candidate")
	}
	if _, ok := candidateByCoordinate(set, "odd-beta"); ok {
		t.Fatal("ignored parent candidate was retained")
	}
}

func TestDeriveIgnoredIndividualSourcesExcludeSemanticSignals(t *testing.T) {
	f := newEligibilityFixture("ignored-source")
	for _, d := range []string{"src/pkg", "src/only-ignored", "src/manifest"} {
		f.addDirectory(d)
	}
	aRef := f.addEvidence("go_package_clause", "src/pkg/a.go", "pkg")
	bRef := f.addEvidence("go_package_clause", "src/pkg/b.go", "pkg")
	a := f.addFile("src/pkg/a.go")
	b := f.addFile("src/pkg/b.go")
	f.addGoPackage("src/pkg", "pkg", aRef, bRef)
	f.addIgnored("src/pkg/b.go")
	secretRef := f.addEvidence("go_package_clause", "src/only-ignored/secret.go", "secret")
	secretFile := f.addFile("src/only-ignored/secret.go")
	secretPackage := f.addGoPackage("src/only-ignored", "only_ignored", secretRef)
	f.addIgnored("src/only-ignored/secret.go")
	manifestRef := f.addEvidence("package_json", "src/manifest/package.json", "name:manifest")
	manifest := f.addNPMPackage("src/manifest", "manifest", manifestRef)
	f.addIgnored("src/manifest/package.json")
	s := f.snapshot(t)
	set := deriveEligibility(t, s)
	if c, ok := candidateByCoordinate(set, "src/pkg"); !ok {
		t.Fatal("eligible source package candidate was dropped")
	} else {
		found := false
		for _, signal := range c.Signals {
			if signal.Kind == "go_package" {
				found = true
				if !contains(signal.EvidenceRefs, aRef) || contains(signal.EvidenceRefs, bRef) {
					t.Fatalf("mixed-source package refs not filtered: %#v", signal)
				}
			}
		}
		if !found {
			t.Fatal("eligible Go source signal was dropped")
		}
		if !contains(c.PhysicalNodeIDs, a) {
			t.Fatal("eligible source file was dropped from physical membership")
		}
		if contains(c.PhysicalNodeIDs, b) {
			t.Fatal("ignored source file retained as candidate member")
		}
	}
	if c, ok := candidateByCoordinate(set, "src/only-ignored"); ok {
		for _, signal := range c.Signals {
			if signal.Kind == "go_package" {
				t.Fatalf("fully ignored Go source retained semantic signal: %#v", signal)
			}
		}
		if contains(c.PhysicalNodeIDs, secretFile) || contains(c.PhysicalNodeIDs, secretPackage) {
			t.Fatal("fully ignored Go source retained physical semantic membership")
		}
	}
	if c, ok := candidateByCoordinate(set, "src/manifest"); ok {
		for _, signal := range c.Signals {
			if signal.Kind == "manifest_boundary" {
				t.Fatalf("ignored npm manifest retained semantic signal: %#v", signal)
			}
		}
		if contains(c.PhysicalNodeIDs, manifest) {
			t.Fatal("ignored manifest retained physical semantic membership")
		}
	}
	if !hasPhysicalNode(s.Nodes, secretFile) || !hasPhysicalNode(s.Nodes, manifest) {
		t.Fatal("ignored semantic source facts disappeared from physical snapshot")
	}
}

func TestDeriveNonIgnoredZeroEvidenceWorkspaceSupportRemains(t *testing.T) {
	sub := "nonignored-workspace"
	module := Node{ID: NodeID(sub, "go.module", "app/go.mod#module"), Kind: "go.module", Coordinate: "app/go.mod#module", Name: "example/app", Attributes: map[string]string{"directory": "app", "module": "example/app"}}
	workspace := Node{ID: NodeID(sub, "go.workspace", "go.work#workspace"), Kind: "go.workspace", Coordinate: "go.work#workspace", Name: "go.work", Attributes: map[string]string{"directory": "."}}
	edge := Edge{Kind: "workspace.use", From: workspace.ID, To: module.ID, Resolution: "RESOLVED"}
	edge.ID = EdgeID(edge.Kind, edge.From, edge.To)
	s, err := BuildSnapshot(ObservationBatch{Producer: candidateProducer(), Subject: Subject{ID: sub}, Nodes: []Node{module, workspace}, Edges: []Edge{edge}}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := candidateByCoordinate(deriveEligibility(t, s), "app")
	if !ok || c.Confidence != ConfidenceSupported || c.ReviewRequired {
		t.Fatalf("nonignored no-evidence workspace support regressed: %#v present=%v", c, ok)
	}
	found := false
	for _, signal := range c.Signals {
		if signal.Kind == "workspace_use_support" {
			found = true
			if len(signal.NodeIDs) == 0 || len(signal.EvidenceRefs) != 0 {
				t.Fatalf("unexpected no-evidence support signal: %#v", signal)
			}
		}
	}
	if !found {
		t.Fatal("nonignored workspace.use support was discarded without evidence")
	}
}

func TestDeriveCoverageGapsRespectIgnoredAndEligibleBoundaries(t *testing.T) {
	for _, state := range []string{"FAILED", "UNAVAILABLE", "PARTIAL"} {
		t.Run("global_"+strings.ToLower(state), func(t *testing.T) {
			f := newEligibilityFixture("global-" + state)
			f.addDirectory("pkg")
			ref := f.addEvidence("go_package_clause", "pkg/a.go", "pkg")
			f.addGoPackage("pkg", "pkg", ref)
			f.batch.Coverage = []Coverage{{Capability: "filesystem", State: state}}
			s := f.snapshot(t)
			c, ok := candidateByCoordinate(deriveEligibility(t, s), "pkg")
			if !ok || c.Confidence != ConfidenceUnresolved || !c.ReviewRequired {
				t.Fatalf("global %s not conservative: %#v present=%v", state, c, ok)
			}
		})
	}

	f := newEligibilityFixture("local-eligible-gap")
	f.addDirectory("pkg")
	f.addDirectory("pkg/ignored")
	ref := f.addEvidence("go_package_clause", "pkg/a.go", "pkg")
	f.addGoPackage("pkg", "pkg", ref)
	f.addIgnored("pkg/ignored")
	// Locality comes from Unknown.Coordinate, even if its evidence path happens
	// to fall under an ignored marker.
	uRef := f.addEvidence("source_read", "pkg/ignored/source", "unavailable")
	f.addUnknown("go_source", "pkg/b.go", "source_unavailable", uRef)
	// Complete coverage makes the Unknown.Coordinate the only reason to degrade;
	// evidence-path filtering cannot accidentally hide this eligible unknown.
	f.batch.Coverage = []Coverage{{Capability: "filesystem", State: "COMPLETE"}, {Capability: "go_source", State: "COMPLETE"}}
	c, ok := candidateByCoordinate(deriveEligibility(t, f.snapshot(t)), "pkg")
	if !ok || c.Confidence != ConfidenceUnresolved || !c.ReviewRequired {
		t.Fatalf("eligible source unknown did not degrade candidate: %#v present=%v", c, ok)
	}

	f = newEligibilityFixture("local-boundary-gap")
	f.addDirectory("pkg")
	ref = f.addEvidence("go_package_clause", "pkg/a.go", "pkg")
	f.addGoPackage("pkg", "pkg", ref)
	f.addUnknown("go_module", "pkg/go.mod", "module_unavailable")
	f.batch.Coverage = []Coverage{{Capability: "go_manifests", State: "PARTIAL"}}
	c, ok = candidateByCoordinate(deriveEligibility(t, f.snapshot(t)), "pkg")
	if !ok || c.Confidence != ConfidenceUnresolved {
		t.Fatalf("eligible boundary unknown did not degrade candidate: %#v present=%v", c, ok)
	}
}

func TestDeriveEligibilityReorderedInputsRemainByteStable(t *testing.T) {
	f := newEligibilityFixture("eligibility-order")
	d := f.addDirectory("pkg")
	file := f.addFile("pkg/a.go")
	ref := f.addEvidence("go_package_clause", "pkg/a.go", "pkg")
	f.addGoPackage("pkg", "pkg", ref)
	f.addContainment(d, file)
	f.addIgnored("pkg/ignored")
	uRef := f.addEvidence("symlink", "pkg/ignored/link", "not_followed")
	f.addUnknown("symlink", "pkg/ignored/link", "symlink_not_followed", uRef)
	f.batch.Coverage = []Coverage{{Capability: "filesystem", State: "PARTIAL"}, {Capability: "git", State: "COMPLETE"}}
	f.batch.Diagnostics = []Diagnostic{{Code: "gap", Severity: "WARNING", Coordinate: "pkg/ignored/link", Message: "ignored gap"}}
	a := f.snapshot(t)
	aSnapshot, _ := SerializeSnapshot(a)
	aSet := deriveEligibility(t, a)
	aCandidates, _ := SerializeCandidates(aSet)
	batch := ObservationBatch{Producer: a.Producer, Subject: a.Subject, Nodes: append([]Node(nil), a.Nodes...), Edges: append([]Edge(nil), a.Edges...), Evidence: append([]Evidence(nil), a.Evidence...), Coverage: append([]Coverage(nil), a.Coverage...), Unknowns: append([]Unknown(nil), a.Unknowns...), Diagnostics: append([]Diagnostic(nil), a.Diagnostics...)}
	reverseNodes := func(xs []Node) {
		for i, j := 0, len(xs)-1; i < j; i, j = i+1, j-1 {
			xs[i], xs[j] = xs[j], xs[i]
		}
	}
	reverseNodes(batch.Nodes)
	for i, j := 0, len(batch.Edges)-1; i < j; i, j = i+1, j-1 {
		batch.Edges[i], batch.Edges[j] = batch.Edges[j], batch.Edges[i]
	}
	for i, j := 0, len(batch.Evidence)-1; i < j; i, j = i+1, j-1 {
		batch.Evidence[i], batch.Evidence[j] = batch.Evidence[j], batch.Evidence[i]
	}
	for i, j := 0, len(batch.Coverage)-1; i < j; i, j = i+1, j-1 {
		batch.Coverage[i], batch.Coverage[j] = batch.Coverage[j], batch.Coverage[i]
	}
	for i, j := 0, len(batch.Unknowns)-1; i < j; i, j = i+1, j-1 {
		batch.Unknowns[i], batch.Unknowns[j] = batch.Unknowns[j], batch.Unknowns[i]
	}
	for i, j := 0, len(batch.Diagnostics)-1; i < j; i, j = i+1, j-1 {
		batch.Diagnostics[i], batch.Diagnostics[j] = batch.Diagnostics[j], batch.Diagnostics[i]
	}
	// Rotate nodes so the ignored marker/records take a different insertion position.
	if len(batch.Nodes) > 1 {
		batch.Nodes = append(batch.Nodes[1:], batch.Nodes[0])
	}
	b, err := BuildSnapshot(batch, Config{})
	if err != nil {
		t.Fatal(err)
	}
	bSnapshot, _ := SerializeSnapshot(b)
	bSet := deriveEligibility(t, b)
	bCandidates, _ := SerializeCandidates(bSet)
	if !bytes.Equal(aSnapshot, bSnapshot) || !bytes.Equal(aCandidates, bCandidates) || a.Digests != b.Digests || aSet.Digest != bSet.Digest {
		t.Fatal("reordered physical/ignored/unknown input changed serialized artifacts or digests")
	}
}

func TestDeriveIgnoredEligibilitySurvivesSnapshotJSONRoundTrip(t *testing.T) {
	f := newEligibilityFixture("portable-ignored")
	f.addDirectory("pkg")
	ref := f.addEvidence("go_package_clause", "pkg/a.go", "pkg")
	f.addGoPackage("pkg", "pkg", ref)
	f.addIgnored("pkg/ignored")
	uRef := f.addEvidence("symlink", "pkg/ignored/link", "not_followed")
	f.addUnknown("symlink", "pkg/ignored/link", "symlink_not_followed", uRef)
	f.batch.Coverage = []Coverage{{Capability: "filesystem", State: "PARTIAL"}}
	f.batch.Diagnostics = []Diagnostic{{Code: "ignored_gap", Severity: "WARNING", Coordinate: "pkg/ignored/link", Message: "ignored path gap"}}
	s := f.snapshot(t)
	s.Subject.Root = "/tmp/physical/checkout"
	first := deriveEligibility(t, s)
	firstJSON, err := SerializeCandidates(first)
	if err != nil {
		t.Fatal(err)
	}
	snapshotJSON, err := SerializeSnapshot(s)
	if err != nil {
		t.Fatal(err)
	}
	var reloaded PhysicalSnapshot
	if err = json.Unmarshal(snapshotJSON, &reloaded); err != nil {
		t.Fatal(err)
	}
	if reloaded.Subject.Root != "" {
		t.Fatal("absolute subject root unexpectedly serialized")
	}
	second := deriveEligibility(t, reloaded)
	secondJSON, err := SerializeCandidates(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) || first.Digest != second.Digest {
		t.Fatal("snapshot JSON round trip changed ignored eligibility")
	}
}

func hasPhysicalNode(nodes []Node, id string) bool {
	for _, n := range nodes {
		if n.ID == id {
			return true
		}
	}
	return false
}
