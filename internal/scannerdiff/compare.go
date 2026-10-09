// Package scannerdiff provides read-only, bounded comparison of compatible WSP Scanner artifacts.
package scannerdiff

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"reflect"
	"sort"
	"strings"
)

type candidate struct {
	ID         string   `json:"id"`
	Coordinate string   `json:"coordinate"`
	Label      string   `json:"label"`
	ParentID   string   `json:"parent_id,omitempty"`
	Confidence string   `json:"confidence"`
	Review     bool     `json:"review_required"`
	Reasons    []string `json:"review_reasons"`
}
type node struct {
	Kind       string            `json:"kind"`
	Coordinate string            `json:"coordinate"`
	Attributes map[string]string `json:"attributes"`
}
type unknown struct {
	Kind       string `json:"kind"`
	Coordinate string `json:"coordinate"`
	Reason     string `json:"reason"`
}
type coverage struct {
	Capability string `json:"capability"`
	State      string `json:"state"`
}
type artifact struct {
	Snapshot struct {
		Schema  int `json:"schema_version"`
		Subject struct {
			ID       string `json:"id"`
			Revision string `json:"revision"`
		} `json:"subject"`
		Producer json.RawMessage   `json:"producer"`
		Config   json.RawMessage   `json:"config"`
		Digests  map[string]string `json:"digests"`
		Nodes    []node            `json:"nodes"`
		Unknowns []unknown         `json:"unknowns"`
		Coverage []coverage        `json:"coverage"`
	} `json:"snapshot"`
	Candidates struct {
		Subject struct {
			ID string `json:"id"`
		} `json:"subject"`
		Producer json.RawMessage `json:"producer"`
		Config   json.RawMessage `json:"config"`
		Digest   string          `json:"digest"`
		Source   string          `json:"source_snapshot_digest"`
		Items    []candidate     `json:"candidates"`
	} `json:"candidates"`
}
type reference struct {
	ID       string `json:"id"`
	Bindings []struct {
		Node       string `json:"lens_node"`
		Coordinate string `json:"coordinate"`
	} `json:"bindings"`
	Groups []struct {
		Node   string `json:"lens_node"`
		Prefix string `json:"coordinate_prefix"`
	} `json:"groups"`
	Relations []struct {
		ID   string `json:"id"`
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"relations"`
}
type affected struct {
	Nodes     []string `json:"lens_nodes"`
	Relations []string `json:"lens_relations"`
}
type delta struct {
	Change   string     `json:"change"`
	Before   *candidate `json:"before,omitempty"`
	After    *candidate `json:"after,omitempty"`
	Affected affected   `json:"affected_declared_regions"`
}
type relocation struct {
	Identity   string   `json:"manifest_identity"`
	Before     string   `json:"before"`
	After      string   `json:"after"`
	Assessment string   `json:"assessment"`
	Affected   affected `json:"affected_declared_regions"`
}
type unknownDelta struct {
	Kind     string   `json:"kind"`
	Reason   string   `json:"reason"`
	Region   string   `json:"affected_coordinate"`
	Before   int      `json:"before"`
	After    int      `json:"after"`
	Affected affected `json:"affected_declared_regions"`
}
type coverageDelta struct {
	Capability string `json:"capability"`
	Before     string `json:"before"`
	After      string `json:"after"`
}
type contentChange struct {
	Coordinate string `json:"coordinate"`
	Candidate  string `json:"nearest_candidate"`
	Assessment string `json:"assessment"`
}
type identity struct {
	Subject         string            `json:"subject"`
	Revision        string            `json:"revision"`
	RawSHA          string            `json:"raw_sha256"`
	RawBytes        int               `json:"raw_bytes"`
	Digests         map[string]string `json:"snapshot_digests"`
	CandidateDigest string            `json:"candidate_digest"`
	Candidates      int               `json:"candidate_count"`
	Review          int               `json:"review_count"`
	Unknown         int               `json:"unknown_count"`
}
type result struct {
	Model        string          `json:"projection_model"`
	Reference    string          `json:"reference_id,omitempty"`
	ReferenceSHA string          `json:"reference_sha256,omitempty"`
	Baseline     identity        `json:"baseline"`
	Current      identity        `json:"current"`
	Domains      map[string]bool `json:"digest_changed"`
	Deltas       []delta         `json:"candidate_deltas"`
	Relocations  []relocation    `json:"relocation_hypotheses"`
	ReviewDelta  []string        `json:"review_required_changed_coordinates"`
	Coverage     []coverageDelta `json:"coverage_capability_deltas"`
	Unknowns     []unknownDelta  `json:"unknown_group_deltas"`
	Content      []contentChange `json:"same_path_file_content_changes"`
	Added        int             `json:"candidate_added"`
	Removed      int             `json:"candidate_removed"`
	Changed      int             `json:"candidate_changed"`
	Suppressed   int             `json:"unchanged_candidates_suppressed"`
	Constraints  []string        `json:"interpretation_constraints"`
}

func hash(b []byte) string    { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func within(c, p string) bool { return p == "." || c == p || strings.HasPrefix(c, p+"/") }
func valid(c string) bool {
	return c != "" && !path.IsAbs(c) && path.Clean(c) == c && c != ".." && !strings.HasPrefix(c, "../")
}
func sameJSON(a, b json.RawMessage) bool {
	var x, y any
	return json.Unmarshal(a, &x) == nil && json.Unmarshal(b, &y) == nil && reflect.DeepEqual(x, y)
}
func sameModels(a, b json.RawMessage) bool {
	var x, y map[string]any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	// Observer and derivation have distinct producer IDs in the existing contract.
	delete(x, "id")
	delete(y, "id")
	return reflect.DeepEqual(x, y)
}
func validate(a artifact) error {
	s, c := a.Snapshot, a.Candidates
	if s.Schema != 1 || s.Subject.ID == "" || c.Subject.ID != s.Subject.ID || c.Source != s.Digests["structure"] || c.Digest == "" {
		return errors.New("invalid or incoherent artifact provenance")
	}
	if !sameModels(s.Producer, c.Producer) || !sameJSON(s.Config, c.Config) {
		return errors.New("snapshot/candidate producer or configuration mismatch")
	}
	for _, k := range []string{"structure", "evidence", "coverage", "producer", "config", "artifact"} {
		if s.Digests[k] == "" {
			return fmt.Errorf("missing %s digest", k)
		}
	}
	ids, coords := map[string]bool{}, map[string]bool{}
	for _, v := range c.Items {
		if !valid(v.Coordinate) || v.ID == "" || ids[v.ID] || coords[v.Coordinate] {
			return errors.New("invalid/duplicate candidate identity")
		}
		ids[v.ID] = true
		coords[v.Coordinate] = true
	}
	for _, n := range s.Nodes {
		if !valid(n.Coordinate) {
			return errors.New("invalid node coordinate")
		}
	}
	for _, u := range s.Unknowns {
		if !valid(u.Coordinate) {
			return errors.New("invalid unknown coordinate")
		}
	}
	return nil
}
func identify(a artifact, raw []byte) identity {
	n := 0
	for _, c := range a.Candidates.Items {
		if c.Review {
			n++
		}
	}
	return identity{a.Snapshot.Subject.ID, a.Snapshot.Subject.Revision, hash(raw), len(raw), a.Snapshot.Digests, a.Candidates.Digest, len(a.Candidates.Items), n, len(a.Snapshot.Unknowns)}
}
func affectedAt(r reference, coords ...string) affected {
	nodes, rels := map[string]bool{}, map[string]bool{}
	for _, coord := range coords {
		for _, b := range r.Bindings {
			if within(coord, b.Coordinate) || within(b.Coordinate, coord) {
				nodes[b.Node] = true
			}
		}
		for _, g := range r.Groups {
			if within(coord, g.Prefix) || within(g.Prefix, coord) {
				nodes[g.Node] = true
			}
		}
	}
	for _, rel := range r.Relations {
		if nodes[rel.From] || nodes[rel.To] {
			rels[rel.ID] = true
		}
	}
	return affected{keys(nodes), keys(rels)}
}
func keys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func candidateMap(a artifact) map[string]candidate {
	m := map[string]candidate{}
	for _, c := range a.Candidates.Items {
		c.Reasons = append([]string{}, c.Reasons...)
		sort.Strings(c.Reasons)
		m[c.Coordinate] = c
	}
	return m
}
func manifestMap(a artifact) map[string][]string {
	m := map[string][]string{}
	for _, n := range a.Snapshot.Nodes {
		var value string
		switch n.Kind {
		case "npm.package":
			value = n.Attributes["name"]
		case "go.module":
			value = n.Attributes["module"]
		default:
			continue
		}
		if value != "" {
			k := n.Kind + ":" + value
			m[k] = append(m[k], n.Attributes["directory"])
		}
	}
	return m
}
func unknownMap(a artifact) map[string]int {
	m := map[string]int{}
	for _, u := range a.Snapshot.Unknowns {
		m[u.Kind+"\x00"+u.Reason+"\x00"+u.Coordinate]++
	}
	return m
}
func compare(a, b artifact, rawA, rawB, rawR []byte, r reference) (result, error) {
	out := result{}
	if err := validate(a); err != nil {
		return out, err
	}
	if err := validate(b); err != nil {
		return out, err
	}
	if a.Snapshot.Subject.ID != b.Snapshot.Subject.ID {
		return out, errors.New("different subjects are not comparable")
	}
	if !sameJSON(a.Snapshot.Producer, b.Snapshot.Producer) || !sameJSON(a.Candidates.Producer, b.Candidates.Producer) || !sameJSON(a.Snapshot.Config, b.Snapshot.Config) || a.Snapshot.Digests["producer"] != b.Snapshot.Digests["producer"] || a.Snapshot.Digests["config"] != b.Snapshot.Digests["config"] {
		return out, errors.New("producer/config changed: rescan with matching versions/configuration")
	}
	for _, x := range r.Bindings {
		if !valid(x.Coordinate) {
			return out, errors.New("invalid reference binding")
		}
	}
	for _, x := range r.Groups {
		if !valid(x.Prefix) {
			return out, errors.New("invalid reference region")
		}
	}
	out = result{Model: "wsp-scanner-drift-v1", Reference: r.ID, Baseline: identify(a, rawA), Current: identify(b, rawB), Domains: map[string]bool{}, Deltas: []delta{}, Relocations: []relocation{}, ReviewDelta: []string{}, Coverage: []coverageDelta{}, Unknowns: []unknownDelta{}, Content: []contentChange{}, Constraints: []string{
		"Physical structure digest includes file content hashes; changed digest alone does not imply boundary drift.",
		"Relocation matches require globally unique unchanged manifest identity; they remain hypotheses, not automatic semantic reparenting.",
		"Affected Lens regions are coordinate intersections with frozen explicit bindings; no logical relation is inferred.",
		"Unknown deltas are counted kind/reason/coordinate groups. Coverage capabilities are global, not region-attributed.",
		"No reference/cursor/Lens update is performed. All calibration proposals require Human review."}}
	if len(rawR) != 0 {
		out.ReferenceSHA = hash(rawR)
	}
	for _, k := range []string{"structure", "evidence", "coverage", "producer", "config"} {
		out.Domains[k] = a.Snapshot.Digests[k] != b.Snapshot.Digests[k]
	}
	out.Domains["candidate_set"] = a.Candidates.Digest != b.Candidates.Digest
	ma, mb := candidateMap(a), candidateMap(b)
	all := map[string]bool{}
	for k := range ma {
		all[k] = true
	}
	for k := range mb {
		all[k] = true
	}
	for _, coord := range keys(all) {
		x, xa := ma[coord]
		y, ya := mb[coord]
		d := delta{Affected: affectedAt(r, coord)}
		switch {
		case !xa:
			d.Change = "added"
			d.After = &y
			out.Added++
		case !ya:
			d.Change = "removed"
			d.Before = &x
			out.Removed++
		case !reflect.DeepEqual(x, y):
			d.Change = "changed"
			d.Before = &x
			d.After = &y
			out.Changed++
		default:
			out.Suppressed++
			continue
		}
		out.Deltas = append(out.Deltas, d)
		if (xa && x.Review) != (ya && y.Review) {
			out.ReviewDelta = append(out.ReviewDelta, coord)
		}
	}
	pa, pb := manifestMap(a), manifestMap(b)
	for _, k := range keys(pa) {
		x, y := pa[k], pb[k]
		if len(x) == 1 && len(y) == 1 && x[0] != y[0] {
			_, oldStill := mb[x[0]]
			_, newBefore := ma[y[0]]
			if !oldStill && !newBefore {
				out.Relocations = append(out.Relocations, relocation{k, x[0], y[0], "possible relocation/reidentification; semantic parent unresolved", affectedAt(r, x[0], y[0])})
			}
		}
	}
	ua, ub := unknownMap(a), unknownMap(b)
	uk := map[string]bool{}
	for k := range ua {
		uk[k] = true
	}
	for k := range ub {
		uk[k] = true
	}
	for _, k := range keys(uk) {
		if ua[k] != ub[k] {
			p := strings.Split(k, "\x00")
			out.Unknowns = append(out.Unknowns, unknownDelta{p[0], p[1], p[2], ua[k], ub[k], affectedAt(r, p[2])})
		}
	}
	ca, cb := map[string]string{}, map[string]string{}
	ck := map[string]bool{}
	for _, x := range a.Snapshot.Coverage {
		ca[x.Capability] = x.State
		ck[x.Capability] = true
	}
	for _, x := range b.Snapshot.Coverage {
		cb[x.Capability] = x.State
		ck[x.Capability] = true
	}
	for _, k := range keys(ck) {
		if ca[k] != cb[k] {
			out.Coverage = append(out.Coverage, coverageDelta{k, ca[k], cb[k]})
		}
	}
	fa := map[string]string{}
	for _, n := range a.Snapshot.Nodes {
		if n.Kind == "file" {
			fa[n.Coordinate] = n.Attributes["content_sha256"]
		}
	}
	for _, n := range b.Snapshot.Nodes {
		if n.Kind != "file" {
			continue
		}
		old, exists := fa[n.Coordinate]
		if !exists || old == n.Attributes["content_sha256"] {
			continue
		}
		nearest := ""
		for coord := range mb {
			if within(n.Coordinate, coord) && len(coord) > len(nearest) {
				nearest = coord
			}
		}
		assessment := "content change alone does not imply Lens update; no candidate boundary change at nearest coordinate"
		if x, ok := ma[nearest]; !ok || !reflect.DeepEqual(x, mb[nearest]) {
			assessment = "content change overlaps candidate delta; inspect if semantic meaning matters"
		}
		out.Content = append(out.Content, contentChange{n.Coordinate, nearest, assessment})
	}
	sort.Slice(out.Content, func(i, j int) bool { return out.Content[i].Coordinate < out.Content[j].Coordinate })
	return out, nil
}
func Run(args []string, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("wsp-scanner-compare", flag.ContinueOnError)
	f.SetOutput(stderr)
	baseline := f.String("baseline", "", "frozen combined Scanner artifact A")
	current := f.String("current", "", "combined Scanner artifact B")
	ref := f.String("reference", "", "optional declared Lens reference-map JSON")
	budget := f.Int("max-bytes", 16384, "output byte cap; over-budget fails without stdout")
	if err := f.Parse(args); err != nil {
		return 2
	}
	if *baseline == "" || *current == "" || f.NArg() != 0 || *budget <= 0 {
		fmt.Fprintln(stderr, "baseline/current and positive budget required")
		return 2
	}
	aRaw, e := os.ReadFile(*baseline)
	if e != nil {
		fmt.Fprintln(stderr, e)
		return 2
	}
	bRaw, e := os.ReadFile(*current)
	if e != nil {
		fmt.Fprintln(stderr, e)
		return 2
	}
	var rRaw []byte
	var r reference
	if *ref != "" {
		rRaw, e = os.ReadFile(*ref)
		if e != nil {
			fmt.Fprintln(stderr, e)
			return 2
		}
	}
	var a, b artifact
	inputs := []struct {
		raw    []byte
		target any
	}{{aRaw, &a}, {bRaw, &b}}
	if len(rRaw) != 0 {
		inputs = append(inputs, struct {
			raw    []byte
			target any
		}{rRaw, &r})
	}
	for _, v := range inputs {
		decoder := json.NewDecoder(bytes.NewReader(v.raw))
		if e = decoder.Decode(v.target); e == nil {
			var extra any
			if decoder.Decode(&extra) != io.EOF {
				e = errors.New("trailing JSON")
			}
		}
		if e != nil {
			fmt.Fprintln(stderr, e)
			return 2
		}
	}
	out, e := compare(a, b, aRaw, bRaw, rRaw, r)
	if e != nil {
		fmt.Fprintln(stderr, e)
		return 2
	}
	raw, e := json.MarshalIndent(out, "", "  ")
	if e != nil {
		fmt.Fprintln(stderr, e)
		return 2
	}
	raw = append(raw, '\n')
	if len(raw) > *budget {
		fmt.Fprintf(stderr, "projection over budget: %d > %d; no partial output\n", len(raw), *budget)
		return 2
	}
	if _, e = stdout.Write(raw); e != nil {
		fmt.Fprintln(stderr, e)
		return 1
	}
	return 0
}
