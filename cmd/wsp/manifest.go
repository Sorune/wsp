package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Sorune/wsp/internal/config"
	"github.com/Sorune/wsp/internal/inspect"
	"github.com/Sorune/wsp/internal/scanner"
)

const (
	defaultManifestOutputBudget = 16384
	maxProposalEntities         = 128
	maxProposalRelations        = 256
	maxProposalUnknowns         = 64
	maxProposalCandidates       = 128
)

type proposalEntity struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Status   string   `json:"status"`
	Evidence []string `json:"evidence_refs,omitempty"`
}

type proposalRelation struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	From       string   `json:"from"`
	FromPath   string   `json:"from_path"`
	To         string   `json:"to"`
	ToPath     string   `json:"to_path"`
	Status     string   `json:"status"`
	Resolution string   `json:"resolution,omitempty"`
	Evidence   []string `json:"evidence_refs,omitempty"`
}

type proposalCandidate struct {
	ID             string   `json:"id"`
	Path           string   `json:"path"`
	Label          string   `json:"label"`
	ParentID       string   `json:"parent_id,omitempty"`
	Confidence     string   `json:"confidence"`
	ReviewRequired bool     `json:"review_required"`
	ReviewReasons  []string `json:"review_reasons,omitempty"`
}

type proposalUnknown struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type manifestDraftReport struct {
	Status               string              `json:"status"`
	Mutation             string              `json:"mutation"`
	Provenance           string              `json:"provenance"`
	RepositoryID         string              `json:"repository_id"`
	Revision             string              `json:"revision"`
	WorkingTree          string              `json:"working_tree"`
	DraftBytes           int                 `json:"draft_bytes"`
	WorkspaceID          string              `json:"workspace_id"`
	RepositoryCount      int                 `json:"repository_count"`
	ProjectCount         int                 `json:"project_count"`
	RelationCount        int                 `json:"relation_count"`
	Scanner              string              `json:"scanner"`
	Coverage             []scanner.Coverage  `json:"coverage"`
	Observed             map[string]int      `json:"observed_counts"`
	Unknowns             map[string]int      `json:"unknown_counts"`
	Candidates           int                 `json:"candidate_count"`
	ProposalStatus       string              `json:"proposal_status"`
	ProposedEntities     []proposalEntity    `json:"proposed_entities"`
	ProposedRelations    []proposalRelation  `json:"proposed_relations"`
	StructuralCandidates []proposalCandidate `json:"structural_candidates"`
	ProposedUnknowns     []proposalUnknown   `json:"proposed_unknowns"`
	OmittedEntities      int                 `json:"omitted_entities"`
	OmittedRelations     int                 `json:"omitted_relations"`
	OmittedCandidates    int                 `json:"omitted_candidates"`
	OmittedUnknowns      int                 `json:"omitted_unknowns"`
	TotalEntities        int                 `json:"total_entities"`
	ReturnedEntities     int                 `json:"returned_entities"`
	TotalRelations       int                 `json:"total_relations"`
	ReturnedRelations    int                 `json:"returned_relations"`
	TotalCandidates      int                 `json:"total_candidates"`
	ReturnedCandidates   int                 `json:"returned_candidates"`
	TotalUnknowns        int                 `json:"total_unknowns"`
	ReturnedUnknowns     int                 `json:"returned_unknowns"`
	Draft                string              `json:"draft,omitempty"`
}

func manifestCommand(args []string) error {
	if len(args) == 0 || hasHelpArg(args) {
		manifestUsage()
		return nil
	}
	switch args[0] {
	case "draft":
		return manifestDraftCommand(args[1:])
	case "validate":
		return manifestValidateCommand(args[1:])
	default:
		return fail("USAGE", "manifest requires draft|validate", 2)
	}
}

func manifestUsage() {
	fmt.Print(`Usage: wsp manifest draft [repository] [--json] [--max-output-bytes N]
       wsp manifest validate --input FILE [--json]

draft prints a deterministic, preview-only DRAFT; it never creates .wsp,
rewrites a manifest, or accepts Project semantics. Scanner observations that
cannot be represented safely remain proposed evidence/UNKNOWN in bounded JSON.
The JSON companion lists concrete paths and PROPOSED graph edges; it does not
install them as Lens-visible manifest relations.
The default output budget is 16384 bytes; over-budget output fails whole.
validate reads and validates a draft file without installing it or changing the
target. Human review and the existing authorized manifest workflow remain the
acceptance gate.
`)
}

func manifestDraftCommand(args []string) error {
	root, jsonOut, budget, err := parseManifestDraftArgs(args)
	if err != nil {
		return fail("USAGE", err.Error(), 2)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return fail("TARGET_UNAVAILABLE", err.Error(), 3)
	}
	if resolved, e := filepath.EvalSymlinks(abs); e == nil {
		abs = resolved
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return fail("TARGET_NOT_FOUND", "repository root is unavailable or not a directory", 3)
	}
	base := filepath.Base(filepath.Clean(abs))
	repoID, repoName, revision, workingTree := "repository:"+base, base, "UNKNOWN", "UNKNOWN"
	if doc, inspectErr := inspect.Repository(abs); inspectErr == nil {
		for _, entity := range doc.Entities {
			if entity.Kind == "REPOSITORY" {
				if entity.Name != "UNKNOWN" && entity.ID != "repository:unknown" {
					repoID, repoName = entity.ID, entity.Name
				}
				revision = entity.Attributes["revision"]
				workingTree = entity.Attributes["working_tree"]
				break
			}
		}
	}
	workspaceID := "workspace:" + strings.TrimPrefix(repoID, "repository:")
	subjectID := repoID
	if strings.TrimSpace(subjectID) == "repository:" {
		subjectID = "repository:" + base
	}
	batch, err := scanner.Observe(scanner.Request{Path: abs, SubjectID: subjectID})
	if err != nil {
		return fail("INVALID_REQUEST", "scanner observation failed: "+err.Error(), 2)
	}
	snapshot, err := scanner.BuildSnapshot(batch, scanner.Config{})
	if err != nil {
		return fail("INTERNAL_FAILURE", "scanner snapshot validation failed: "+err.Error(), 1)
	}
	candidates, err := scanner.Derive(snapshot)
	if err != nil {
		return fail("INTERNAL_FAILURE", "scanner candidate derivation failed: "+err.Error(), 1)
	}
	manifest := config.Manifest{
		SchemaVersion: 1,
		WorkspaceID:   workspaceID,
		Repositories:  []config.Item{{ID: repoID, Name: repoName, Path: "."}},
		// The schema makes every relation Lens-visible. Scanner edges are
		// evidence only, so this proposal deliberately declares no relations.
	}
	body, err := config.Render(manifest)
	if err != nil {
		return failFor(err)
	}
	coverage := append([]scanner.Coverage(nil), snapshot.Coverage...)
	sort.Slice(coverage, func(i, j int) bool { return coverage[i].Capability < coverage[j].Capability })
	observed := map[string]int{}
	for _, n := range snapshot.Nodes {
		observed[n.Kind]++
	}
	unknowns := map[string]int{}
	for _, u := range snapshot.Unknowns {
		unknowns[u.Kind]++
	}
	proposedEntities, omittedEntities, selectedNodes := proposedScannerEntities(snapshot.Nodes)
	proposedRelations, omittedRelations := proposedScannerRelations(snapshot.Edges, selectedNodes)
	structuralCandidates, omittedCandidates := proposedScannerCandidates(candidates.Candidates)
	proposedUnknowns, omittedUnknowns := proposedScannerUnknowns(snapshot.Unknowns)
	draft := "# STATUS: DRAFT (preview only; Human review required)\n# PROVENANCE: scanner-proposed; physical evidence does not accept Project semantics\n# REVIEW: no Project ownership, maturity, deployment, session, or logical relations are inferred\n" +
		"# COVERAGE: " + coverageSummary(coverage) + "\n" +
		"# EVIDENCE: observed_counts=" + countSummary(observed) + "; unknown_counts=" + countSummary(unknowns) + "; candidates=" + strconv.Itoa(len(candidates.Candidates)) + "\n" + string(body)
	report := manifestDraftReport{
		Status: "DRAFT", Mutation: "NONE", Provenance: "scanner-proposed",
		RepositoryID: repoID, Revision: revision, WorkingTree: workingTree,
		DraftBytes: len(draft), WorkspaceID: workspaceID, RepositoryCount: 1,
		ProjectCount: 0, RelationCount: 0, Scanner: snapshot.Producer.ID + "/" + snapshot.Producer.Version,
		Coverage: coverage, Observed: observed, Unknowns: unknowns, Candidates: len(candidates.Candidates),
		ProposedEntities: proposedEntities, ProposedRelations: proposedRelations,
		StructuralCandidates: structuralCandidates, ProposedUnknowns: proposedUnknowns,
		OmittedEntities: omittedEntities, OmittedRelations: omittedRelations,
		OmittedCandidates: omittedCandidates, OmittedUnknowns: omittedUnknowns,
		TotalEntities: len(proposedEntities) + omittedEntities, ReturnedEntities: len(proposedEntities),
		TotalRelations: len(proposedRelations) + omittedRelations, ReturnedRelations: len(proposedRelations),
		TotalCandidates: len(structuralCandidates) + omittedCandidates, ReturnedCandidates: len(structuralCandidates),
		TotalUnknowns: len(proposedUnknowns) + omittedUnknowns, ReturnedUnknowns: len(proposedUnknowns),
	}
	report.ProposalStatus = proposalCoverageStatus(report)
	var output []byte
	if jsonOut {
		output, err = marshalBoundedDraftReport(&report, budget)
		if err != nil && strings.Contains(err.Error(), "OUTPUT_BUDGET_EXCEEDED") {
			return fail("OUTPUT_BUDGET_EXCEEDED", strings.TrimPrefix(err.Error(), "OUTPUT_BUDGET_EXCEEDED: "), 5)
		}
	} else {
		output = []byte(draft)
	}
	if err != nil {
		return fail("INTERNAL_FAILURE", "serialize draft report: "+err.Error(), 1)
	}
	if len(output) > budget {
		return fail("OUTPUT_BUDGET_EXCEEDED", fmt.Sprintf("draft output is %d bytes; budget is %d; no output emitted", len(output), budget), 5)
	}
	_, err = os.Stdout.Write(output)
	return err
}

func proposalCoverageStatus(report manifestDraftReport) string {
	if report.OmittedEntities+report.OmittedRelations+report.OmittedCandidates+report.OmittedUnknowns > 0 || len(report.ProposedUnknowns) > 0 {
		return "PARTIAL"
	}
	for _, item := range report.Coverage {
		if item.State != "COMPLETE" && item.State != "NOT_APPLICABLE" {
			return "PARTIAL"
		}
	}
	return "COMPLETE"
}

func marshalBoundedDraftReport(report *manifestDraftReport, budget int) ([]byte, error) {
	for {
		report.ReturnedEntities = len(report.ProposedEntities)
		report.ReturnedRelations = len(report.ProposedRelations)
		report.ReturnedCandidates = len(report.StructuralCandidates)
		report.ReturnedUnknowns = len(report.ProposedUnknowns)
		report.OmittedEntities = report.TotalEntities - report.ReturnedEntities
		report.OmittedRelations = report.TotalRelations - report.ReturnedRelations
		report.OmittedCandidates = report.TotalCandidates - report.ReturnedCandidates
		report.OmittedUnknowns = report.TotalUnknowns - report.ReturnedUnknowns
		report.ProposalStatus = proposalCoverageStatus(*report)
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return nil, err
		}
		data = append(data, '\n')
		if len(data) <= budget {
			return data, nil
		}
		// Deterministic whole-record trimming keeps concrete proposal graph
		// records ahead of low-priority directory/candidate detail.
		switch {
		case len(report.ProposedUnknowns) > 0:
			report.ProposedUnknowns = report.ProposedUnknowns[:len(report.ProposedUnknowns)-1]
		case len(report.ProposedEntities) > 8:
			report.ProposedEntities = report.ProposedEntities[:len(report.ProposedEntities)-1]
		case len(report.StructuralCandidates) > 8:
			report.StructuralCandidates = report.StructuralCandidates[:len(report.StructuralCandidates)-1]
		case len(report.ProposedRelations) > 8:
			report.ProposedRelations = report.ProposedRelations[:len(report.ProposedRelations)-1]
		case len(report.StructuralCandidates) > 1:
			report.StructuralCandidates = report.StructuralCandidates[:len(report.StructuralCandidates)-1]
		case len(report.ProposedEntities) > 1:
			report.ProposedEntities = report.ProposedEntities[:len(report.ProposedEntities)-1]
		case len(report.ProposedRelations) > 1:
			report.ProposedRelations = report.ProposedRelations[:len(report.ProposedRelations)-1]
		case len(report.StructuralCandidates) > 0:
			report.StructuralCandidates = report.StructuralCandidates[:len(report.StructuralCandidates)-1]
		case len(report.ProposedEntities) > 0:
			report.ProposedEntities = report.ProposedEntities[:len(report.ProposedEntities)-1]
		case len(report.ProposedRelations) > 0:
			report.ProposedRelations = report.ProposedRelations[:len(report.ProposedRelations)-1]
		default:
			return nil, fmt.Errorf("OUTPUT_BUDGET_EXCEEDED: budget cannot fit the minimum complete JSON review record")
		}
	}
}

func coverageSummary(items []scanner.Coverage) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, item.Capability+"="+item.State)
	}
	return strings.Join(parts, ",")
}

func countSummary(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+strconv.Itoa(counts[key]))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ",")
}

func proposedScannerEntities(nodes []scanner.Node) ([]proposalEntity, int, map[string]scanner.Node) {
	priority := map[string]int{"go.module": 0, "npm.package": 0, "go.package": 0, "pnpm.workspace": 1, "go.workspace": 1, "directory": 2}
	selected := make([]scanner.Node, 0, len(nodes))
	for _, n := range nodes {
		if _, ok := priority[n.Kind]; ok {
			selected = append(selected, n)
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		pi, pj := priority[selected[i].Kind], priority[selected[j].Kind]
		if pi != pj {
			return pi < pj
		}
		if selected[i].Coordinate != selected[j].Coordinate {
			return selected[i].Coordinate < selected[j].Coordinate
		}
		return selected[i].ID < selected[j].ID
	})
	total := len(selected)
	if len(selected) > maxProposalEntities {
		selected = selected[:maxProposalEntities]
	}
	out := make([]proposalEntity, 0, len(selected))
	byID := make(map[string]scanner.Node, len(selected))
	for _, n := range selected {
		path := n.Coordinate
		if n.Attributes["directory"] != "" {
			path = n.Attributes["directory"]
		}
		refs := append([]string(nil), n.EvidenceRefs...)
		sort.Strings(refs)
		out = append(out, proposalEntity{ID: n.ID, Kind: n.Kind, Name: n.Name, Path: path, Status: "PROPOSED", Evidence: refs})
		byID[n.ID] = n
	}
	return out, total - len(out), byID
}

func proposedScannerRelations(edges []scanner.Edge, nodes map[string]scanner.Node) ([]proposalRelation, int) {
	out := make([]proposalRelation, 0, len(edges))
	for _, edge := range edges {
		from, fromOK := nodes[edge.From]
		to, toOK := nodes[edge.To]
		if !fromOK || !toOK || (edge.Kind != "filesystem.contains" && edge.Kind != "workspace.use") {
			continue
		}
		fromPath, toPath := from.Coordinate, to.Coordinate
		if from.Attributes["directory"] != "" {
			fromPath = from.Attributes["directory"]
		}
		if to.Attributes["directory"] != "" {
			toPath = to.Attributes["directory"]
		}
		refs := append([]string(nil), edge.EvidenceRefs...)
		sort.Strings(refs)
		out = append(out, proposalRelation{ID: edge.ID, Type: edge.Kind, From: edge.From, FromPath: fromPath, To: edge.To, ToPath: toPath, Status: "PROPOSED", Resolution: edge.Resolution, Evidence: refs})
	}
	sort.Slice(out, func(i, j int) bool {
		priority := func(kind string) int {
			if kind == "workspace.use" {
				return 0
			}
			return 1
		}
		pi, pj := priority(out[i].Type), priority(out[j].Type)
		if pi != pj {
			return pi < pj
		}
		if out[i].FromPath != out[j].FromPath {
			return out[i].FromPath < out[j].FromPath
		}
		if out[i].ToPath != out[j].ToPath {
			return out[i].ToPath < out[j].ToPath
		}
		return out[i].ID < out[j].ID
	})
	total := len(out)
	if len(out) > maxProposalRelations {
		out = out[:maxProposalRelations]
	}
	return out, total - len(out)
}

func proposedScannerCandidates(candidates []scanner.Candidate) ([]proposalCandidate, int) {
	items := append([]scanner.Candidate(nil), candidates...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	total := len(items)
	if len(items) > maxProposalCandidates {
		items = items[:maxProposalCandidates]
	}
	out := make([]proposalCandidate, 0, len(items))
	for _, item := range items {
		out = append(out, proposalCandidate{ID: item.ID, Path: item.Coordinate, Label: item.Label, ParentID: item.ParentID, Confidence: string(item.Confidence), ReviewRequired: item.ReviewRequired, ReviewReasons: append([]string(nil), item.ReviewReasons...)})
	}
	return out, total - len(out)
}

func proposedScannerUnknowns(unknowns []scanner.Unknown) ([]proposalUnknown, int) {
	items := append([]scanner.Unknown(nil), unknowns...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Coordinate != items[j].Coordinate {
			return items[i].Coordinate < items[j].Coordinate
		}
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Reason < items[j].Reason
	})
	total := len(items)
	if len(items) > maxProposalUnknowns {
		items = items[:maxProposalUnknowns]
	}
	out := make([]proposalUnknown, 0, len(items))
	for _, item := range items {
		out = append(out, proposalUnknown{Kind: item.Kind, Path: item.Coordinate, Reason: item.Reason})
	}
	return out, total - len(out)
}

func parseManifestDraftArgs(args []string) (string, bool, int, error) {
	root, jsonOut, budget := ".", false, defaultManifestOutputBudget
	seenRoot, seenJSON, seenBudget := false, false, false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			if seenJSON {
				return "", false, 0, fmt.Errorf("repeated --json")
			}
			seenJSON = true
			jsonOut = true
		case "--max-output-bytes":
			if seenBudget || i+1 >= len(args) {
				return "", false, 0, fmt.Errorf("--max-output-bytes requires one value")
			}
			seenBudget = true
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1024 || n > 1<<20 {
				return "", false, 0, fmt.Errorf("--max-output-bytes must be between 1024 and 1048576")
			}
			budget = n
		default:
			if strings.HasPrefix(args[i], "-") {
				return "", false, 0, fmt.Errorf("unsupported option %s", args[i])
			}
			if seenRoot {
				return "", false, 0, fmt.Errorf("one repository target allowed")
			}
			root, seenRoot = args[i], true
		}
	}
	return targetPath(root), jsonOut, budget, nil
}

func manifestValidateCommand(args []string) error {
	input, jsonOut := "", false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--input":
			if input != "" || i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return fail("USAGE", "manifest validate --input FILE [--json]", 2)
			}
			i++
			input = args[i]
		case "--json":
			if jsonOut {
				return fail("USAGE", "repeated --json", 2)
			}
			jsonOut = true
		default:
			return fail("USAGE", "manifest validate --input FILE [--json]", 2)
		}
	}
	if input == "" {
		return fail("USAGE", "manifest validate requires --input FILE", 2)
	}
	input = targetPath(input)
	m, err := config.LoadFile(input)
	if err != nil {
		return failFor(err)
	}
	if jsonOut {
		out, err := json.MarshalIndent(map[string]any{"status": "VALID", "mutation": "NONE", "path": input, "workspace_id": m.WorkspaceID, "repositories": len(m.Repositories), "projects": len(m.Projects), "relations": len(m.Relations)}, "", "  ")
		if err != nil {
			return fail("INTERNAL_FAILURE", err.Error(), 1)
		}
		fmt.Println(string(out))
	} else {
		fmt.Printf("STATUS: VALID\nMUTATION: NONE\nWORKSPACE: %s\nREPOSITORIES: %d\nPROJECTS: %d\nRELATIONS: %d\n", m.WorkspaceID, len(m.Repositories), len(m.Projects), len(m.Relations))
	}
	return nil
}
