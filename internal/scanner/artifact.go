package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func digest(v any) (string, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return "", e
	}
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:]), nil
}
func normalizeConfig(c Config) (Config, error) {
	out := Config{SourceRoots: append([]string(nil), c.SourceRoots...), NamespacePrefixes: append([]string(nil), c.NamespacePrefixes...)}
	if len(out.NamespacePrefixes) != 0 {
		return out, fmt.Errorf("namespace prefixes are not supported by this observer")
	}
	if len(out.SourceRoots) == 0 {
		out.SourceRoots = []string{"."}
	}
	for _, r := range out.SourceRoots {
		if !validCoordinate(r) {
			return out, fmt.Errorf("invalid source root %q", r)
		}
	}
	for _, p := range out.NamespacePrefixes {
		if strings.TrimSpace(p) == "" || strings.ContainsAny(p, "\r\n\x00") {
			return out, fmt.Errorf("invalid namespace prefix %q", p)
		}
	}
	sort.Strings(out.SourceRoots)
	sort.Strings(out.NamespacePrefixes)
	if _, e := sortUniqueStrings(out.SourceRoots, "source root"); e != nil {
		return out, e
	}
	if _, e := sortUniqueStrings(out.NamespacePrefixes, "namespace prefix"); e != nil {
		return out, e
	}
	return out, nil
}

// BuildSnapshot validates and normalizes a batch, then computes independent
// structure, evidence, coverage, producer, config, and whole-artifact digests.
func BuildSnapshot(batch ObservationBatch, config Config) (PhysicalSnapshot, error) {
	b, e := Normalize(batch)
	if e != nil {
		return PhysicalSnapshot{}, e
	}
	c, e := normalizeConfig(config)
	if e != nil {
		return PhysicalSnapshot{}, e
	}
	s := PhysicalSnapshot{SchemaVersion: 1, Producer: b.Producer, Subject: b.Subject, Config: c, Nodes: b.Nodes, Edges: b.Edges, Evidence: b.Evidence, Coverage: b.Coverage, Unknowns: b.Unknowns, Diagnostics: b.Diagnostics}
	// Structural projection deliberately contains no revision, branch, local
	// root, evidence references, source paths, or line/column locations.
	type structNode struct {
		ID, Kind, Coordinate, Name string
		Attributes                 map[string]string
	}
	type structEdge struct{ ID, Kind, From, To, Resolution string }
	type structUnknown struct{ ID, Kind, Coordinate, Reason string }
	structure := struct {
		SubjectID string
		Nodes     []structNode
		Edges     []structEdge
		Unknowns  []structUnknown
	}{SubjectID: b.Subject.ID}
	for _, n := range b.Nodes {
		structure.Nodes = append(structure.Nodes, structNode{n.ID, n.Kind, n.Coordinate, n.Name, n.Attributes})
	}
	for _, x := range b.Edges {
		structure.Edges = append(structure.Edges, structEdge{x.ID, x.Kind, x.From, x.To, x.Resolution})
	}
	for _, u := range b.Unknowns {
		structure.Unknowns = append(structure.Unknowns, structUnknown{u.ID, u.Kind, u.Coordinate, u.Reason})
	}
	if s.Digests.Structure, e = digest(structure); e != nil {
		return PhysicalSnapshot{}, e
	}
	if s.Digests.Evidence, e = digest(struct {
		Evidence    []Evidence
		NodeRefs    map[string][]string
		EdgeRefs    map[string][]string
		UnknownRefs map[string][]string
	}{b.Evidence, collectNodeRefs(b.Nodes), collectEdgeRefs(b.Edges), collectUnknownRefs(b.Unknowns)}); e != nil {
		return PhysicalSnapshot{}, e
	}
	if s.Digests.Coverage, e = digest(struct {
		Coverage []Coverage
		Unknowns []structUnknown
	}{b.Coverage, structure.Unknowns}); e != nil {
		return PhysicalSnapshot{}, e
	}
	if s.Digests.Producer, e = digest(b.Producer); e != nil {
		return PhysicalSnapshot{}, e
	}
	if s.Digests.Config, e = digest(c); e != nil {
		return PhysicalSnapshot{}, e
	}
	s.Digests.Artifact, e = digest(s)
	if e != nil {
		return PhysicalSnapshot{}, e
	}
	return s, nil
}
func collectNodeRefs(ns []Node) map[string][]string {
	m := map[string][]string{}
	for _, n := range ns {
		m[n.ID] = n.EvidenceRefs
	}
	return m
}
func collectEdgeRefs(es []Edge) map[string][]string {
	m := map[string][]string{}
	for _, x := range es {
		m[x.ID] = x.EvidenceRefs
	}
	return m
}
func collectUnknownRefs(us []Unknown) map[string][]string {
	m := map[string][]string{}
	for _, u := range us {
		m[u.ID] = u.EvidenceRefs
	}
	return m
}

func nonNilSlice[T any](in []T) []T {
	if in == nil {
		return []T{}
	}
	return in
}

func normalizeSnapshotCollections(s PhysicalSnapshot) PhysicalSnapshot {
	s.Config.SourceRoots = nonNilSlice(s.Config.SourceRoots)
	s.Config.NamespacePrefixes = nonNilSlice(s.Config.NamespacePrefixes)
	s.Nodes = nonNilSlice(s.Nodes)
	s.Edges = nonNilSlice(s.Edges)
	s.Evidence = nonNilSlice(s.Evidence)
	s.Coverage = nonNilSlice(s.Coverage)
	s.Unknowns = nonNilSlice(s.Unknowns)
	s.Diagnostics = nonNilSlice(s.Diagnostics)
	return s
}

func normalizeCandidateCollections(s DerivedCandidateSet) DerivedCandidateSet {
	s.Config.SourceRoots = nonNilSlice(s.Config.SourceRoots)
	s.Config.NamespacePrefixes = nonNilSlice(s.Config.NamespacePrefixes)
	s.Candidates = nonNilSlice(s.Candidates)
	s.UnmappedNodeIDs = nonNilSlice(s.UnmappedNodeIDs)
	s.Coverage = nonNilSlice(s.Coverage)
	s.Unknowns = nonNilSlice(s.Unknowns)
	s.Diagnostics = nonNilSlice(s.Diagnostics)
	for i := range s.Candidates {
		s.Candidates[i].PhysicalNodeIDs = nonNilSlice(s.Candidates[i].PhysicalNodeIDs)
		s.Candidates[i].Signals = nonNilSlice(s.Candidates[i].Signals)
		s.Candidates[i].ReviewReasons = nonNilSlice(s.Candidates[i].ReviewReasons)
		for j := range s.Candidates[i].Signals {
			s.Candidates[i].Signals[j].NodeIDs = nonNilSlice(s.Candidates[i].Signals[j].NodeIDs)
			s.Candidates[i].Signals[j].EvidenceRefs = nonNilSlice(s.Candidates[i].Signals[j].EvidenceRefs)
		}
	}
	return s
}

type Bundle struct {
	Snapshot   PhysicalSnapshot    `json:"snapshot"`
	Candidates DerivedCandidateSet `json:"candidates"`
}

// SerializeSnapshot emits deterministic JSON (including a trailing newline).
func SerializeSnapshot(s PhysicalSnapshot) ([]byte, error) {
	s = normalizeSnapshotCollections(s)
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return nil, e
	}
	return append(b, '\n'), nil
}

// SerializeCandidates emits deterministic JSON for a derived candidate set.
func SerializeCandidates(s DerivedCandidateSet) ([]byte, error) {
	s = normalizeCandidateCollections(s)
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return nil, e
	}
	return append(b, '\n'), nil
}

// SerializeBundle emits the public combined Scanner artifact with stable arrays.
func SerializeBundle(snapshot PhysicalSnapshot, candidates DerivedCandidateSet) ([]byte, error) {
	b, e := json.MarshalIndent(Bundle{
		Snapshot:   normalizeSnapshotCollections(snapshot),
		Candidates: normalizeCandidateCollections(candidates),
	}, "", "  ")
	if e != nil {
		return nil, e
	}
	return append(b, '\n'), nil
}
