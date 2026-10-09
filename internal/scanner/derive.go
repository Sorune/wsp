package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"sort"
	"strings"
)

type derivationSeed struct {
	coord, label string
	members      map[string]bool
	signals      map[string]map[string]bool
	reasons      map[string]bool
}

// Derive turns physical observations into structural hypotheses. Only
// mechanically suspicious candidates require review. It does not mutate or
// feed Lens projections.
func Derive(s PhysicalSnapshot) (DerivedCandidateSet, error) {
	if len(s.Config.NamespacePrefixes) > 0 {
		return DerivedCandidateSet{}, errors.New("namespace_prefixes are unsupported by the current candidate derivation model")
	}
	canonical, err := BuildSnapshot(ObservationBatch{Producer: s.Producer, Subject: s.Subject, Nodes: s.Nodes, Edges: s.Edges, Evidence: s.Evidence, Coverage: s.Coverage, Unknowns: s.Unknowns, Diagnostics: s.Diagnostics}, s.Config)
	if err != nil {
		return DerivedCandidateSet{}, err
	}
	if s.Digests.Structure != "" && s.Digests.Structure != canonical.Digests.Structure {
		return DerivedCandidateSet{}, errors.New("snapshot structure digest mismatch")
	}
	s = canonical
	for _, n := range s.Nodes {
		if d := n.Attributes["directory"]; d != "" && !validCoordinate(d) {
			return DerivedCandidateSet{}, errors.New("snapshot contains invalid structural directory coordinate")
		}
	}
	if err := validateContainment(s.Nodes, s.Edges); err != nil {
		return DerivedCandidateSet{}, err
	}
	ignoredPaths := ignoredPathSet(s.Nodes)
	evidenceByID := make(map[string]Evidence, len(s.Evidence))
	for _, e := range s.Evidence {
		evidenceByID[e.ID] = e
	}
	deriveNodes := make([]Node, 0, len(s.Nodes))
	eligibleNodeIDs := make(map[string]bool, len(s.Nodes))
	for _, n := range s.Nodes {
		eligible, filtered := eligibleDerivationNode(n, evidenceByID, ignoredPaths)
		if !eligible {
			continue
		}
		deriveNodes = append(deriveNodes, filtered)
		eligibleNodeIDs[n.ID] = true
	}
	deriveEdges := make([]Edge, 0, len(s.Edges))
	for _, e := range s.Edges {
		if !eligibleNodeIDs[e.From] || !eligibleNodeIDs[e.To] {
			continue
		}
		if e.Kind == "workspace.use" {
			refs := make([]string, 0, len(e.EvidenceRefs))
			for _, ref := range e.EvidenceRefs {
				if ev, ok := evidenceByID[ref]; ok && !isIgnoredPath(ev.Path, ignoredPaths) {
					refs = append(refs, ref)
				}
			}
			if len(e.EvidenceRefs) > 0 && len(refs) == 0 {
				continue
			}
			e.EvidenceRefs = refs
		}
		deriveEdges = append(deriveEdges, e)
	}
	producer := Producer{ID: "wsp-scanner-derivation", Version: ScannerVersion, ModelVersion: ObservationModelVersion, DerivationModelVersion: DerivationModelVersion}
	out := DerivedCandidateSet{Subject: s.Subject, Producer: producer, SourceSnapshotDigest: s.Digests.Structure, Config: s.Config, Coverage: append([]Coverage(nil), s.Coverage...), Unknowns: append([]Unknown(nil), s.Unknowns...), Diagnostics: append([]Diagnostic(nil), s.Diagnostics...)}
	nodes := map[string]Node{}
	for _, n := range deriveNodes {
		if n.ID == "" {
			return DerivedCandidateSet{}, errors.New("snapshot contains node with empty id")
		}
		if _, ok := nodes[n.ID]; ok {
			return DerivedCandidateSet{}, errors.New("snapshot contains duplicate node id")
		}
		nodes[n.ID] = n
		if d := n.Attributes["directory"]; d != "" && !validCoordinate(d) {
			return DerivedCandidateSet{}, errors.New("snapshot contains invalid structural directory coordinate")
		}
	}
	children := map[string][]string{}
	edgeRefs := map[string][]string{}
	parent := map[string]string{}
	for _, e := range deriveEdges {
		edgeRefs[e.ID] = e.EvidenceRefs
		if e.Kind == "contains" || e.Kind == "directory.contains" || e.Kind == "filesystem.contains" {
			from, fromOK := nodes[e.From]
			to, toOK := nodes[e.To]
			if !fromOK || !toOK {
				return DerivedCandidateSet{}, errors.New("containment edge has dangling endpoint")
			}
			if from.Kind != "directory" && from.Kind != "filesystem.directory" {
				return DerivedCandidateSet{}, errors.New("containment parent is not a directory")
			}
			if to.Kind != "directory" && to.Kind != "filesystem.directory" && to.Kind != "file" {
				return DerivedCandidateSet{}, errors.New("containment child is not a filesystem entry")
			}
			if path.Dir(cleanCoordinate(to.Coordinate)) != cleanCoordinate(from.Coordinate) {
				return DerivedCandidateSet{}, errors.New("containment edge disagrees with physical coordinates")
			}
			if old := parent[to.ID]; old != "" && old != e.From {
				return DerivedCandidateSet{}, errors.New("filesystem entry has multiple containment parents")
			}
			parent[to.ID] = e.From
			children[e.From] = append(children[e.From], e.To)
		}
	}
	for start := range parent {
		seen := map[string]bool{}
		cur := start
		for cur != "" {
			if seen[cur] {
				return DerivedCandidateSet{}, errors.New("containment graph contains a cycle")
			}
			seen[cur] = true
			cur = parent[cur]
		}
	}
	for k := range children {
		sort.Strings(children[k])
	}

	seeds := map[string]*derivationSeed{}
	get := func(coord, label string) *derivationSeed {
		coord = cleanCoordinate(coord)
		if coord == "" {
			coord = "."
		}
		if x := seeds[coord]; x != nil {
			return x
		}
		x := &derivationSeed{coord: coord, label: label, members: map[string]bool{}, signals: map[string]map[string]bool{}, reasons: map[string]bool{}}
		seeds[coord] = x
		return x
	}
	addSignal := func(x *derivationSeed, kind, value, nodeID string) {
		if x.signals[kind] == nil {
			x.signals[kind] = map[string]bool{}
		}
		x.signals[kind][value+"\x00"+nodeID] = true
		if nodeID != "" {
			x.members[nodeID] = true
		}
	}
	// Filesystem directory boundaries and containment are one structural class.
	for _, n := range deriveNodes {
		if n.Kind == "directory" || n.Kind == "filesystem.directory" {
			coord := n.Coordinate
			if coord == "" {
				coord = n.Name
			}
			x := get(coord, leaf(coord))
			addSignal(x, "filesystem_directory", coord, n.ID)
		}
	}
	// Package and manifest observations are independent structural classes.
	for _, n := range deriveNodes {
		switch n.Kind {
		case "go.package":
			coord := n.Attributes["directory"]
			if coord == "" {
				coord = derivationNodePath(n)
			}
			x := get(coord, leaf(coord))
			addSignal(x, "go_package", n.Attributes["package"], n.ID)
		case "go.module", "npm.workspace", "npm.package":
			coord := n.Attributes["directory"]
			if coord == "" {
				coord = path.Dir(derivationNodePath(n))
			}
			if coord == "" {
				coord = "."
			}
			x := get(coord, leaf(coord))
			addSignal(x, "manifest_boundary", n.Kind+":"+n.Attributes["module"]+n.Attributes["package"]+n.Attributes["name"], n.ID)
		}
	}
	for _, e := range deriveEdges {
		if e.Kind != "workspace.use" {
			continue
		}
		if n, ok := nodes[e.To]; ok {
			coord := n.Attributes["directory"]
			if coord == "" {
				coord = path.Dir(derivationNodePath(n))
			}
			if x := seeds[cleanCoordinate(coord)]; x != nil {
				addSignal(x, "workspace_use_support", e.Kind, n.ID)
			}
		}
	}

	// Attach physical descendants to their nearest directory candidate; a lone
	// file never creates a logical group.
	dirCoords := map[string]string{}
	for _, n := range deriveNodes {
		if n.Kind == "directory" || n.Kind == "filesystem.directory" {
			c := cleanCoordinate(n.Coordinate)
			if c == "" {
				c = "."
			}
			dirCoords[n.ID] = c
		}
	}
	for _, n := range deriveNodes {
		if n.Kind == "directory" || n.Kind == "filesystem.directory" {
			continue
		}
		id := n.ID
		seen := map[string]bool{}
		for id != "" && !seen[id] {
			seen[id] = true
			if coord, ok := dirCoords[id]; ok {
				if x := seeds[coord]; x != nil {
					x.members[n.ID] = true
				}
				break
			}
			id = parent[id]
		}
	}

	// Detect conflicting package identities at a single physical boundary.
	for _, x := range seeds {
		pkgs := map[string]bool{}
		for _, n := range deriveNodes {
			if n.Kind == "go.package" {
				c := cleanCoordinate(n.Attributes["directory"])
				if c == "" {
					c = cleanCoordinate(derivationNodePath(n))
				}
				if c == x.coord {
					pkg := n.Attributes["package"]
					if pkg == "" {
						pkg = n.Name
					}
					pkgs[packageFamily(pkg)] = true
				}
			}
		}
		if len(pkgs) > 1 {
			x.reasons["package_identity_conflict"] = true
		}
		for name := range pkgs {
			if name != "" && name != "main" && name != packageFamily(x.label) {
				x.reasons["package_directory_disagreement"] = true
			}
		}
		if genericName(x.label) {
			x.reasons["generic_name_review"] = true
		}
	}

	coords := make([]string, 0, len(seeds))
	for c, x := range seeds {
		if len(x.members) > 0 || len(x.signals) > 0 {
			if c == "." && onlyFilesystemSignal(x) {
				continue
			}
			if isConfiguredTransportPath(c, s.Config.SourceRoots) && onlyFilesystemSignal(x) {
				continue
			}
			coords = append(coords, c)
		}
	}
	sort.Strings(coords)
	for _, c := range coords {
		x := seeds[c]
		ids := sortedKeys(x.members)
		sigs := make([]Signal, 0, len(x.signals))
		classes := 0
		supporting := false
		for kind, vals := range x.signals {
			if kind == "workspace_use_support" {
				supporting = true
				nodeIDs := []string{}
				refs := []string{}
				for key := range vals {
					parts := strings.SplitN(key, "\x00", 2)
					if len(parts) > 1 {
						nodeIDs = append(nodeIDs, parts[1])
					}
				}
				for _, e := range deriveEdges {
					if e.Kind == "workspace.use" && containsString(nodeIDs, e.To) {
						refs = append(refs, e.EvidenceRefs...)
					}
				}
				sigs = append(sigs, Signal{Kind: kind, Value: "workspace.use", NodeIDs: uniqueSorted(nodeIDs), EvidenceRefs: uniqueSorted(refs)})
				continue
			}
			classes++
			vv := sortedKeys(vals)
			nodeIDs := []string{}
			valueSet := map[string]bool{}
			for _, v := range vv {
				parts := strings.SplitN(v, "\x00", 2)
				valueSet[parts[0]] = true
				if len(parts) > 1 && parts[1] != "" {
					nodeIDs = append(nodeIDs, parts[1])
				}
			}
			values := sortedKeys(valueSet)
			val := strings.Join(values, "|")
			refs := []string{}
			for _, id := range nodeIDs {
				if n, ok := nodes[id]; ok {
					refs = append(refs, n.EvidenceRefs...)
				} else {
					refs = append(refs, edgeRefs[id]...)
				}
			}
			sigs = append(sigs, Signal{Kind: kind, Value: val, NodeIDs: uniqueSorted(nodeIDs), EvidenceRefs: uniqueSorted(refs)})
		}
		sort.Slice(sigs, func(i, j int) bool { return sigs[i].Kind < sigs[j].Kind })
		conf := ConfidenceWeak
		reasons := sortedKeys(x.reasons)
		if x.reasons["package_identity_conflict"] || x.reasons["package_directory_disagreement"] {
			conf = ConfidenceConflicted
		}
		if conf != ConfidenceConflicted {
			if relevantCoverageGap(s, c, x, ignoredPaths) {
				conf = ConfidenceUnresolved
				reasons = append(reasons, "relevant_coverage_gap")
			} else if classes >= 2 {
				conf = ConfidenceStrong
			} else if classes == 1 {
				if supporting {
					conf = ConfidenceSupported
				} else {
					conf = ConfidenceWeak
				}
			} else {
				conf = ConfidenceUnresolved
			}
		}
		if conf == ConfidenceWeak {
			reasons = append(reasons, "single_structural_signal")
			sort.Strings(reasons)
		}
		// Candidate identity follows its subject/model/boundary; current members
		// and evidence may change without renaming the structural hypothesis.
		identity := struct {
			Subject, Model, Coordinate string
		}{s.Subject.ID, DerivationModelVersion, c}
		raw, _ := json.Marshal(identity)
		sum := sha256.Sum256(raw)
		out.Candidates = append(out.Candidates, Candidate{ID: "sc-" + hex.EncodeToString(sum[:12]), Coordinate: c, Label: x.label, PhysicalNodeIDs: ids, Signals: sigs, Confidence: conf, ReviewReasons: uniqueSorted(reasons)})
	}
	// Parent relationships follow the observed directory containment topology.
	dirNodeAt := map[string]string{}
	for id, c := range dirCoords {
		dirNodeAt[c] = id
	}
	for i := range out.Candidates {
		cur := dirNodeAt[out.Candidates[i].Coordinate]
		seen := map[string]bool{}
		for cur != "" && parent[cur] != "" && !seen[parent[cur]] {
			p := parent[cur]
			seen[p] = true
			cur = p
			if pc, ok := dirCoords[p]; ok {
				for j := range out.Candidates {
					if out.Candidates[j].Coordinate == pc {
						out.Candidates[i].ParentID = out.Candidates[j].ID
						cur = ""
						break
					}
				}
			}
		}
	}
	covered := map[string]bool{}
	for _, c := range out.Candidates {
		for _, id := range c.PhysicalNodeIDs {
			covered[id] = true
		}
	}
	for _, n := range s.Nodes {
		if n.Kind == "file" || strings.HasPrefix(n.Kind, "file.") {
			if !covered[n.ID] {
				out.UnmappedNodeIDs = append(out.UnmappedNodeIDs, n.ID)
			}
		}
	}
	out.UnmappedNodeIDs = uniqueSorted(out.UnmappedNodeIDs)
	// A single repeated technical-layer leaf is suspicious and remains in its
	// observed hierarchy; no synthetic common parent is ever created.
	leaves := map[string]int{}
	for _, c := range out.Candidates {
		if strings.Contains(c.Coordinate, "/") {
			l := leaf(c.Coordinate)
			if technicalLayer(l) {
				leaves[l]++
			}
		}
	}
	for i := range out.Candidates {
		if leaves[out.Candidates[i].Label] > 1 {
			out.Candidates[i].ReviewReasons = uniqueSorted(append(out.Candidates[i].ReviewReasons, "repeated_layer_pattern"))
		}
		c := &out.Candidates[i]
		c.ReviewRequired = requiresReview(*c)
	}
	out.Digest = candidateDigest(out)
	return out, nil
}

func validateContainment(nodes []Node, edges []Edge) error {
	byID := make(map[string]Node, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	parent := map[string]string{}
	for _, e := range edges {
		if e.Kind != "contains" && e.Kind != "directory.contains" && e.Kind != "filesystem.contains" {
			continue
		}
		from, fromOK := byID[e.From]
		to, toOK := byID[e.To]
		if !fromOK || !toOK {
			return errors.New("containment edge has dangling endpoint")
		}
		if from.Kind != "directory" && from.Kind != "filesystem.directory" {
			return errors.New("containment parent is not a directory")
		}
		if to.Kind != "directory" && to.Kind != "filesystem.directory" && to.Kind != "file" {
			return errors.New("containment child is not a filesystem entry")
		}
		if path.Dir(cleanCoordinate(to.Coordinate)) != cleanCoordinate(from.Coordinate) {
			return errors.New("containment edge disagrees with physical coordinates")
		}
		if old := parent[to.ID]; old != "" && old != e.From {
			return errors.New("filesystem entry has multiple containment parents")
		}
		parent[to.ID] = e.From
	}
	for start := range parent {
		seen := map[string]bool{}
		for cur := start; cur != ""; cur = parent[cur] {
			if seen[cur] {
				return errors.New("containment graph contains a cycle")
			}
			seen[cur] = true
		}
	}
	return nil
}

func ignoredPathSet(nodes []Node) map[string]bool {
	ignored := map[string]bool{}
	for _, n := range nodes {
		if n.Kind == "git.ignored" {
			ignored[cleanCoordinate(n.Coordinate)] = true
		}
	}
	return ignored
}

func isIgnoredPath(coord string, ignored map[string]bool) bool {
	coord = cleanCoordinate(coord)
	for root := range ignored {
		if root == "." || coord == root || strings.HasPrefix(coord, root+"/") {
			return true
		}
	}
	return false
}

func eligibleEvidenceRefs(refs []string, evidence map[string]Evidence, ignored map[string]bool) []string {
	eligible := make([]string, 0, len(refs))
	for _, ref := range refs {
		ev, ok := evidence[ref]
		if ok && !isIgnoredPath(ev.Path, ignored) {
			eligible = append(eligible, ref)
		}
	}
	return eligible
}

func derivationNodePath(n Node) string {
	switch n.Kind {
	case "go.package":
		if d := n.Attributes["directory"]; d != "" {
			return d
		}
		pkg := n.Attributes["package"]
		if pkg == "" {
			pkg = n.Name
		}
		return strings.TrimSuffix(n.Coordinate, "#package/"+pkg)
	case "go.module":
		return strings.TrimSuffix(n.Coordinate, "#module")
	case "go.workspace", "npm.workspace":
		return strings.TrimSuffix(n.Coordinate, "#workspace")
	case "npm.package":
		return strings.TrimSuffix(n.Coordinate, "#package")
	default:
		return n.Coordinate
	}
}

func eligibleDerivationNode(n Node, evidence map[string]Evidence, ignored map[string]bool) (bool, Node) {
	if n.Kind == "git.ignored" {
		return false, n
	}
	if isIgnoredPath(derivationNodePath(n), ignored) || isIgnoredPath(n.Attributes["directory"], ignored) {
		return false, n
	}
	if n.Kind == "directory" || n.Kind == "filesystem.directory" || n.Kind == "file" || strings.HasPrefix(n.Kind, "file.") {
		return true, n
	}
	switch n.Kind {
	case "go.package":
		if isIgnoredPath(n.Attributes["directory"], ignored) {
			return false, n
		}
		refs := eligibleEvidenceRefs(n.EvidenceRefs, evidence, ignored)
		if len(n.EvidenceRefs) > 0 {
			if len(refs) == 0 {
				return false, n
			}
			n.EvidenceRefs = refs
		}
	case "go.module", "go.workspace", "npm.package", "npm.workspace":
		source := derivationNodePath(n)
		if isIgnoredPath(source, ignored) || isIgnoredPath(n.Attributes["directory"], ignored) {
			return false, n
		}
		refs := eligibleEvidenceRefs(n.EvidenceRefs, evidence, ignored)
		if len(n.EvidenceRefs) > 0 {
			if len(refs) == 0 {
				return false, n
			}
			n.EvidenceRefs = refs
		}
	}
	return true, n
}

func cleanCoordinate(s string) string {
	if validCoordinate(s) {
		return s
	}
	s = strings.ReplaceAll(s, "\\", "/")
	s = path.Clean(s)
	if s == "" {
		return "."
	}
	return strings.TrimPrefix(s, "./")
}
func leaf(s string) string {
	s = cleanCoordinate(s)
	if s == "." {
		return "."
	}
	return path.Base(s)
}
func genericName(s string) bool {
	switch strings.ToLower(s) {
	case "common", "shared", "util", "utils", "core", "base", "misc", "legacy", "helper", "helpers", "support":
		return true
	}
	return false
}
func packageFamily(name string) string { return strings.TrimSuffix(name, "_test") }
func technicalLayer(s string) bool {
	switch strings.ToLower(s) {
	case "service", "controller", "repository", "model", "domain", "handler", "api", "ui", "view", "component", "adapter", "client", "store":
		return true
	}
	return false
}
func sortedKeys(m map[string]bool) []string {
	a := make([]string, 0, len(m))
	for k := range m {
		a = append(a, k)
	}
	sort.Strings(a)
	return a
}
func uniqueSorted(a []string) []string {
	m := map[string]bool{}
	for _, s := range a {
		if s != "" {
			m[s] = true
		}
	}
	return sortedKeys(m)
}
func candidateDigest(s DerivedCandidateSet) string {
	// This digest covers the logical candidate tree only. Physical membership,
	// signal provenance, and snapshot-level metadata remain outside this domain.
	type dCandidate struct {
		ID, Coordinate, Label, ParentID string
		Confidence                      Confidence
		ReviewRequired                  bool
		ReviewReasons                   []string
	}
	ds := make([]dCandidate, 0, len(s.Candidates))
	for _, c := range s.Candidates {
		ds = append(ds, dCandidate{c.ID, c.Coordinate, c.Label, c.ParentID, c.Confidence, c.ReviewRequired, uniqueSorted(c.ReviewReasons)})
	}
	v := struct {
		SubjectID, DerivationModel string
		Candidates                 []dCandidate
	}{s.Subject.ID, s.Producer.DerivationModelVersion, ds}
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func requiresReview(c Candidate) bool {
	if c.Confidence == ConfidenceConflicted || c.Confidence == ConfidenceUnresolved {
		return true
	}
	for _, reason := range c.ReviewReasons {
		if reason != "" && reason != "single_structural_signal" {
			return true
		}
	}
	return false
}

func onlyFilesystemSignal(x *derivationSeed) bool {
	return len(x.signals) == 1 && x.signals["filesystem_directory"] != nil
}
func isConfiguredTransportPath(coord string, roots []string) bool {
	for _, root := range roots {
		root = cleanCoordinate(root)
		if root != "." && (root == coord || strings.HasPrefix(root, coord+"/")) {
			return true
		}
	}
	return false
}
func containsString(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}
func relevantCoverageGap(s PhysicalSnapshot, coord string, x *derivationSeed, ignored map[string]bool) bool {
	localized := map[string]bool{}
	for _, u := range s.Unknowns {
		localized[strings.ToLower(u.Kind)] = true
		if isIgnoredPath(u.Coordinate, ignored) {
			continue
		}
		c := cleanCoordinate(u.Coordinate)
		if c == coord || strings.HasPrefix(c, coord+"/") || strings.HasPrefix(coord, c+"/") {
			return true
		}
	}
	for _, c := range s.Coverage {
		if c.State == "UNAVAILABLE" || c.State == "FAILED" || c.State == "PARTIAL" {
			cap := strings.ToLower(c.Capability)
			relevant := (strings.Contains(cap, "filesystem") && x.signals["filesystem_directory"] != nil) || (strings.Contains(cap, "go") && x.signals["go_package"] != nil) || (strings.Contains(cap, "manifest") && x.signals["manifest_boundary"] != nil)
			if relevant && (c.State != "PARTIAL" || !hasLocalizedGap(localized, cap)) {
				return true
			}
		}
	}
	return false
}
func hasLocalizedGap(kinds map[string]bool, cap string) bool {
	for k := range kinds {
		if strings.Contains(cap, "filesystem") && (k == "filesystem" || k == "file" || k == "symlink" || k == "nonregular" || k == "nested_repository") {
			return true
		}
		if strings.Contains(cap, "go") && k == "go_source" {
			return true
		}
		if strings.Contains(cap, "manifest") && (strings.Contains(k, "manifest") || strings.Contains(k, "workspace") || k == "go_module") {
			return true
		}
	}
	return false
}
