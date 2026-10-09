// Package scannerview provides bounded, deterministic AI-facing projections of Scanner artifacts.
package scannerview

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
)

const (
	defaultLimit    = 40
	defaultMaxBytes = 16384
	defaultDepth    = 2
)

type producer struct {
	ID                     string `json:"id"`
	Version                string `json:"version"`
	ModelVersion           string `json:"model_version"`
	DerivationModelVersion string `json:"derivation_model_version"`
}
type subject struct {
	ID          string `json:"id"`
	Revision    string `json:"revision"`
	Branch      string `json:"branch"`
	WorkingTree string `json:"working_tree"`
}
type digests struct {
	Structure string `json:"structure"`
	Evidence  string `json:"evidence"`
	Coverage  string `json:"coverage"`
	Producer  string `json:"producer"`
	Config    string `json:"config"`
	Artifact  string `json:"artifact"`
}
type coverage struct {
	Capability string `json:"capability"`
	State      string `json:"state"`
}
type evidence struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	Detail string `json:"detail,omitempty"`
}
type node struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Coordinate    string `json:"coordinate"`
	rawCoordinate string
	Name          string            `json:"name"`
	Attributes    map[string]string `json:"attributes"`
	EvidenceRefs  []string          `json:"evidence_refs"`
}
type edge struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	From         string   `json:"from"`
	To           string   `json:"to"`
	Resolution   string   `json:"resolution"`
	EvidenceRefs []string `json:"evidence_refs"`
}
type unknown struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Coordinate    string `json:"coordinate"`
	rawCoordinate string
	Reason        string   `json:"reason"`
	EvidenceRefs  []string `json:"evidence_refs"`
}
type signal struct {
	Kind         string   `json:"kind"`
	Value        string   `json:"value"`
	NodeIDs      []string `json:"node_ids"`
	EvidenceRefs []string `json:"evidence_refs"`
}
type candidate struct {
	ID              string `json:"id"`
	Coordinate      string `json:"coordinate"`
	rawCoordinate   string
	Label           string   `json:"label"`
	ParentID        string   `json:"parent_id,omitempty"`
	PhysicalNodeIDs []string `json:"physical_node_ids"`
	Signals         []signal `json:"signals"`
	Confidence      string   `json:"confidence"`
	ReviewRequired  bool     `json:"review_required"`
	ReviewReasons   []string `json:"review_reasons"`
}
type snapshot struct {
	SchemaVersion int        `json:"schema_version"`
	Producer      producer   `json:"producer"`
	Subject       subject    `json:"subject"`
	Nodes         []node     `json:"nodes"`
	Edges         []edge     `json:"edges"`
	Evidence      []evidence `json:"evidence"`
	Coverage      []coverage `json:"coverage"`
	Unknowns      []unknown  `json:"unknowns"`
	Digests       digests    `json:"digests"`
}
type candidateSet struct {
	Producer             producer    `json:"producer"`
	Subject              subject     `json:"subject"`
	SourceSnapshotDigest string      `json:"source_snapshot_digest"`
	Digest               string      `json:"digest"`
	Candidates           []candidate `json:"candidates"`
	Coverage             []coverage  `json:"coverage"`
	Unknowns             []unknown   `json:"unknowns"`
}
type artifact struct {
	Snapshot   snapshot     `json:"snapshot"`
	Candidates candidateSet `json:"candidates"`
}
type options struct {
	input, level                            string
	coordinates, prefixes, ids, confidences []string
	review                                  bool
	limit, maxBytes, regionDepth            int
}

func cleanCoordinate(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if strings.HasPrefix(s, "/") || path.IsAbs(s) || (len(s) >= 2 && s[1] == ':') {
		return "", fmt.Errorf("coordinate must be repository-relative: %q", s)
	}
	clean := path.Clean(s)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("coordinate escapes repository: %q", s)
	}
	return clean, nil
}
func within(coord, prefix string) bool {
	if prefix == "." {
		return true
	}
	return coord == prefix || strings.HasPrefix(coord, prefix+"/")
}
func matches(c candidate, o options) bool {
	// Repeated values within each dimension are OR; the dimension itself is ANDed.
	if len(o.coordinates) > 0 {
		ok := false
		for _, v := range o.coordinates {
			if c.Coordinate == v {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(o.prefixes) > 0 {
		ok := false
		for _, v := range o.prefixes {
			if within(c.Coordinate, v) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(o.ids) > 0 {
		ok := false
		for _, v := range o.ids {
			if c.ID == v {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(o.confidences) > 0 {
		ok := false
		for _, v := range o.confidences {
			if c.Confidence == v {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return !o.review || c.ReviewRequired
}

func unknownMatchesPaths(u unknown, coords, prefixes []string) bool {
	if len(coords)+len(prefixes) == 0 {
		return false
	}
	if len(coords) > 0 {
		ok := false
		for _, c := range coords {
			if within(u.Coordinate, c) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(prefixes) > 0 {
		ok := false
		for _, p := range prefixes {
			if within(u.Coordinate, p) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
func parentCoordinate(c candidate, all []candidate) string {
	for _, p := range all {
		if p.ID == c.ParentID {
			return p.Coordinate
		}
	}
	return ""
}

type compactCandidate struct {
	ID               string   `json:"id"`
	Coordinate       string   `json:"coordinate"`
	Label            string   `json:"label"`
	Confidence       string   `json:"confidence"`
	ReviewRequired   bool     `json:"review_required"`
	ReviewReasons    []string `json:"review_reasons"`
	ParentID         string   `json:"parent_id,omitempty"`
	ParentCoordinate string   `json:"parent_coordinate,omitempty"`
}

func compact(c candidate, all []candidate) compactCandidate {
	return compactCandidate{c.ID, c.Coordinate, c.Label, c.Confidence, c.ReviewRequired, c.ReviewReasons, c.ParentID, parentCoordinate(c, all)}
}

type resultCounts struct {
	Matched  int `json:"matched"`
	Returned int `json:"returned"`
	Omitted  int `json:"omitted"`
}
type unknownGroup struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Region string `json:"region"`
	Count  int    `json:"count"`
}

func region(coord string, depth int) string {
	if coord == "" {
		return "<unscoped>"
	}
	parts := strings.Split(coord, "/")
	if len(parts) > depth {
		parts = parts[:depth]
	}
	return strings.Join(parts, "/")
}
func segmentDepth(coord string) int {
	if coord == "" || coord == "." {
		return 0
	}
	return len(strings.Split(coord, "/"))
}
func groupUnknowns(us []unknown, depth int) ([]unknownGroup, map[string]int) {
	by := map[string]int{}
	groups := map[[3]string]int{}
	for _, u := range us {
		by[u.Kind]++
		k := [3]string{u.Kind, u.Reason, region(u.Coordinate, depth)}
		groups[k]++
	}
	keys := make([][3]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		for n := 0; n < 3; n++ {
			if keys[i][n] != keys[j][n] {
				return keys[i][n] < keys[j][n]
			}
		}
		return false
	})
	out := make([]unknownGroup, 0, len(keys))
	for _, k := range keys {
		out = append(out, unknownGroup{k[0], k[1], k[2], groups[k]})
	}
	return out, by
}
func groupUnknownsAtBoundaries(us []unknown, depth int, boundaries []string) []unknownGroup {
	groups := map[[3]string]int{}
	for _, u := range us {
		boundaryDepth := 0
		for _, p := range boundaries {
			if within(u.Coordinate, p) {
				d := segmentDepth(p)
				if boundaryDepth == 0 || d < boundaryDepth {
					boundaryDepth = d
				}
			}
		}
		effectiveDepth := depth
		if boundaryDepth > effectiveDepth {
			effectiveDepth = boundaryDepth
		}
		k := [3]string{u.Kind, u.Reason, region(u.Coordinate, effectiveDepth)}
		groups[k]++
	}
	keys := make([][3]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		for n := 0; n < 3; n++ {
			if keys[i][n] != keys[j][n] {
				return keys[i][n] < keys[j][n]
			}
		}
		return false
	})
	out := make([]unknownGroup, 0, len(keys))
	for _, k := range keys {
		out = append(out, unknownGroup{k[0], k[1], k[2], groups[k]})
	}
	return out
}
func allCandidateCoords(cs []candidate) []string {
	a := make([]string, 0, len(cs))
	for _, c := range cs {
		a = append(a, c.Coordinate)
	}
	return a
}
func rawL3(a artifact, selected []candidate, depth int, queryCoords, prefixes []string) map[string]any {
	ids := map[string]bool{}
	refs := map[string]bool{}
	coords := []string{}
	for _, c := range selected {
		coords = append(coords, c.Coordinate)
		for _, id := range c.PhysicalNodeIDs {
			ids[id] = true
		}
		for _, s := range c.Signals {
			for _, r := range s.EvidenceRefs {
				refs[r] = true
			}
		}
	}
	for _, n := range a.Snapshot.Nodes {
		if ids[n.ID] {
			for _, r := range n.EvidenceRefs {
				refs[r] = true
			}
		}
	}
	for _, e := range a.Snapshot.Edges {
		if ids[e.From] && ids[e.To] {
			for _, r := range e.EvidenceRefs {
				refs[r] = true
			}
		}
	}
	unknownCoords := queryCoords
	if len(queryCoords)+len(prefixes) == 0 {
		unknownCoords = coords
	}
	us := make([]unknown, 0)
	for _, u := range a.Snapshot.Unknowns {
		if unknownMatchesPaths(u, unknownCoords, prefixes) {
			us = append(us, u)
		}
	}
	for _, u := range us {
		for _, r := range u.EvidenceRefs {
			refs[r] = true
		}
	}
	nodes := []node{}
	for _, n := range a.Snapshot.Nodes {
		if ids[n.ID] {
			nodes = append(nodes, n)
		}
	}
	edges := []edge{}
	for _, e := range a.Snapshot.Edges {
		if ids[e.From] && ids[e.To] {
			edges = append(edges, e)
		}
	}
	ev := []evidence{}
	for _, e := range a.Snapshot.Evidence {
		if refs[e.ID] {
			ev = append(ev, e)
		}
	}
	selectedRaw := append([]candidate(nil), selected...)
	for i := range selectedRaw {
		selectedRaw[i].Coordinate = selectedRaw[i].rawCoordinate
	}
	for i := range nodes {
		nodes[i].Coordinate = nodes[i].rawCoordinate
	}
	for i := range us {
		us[i].Coordinate = us[i].rawCoordinate
	}
	refList := make([]string, 0, len(refs))
	for r := range refs {
		refList = append(refList, r)
	}
	sort.Strings(refList)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	sort.Slice(ev, func(i, j int) bool { return ev[i].ID < ev[j].ID })
	sort.Slice(us, func(i, j int) bool {
		if us[i].Coordinate != us[j].Coordinate {
			return us[i].Coordinate < us[j].Coordinate
		}
		if us[i].Kind != us[j].Kind {
			return us[i].Kind < us[j].Kind
		}
		return us[i].ID < us[j].ID
	})
	return map[string]any{"task_candidates": selectedRaw, "members": nodes, "related_edges": edges, "evidence_refs": refList, "evidence": ev, "unknown_records": us}
}
func validateArtifact(a artifact) error {
	if a.Snapshot.SchemaVersion != 1 {
		return fmt.Errorf("unsupported or missing snapshot schema_version: %d", a.Snapshot.SchemaVersion)
	}
	if a.Snapshot.Subject.ID == "" || a.Snapshot.Subject.Revision == "" || a.Candidates.Subject.ID == "" || a.Candidates.Subject.Revision == "" {
		return errors.New("snapshot and candidates subjects require id and revision")
	}
	if a.Snapshot.Subject.ID != a.Candidates.Subject.ID || a.Snapshot.Subject.Revision != a.Candidates.Subject.Revision {
		return errors.New("snapshot and candidates subject id/revision mismatch")
	}
	if a.Snapshot.Producer.ID == "" || a.Snapshot.Producer.Version == "" || a.Candidates.Producer.ID == "" || a.Candidates.Producer.Version == "" {
		return errors.New("snapshot and candidates producer identity/version are required")
	}
	if a.Snapshot.Digests.Artifact == "" || a.Candidates.SourceSnapshotDigest == "" || a.Candidates.Digest == "" {
		return errors.New("snapshot and candidates provenance digests are required")
	}
	if a.Candidates.SourceSnapshotDigest != a.Snapshot.Digests.Structure {
		return errors.New("candidates source_snapshot_digest does not match snapshot structure digest")
	}
	return nil
}
func makeResult(a artifact, raw []byte, o options) (any, error) {
	if err := validateArtifact(a); err != nil {
		return nil, err
	}
	cs := a.Candidates.Candidates
	for i := range cs {
		cs[i].rawCoordinate = cs[i].Coordinate
		c, err := cleanCoordinate(cs[i].Coordinate)
		if err != nil {
			return nil, err
		}
		cs[i].Coordinate = c
	}
	for i := range a.Snapshot.Nodes {
		a.Snapshot.Nodes[i].rawCoordinate = a.Snapshot.Nodes[i].Coordinate
		c, err := cleanCoordinate(a.Snapshot.Nodes[i].Coordinate)
		if err != nil {
			return nil, err
		}
		a.Snapshot.Nodes[i].Coordinate = c
	}
	for i := range a.Snapshot.Unknowns {
		a.Snapshot.Unknowns[i].rawCoordinate = a.Snapshot.Unknowns[i].Coordinate
		c, err := cleanCoordinate(a.Snapshot.Unknowns[i].Coordinate)
		if err != nil {
			return nil, err
		}
		a.Snapshot.Unknowns[i].Coordinate = c
	}
	selected := []candidate{}
	for _, c := range cs {
		if matches(c, o) {
			selected = append(selected, c)
		}
	}
	if o.level == "L2" && len(o.coordinates)+len(o.prefixes)+len(o.ids)+len(o.confidences) == 0 && !o.review {
		selected = []candidate{}
	}
	review := []candidate{}
	for _, c := range cs {
		if c.ReviewRequired {
			review = append(review, c)
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].Coordinate != selected[j].Coordinate {
			return selected[i].Coordinate < selected[j].Coordinate
		}
		return selected[i].ID < selected[j].ID
	})
	sort.Slice(review, func(i, j int) bool {
		if review[i].Coordinate != review[j].Coordinate {
			return review[i].Coordinate < review[j].Coordinate
		}
		return review[i].ID < review[j].ID
	})
	_, byKind := groupUnknowns(a.Snapshot.Unknowns, o.regionDepth)
	h := sha256.Sum256(raw)
	base := map[string]any{"level": o.level, "subject": map[string]string{"id": a.Snapshot.Subject.ID, "revision": a.Snapshot.Subject.Revision, "branch": a.Snapshot.Subject.Branch}, "versions": map[string]string{"scanner": a.Snapshot.Producer.Version, "observation_model": a.Snapshot.Producer.ModelVersion, "derivation": a.Snapshot.Producer.DerivationModelVersion}, "counts": map[string]int{"candidates": len(cs), "review_required": len(review), "unknowns": len(a.Snapshot.Unknowns)}, "coverage": map[string]any{"scope": "global/unscoped", "items": a.Snapshot.Coverage}, "provenance": map[string]any{"digests": a.Snapshot.Digests, "raw_sha256": hex.EncodeToString(h[:])}}
	if o.level == "L0" {
		return base, nil
	}
	if o.level == "L1" || o.level == "L2" {
		limit := o.limit
		if limit < 0 {
			limit = 0
		}
		returned := selected
		if len(returned) > limit {
			returned = returned[:limit]
		}
		compactList := make([]compactCandidate, 0, len(returned))
		for _, c := range returned {
			compactList = append(compactList, compact(c, cs))
		}
		base["task_candidates"] = compactList
		base["task_candidate_counts"] = resultCounts{len(selected), len(returned), len(selected) - len(returned)}
		if o.level == "L1" {
			return base, nil
		}
		rv := review
		if len(rv) > o.limit {
			rv = rv[:o.limit]
		}
		reviewList := make([]compactCandidate, 0, len(rv))
		for _, c := range rv {
			reviewList = append(reviewList, compact(c, cs))
		}
		base["review_candidates"] = reviewList
		base["review_candidate_counts"] = resultCounts{len(review), len(rv), len(review) - len(rv)}
		base["unknown_summary"] = map[string]any{"total": len(a.Snapshot.Unknowns), "by_kind": byKind}
		coords := append(allCandidateCoords(selected), o.coordinates...)
		selectionRoots := append(append([]string(nil), coords...), o.prefixes...)
		scopeRoots := append([]string(nil), selectionRoots...)
		unboundedReviewScope := len(scopeRoots) == 0
		for _, c := range review {
			include := unboundedReviewScope
			for _, root := range scopeRoots {
				if within(c.Coordinate, root) {
					include = true
					break
				}
			}
			if include {
				coords = append(coords, c.Coordinate)
				selectionRoots = append(selectionRoots, c.Coordinate)
			}
		}
		aff := make([]unknown, 0)
		for _, u := range a.Snapshot.Unknowns {
			matched := false
			if len(o.coordinates)+len(o.prefixes) > 0 {
				matched = unknownMatchesPaths(u, o.coordinates, o.prefixes)
			} else {
				for _, root := range coords {
					if within(u.Coordinate, root) {
						matched = true
						break
					}
				}
			}
			if matched {
				aff = append(aff, u)
			}
		}
		affectedGroups := groupUnknownsAtBoundaries(aff, o.regionDepth, selectionRoots)
		_, affectedKinds := groupUnknowns(aff, o.regionDepth)
		base["affected_unknown_summary"] = map[string]any{"total": len(aff), "by_kind": affectedKinds, "groups": affectedGroups}
		return base, nil
	}
	if o.level == "L3" {
		if len(o.coordinates)+len(o.prefixes)+len(o.ids) == 0 {
			return nil, errors.New("L3 requires --coordinate, --prefix, or --id selection")
		}
		data := rawL3(a, selected, o.regionDepth, o.coordinates, o.prefixes)
		return map[string]any{"level": "L3", "subject": base["subject"], "provenance": base["provenance"], "selection_counts": resultCounts{len(selected), len(selected), 0}, "data": data}, nil
	}
	return nil, errors.New("level must be L0, L1, L2, or L3")
}
func parseOptions(args []string) (options, error) {
	var o options
	fs := flag.NewFlagSet("scanner-consumption", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&o.input, "input", "", "combined Scanner JSON file")
	fs.StringVar(&o.level, "level", "L0", "output level")
	fs.Var((*stringList)(&o.coordinates), "coordinate", "exact coordinate filter (repeatable)")
	fs.Var((*stringList)(&o.prefixes), "prefix", "segment-aware coordinate prefix (repeatable)")
	fs.Var((*stringList)(&o.ids), "id", "candidate id filter (repeatable)")
	fs.Var((*stringList)(&o.confidences), "confidence", "confidence filter (repeatable)")
	fs.BoolVar(&o.review, "review-required", false, "include only review-required candidates")
	fs.IntVar(&o.limit, "limit", defaultLimit, "maximum candidates per list")
	fs.IntVar(&o.maxBytes, "max-bytes", defaultMaxBytes, "maximum output bytes")
	fs.IntVar(&o.regionDepth, "region-depth", defaultDepth, "literal coordinate segments per display group")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() != 0 {
		return o, errors.New("unexpected positional arguments")
	}
	if o.input == "" {
		return o, errors.New("--input is required")
	}
	if o.maxBytes < 1 || o.limit < 0 || o.regionDepth < 1 {
		return o, errors.New("--max-bytes and --region-depth must be positive; --limit cannot be negative")
	}
	for i := range o.coordinates {
		c, e := cleanCoordinate(o.coordinates[i])
		if e != nil {
			return o, e
		}
		o.coordinates[i] = c
	}
	for i := range o.prefixes {
		c, e := cleanCoordinate(o.prefixes[i])
		if e != nil {
			return o, e
		}
		o.prefixes[i] = c
	}
	for _, c := range o.confidences {
		switch c {
		case "STRONG", "SUPPORTED", "WEAK", "CONFLICTED", "UNRESOLVED":
		default:
			return o, fmt.Errorf("unknown confidence %q", c)
		}
	}
	if o.level != "L0" && o.level != "L1" && o.level != "L2" && o.level != "L3" {
		return o, errors.New("level must be L0, L1, L2, or L3")
	}
	return o, nil
}

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }
func Run(args []string, stdout, stderr io.Writer) int {
	o, err := parseOptions(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	raw, err := os.ReadFile(o.input)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var a artifact
	if err = json.Unmarshal(raw, &a); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	res, err := makeResult(a, raw, o)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(b)+1 > o.maxBytes {
		fmt.Fprintf(stderr, "output exceeds --max-bytes (%d > %d)\n", len(b)+1, o.maxBytes)
		return 2
	}
	if _, err = stdout.Write(append(b, '\n')); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
