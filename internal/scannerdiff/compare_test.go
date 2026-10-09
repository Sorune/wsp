package scannerdiff

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sample(t *testing.T) artifact {
	t.Helper()
	var a artifact
	raw := `{"snapshot":{"schema_version":1,"subject":{"id":"fixture","revision":"A"},"producer":{"id":"observer","version":"1"},"config":{},"digests":{"structure":"sA","evidence":"eA","coverage":"cA","producer":"p","config":"c","artifact":"a"},"nodes":[{"kind":"npm.package","coordinate":"packages/foo/package.json","attributes":{"name":"@fixture/foo","directory":"packages/foo"}},{"kind":"file","coordinate":"apps/studio/src/index.ts","attributes":{"content_sha256":"before"}}],"unknowns":[],"coverage":[{"capability":"filesystem","state":"COMPLETE"}]},"candidates":{"subject":{"id":"fixture"},"producer":{"id":"derivation","version":"1"},"config":{},"source_snapshot_digest":"sA","digest":"dA","candidates":[{"id":"foo","coordinate":"packages/foo","label":"foo","confidence":"STRONG","review_required":false,"review_reasons":[]},{"id":"studio","coordinate":"apps/studio","label":"studio","confidence":"STRONG","review_required":false,"review_reasons":[]}]}}`
	if e := json.Unmarshal([]byte(raw), &a); e != nil {
		t.Fatal(e)
	}
	return a
}
func clone(t *testing.T, a artifact) artifact {
	t.Helper()
	raw, _ := json.Marshal(a)
	var b artifact
	if e := json.Unmarshal(raw, &b); e != nil {
		t.Fatal(e)
	}
	return b
}
func ref() reference {
	var r reference
	_ = json.Unmarshal([]byte(`{"id":"A","bindings":[{"lens_node":"logical:foo","coordinate":"packages/foo"}],"groups":[{"lens_node":"logical:packages","coordinate_prefix":"packages"}],"relations":[{"id":"r:foo","from":"logical:packages","to":"logical:foo"}]}`), &r)
	return r
}
func changeDigests(b *artifact) {
	b.Snapshot.Digests["structure"] = "sB"
	b.Snapshot.Digests["evidence"] = "eB"
	b.Candidates.Source = "sB"
}
func TestInternalContentAndEvidenceDoNotBecomeBoundaryDrift(t *testing.T) {
	a := sample(t)
	b := clone(t, a)
	changeDigests(&b)
	b.Snapshot.Nodes[1].Attributes["content_sha256"] = "after"
	x, e := compare(a, b, nil, nil, nil, ref())
	if e != nil {
		t.Fatal(e)
	}
	if len(x.Deltas) != 0 || len(x.Content) != 1 || x.Suppressed != 2 || x.Domains["candidate_set"] || !x.Domains["structure"] || !x.Domains["evidence"] {
		t.Fatalf("bad internal edit projection: %+v", x)
	}
	b = clone(t, a)
	b.Snapshot.Digests["evidence"] = "line-moved"
	x, e = compare(a, b, nil, nil, nil, ref())
	if e != nil || len(x.Deltas) != 0 || x.Domains["structure"] || !x.Domains["evidence"] {
		t.Fatal("line-only evidence shift became structure drift", e)
	}
}
func TestMoveAddRemoveUnknownAndReviewRemainSeparate(t *testing.T) {
	a := sample(t)
	b := clone(t, a)
	changeDigests(&b)
	b.Candidates.Digest = "dB"
	b.Candidates.Items[0].ID = "moved"
	b.Candidates.Items[0].Coordinate = "packages/platform/foo"
	b.Snapshot.Nodes[0].Attributes["directory"] = "packages/platform/foo"
	b.Candidates.Items = append(b.Candidates.Items, candidate{ID: "new", Coordinate: "packages/new", Confidence: "UNRESOLVED", Review: true})
	a.Candidates.Items = append(a.Candidates.Items, candidate{ID: "removed", Coordinate: "packages/removed", Confidence: "WEAK"})
	b.Snapshot.Unknowns = append(b.Snapshot.Unknowns, unknown{Kind: "unsupported_manifest", Coordinate: "packages/new/build.gradle.kts", Reason: "manifest_format_not_supported"})
	b.Snapshot.Digests["coverage"] = "cB"
	x, e := compare(a, b, nil, nil, nil, ref())
	if e != nil {
		t.Fatal(e)
	}
	if x.Added != 2 || x.Removed != 2 || len(x.Relocations) != 1 || len(x.Unknowns) != 1 || len(x.ReviewDelta) != 1 || len(x.Coverage) != 0 || !x.Domains["coverage"] {
		t.Fatalf("bad separation: %+v", x)
	}
	if !reflect.DeepEqual(x.Relocations[0].Affected.Nodes, []string{"logical:foo", "logical:packages"}) {
		t.Fatal("lost reference binding intersection")
	}
	// Identity conflicts are not resolved by guessing a move.
	b.Snapshot.Nodes = append(b.Snapshot.Nodes, b.Snapshot.Nodes[0])
	x, e = compare(a, b, nil, nil, nil, ref())
	if e != nil || len(x.Relocations) != 0 {
		t.Fatal("ambiguous identity became automatic relocation", e)
	}
}
func TestIncompatibleOrIncoherentArtifactsFail(t *testing.T) {
	for _, kind := range []string{"subject", "producer", "config", "candidate-model", "source", "coordinate", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			a := sample(t)
			b := clone(t, a)
			switch kind {
			case "subject":
				b.Snapshot.Subject.ID = "other"
				b.Candidates.Subject.ID = "other"
			case "producer":
				b.Snapshot.Digests["producer"] = "different"
			case "config":
				b.Snapshot.Config = json.RawMessage(`{"source_roots":["src"]}`)
				b.Candidates.Config = b.Snapshot.Config
			case "candidate-model":
				b.Candidates.Producer = json.RawMessage(`{"id":"derivation","version":"2"}`)
			case "source":
				b.Candidates.Source = "unrelated"
			case "coordinate":
				b.Candidates.Items[0].Coordinate = "../escape"
			case "duplicate":
				b.Candidates.Items = append(b.Candidates.Items, b.Candidates.Items[0])
			}
			if _, e := compare(a, b, nil, nil, nil, ref()); e == nil {
				t.Fatal("incompatible artifact accepted")
			}
		})
	}
}
func TestCLIIsBoundedDeterministicAndReadOnly(t *testing.T) {
	a := sample(t)
	b := clone(t, a)
	changeDigests(&b)
	b.Snapshot.Nodes[1].Attributes["content_sha256"] = "after"
	dir := t.TempDir()
	paths := []string{}
	raws := [][]byte{}
	for i, v := range []any{a, b, ref()} {
		raw, _ := json.Marshal(v)
		p := filepath.Join(dir, string(rune('a'+i))+".json")
		if e := os.WriteFile(p, raw, 0600); e != nil {
			t.Fatal(e)
		}
		paths = append(paths, p)
		raws = append(raws, raw)
	}
	args := []string{"--baseline", paths[0], "--current", paths[1], "--reference", paths[2]}
	var x, y, err bytes.Buffer
	if Run(args, &x, &err) != 0 || Run(args, &y, &err) != 0 || !bytes.Equal(x.Bytes(), y.Bytes()) {
		t.Fatal("non-deterministic CLI", err.String())
	}
	for _, forbidden := range []string{"physical_node_ids", "signals", "evidence_refs", "\"unknowns\""} {
		if strings.Contains(x.String(), forbidden) {
			t.Fatal("raw field leaked", forbidden)
		}
	}
	for i, p := range paths {
		raw, _ := os.ReadFile(p)
		if !bytes.Equal(raw, raws[i]) {
			t.Fatal("input mutated")
		}
	}
	x.Reset()
	if Run(append(args, "--max-bytes", "1"), &x, &err) == 0 || x.Len() != 0 {
		t.Fatal("over-budget output not fail-closed")
	}
	if e := os.WriteFile(paths[0], append(raws[0], []byte(` {}`)...), 0600); e != nil {
		t.Fatal(e)
	}
	x.Reset()
	if Run(args, &x, &err) == 0 || x.Len() != 0 {
		t.Fatal("trailing JSON accepted")
	}
}
func TestRealExperiment(t *testing.T) {
	dir := os.Getenv("SCANNER_CALIBRATION_EXPERIMENT")
	if dir == "" {
		t.Skip("set SCANNER_CALIBRATION_EXPERIMENT for disposable A/I/B integration")
	}
	load := func(name string) (artifact, []byte) {
		raw, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil {
			t.Fatal(e)
		}
		var a artifact
		if e = json.Unmarshal(raw, &a); e != nil {
			t.Fatal(e)
		}
		return a, raw
	}
	a, ar := load("frozen-A/scan.json")
	i, ir := load("internal-only.json")
	b, br := load("current-B.json")
	rr, e := os.ReadFile(filepath.Join(dir, "frozen-A/reference.json"))
	if e != nil {
		t.Fatal(e)
	}
	var r reference
	if e = json.Unmarshal(rr, &r); e != nil {
		t.Fatal(e)
	}
	x, e := compare(a, i, ar, ir, rr, r)
	if e != nil || len(x.Deltas) != 0 || x.Domains["candidate_set"] || len(x.Content) != 1 {
		t.Fatal("real internal-only handling failed", e)
	}
	x, e = compare(a, b, ar, br, rr, r)
	if e != nil {
		t.Fatal(e)
	}
	changes := map[string]string{}
	for _, d := range x.Deltas {
		c := d.After
		if c == nil {
			c = d.Before
		}
		changes[c.Coordinate] = d.Change
	}
	for c, want := range map[string]string{"packages/public/browser": "removed", "packages/platform/browser": "added", "packages/public/telemetry": "added", "packages/public/content": "removed", "packages/lab/experimental": "added"} {
		if changes[c] != want {
			t.Errorf("%s: got %s want %s", c, changes[c], want)
		}
	}
	if len(x.Relocations) != 1 || len(x.Unknowns) != 1 || x.Unknowns[0].After != 1 || x.Suppressed != len(a.Candidates.Items)-x.Removed-x.Changed {
		t.Fatal("real drift summary inconsistent")
	}
	for j := 0; j < 10; j++ {
		again, e := compare(a, b, ar, br, rr, r)
		v, _ := json.Marshal(x)
		w, _ := json.Marshal(again)
		if e != nil || !bytes.Equal(v, w) {
			t.Fatal("unstable real diff")
		}
	}
}

func TestCompareWithoutReferenceIsValid(t *testing.T) {
	a := sample(t)
	b := clone(t, a)
	changeDigests(&b)
	b.Snapshot.Nodes[1].Attributes["content_sha256"] = "after"
	x, e := compare(a, b, nil, nil, nil, reference{})
	if e != nil {
		t.Fatal(e)
	}
	if x.Reference != "" || x.ReferenceSHA != "" {
		t.Fatalf("optional reference unexpectedly populated: %+v", x)
	}
	if len(x.Content) != 1 || len(x.Deltas) != 0 {
		t.Fatalf("reference-free comparison lost mechanical drift: %+v", x)
	}
}
