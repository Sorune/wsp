package scannerview

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture() map[string]any {
	return map[string]any{
		"snapshot": map[string]any{
			"schema_version": 1,
			"producer":       map[string]any{"id": "scanner", "version": "0.1.0", "model_version": "1", "derivation_model_version": "1"},
			"subject":        map[string]any{"id": "fixture", "revision": "abc123", "branch": "main", "working_tree": "clean"},
			"digests":        map[string]any{"structure": "s", "evidence": "e", "coverage": "c", "producer": "p", "config": "c2", "artifact": "a"},
			"coverage":       []any{map[string]any{"capability": "go", "state": "PARTIAL"}},
			"nodes": []any{
				fixtureNode("n1", "module", "src/alpha", "alpha", []string{"ev1"}), fixtureNode("n2", "file", "src/alpha/a.go", "a.go", []string{"ev1"}), fixtureNode("n3", "directory", "src/alphabet", "alphabet", []string{"ev2"}), fixtureNode("n4", "file", "other/b.go", "b.go", []string{"ev3"}),
			},
			"edges":    []any{map[string]any{"id": "edge1", "kind": "contains", "from": "n1", "to": "n2", "resolution": "RESOLVED", "evidence_refs": []string{"ev1"}}, map[string]any{"id": "edge2", "kind": "contains", "from": "n3", "to": "n4", "resolution": "RESOLVED", "evidence_refs": []string{"ev3"}}},
			"evidence": []any{ev("ev1", "source", "src/alpha/a.go"), ev("ev2", "source", "src/alphabet/x.go"), ev("ev3", "source", "other/b.go"), ev("ev4", "manifest", "go.mod")},
			"unknowns": []any{unk("u1", "parse", "src/alpha/bad.go", "unsupported_syntax", "ev1"), unk("u2", "read", "src/alphabet/x.go", "permission_denied", "ev2"), unk("u3", "parse", "other/bad.go", "unsupported_syntax", "ev3")},
		},
		"candidates": map[string]any{
			"producer":               map[string]any{"id": "scanner", "version": "0.1.0", "model_version": "1", "derivation_model_version": "1"},
			"subject":                map[string]any{"id": "fixture", "revision": "abc123", "branch": "main", "working_tree": "clean"},
			"source_snapshot_digest": "s",
			"digest":                 "candidate-set-digest",
			"coverage":               []any{map[string]any{"capability": "go", "state": "PARTIAL"}},
			"unknowns":               []any{},
			"candidates": []any{
				cand("c-alpha", "src/alpha", "Alpha", "SUPPORTED", false, []string{"n1", "n2"}, []string{"ev1"}),
				cand("c-alphabet", "src/alphabet", "Alphabet", "WEAK", true, []string{"n3"}, []string{"ev2"}),
				cand("c-other", "other", "Other", "CONFLICTED", true, []string{"n4"}, []string{"ev3"}),
			},
		},
	}
}
func fixtureNode(id, kind, coord, name string, refs []string) any {
	return map[string]any{"id": id, "kind": kind, "coordinate": coord, "name": name, "attributes": map[string]string{"marker": "physical_node_ids signals evidence_refs unknownID records"}, "evidence_refs": refs}
}
func ev(id, kind, path string) any {
	return map[string]any{"id": id, "kind": kind, "path": path, "line": 7, "detail": "evidence_refs unknownID"}
}
func unk(id, kind, coord, reason, ref string) any {
	return map[string]any{"id": id, "kind": kind, "coordinate": coord, "reason": reason, "evidence_refs": []string{ref}}
}
func cand(id, coord, label, confidence string, review bool, nodes []string, refs []string) any {
	return map[string]any{"id": id, "coordinate": coord, "label": label, "physical_node_ids": nodes, "signals": []any{map[string]any{"kind": "package", "value": label, "node_ids": nodes, "evidence_refs": refs}}, "confidence": confidence, "review_required": review, "review_reasons": []string{"fixture_reason"}}
}

func writeFixture(t *testing.T) (string, []byte) {
	t.Helper()
	return writeArtifact(t, fixture())
}
func writeArtifact(t *testing.T, value map[string]any) (string, []byte) {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "combined.json")
	if err = os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	return p, b
}
func runCLI(t *testing.T, input string, args ...string) ([]byte, []byte, error) {
	t.Helper()
	argv := []string{"--input", input}
	argv = append(argv, args...)
	var out, stderr strings.Builder
	code := Run(argv, &out, &stderr)
	if code != 0 {
		return []byte(out.String()), []byte(stderr.String()), fmt.Errorf("exit %d", code)
	}
	return []byte(out.String()), []byte(stderr.String()), nil
}
func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var x map[string]any
	if err := json.Unmarshal(b, &x); err != nil {
		t.Fatalf("invalid JSON %q: %v", b, err)
	}
	return x
}
func obj(v any) map[string]any { m, _ := v.(map[string]any); return m }
func list(v any) []any         { a, _ := v.([]any); return a }
func stringsIn(v any) []string {
	var out []string
	switch x := v.(type) {
	case map[string]any:
		for k, y := range x {
			out = append(out, k)
			out = append(out, stringsIn(y)...)
		}
	case []any:
		for _, y := range x {
			out = append(out, stringsIn(y)...)
		}
	case string:
		out = append(out, x)
	}
	return out
}
func has(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func TestLevelsKeepRawProvenanceAndHideDrillFields(t *testing.T) {
	p, raw := writeFixture(t)
	sum := sha256.Sum256(raw)
	want := hex.EncodeToString(sum[:])
	for _, level := range []string{"L0", "L1", "L2"} {
		t.Run(level, func(t *testing.T) {
			out, _, err := runCLI(t, p, "--level", level)
			if err != nil {
				t.Fatal(err)
			}
			got := decode(t, out)
			prov := obj(got["provenance"])
			if prov["raw_sha256"] != want {
				t.Fatalf("raw hash=%v want %s", prov["raw_sha256"], want)
			}
			for _, k := range []string{"physical_node_ids", "signals", "evidence_refs", "unknown_records", "members", "related_edges"} {
				if has(stringsIn(got), k) {
					t.Fatalf("%s leaked at %s", k, level)
				}
			}
			for _, token := range []string{"n1", "ev1", "u1"} {
				if has(stringsIn(got), token) {
					t.Fatalf("raw identifier %s leaked at %s", token, level)
				}
			}
			after, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(raw) {
				t.Fatal("input artifact bytes changed")
			}
		})
	}
	// Same input bytes and digest are retained regardless of filtering/level.
	for _, args := range [][]string{{"--level", "L0"}, {"--level", "L1", "--prefix", "src/alpha"}, {"--level", "L2", "--review-required"}, {"--level", "L3", "--coordinate", "src/alpha"}} {
		out, _, err := runCLI(t, p, args...)
		if err != nil {
			t.Fatal(err)
		}
		got := decode(t, out)
		if obj(got["provenance"])["raw_sha256"] != want {
			t.Fatalf("provenance hash changed for %v", args)
		}
	}
}

func TestFiltersAndTaskVsGlobalReview(t *testing.T) {
	p, _ := writeFixture(t)
	check := func(args []string, want string) {
		t.Helper()
		out, _, err := runCLI(t, p, args...)
		if err != nil {
			t.Fatal(err)
		}
		x := decode(t, out)
		a := list(x["task_candidates"])
		if len(a) != 1 || obj(a[0])["id"] != want {
			t.Fatalf("%v task=%v want %s", args, a, want)
		}
	}
	check([]string{"--level", "L1", "--coordinate", "src/alpha"}, "c-alpha")
	check([]string{"--level", "L1", "--prefix", "src/alpha"}, "c-alpha")
	check([]string{"--level", "L1", "--id", "c-alphabet"}, "c-alphabet")
	check([]string{"--level", "L1", "--confidence", "SUPPORTED"}, "c-alpha")
	check([]string{"--level", "L1", "--prefix", "src", "--review-required", "--confidence", "WEAK"}, "c-alphabet")
	out, _, err := runCLI(t, p, "--level", "L1", "--coordinate", "src/alpha")
	if err != nil {
		t.Fatal(err)
	}
	compact := obj(list(decode(t, out)["task_candidates"])[0])
	if compact["id"] != "c-alpha" || compact["coordinate"] != "src/alpha" || compact["confidence"] != "SUPPORTED" || compact["review_required"] != false || len(list(compact["review_reasons"])) != 1 {
		t.Fatalf("compact candidate lost identity/confidence/review fields: %v", compact)
	}
	out, _, err = runCLI(t, p, "--level", "L1", "--coordinate", "src/alpha", "--coordinate", "src/alphabet")
	if err != nil {
		t.Fatal(err)
	}
	x := decode(t, out)
	if len(list(x["task_candidates"])) != 2 {
		t.Fatalf("repeated exact coordinates were not ORed: %v", x["task_candidates"])
	}
	out, _, err = runCLI(t, p, "--level", "L1", "--prefix", "src")
	if err != nil {
		t.Fatal(err)
	}
	x = decode(t, out)
	if len(list(x["task_candidates"])) != 2 {
		t.Fatalf("segment-aware prefix matched wrong candidates: %v", x["task_candidates"])
	}
	out, _, err = runCLI(t, p, "--level", "L1", "--prefix", "src/app")
	if err != nil {
		t.Fatal(err)
	}
	x = decode(t, out)
	if len(list(x["task_candidates"])) != 0 {
		t.Fatalf("segment prefix matched src/application: %v", x["task_candidates"])
	}
	out, _, err = runCLI(t, p, "--level", "L2", "--prefix", "src/alpha")
	if err != nil {
		t.Fatal(err)
	}
	x = decode(t, out)
	if len(list(x["task_candidates"])) != 1 || obj(list(x["task_candidates"])[0])["id"] != "c-alpha" {
		t.Fatalf("task matches not filtered: %v", x["task_candidates"])
	}
	if len(list(x["review_candidates"])) != 2 {
		t.Fatalf("global review queue was task-filtered: %v", x["review_candidates"])
	}
}

func TestL2UnknownScopesAndCounts(t *testing.T) {
	p, _ := writeFixture(t)
	out, _, err := runCLI(t, p, "--level", "L2", "--coordinate", "src/alpha", "--region-depth", "2")
	if err != nil {
		t.Fatal(err)
	}
	x := decode(t, out)
	cov := obj(x["coverage"])
	if cov["scope"] != "global/unscoped" || len(list(cov["items"])) != 1 {
		t.Fatalf("coverage scope/rows changed: %v", cov)
	}
	s := obj(x["unknown_summary"])
	if s["total"] != float64(3) || obj(s["by_kind"])["parse"] != float64(2) {
		t.Fatalf("global unknown aggregates incorrect: %v", s)
	}
	a := obj(x["affected_unknown_summary"])
	if a["total"] != float64(1) || obj(a["by_kind"])["parse"] != float64(1) || len(list(a["groups"])) != 1 {
		t.Fatalf("affected unknown scope incorrect: %v", a)
	}
	counts := obj(x["task_candidate_counts"])
	if counts["matched"] != float64(1) || counts["returned"] != float64(1) || counts["omitted"] != float64(0) {
		t.Fatalf("counts=%v", counts)
	}
}

func TestL2DefaultReviewUnknownScopeAndFilterIntersection(t *testing.T) {
	p, _ := writeFixture(t)
	out, _, err := runCLI(t, p, "--level", "L2")
	if err != nil {
		t.Fatal(err)
	}
	x := decode(t, out)
	if len(list(x["task_candidates"])) != 0 || len(list(x["review_candidates"])) != 2 {
		t.Fatalf("unfiltered L2 should show no task matches and the global review queue: %v", x)
	}
	a := obj(x["affected_unknown_summary"])
	if a["total"] != float64(2) || obj(a["by_kind"])["read"] != float64(1) || obj(a["by_kind"])["parse"] != float64(1) {
		t.Fatalf("default affected summary missed a review region: %v", a)
	}
	out, _, err = runCLI(t, p, "--level", "L2", "--coordinate", "src/alpha", "--prefix", "src/alphabet")
	if err != nil {
		t.Fatal(err)
	}
	x = decode(t, out)
	if len(list(x["task_candidates"])) != 0 || obj(x["affected_unknown_summary"])["total"] != float64(0) {
		t.Fatalf("coordinate and prefix dimensions were not intersected: %v", x)
	}
}

func TestRootCoordinateAndL3ContainmentIgnoreDisplayGrouping(t *testing.T) {
	v := fixture()
	snap := obj(v["snapshot"])
	unknowns := list(snap["unknowns"])
	unknowns = append(unknowns,
		unk("u-root", "root", ".", "root_unknown", "ev4"),
		unk("u-nested", "parse", "src/alpha/beta/item.go", "nested_unknown", "ev1"),
		unk("u-neighbor", "read", "src/alpha/gamma/item.go", "neighbor_unknown", "ev2"),
	)
	snap["unknowns"] = unknowns
	set := obj(v["candidates"])
	cs := list(set["candidates"])
	cs = append(cs,
		cand("c-root", ".", "Repository", "SUPPORTED", false, []string{}, []string{}),
		cand("c-deep", "src/alpha/beta", "Beta", "SUPPORTED", false, []string{"n1"}, []string{"ev1"}),
	)
	set["candidates"] = cs
	p, _ := writeArtifact(t, v)
	out, _, err := runCLI(t, p, "--level", "L3", "--coordinate", ".")
	if err != nil {
		t.Fatal(err)
	}
	x := decode(t, out)
	d := obj(x["data"])
	if len(list(d["task_candidates"])) != 1 || obj(list(d["task_candidates"])[0])["coordinate"] != "." {
		t.Fatalf("root coordinate was not accepted/preserved: %v", d)
	}
	rootUnknown := false
	for _, item := range list(d["unknown_records"]) {
		if obj(item)["id"] == "u-root" {
			rootUnknown = true
		}
	}
	if !rootUnknown {
		t.Fatalf("root selection lost root unknown: %v", d["unknown_records"])
	}
	out, _, err = runCLI(t, p, "--level", "L2", "--prefix", "src/alpha")
	if err != nil {
		t.Fatal(err)
	}
	affected := obj(decode(t, out)["affected_unknown_summary"])
	for _, group := range list(affected["groups"]) {
		if obj(group)["region"] != "src/alpha" {
			t.Fatalf("nested candidate changed the query-boundary group: %v", affected["groups"])
		}
	}
	out, _, err = runCLI(t, p, "--level", "L3", "--coordinate", "src/alpha/beta")
	if err != nil {
		t.Fatal(err)
	}
	d = obj(decode(t, out)["data"])
	uids := []string{}
	for _, item := range list(d["unknown_records"]) {
		uids = append(uids, obj(item)["id"].(string))
	}
	if len(uids) != 1 || uids[0] != "u-nested" {
		t.Fatalf("L3 used display grouping instead of selected containment: %v", d["unknown_records"])
	}
}

func TestL3ExplicitSelectionRecoversOnlySelectedEvidence(t *testing.T) {
	p, _ := writeFixture(t)
	out, _, err := runCLI(t, p, "--level", "L3", "--coordinate", "src/alpha")
	if err != nil {
		t.Fatal(err)
	}
	x := decode(t, out)
	d := obj(x["data"])
	if len(list(d["task_candidates"])) != 1 || len(list(d["members"])) != 2 || len(list(d["evidence"])) != 1 {
		t.Fatalf("selected details incomplete or widened: %v", d)
	}
	all := stringsIn(d)
	if has(all, "n3") || has(all, "n4") || has(all, "ev2") || has(all, "ev3") || has(all, "u2") || has(all, "u3") {
		t.Fatalf("unrelated data leaked into selected drill: %v", d)
	}
	if !has(all, "physical_node_ids") || !has(all, "signals") || !has(all, "evidence_refs") || !has(all, "unknown_records") {
		t.Fatalf("drill omitted provenance structure: %v", d)
	}
}

func TestDeterminismBudgetAndInputErrors(t *testing.T) {
	p, _ := writeFixture(t)
	a, _, err := runCLI(t, p, "--level", "L2")
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := runCLI(t, p, "--level", "L2")
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatal("same input produced different JSON")
	}
	out, _, err := runCLI(t, p, "--level", "L1", "--limit", "1")
	if err != nil {
		t.Fatal(err)
	}
	x := decode(t, out)
	c := obj(x["task_candidate_counts"])
	if c["matched"] != float64(3) || c["returned"] != float64(1) || c["omitted"] != float64(2) {
		t.Fatalf("omissions not disclosed: %v", c)
	}
	out, _, err = runCLI(t, p, "--level", "L2", "--limit", "1")
	if err != nil {
		t.Fatal(err)
	}
	x = decode(t, out)
	c = obj(x["review_candidate_counts"])
	if c["matched"] != float64(2) || c["returned"] != float64(1) || c["omitted"] != float64(1) {
		t.Fatalf("review omissions not disclosed: %v", c)
	}
	out, stderr, err := runCLI(t, p, "--level", "L2", "--max-bytes", "5")
	if err == nil || len(out) != 0 || len(stderr) == 0 {
		t.Fatalf("budget must fail closed stdout=%q stderr=%q err=%v", out, stderr, err)
	}
	full, _, err := runCLI(t, p, "--level", "L2", "--max-bytes", "100000")
	if err != nil {
		t.Fatal(err)
	}
	if len(full) < 2 {
		t.Fatalf("unexpectedly short output %q", full)
	}
	_, _, err = runCLI(t, p, "--level", "L2", "--max-bytes", fmt.Sprint(len(full)-1))
	if err == nil {
		t.Fatal("output budget failed to count final newline")
	}
	fit, _, err := runCLI(t, p, "--level", "L2", "--max-bytes", fmt.Sprint(len(full)))
	if err != nil || string(fit) != string(full) {
		t.Fatalf("output should fit exact newline-inclusive budget: err=%v", err)
	}
	for _, bad := range []string{"{not json}", `{"snapshot":{},"candidates":{}}`, `{"snapshot":{"subject":{"id":"one"},"nodes":[]},"candidates":{"subject":{"id":"two"},"candidates":[]}}`} {
		q := filepath.Join(t.TempDir(), "bad.json")
		_ = os.WriteFile(q, []byte(bad), 0600)
		out, _, err = runCLI(t, q, "--level", "L0")
		if err == nil || len(out) != 0 {
			t.Fatalf("bad input accepted or leaked raw bytes: %q", out)
		}
	}
	for _, args := range [][]string{{"--level", "L3"}, {"--level", "L3", "--prefix", "../escape"}} {
		out, _, err = runCLI(t, p, args...)
		if err == nil || len(out) != 0 {
			t.Fatalf("invalid drill request accepted: args=%v stdout=%q", args, out)
		}
	}
}
