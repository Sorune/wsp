package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
)

// StableID hashes a sequence with length prefixes, avoiding delimiter
// ambiguity while keeping identities independent of machine paths/locations.
func StableID(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		fmt.Fprintf(h, "%d:", len([]byte(p)))
		h.Write([]byte(p))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
func NodeID(subjectID, kind, coordinate string) string {
	return StableID("node", subjectID, kind, coordinate)
}
func EdgeID(kind, from, to string) string { return StableID("edge", kind, from, to) }

func validCoordinate(s string) bool {
	if s == "." {
		return true
	}
	if s == "" || strings.ContainsAny(s, "\\\x00\r\n") || strings.HasPrefix(s, "/") || path.IsAbs(s) || (len(s) > 1 && s[1] == ':') || path.Clean(s) != s {
		return false
	}
	for _, p := range strings.Split(s, "/") {
		if p == "" || p == "." || p == ".." {
			return false
		}
	}
	return true
}
func cloneStrings(in []string) []string { return append([]string(nil), in...) }
func sortUniqueStrings(s []string, what string) ([]string, error) {
	sort.Strings(s)
	for i := 1; i < len(s); i++ {
		if s[i] == s[i-1] {
			return nil, fmt.Errorf("duplicate %s %q", what, s[i])
		}
	}
	return s, nil
}

// Normalize copies, validates, and deterministically orders a batch. It never
// changes the caller's maps, slices, or structs.
func Normalize(in ObservationBatch) (ObservationBatch, error) {
	out := in
	out.Nodes = append([]Node(nil), in.Nodes...)
	out.Edges = append([]Edge(nil), in.Edges...)
	out.Evidence = append([]Evidence(nil), in.Evidence...)
	out.Coverage = append([]Coverage(nil), in.Coverage...)
	out.Unknowns = append([]Unknown(nil), in.Unknowns...)
	out.Diagnostics = append([]Diagnostic(nil), in.Diagnostics...)
	if strings.TrimSpace(out.Subject.ID) == "" || strings.TrimSpace(out.Producer.ID) == "" || strings.TrimSpace(out.Producer.Version) == "" || strings.TrimSpace(out.Producer.ModelVersion) == "" || strings.TrimSpace(out.Producer.DerivationModelVersion) == "" {
		return out, fmt.Errorf("subject and producer identity/version are required")
	}
	for i := range out.Nodes {
		n := &out.Nodes[i]
		if n.Kind == "" || n.Name == "" || !validCoordinate(n.Coordinate) {
			return out, fmt.Errorf("invalid node %q kind/name/coordinate", n.ID)
		}
		if n.ID != NodeID(out.Subject.ID, n.Kind, n.Coordinate) {
			return out, fmt.Errorf("node ID mismatch at %q", n.Coordinate)
		}
		m := map[string]string{}
		for k, v := range n.Attributes {
			m[k] = v
		}
		n.Attributes = m
		n.EvidenceRefs = cloneStrings(n.EvidenceRefs)
		if _, e := sortUniqueStrings(n.EvidenceRefs, "node evidence reference"); e != nil {
			return out, e
		}
	}
	for i := range out.Edges {
		e := &out.Edges[i]
		if e.Kind == "" || e.From == "" || e.To == "" || (e.Resolution != "" && !oneOf(e.Resolution, "RESOLVED", "UNRESOLVED", "AMBIGUOUS", "OBSERVED", "EXPLICIT_LITERAL")) || e.ID != EdgeID(e.Kind, e.From, e.To) {
			return out, fmt.Errorf("invalid edge %q", e.ID)
		}
		e.EvidenceRefs = cloneStrings(e.EvidenceRefs)
		if _, err := sortUniqueStrings(e.EvidenceRefs, "edge evidence reference"); err != nil {
			return out, err
		}
	}
	for i := range out.Evidence {
		e := &out.Evidence[i]
		if e.Kind == "" || e.Path == "" || e.Line < 0 || e.Column < 0 {
			return out, fmt.Errorf("invalid evidence %q", e.ID)
		}
		if e.ID != StableID("evidence", e.Kind, e.Path, fmt.Sprint(e.Line), fmt.Sprint(e.Column), e.Detail) {
			return out, fmt.Errorf("evidence ID mismatch %q", e.ID)
		}
	}
	for i := range out.Unknowns {
		u := &out.Unknowns[i]
		if u.Kind == "" || u.Reason == "" || !validCoordinate(u.Coordinate) || u.ID != StableID("unknown", out.Subject.ID, u.Kind, u.Coordinate) {
			return out, fmt.Errorf("invalid unknown %q", u.ID)
		}
		u.EvidenceRefs = cloneStrings(u.EvidenceRefs)
		if _, err := sortUniqueStrings(u.EvidenceRefs, "unknown evidence reference"); err != nil {
			return out, err
		}
	}
	for _, c := range out.Coverage {
		if c.Capability == "" || !oneOf(c.State, "COMPLETE", "PARTIAL", "UNAVAILABLE", "NOT_APPLICABLE", "UNSUPPORTED", "FAILED") {
			return out, fmt.Errorf("invalid coverage %q/%q", c.Capability, c.State)
		}
	}
	for _, d := range out.Diagnostics {
		if d.Code == "" || d.Message == "" || !oneOf(d.Severity, "INFO", "WARNING", "ERROR") || (d.Coordinate != "" && !validCoordinate(d.Coordinate)) {
			return out, fmt.Errorf("invalid diagnostic %q", d.Code)
		}
	}
	sort.Slice(out.Nodes, func(i, j int) bool { return out.Nodes[i].ID < out.Nodes[j].ID })
	sort.Slice(out.Edges, func(i, j int) bool { return out.Edges[i].ID < out.Edges[j].ID })
	sort.Slice(out.Evidence, func(i, j int) bool { return out.Evidence[i].ID < out.Evidence[j].ID })
	sort.Slice(out.Coverage, func(i, j int) bool {
		if out.Coverage[i].Capability != out.Coverage[j].Capability {
			return out.Coverage[i].Capability < out.Coverage[j].Capability
		}
		return out.Coverage[i].State < out.Coverage[j].State
	})
	for i := 1; i < len(out.Coverage); i++ {
		if out.Coverage[i].Capability == out.Coverage[i-1].Capability {
			return out, fmt.Errorf("duplicate coverage capability %s", out.Coverage[i].Capability)
		}
	}
	sort.Slice(out.Unknowns, func(i, j int) bool { return out.Unknowns[i].ID < out.Unknowns[j].ID })
	key := func(v any) string { b, _ := json.Marshal(v); return string(b) }
	sort.Slice(out.Diagnostics, func(i, j int) bool { return key(out.Diagnostics[i]) < key(out.Diagnostics[j]) })
	for i := 1; i < len(out.Nodes); i++ {
		if out.Nodes[i].ID == out.Nodes[i-1].ID {
			return out, fmt.Errorf("duplicate node %s", out.Nodes[i].ID)
		}
	}
	for i := 1; i < len(out.Edges); i++ {
		if out.Edges[i].ID == out.Edges[i-1].ID {
			return out, fmt.Errorf("duplicate edge %s", out.Edges[i].ID)
		}
	}
	for i := 1; i < len(out.Evidence); i++ {
		if out.Evidence[i].ID == out.Evidence[i-1].ID {
			return out, fmt.Errorf("duplicate evidence %s", out.Evidence[i].ID)
		}
	}
	for i := 1; i < len(out.Unknowns); i++ {
		if out.Unknowns[i].ID == out.Unknowns[i-1].ID {
			return out, fmt.Errorf("duplicate unknown %s", out.Unknowns[i].ID)
		}
	}
	nodes := map[string]bool{}
	for _, n := range out.Nodes {
		nodes[n.ID] = true
	}
	ev := map[string]bool{}
	for _, e := range out.Evidence {
		ev[e.ID] = true
	}
	for _, e := range out.Edges {
		if !nodes[e.From] || !nodes[e.To] {
			return out, fmt.Errorf("edge %s has dangling endpoint", e.ID)
		}
	}
	for _, n := range out.Nodes {
		if err := checkRefs(n.EvidenceRefs, ev); err != nil {
			return out, err
		}
	}
	for _, e := range out.Edges {
		if err := checkRefs(e.EvidenceRefs, ev); err != nil {
			return out, err
		}
	}
	for _, u := range out.Unknowns {
		if err := checkRefs(u.EvidenceRefs, ev); err != nil {
			return out, err
		}
	}
	return out, nil
}
func checkRefs(refs []string, valid map[string]bool) error {
	for _, r := range refs {
		if !valid[r] {
			return fmt.Errorf("dangling evidence reference %s", r)
		}
	}
	return nil
}
func oneOf(v string, choices ...string) bool {
	for _, c := range choices {
		if v == c {
			return true
		}
	}
	return false
}
